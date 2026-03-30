"""
Chat endpoint — JWT-protected, streaming SSE response via LiteLLM proxy.

Flow:
  1. Validate JWT → extract user_id
  2. Verify session exists and belongs to this user
  3. Load last 10 exchanges (20 messages) for context window
  4. Build messages: system prompt + history + new student message
  5. Stream from AI Gateway (LiteLLM) via OpenAI-compatible SDK
  6. Persist student message (pre-stream) + completed tutor reply (post-stream)
  7. Emit SSE: delta chunks → done  (error event on failure with fallback text)

SSE event format:
  data: {"type": "delta",  "content": "<token>", "message_id": "<uuid>"}
  data: {"type": "done",   "content": "",         "message_id": "<uuid>"}
  data: {"type": "error",  "content": "<msg>",    "message_id": ""}
"""
import json
import logging
import time
import uuid
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException
from fastapi.responses import StreamingResponse
from openai import APIConnectionError, APIStatusError, APITimeoutError
from sqlalchemy.ext.asyncio import AsyncSession

from .auth import JWTClaims, require_auth
from .config import settings
from .database import get_db
from .gateway import get_gateway_client
from .history import append_message, get_history, get_session
from .models import MessageRequest

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/sessions", tags=["chat"])

PROFESSOR_NOVA_SYSTEM_PROMPT = """\
You are Professor Nova, the AI tutor for TeachersLounge — a gamified learning \
platform. You are brilliant, patient, encouraging, and a little bit nerdy. You \
use vivid analogies, celebrate curiosity, and make hard concepts feel approachable.

Guidelines:
- Ask a clarifying question before a long explanation if the question is vague.
- Use concrete examples. Always pair an abstraction with something tangible.
- When a student is wrong, be gentle but clear — explain why, don't just give the answer.
- Use LaTeX notation for math/formulas: $E = mc^2$ inline, $$...$$ for display.
- Keep responses focused. If a topic is huge, offer to go deeper on a specific part.
- You do not yet have access to the student's uploaded course materials (that's \
  coming in Phase 2). For now, draw on your broad knowledge and be transparent \
  when you're working from general knowledge rather than their specific textbook.
"""

FALLBACK_MESSAGE = (
    "I'm having a moment of technical difficulty — my connection to the knowledge "
    "network seems unstable. Could you try sending your question again? I'll be right "
    "here when you do. 🔧"
)


def _history_to_messages(interactions) -> list[dict]:
    return [
        {"role": "user" if i.role == "student" else "assistant", "content": i.content}
        for i in interactions
    ]


async def _sse(event_type: str, content: str = "", message_id: str = "") -> str:
    payload = json.dumps({"type": event_type, "content": content, "message_id": message_id})
    return f"data: {payload}\n\n"


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

    # Last 10 exchanges = 20 messages (10 student + 10 tutor)
    history = await get_history(db, session_id, limit=settings.max_history_messages)

    # Persist student message before streaming — durable even if stream fails
    await append_message(db, session_id, user.user_id, role="student", content=body.content)

    messages = [
        {"role": "system", "content": PROFESSOR_NOVA_SYSTEM_PROMPT},
        *_history_to_messages(history),
        {"role": "user", "content": body.content},
    ]

    client = get_gateway_client()
    tutor_msg_id = str(uuid.uuid4())

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
                    yield await _sse("delta", content=delta.content, message_id=tutor_msg_id)

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
            yield await _sse("done", message_id=tutor_msg_id)

        except (APIConnectionError, APITimeoutError) as exc:
            logger.warning("AI Gateway unreachable: %s", exc)
            # Persist fallback so history stays coherent
            await append_message(
                db, session_id, user.user_id, role="tutor", content=FALLBACK_MESSAGE
            )
            yield await _sse("delta", content=FALLBACK_MESSAGE, message_id=tutor_msg_id)
            yield await _sse("done", message_id=tutor_msg_id)

        except APIStatusError as exc:
            logger.error("AI Gateway error %s: %s", exc.status_code, exc.message)
            await append_message(
                db, session_id, user.user_id, role="tutor", content=FALLBACK_MESSAGE
            )
            yield await _sse("delta", content=FALLBACK_MESSAGE, message_id=tutor_msg_id)
            yield await _sse("done", message_id=tutor_msg_id)

        except Exception as exc:
            logger.exception("Unexpected error in chat stream: %s", exc)
            yield await _sse("error", content="An unexpected error occurred. Please try again.")

    return StreamingResponse(
        stream_generator(),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "X-Accel-Buffering": "no",   # disable nginx buffering for SSE
        },
    )
