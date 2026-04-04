from datetime import datetime
from enum import Enum
from uuid import UUID

from pydantic import BaseModel, Field


class Role(str, Enum):
    student = "student"
    tutor = "tutor"


# ── Request / Response DTOs ───────────────────────────────────────────────────

class CreateSessionRequest(BaseModel):
    course_id: UUID | None = None       # user_id comes from JWT, not request body


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


# ── Spaced-repetition review DTOs ────────────────────────────────────────────

class ReviewQueueItem(BaseModel):
    concept_id: UUID
    concept_name: str
    mastery_score: float
    ease_factor: float
    interval_days: int
    repetitions: int
    next_review_at: datetime | None
    last_reviewed_at: datetime | None
    is_overdue: bool


class ReviewQueueResponse(BaseModel):
    items: list[ReviewQueueItem]
    total_due: int
    total_upcoming: int


class AnswerRequest(BaseModel):
    quality: int = Field(..., ge=0, le=5, description="Review quality 0-5 (0=blackout, 5=perfect)")


class AnswerResponse(BaseModel):
    concept_id: UUID
    quality: int
    mastery_before: float
    mastery_after: float
    ease_factor: float
    interval_days: int
    repetitions: int
    next_review_at: datetime


class ReviewStatsResponse(BaseModel):
    total_concepts_studied: int
    total_reviews: int
    due_now: int
    due_today: int
    due_this_week: int
    average_mastery: float
    average_ease_factor: float


# ── SSE event shapes ──────────────────────────────────────────────────────────

class SSEEvent(BaseModel):
    """Single token/chunk emitted over the SSE stream."""
    type: str   # "delta" | "sources" | "done" | "error"
    content: str = ""
    message_id: str = ""
    # Populated on "sources" events — list of curriculum chunks used for grounding
    sources: list[dict] | None = None


# ── Concept Dependency Graph DTOs ────────────────────────────────────────────


class ConceptResponse(BaseModel):
    id: UUID
    course_id: UUID
    name: str
    description: str
    path: str
    prerequisite_ids: list[UUID] = []


class ConceptCreate(BaseModel):
    name: str = Field(..., max_length=255)
    description: str = ""
    path: str = Field(..., max_length=1000)
    prerequisite_ids: list[UUID] = []


class PrerequisiteEdge(BaseModel):
    concept_id: UUID
    prerequisite_id: UUID
    weight: float = Field(1.0, ge=0.0, le=1.0)


class MasteryEntry(BaseModel):
    concept_id: UUID
    concept_name: str
    mastery_score: float
    last_reviewed_at: datetime | None = None
    next_review_at: datetime | None = None


class GapInfo(BaseModel):
    concept_id: UUID
    concept_name: str
    mastery_score: float
    required_by: list[UUID]


class GapDetectionResponse(BaseModel):
    target_concept_id: UUID
    target_concept_name: str
    gaps: list[GapInfo]


class RemediationStep(BaseModel):
    order: int
    concept_id: UUID
    concept_name: str
    mastery_score: float
    reason: str


class RemediationPathResponse(BaseModel):
    target_concept_id: UUID
    target_concept_name: str
    steps: list[RemediationStep]
