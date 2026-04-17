"""Chat endpoint — JWT-protected, streaming SSE response via LiteLLM proxy.

Flow:
  1. Validate JWT → extract user_id
  2. Verify session exists and belongs to this user
  3. Load last 10 exchanges (20 messages) for context window
  4. Fetch student's Felder-Silverman learning-style dials from local DB (non-fatal)
  5. Build messages:
       - If session.course_id is set: agentic RAG (retrieve chunks → enriched prompt)
       - Otherwise: base Professor Nova prompt (no course materials)
       - Append style guidance addendum to the system prompt
  6. Stream from AI Gateway (LiteLLM) via OpenAI-compatible SDK
  7. Persist student message (pre-stream) + completed tutor reply (post-stream)
  8. Update learning-style dials from student message signals (post-stream, non-fatal)
  9. Check for due SRS reviews and emit review_reminder event when concepts are due
 10. Emit SSE: delta chunks → sources → done  (error event on failure)

SSE event format:
  data: {"type": "delta",           "content": "<token>",  "message_id": "<uuid>"}
  data: {"type": "sources",         "content": "",          "message_id": "<uuid>", "sources": [...]}
  data: {"type": "review_reminder", "content": "<prompt>",  "message_id": "<uuid>"}
  data: {"type": "done",            "content": "",          "message_id": "<uuid>"}
  data: {"type": "error",           "content": "<msg>",     "message_id": ""}

sources event is emitted only when the session has a course_id and chunks were
retrieved. Each source object contains: chunk_id, chapter, section, page,
content_type, score.
"""
import json
import logging
import re
import time
import uuid
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException
from fastapi.responses import StreamingResponse
from openai import APIConnectionError, APIStatusError, APITimeoutError
from opentelemetry import trace
from sqlalchemy.ext.asyncio import AsyncSession

from .auth import JWTClaims, require_auth
from .config import settings
from .context import build_pruned_history, log_token_usage
from .database import get_db
from .gateway import get_gateway_client
from .history import append_message, get_session
from .knowledge_model import get_dials, get_due_review_prompt, update_learning_profile_dials
from .models import MessageRequest
from .rag import reformulate_query
from .rag_agent import PROFESSOR_NOVA_SYSTEM_PROMPT, build_rag_context
from .search_client import fetch_diagram_chunks
from .spaced_repetition import get_session_start_reminder
from .style_detector import build_style_prompt_section, detect_signals, update_dials

# Keywords that suggest the student is asking about something visual / structural.
# When matched, we fetch a diagram from the CLIP index to embed in the response.
_VISUAL_QUERY_PATTERNS = re.compile(
    r"\b(diagram|structure|molecule|formula|look like|draw|figure|"
    r"benzene|ring|bond|orbital|cell|anatomy|circuit|graph|chart)\b",
    re.IGNORECASE,
)

logger = logging.getLogger(__name__)
_tracer = trace.get_tracer("tutoring-service.chat")

router = APIRouter(prefix="/sessions", tags=["chat"])

FALLBACK_MESSAGE = (
    "I'm having a moment of technical difficulty — my connection to the knowledge "
    "network seems unstable. Could you try sending your question again? I'll be right "
    "here when you do. 🔧"
)


def _history_to_messages(interactions) -> list[dict]:
    """Convert ORM Interaction rows to OpenAI-style message dicts.

    Args:
        interactions: Ordered list of Interaction ORM objects.

    Returns:
        List of {"role": ..., "content": ...} dicts ready for the chat API.
    """
    return [
        {"role": "user" if i.role == "student" else "assistant", "content": i.content}
        for i in interactions
    ]


def _sse(
    event_type: str,
    content: str = "",
    message_id: str = "",
    sources=None,
    diagram=None,
) -> str:
    """Serialise a single SSE frame.

    Args:
        event_type: One of ``"delta"``, ``"sources"``, ``"done"``, ``"error"``,
            ``"diagram"``.
        content: Token text for delta events; empty string for others.
        message_id: UUID string of the tutor turn being streamed.
        sources: List of source dicts; included only on ``"sources"`` events.
        diagram: Single diagram dict; included only on ``"diagram"`` events.

    Returns:
        SSE-formatted string ready to ``yield`` from the stream generator.
    """
    payload: dict = {"type": event_type, "content": content, "message_id": message_id}
    if sources is not None:
        payload["sources"] = sources
    if diagram is not None:
        payload["diagram"] = diagram
    return f"data: {json.dumps(payload)}\n\n"


async def _chat_stream_generator(  # noqa: PLR0913
    db: AsyncSession,
    session_id: UUID,
    user_id: UUID,
    has_rag: bool,
    messages: list[dict],
    client,
    tutor_msg_id: str,
    sources_payload: list[dict],
    diagram_results: list,
    current_dials: dict,
    student_message: str,
):
    """Async generator that streams SSE events for one tutor turn.

    Streams delta tokens from the LiteLLM gateway, then emits sources, diagram,
    and review_reminder events before a final done frame.  Handles AI Gateway
    errors gracefully by falling back to a canned message.

    Args:
        db: Async SQLAlchemy session (used for post-stream persistence).
        session_id: UUID of the tutoring session.
        user_id: UUID of the authenticated student.
        has_rag: Whether the session has a course_id (used for tracing).
        messages: OpenAI-format message list to send to the gateway.
        client: AsyncOpenAI client pointed at the LiteLLM proxy.
        tutor_msg_id: UUID string for this tutor turn (tagged on all SSE frames).
        sources_payload: Pre-built list of source dicts to emit after streaming.
        diagram_results: Pre-fetched diagram chunks to emit as diagram events.
        current_dials: Student's Felder-Silverman dial values for signal updating.
        student_message: Raw student text (used for dial-signal detection).

    Yields:
        SSE-formatted strings ready to send over the event stream.
    """
    full_response: list[str] = []
    start_ms = int(time.time() * 1000)

    with _tracer.start_as_current_span("llm.generate") as span:
        span.set_attribute("model", settings.tutor_primary_model)
        span.set_attribute("has_rag", has_rag)
        span.set_attribute("message_count", len(messages))

    try:
        stream = await client.chat.completions.create(
            model=settings.tutor_primary_model,
            messages=messages,
            stream=True,
            max_tokens=4096,
        )

        async for chunk in stream:
            delta = chunk.choices[0].delta
            if delta.content:
                full_response.append(delta.content)
                yield _sse("delta", content=delta.content, message_id=tutor_msg_id)

        complete_text = "".join(full_response)
        elapsed_ms = int(time.time() * 1000) - start_ms

        await append_message(
            db, session_id, user_id, role="tutor",
            content=complete_text, response_time_ms=elapsed_ms,
        )

        if sources_payload:
            yield _sse("sources", message_id=tutor_msg_id, sources=sources_payload)

        for diagram in diagram_results:
            yield _sse(
                "diagram",
                message_id=tutor_msg_id,
                diagram={
                    "diagram_id": diagram.diagram_id,
                    "gcs_path": diagram.gcs_path,
                    "caption": diagram.caption,
                    "figure_type": diagram.figure_type,
                    "score": round(diagram.score, 4),
                },
            )

        # Step 4: Update dials from this message's signals (non-fatal)
        signals = detect_signals(student_message)
        if signals:
            updated_dials = update_dials(current_dials, signals)
            try:
                await update_learning_profile_dials(db, user_id, updated_dials)
                await db.commit()
            except Exception:  # noqa: BLE001
                logger.warning("Failed to persist learning-style dials (user omitted)")

        # Step 5: Proactive SRS review reminder (non-fatal)
        try:
            review_prompt = await get_due_review_prompt(db, user_id)
            if review_prompt:
                yield _sse("review_reminder", content=review_prompt, message_id=tutor_msg_id)
        except Exception:  # noqa: BLE001
            logger.warning("Failed to fetch review reminder (user omitted)")

        yield _sse("done", message_id=tutor_msg_id)

    except (APIConnectionError, APITimeoutError) as exc:
        logger.warning("AI Gateway unreachable: %s", exc)
        await append_message(db, session_id, user_id, role="tutor", content=FALLBACK_MESSAGE)
        yield _sse("delta", content=FALLBACK_MESSAGE, message_id=tutor_msg_id)
        yield _sse("done", message_id=tutor_msg_id)

    except APIStatusError as exc:
        logger.error("AI Gateway error %s: %s", exc.status_code, exc.message)
        await append_message(db, session_id, user_id, role="tutor", content=FALLBACK_MESSAGE)
        yield _sse("delta", content=FALLBACK_MESSAGE, message_id=tutor_msg_id)
        yield _sse("done", message_id=tutor_msg_id)

    except Exception as exc:
        logger.exception("Unexpected error in chat stream: %s", exc)
        yield _sse("error", content="An unexpected error occurred. Please try again.")


@router.post("/{session_id}/messages")
async def send_message(
    session_id: UUID,
    body: MessageRequest,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Send a student message and receive a streaming SSE response from Professor Nova."""
    session = await get_session(db, session_id)
    if session is None:
        raise HTTPException(status_code=404, detail="Session not found")
    if session.user_id != user.user_id:
        raise HTTPException(status_code=403, detail="Forbidden")

    student_message = body.content

    client = get_gateway_client()
    # Build history BEFORE persisting the current student turn so the pruned
    # window reflects only *prior* interactions. If we appended first, the
    # freshly-written student row would come back as the final history entry
    # and end up duplicated alongside the explicit user message below.
    history, context_summary = await build_pruned_history(
        db=db,
        client=client,
        session_id=session_id,
        window_size=settings.max_history_messages,
        summarise_threshold=settings.context_summarise_threshold,
        fast_model=settings.tutor_fast_model,
    )
    await append_message(db, session_id, user.user_id, role="student", content=student_message)

    # Step 1: Fetch student's learning-style dials (non-fatal)
    try:
        current_dials = await get_dials(db, user.user_id)
    except Exception:  # noqa: BLE001
        logger.warning("Failed to load learning-style dials (user omitted)")
        current_dials = {
            "active_reflective": 0.0, "sensing_intuitive": 0.0,
            "visual_verbal": 0.0, "sequential_global": 0.0,
        }

    # Step 2: Build system prompt + retrieve grounding chunks
    source_chunks = []
    diagram_results = []
    if session.course_id is not None:
        # Rewrite pronoun-heavy follow-ups into a self-contained retrieval
        # query so the Search Service doesn't lose antecedents at embedding
        # time. Falls back to body.content on any gateway failure.
        retrieval_question = await reformulate_query(
            body.content, history[-4:], client=client,
        )
        base_prompt, source_chunks = await build_rag_context(
            student_id=user.user_id, session_id=session_id,
            question=retrieval_question, course_id=session.course_id, db=db,
        )
        if _VISUAL_QUERY_PATTERNS.search(body.content):
            diagram_results = await fetch_diagram_chunks(
                query=body.content, course_id=session.course_id,
                limit=settings.diagram_limit,
            )
    else:
        base_prompt = PROFESSOR_NOVA_SYSTEM_PROMPT

    system_prompt = base_prompt + build_style_prompt_section(current_dials)
    if context_summary:
        system_prompt += f"\n\n[Earlier conversation summary: {context_summary}]"

    # Session-start spaced-repetition hook: inject due concepts into the
    # system prompt so Professor Nova can weave recall cues into the
    # upcoming turn. Failure is non-fatal — the tutor still runs without it.
    try:
        srs_reminder = await get_session_start_reminder(db, user.user_id)
        if srs_reminder:
            system_prompt += f"\n\n{srs_reminder}"
    except Exception:  # noqa: BLE001
        logger.warning("Failed to fetch spaced-repetition reminder (user omitted)")

    messages = [
        {"role": "system", "content": system_prompt},
        *_history_to_messages(history),
        {"role": "user", "content": body.content},
    ]
    log_token_usage(
        messages,
        model_context_limit=settings.model_context_limit,
        warn_ratio=settings.context_token_warn_ratio,
        session_id=session_id,
    )

    tutor_msg_id = str(uuid.uuid4())
    sources_payload = [
        {
            "chunk_id": c.chunk_id, "chapter": c.chapter, "section": c.section,
            "page": c.page, "content_type": c.content_type, "score": round(c.score, 4),
        }
        for c in source_chunks
    ]

    return StreamingResponse(
        _chat_stream_generator(
            db=db, session_id=session_id, user_id=user.user_id,
            has_rag=session.course_id is not None, messages=messages,
            client=client, tutor_msg_id=tutor_msg_id, sources_payload=sources_payload,
            diagram_results=diagram_results, current_dials=current_dials,
            student_message=student_message,
        ),
        media_type="text/event-stream",
        headers={"Cache-Control": "no-cache", "X-Accel-Buffering": "no"},
    )
