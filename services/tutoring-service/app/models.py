from datetime import datetime
from enum import Enum
from uuid import UUID, uuid4

from pydantic import BaseModel, Field


class Role(str, Enum):
    student = "student"
    tutor = "tutor"


# ── Request / Response DTOs ───────────────────────────────────────────────────

class CreateSessionRequest(BaseModel):
    user_id: UUID
    course_id: UUID | None = None
    student_name: str | None = None  # used to personalise Nova's greeting


class SessionResponse(BaseModel):
    session_id: UUID
    user_id: UUID
    course_id: UUID | None
    created_at: datetime
    message_count: int


class MessageRequest(BaseModel):
    content: str = Field(..., max_length=8000)


class MessageRecord(BaseModel):
    id: UUID
    session_id: UUID
    role: Role
    content: str
    created_at: datetime


class HistoryResponse(BaseModel):
    session_id: UUID
    messages: list[MessageRecord]


# ── SSE event shapes ──────────────────────────────────────────────────────────

class SSEEvent(BaseModel):
    """Single token/chunk emitted over the SSE stream."""
    type: str  # "delta" | "done" | "error"
    content: str = ""
    message_id: str = ""
