"""
/v1/chat — stateless chat endpoint consumed by the Next.js frontend.

The frontend (frontend/app/api/chat/route.ts) sends the full conversation history
on every request (client-side state management) and expects a plain text stream.

Contract:
  POST /v1/chat
  Body:    { "messages": [{"role": "user"|"assistant", "content": "..."}] }
  Auth:    Bearer JWT (optional — frontend doesn't attach it yet in Phase 1;
           when auth is wired up the JWT will carry user_id for DB persistence)
  Returns: text/plain streaming response (raw token chunks, no SSE envelope)

The Professor Nova system prompt is always prepended server-side.
Conversation history is persisted to Postgres when a valid JWT is present.
"""
import logging
import time
from typing import AsyncGenerator

from fastapi import APIRouter, Depends, Request
from fastapi.responses import StreamingResponse
from openai import AsyncOpenAI, APIConnectionError, APIStatusError, APITimeoutError
from pydantic import BaseModel
from sqlalchemy.ext.asyncio import AsyncSession

from .auth import JWTClaims, require_auth
from .chat import FALLBACK_MESSAGE, PROFESSOR_NOVA_SYSTEM_PROMPT, _gateway_client
from .config import settings
from .database import get_db
from .history import append_message, create_session

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/v1", tags=["chat-simple"])


class ChatMessage(BaseModel):
    role: str       # "user" | "assistant"
    content: str


class ChatRequest(BaseModel):
    messages: list[ChatMessage]


async def _try_get_user(request: Request) -> JWTClaims | None:
    """Extract JWT claims if a Bearer token is present; return None otherwise."""
    auth_header = request.headers.get("Authorization", "")
    if not auth_header.startswith("Bearer "):
        return None
    from fastapi.security import HTTPAuthorizationCredentials
    try:
        creds = HTTPAuthorizationCredentials(scheme="Bearer", credentials=auth_header[7:])
        return require_auth(creds)
    except Exception:
        return None


async def _stream_text(
    messages: list[dict],
    client: AsyncOpenAI,
) -> AsyncGenerator[bytes, None]:
    """Yield raw token bytes for a plain text streaming response."""
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
                yield delta.content.encode()
    except (APIConnectionError, APITimeoutError):
        logger.warning("AI Gateway unreachable — returning fallback")
        yield FALLBACK_MESSAGE.encode()
    except APIStatusError as exc:
        logger.error("AI Gateway %s: %s", exc.status_code, exc.message)
        yield FALLBACK_MESSAGE.encode()
    except Exception as exc:
        logger.exception("Unexpected stream error: %s", exc)
        yield b"An unexpected error occurred. Please try again."


@router.post("/chat")
async def chat(
    body: ChatRequest,
    request: Request,
    db: AsyncSession = Depends(get_db),
):
    """
    Stateless chat endpoint. Accepts full conversation history, returns plain text stream.
    Persists student message + tutor reply to Postgres when JWT is valid.
    """
    user = await _try_get_user(request)

    # Build messages with system prompt prepended
    openai_messages = [
        {"role": "system", "content": PROFESSOR_NOVA_SYSTEM_PROMPT},
        *[{"role": m.role, "content": m.content} for m in body.messages],
    ]

    # Persist student's message if authenticated
    session_id = None
    if user and body.messages:
        last = body.messages[-1]
        if last.role == "user":
            session = await create_session(db, user_id=user.user_id)
            session_id = session.id
            await append_message(db, session_id, user.user_id, role="student", content=last.content)

    client = _gateway_client()

    async def generator():
        chunks: list[str] = []
        start_ms = int(time.time() * 1000)
        async for chunk_bytes in _stream_text(openai_messages, client):
            chunks.append(chunk_bytes.decode())
            yield chunk_bytes

        # Persist tutor reply after stream completes
        if user and session_id:
            elapsed_ms = int(time.time() * 1000) - start_ms
            complete = "".join(chunks)
            await append_message(
                db, session_id, user.user_id,
                role="tutor", content=complete, response_time_ms=elapsed_ms,
            )

    return StreamingResponse(
        generator(),
        media_type="text/plain; charset=utf-8",
        headers={
            "Cache-Control": "no-cache",
            "X-Accel-Buffering": "no",
        },
    )
