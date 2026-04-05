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
    type: str   # "delta" | "sources" | "done" | "error" | "diagram" | "molecule_builder"
    content: str = ""
    message_id: str = ""
    # Populated on "sources" events — list of curriculum chunks used for grounding
    sources: list[dict] | None = None
    # Populated on "diagram" events — a single diagram result
    diagram: dict | None = None


# ── Quiz answer DTOs (Phase 6 molecule builder) ───────────────────────────────

class QuizAnswerRequest(BaseModel):
    """Request body for POST /v1/quiz/answer.

    Supports both multiple-choice (chosen_key) and molecule-builder (smiles_answer)
    answer types.  At least one of the two fields must be provided.
    """
    chosen_key: str | None = Field(
        default=None,
        description="Answer key for multiple-choice questions (e.g. 'A', 'B').",
    )
    smiles_answer: str | None = Field(
        default=None,
        max_length=4096,
        description="SMILES string submitted from the molecule builder canvas.",
    )
    expected_smiles: str | None = Field(
        default=None,
        max_length=4096,
        description="Expected SMILES for server-side evaluation (sent by tutor in quiz prompt).",
    )


class QuizAnswerResponse(BaseModel):
    """Result of evaluating a quiz answer."""
    correct: bool
    feedback: str
    answer_type: str   # "multiple_choice" | "smiles"
    submitted: str     # the value that was evaluated


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


class PrerequisiteChainEntry(BaseModel):
    """One concept in the prerequisite chain with the student's current mastery."""

    concept_id: UUID
    concept_name: str
    path: str
    difficulty: float
    mastery_score: float
    mastery_adequate: bool   # True if mastery_score >= gap threshold
    depth: int               # hops from the target concept (1 = direct prerequisite)


class PrerequisiteChainResponse(BaseModel):
    """All transitive prerequisites for a concept, ordered by depth then name."""

    target_concept_id: UUID
    target_concept_name: str
    chain: list[PrerequisiteChainEntry]


class MasteryUpdateRequest(BaseModel):
    """Body for PATCH mastery — caller supplies the new observed mastery score."""

    mastery_score: float = Field(..., ge=0.0, le=1.0,
                                 description="New mastery score (0-1) from interaction evidence")


class MasteryUpdateResponse(BaseModel):
    """Result after updating a student's mastery for a concept."""

    concept_id: UUID
    mastery_before_decay: float   # stored score at time of update
    mastery_after_decay: float    # score after applying forgetting-curve decay
    mastery_updated: float        # final stored value (the new score)
    decay_rate: float
    last_reviewed_at: datetime
