"""
Chat endpoint — streaming SSE response via LiteLLM proxy.

Flow:
  1. Validate session exists and belongs to user (Phase 2+ will enforce auth)
  2. Load conversation history (last N messages for context window)
  3. Build messages array: system prompt + history + new user message
  4. Stream from AI Gateway (LiteLLM) using OpenAI-compatible SDK
  5. Persist user message + completed assistant message to Postgres
  6. Emit SSE events: delta chunks → done
"""
import json
import time
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException
from fastapi.responses import StreamingResponse
from openai import AsyncOpenAI
from sqlalchemy.ext.asyncio import AsyncSession

from .config import settings
from .database import get_db
from .history import append_message, get_history, get_session
from .models import MessageRequest

router = APIRouter(prefix="/sessions", tags=["chat"])

PROFESSOR_NOVA_SYSTEM_PROMPT = """\
You are Professor Nova, the AI tutor for TeachersLounge — a gamified learning \
platform. You are brilliant, patient, encouraging, and a little bit nerdy. You \
use vivid analogies, celebrate curiosity, and make hard concepts feel approachable.

Guidelines:
- Ask clarifying questions before diving into a long explanation if the question is vague.
- Use concrete examples. Always pair an abstraction with something tangible.
- When a student is wrong, be gentle but clear. Explain why, don't just give the answer.
- Use LaTeX notation for math/formulas: $E = mc^2$ inline, $$...$$ for display.
- Keep responses focused. If a topic is huge, offer to go deeper on a specific part.
- You don't have access to the student's course materials yet (Phase 2 will add that).
  For now, draw on your broad knowledge and acknowledge when you're working from \
  general knowledge rather than their specific textbook.
"""


def _build_openai_client() -> AsyncOpenAI:
    return AsyncOpenAI(
        base_url=settings.ai_gateway_url + "/v1",
        api_key=settings.ai_gateway_key,
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
):
    """
    Send a student message and receive a streaming SSE response from Professor Nova.

    SSE event shapes:
      data: {"type": "delta",  "content": "<token>", "message_id": "<uuid>"}
      data: {"type": "done",   "content": "",         "message_id": "<uuid>"}
      data: {"type": "error",  "content": "<msg>",    "message_id": ""}
    """
    session = await get_session(db, session_id)
    if session is None:
        raise HTTPException(status_code=404, detail="Session not found")

    history = await get_history(db, session_id, limit=settings.max_history_messages)

    # Persist the student's message before streaming so it's durable
    student_msg = await append_message(
        db, session_id, session.user_id, role="student", content=body.content
    )

    messages = [
        {"role": "system", "content": PROFESSOR_NOVA_SYSTEM_PROMPT},
        *_history_to_messages(history),
        {"role": "user", "content": body.content},
    ]

    client = _build_openai_client()

    async def stream_generator():
        full_response = []
        start_ms = int(time.time() * 1000)
        tutor_msg_id = None

        try:
            stream = await client.chat.completions.create(
                model=settings.tutor_primary_model,
                messages=messages,
                stream=True,
                max_tokens=4096,
            )

            # Pre-generate the message ID so the client can reference it
            import uuid as _uuid
            tutor_msg_id = str(_uuid.uuid4())

            async for chunk in stream:
                delta = chunk.choices[0].delta
                if delta.content:
                    full_response.append(delta.content)
                    yield await _sse("delta", content=delta.content, message_id=tutor_msg_id)

            # Persist completed assistant response
            complete_text = "".join(full_response)
            elapsed_ms = int(time.time() * 1000) - start_ms
            await append_message(
                db,
                session_id,
                session.user_id,
                role="tutor",
                content=complete_text,
                response_time_ms=elapsed_ms,
            )

            yield await _sse("done", message_id=tutor_msg_id)

        except Exception as exc:
            yield await _sse("error", content=str(exc))

    return StreamingResponse(
        stream_generator(),
        media_type="text/event-stream",
        headers={
            "Cache-Control": "no-cache",
            "X-Accel-Buffering": "no",   # disable nginx buffering
        },
    )
