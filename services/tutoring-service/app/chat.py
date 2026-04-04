"""
Chat endpoint — JWT-protected, streaming SSE response via LiteLLM proxy.

Flow:
  1. Validate JWT → extract user_id
  2. Verify session exists and belongs to this user
  3. Load last 10 exchanges (20 messages) for context window
  4. Fetch student's Felder-Silverman learning-style dials (non-fatal)
  5. Build messages:
       - If session.course_id is set: agentic RAG (retrieve chunks → enriched prompt)
       - Otherwise: base Professor Nova prompt (no course materials)
       - Append style guidance addendum to the system prompt
  6. Stream from AI Gateway (LiteLLM) via OpenAI-compatible SDK
  7. Persist student message (pre-stream) + completed tutor reply (post-stream)
  8. Update learning-style dials from student message signals (post-stream, non-fatal)
  9. Emit SSE: delta chunks → sources → done  (error event on failure)

SSE event format:
  data: {"type": "delta",   "content": "<token>", "message_id": "<uuid>"}
  data: {"type": "sources", "content": "",         "message_id": "<uuid>", "sources": [...]}
  data: {"type": "done",    "content": "",         "message_id": "<uuid>"}
  data: {"type": "error",   "content": "<msg>",    "message_id": ""}

sources event is emitted only when the session has a course_id and chunks were
retrieved. Each source object contains: chunk_id, chapter, section, page,
content_type, score.
"""
import json
import logging
import time
import uuid
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException, Request
from fastapi.responses import StreamingResponse
from openai import APIConnectionError, APIStatusError, APITimeoutError
from sqlalchemy.ext.asyncio import AsyncSession

from .auth import JWTClaims, require_auth
from .config import settings
from .database import get_db
from .gateway import get_gateway_client
from .history import append_message, get_history, get_session
from .models import MessageRequest
from .rag_agent import PROFESSOR_NOVA_SYSTEM_PROMPT, build_rag_context
from .style_detector import DEFAULT_DIALS, build_style_prompt_section, detect_signals, update_dials
from .user_client import UserServiceClient

logger = logging.getLogger(__name__)

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


def _sse(event_type: str, content: str = "", message_id: str = "", sources=None) -> str:
    """Serialise a single SSE frame.

    Args:
        event_type: One of ``"delta"``, ``"sources"``, ``"done"``, ``"error"``.
        content: Token text for delta events; empty string for others.
        message_id: UUID string of the tutor turn being streamed.
        sources: List of source dicts; included only on ``"sources"`` events.

    Returns:
        SSE-formatted string ready to ``yield`` from the stream generator.
    """
    payload: dict = {"type": event_type, "content": content, "message_id": message_id}
    if sources is not None:
        payload["sources"] = sources
    return f"data: {json.dumps(payload)}\n\n"


@router.post("/{session_id}/messages")
async def send_message(
    session_id: UUID,
    body: MessageRequest,
    request: Request,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Send a student message and receive a streaming SSE response from Professor Nova."""
    session = await get_session(db, session_id)
    if session is None:
        raise HTTPException(status_code=404, detail="Session not found")
    if session.user_id != user.user_id:
        raise HTTPException(status_code=403, detail="Forbidden")

    # Last 10 exchanges = 20 messages (10 student + 10 tutor)
    history = await get_history(db, session_id, limit=settings.max_history_messages)

    # Persist student message before streaming — durable even if stream fails
    student_message = body.content
    await append_message(db, session_id, user.user_id, role="student", content=student_message)

    # --- Step 1: Fetch student's current learning-style dials (non-fatal) ---
    raw_token = (request.headers.get("Authorization") or "").removeprefix("Bearer ").strip()
    user_client = UserServiceClient(base_url=settings.user_service_url, bearer_token=raw_token)
    current_dials = await user_client.get_felder_silverman_dials(user.user_id)
    if not current_dials:
        current_dials = dict(DEFAULT_DIALS)

    # --- Step 2: Build system prompt + retrieve grounding chunks ---
    source_chunks = []
    if session.course_id is not None:
        base_prompt, source_chunks = await build_rag_context(
            student_id=user.user_id,
            session_id=session_id,
            question=body.content,
            course_id=session.course_id,
            db=db,
        )
    else:
        base_prompt = PROFESSOR_NOVA_SYSTEM_PROMPT

    # Step 3: Append style guidance to the system prompt
    system_prompt = base_prompt + build_style_prompt_section(current_dials)

    messages = [
        {"role": "system", "content": system_prompt},
        *_history_to_messages(history),
        {"role": "user", "content": body.content},
    ]

    client = get_gateway_client()
    tutor_msg_id = str(uuid.uuid4())

    # Build sources payload once — serialised into the sources event
    sources_payload = [
        {
            "chunk_id": c.chunk_id,
            "chapter": c.chapter,
            "section": c.section,
            "page": c.page,
            "content_type": c.content_type,
            "score": round(c.score, 4),
        }
        for c in source_chunks
    ]

    async def stream_generator():
        full_response: list[str] = []
        start_ms = int(time.time() * 1000)

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
                db,
                session_id,
                user.user_id,
                role="tutor",
                content=complete_text,
                response_time_ms=elapsed_ms,
            )

            if sources_payload:
                yield _sse("sources", message_id=tutor_msg_id, sources=sources_payload)

            yield _sse("done", message_id=tutor_msg_id)

            # --- Step 4: Update dials from this message's signals (post-stream) ---
            signals = detect_signals(student_message)
            if signals:
                updated_dials = update_dials(current_dials, signals)
                ok = await user_client.patch_felder_silverman_dials(user.user_id, updated_dials)
                if not ok:
                    logger.warning("Failed to persist learning-style dials for user %s", user.user_id)

        except (APIConnectionError, APITimeoutError) as exc:
            logger.warning("AI Gateway unreachable: %s", exc)
            await append_message(
                db, session_id, user.user_id, role="tutor", content=FALLBACK_MESSAGE
            )
            yield _sse("delta", content=FALLBACK_MESSAGE, message_id=tutor_msg_id)
            yield _sse("done", message_id=tutor_msg_id)

        except APIStatusError as exc:
            logger.error("AI Gateway error %s: %s", exc.status_code, exc.message)
            await append_message(
                db, session_id, user.user_id, role="tutor", content=FALLBACK_MESSAGE
            )
            yield _sse("delta", content=FALLBACK_MESSAGE, message_id=tutor_msg_id)
            yield _sse("done", message_id=tutor_msg_id)

        except Exception as exc:
            logger.exception("Unexpected error in chat stream: %s", exc)
            yield _sse("error", content="An unexpected error occurred. Please try again.")

    return StreamingResponse(
        stream_generator(),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "X-Accel-Buffering": "no",   # disable nginx buffering for SSE
        },
    )
