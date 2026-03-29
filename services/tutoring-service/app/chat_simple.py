"""
Stateless chat endpoint — accepts a messages array and streams a plain-text response.

Used by the frontend when it manages its own conversation history (e.g., Neon Arcade UI).
No session, no DB writes — pure pass-through to the AI Gateway.

Request:
    POST /v1/chat
    Authorization: Bearer <token>
    Content-Type: application/json
    {
        "messages": [
            {"role": "user",      "content": "What is photosynthesis?"},
            {"role": "assistant", "content": "Photosynthesis is..."},
            {"role": "user",      "content": "Can you give an example?"}
        ]
    }

Response:
    Content-Type: text/plain; charset=utf-8
    Transfer-Encoding: chunked
    (streamed token-by-token)
"""
import logging
from typing import Literal

from fastapi import APIRouter, Depends
from fastapi.responses import StreamingResponse
from openai import APIConnectionError, APIStatusError, APITimeoutError
from pydantic import BaseModel

from .auth import JWTClaims, require_auth
from .config import settings
from .gateway import get_gateway_client

logger = logging.getLogger(__name__)

router = APIRouter(tags=["chat-simple"])

FALLBACK_TEXT = (
    "I'm having a moment of technical difficulty. "
    "Please try again in a moment. 🔧"
)


class ChatMessage(BaseModel):
    role: Literal["user", "assistant", "system"]
    content: str


class SimpleChatRequest(BaseModel):
    messages: list[ChatMessage]


@router.post("/chat")
async def simple_chat(
    body: SimpleChatRequest,
    user: JWTClaims = Depends(require_auth),
):
    """Stream a plain-text response for a stateless messages array."""
    client = get_gateway_client()

    # Prepend system prompt if caller didn't include one
    messages = [m.model_dump() for m in body.messages]
    if not messages or messages[0]["role"] != "system":
        from .chat import PROFESSOR_NOVA_SYSTEM_PROMPT
        messages = [{"role": "system", "content": PROFESSOR_NOVA_SYSTEM_PROMPT}] + messages

    async def stream_generator():
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
                    yield delta.content

        except (APIConnectionError, APITimeoutError) as exc:
            logger.warning("AI Gateway unreachable (simple chat): %s", exc)
            yield FALLBACK_TEXT

        except APIStatusError as exc:
            logger.error("AI Gateway error %s (simple chat): %s", exc.status_code, exc.message)
            yield FALLBACK_TEXT

        except Exception as exc:
            logger.exception("Unexpected error in simple chat: %s", exc)
            yield "An unexpected error occurred. Please try again."

    return StreamingResponse(
        stream_generator(),
        media_type="text/plain; charset=utf-8",
        headers={
            "Cache-Control": "no-cache",
            "X-Accel-Buffering": "no",
        },
    )
