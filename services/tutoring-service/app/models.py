import re
from datetime import datetime
from enum import Enum
from uuid import UUID

from pydantic import BaseModel, Field, field_validator

# Postgres ltree label regex: each dot-separated segment must start with a
# lowercase letter and contain only lowercase alphanumerics + underscores.
_LTREE_PATH_RE = re.compile(r"^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$")


class Role(str, Enum):
    """Message role within a tutoring session."""

    student = "student"
    tutor = "tutor"


# ── Request / Response DTOs ───────────────────────────────────────────────────


class CreateSessionRequest(BaseModel):
    """Request body for POST /v1/sessions."""

    course_id: UUID | None = None  # user_id comes from JWT, not request body


class SessionResponse(BaseModel):
    """Response body returned after creating or fetching a tutoring session."""

    session_id: UUID
    user_id: UUID
    course_id: UUID | None
    created_at: datetime
    message_count: int


class MessageRequest(BaseModel):
    """Request body for sending a student message to a session."""

    content: str = Field(..., max_length=8000)


class MessageRecord(BaseModel):
    """A single message record returned in history responses."""

    id: UUID
    session_id: UUID
    role: Role
    content: str
    created_at: datetime


class HistoryResponse(BaseModel):
    """Response body for GET /sessions/{id}/messages."""

    session_id: UUID
    messages: list[MessageRecord]


# ── Spaced-repetition review DTOs ────────────────────────────────────────────


class ReviewQueueItem(BaseModel):
    """A single concept in the spaced-repetition review queue."""

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
    """Paginated response for GET /v1/reviews/due."""

    items: list[ReviewQueueItem]
    total_due: int
    total_upcoming: int


class AnswerRequest(BaseModel):
    """Request body for submitting a spaced-repetition review answer."""

    quality: int = Field(..., ge=0, le=5, description="Review quality 0-5 (0=blackout, 5=perfect)")


class AnswerResponse(BaseModel):
    """Result of recording a spaced-repetition answer, including updated SRS state."""

    concept_id: UUID
    quality: int
    mastery_before: float
    mastery_after: float
    ease_factor: float
    interval_days: int
    repetitions: int
    next_review_at: datetime


class ReviewStatsResponse(BaseModel):
    """Aggregate spaced-repetition statistics for a student."""

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

    type: str  # "delta" | "sources" | "done" | "error" | "diagram" | "molecule_builder"
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
    answer_type: str  # "multiple_choice" | "smiles"
    submitted: str  # the value that was evaluated


# ── Concept Dependency Graph DTOs ────────────────────────────────────────────


class ConceptResponse(BaseModel):
    """Response body for a single concept node in the dependency graph."""

    id: UUID
    course_id: UUID
    name: str
    description: str
    path: str
    prerequisite_ids: list[UUID] = []


class ConceptCreate(BaseModel):
    """Request body for creating a new concept in the dependency graph."""

    name: str = Field(..., max_length=255)
    description: str = ""
    path: str = Field(..., max_length=1000)
    prerequisite_ids: list[UUID] = []


class PrerequisiteEdge(BaseModel):
    """A directed prerequisite edge between two concepts."""

    concept_id: UUID
    prerequisite_id: UUID
    weight: float = Field(1.0, ge=0.0, le=1.0)


class MasteryEntry(BaseModel):
    """A student's mastery record for one concept, including SRS timing."""

    concept_id: UUID
    concept_name: str
    mastery_score: float
    last_reviewed_at: datetime | None = None
    next_review_at: datetime | None = None


class GapInfo(BaseModel):
    """A prerequisite concept where the student's mastery is below threshold."""

    concept_id: UUID
    concept_name: str
    mastery_score: float
    required_by: list[UUID]


class GapDetectionResponse(BaseModel):
    """Response body for gap detection — lists unmastered prerequisite concepts."""

    target_concept_id: UUID
    target_concept_name: str
    gaps: list[GapInfo]


class RemediationStep(BaseModel):
    """One concept in a topologically-ordered remediation study plan."""

    order: int
    concept_id: UUID
    concept_name: str
    mastery_score: float
    reason: str


class RemediationPathResponse(BaseModel):
    """Ordered remediation study plan for reaching a target concept."""

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
    mastery_adequate: bool  # True if mastery_score >= gap threshold
    depth: int  # hops from the target concept (1 = direct prerequisite)


class PrerequisiteChainResponse(BaseModel):
    """All transitive prerequisites for a concept, ordered by depth then name."""

    target_concept_id: UUID
    target_concept_name: str
    chain: list[PrerequisiteChainEntry]


class MasteryUpdateRequest(BaseModel):
    """Body for PATCH mastery — caller supplies the new observed mastery score."""

    mastery_score: float = Field(
        ..., ge=0.0, le=1.0, description="New mastery score (0-1) from interaction evidence"
    )


class MasteryUpdateResponse(BaseModel):
    """Result after updating a student's mastery for a concept."""

    concept_id: UUID
    mastery_before_decay: float  # stored score at time of update
    mastery_after_decay: float  # score after applying forgetting-curve decay
    mastery_updated: float  # final stored value (the new score)
    decay_rate: float
    last_reviewed_at: datetime


# ── Learning profile DTOs ──────────────────────────────────────────────────────


class LearningProfileDials(BaseModel):
    """Felder-Silverman dial values — all four dimensions, each in [-1, 1]."""

    active_reflective: float = Field(0.0, ge=-1.0, le=1.0)
    sensing_intuitive: float = Field(0.0, ge=-1.0, le=1.0)
    visual_verbal: float = Field(0.0, ge=-1.0, le=1.0)
    sequential_global: float = Field(0.0, ge=-1.0, le=1.0)


class LearningProfileResponse(BaseModel):
    """Response body for GET /students/me/learning-profile."""

    user_id: UUID
    dials: dict[str, float]
    updated_at: datetime | None = None


_VALID_DIAL_KEYS: frozenset[str] = frozenset(
    {"active_reflective", "sensing_intuitive", "visual_verbal", "sequential_global"}
)


class LearningProfileUpdateRequest(BaseModel):
    """Request body for PATCH /students/me/learning-profile.

    Only dial values provided are updated; absent keys are unchanged.
    Each value must be in the range [-1.0, 1.0] and must be one of the four
    Felder-Silverman dimensions.
    """

    dials: dict[str, float] = Field(
        default_factory=dict,
        description="Partial or full Felder-Silverman dial updates. Values must be in [-1, 1].",
    )

    def validate_dial_values(self) -> None:
        """Raise ValueError if any key is unknown or any value is outside [-1, 1]."""
        for k, v in self.dials.items():
            if k not in _VALID_DIAL_KEYS:
                raise ValueError(
                    f"Unknown dial dimension '{k}'. Valid keys: {sorted(_VALID_DIAL_KEYS)}"
                )
            if not -1.0 <= v <= 1.0:
                raise ValueError(f"Dial '{k}' value {v} is outside [-1, 1]")


# ── Misconception DTOs ────────────────────────────────────────────────────────


class MisconceptionLogRequest(BaseModel):
    """Request body for POST /students/me/misconceptions/{concept_id}."""

    description: str = Field(
        ..., max_length=2000, description="Description of the detected misconception."
    )


class MisconceptionEntry(BaseModel):
    """One misconception in the active list response."""

    id: UUID
    concept_id: UUID
    description: str
    confidence: float
    recorded_at: datetime
    recency_weight: float


# ── Concept graph DTOs (tl-mhd) ───────────────────────────────────────────────


class ConceptGraphNodeResponse(BaseModel):
    """Public projection of a :class:`app.orm.ConceptGraphNode` row."""

    concept_id: str
    label: str
    subject: str
    path: str


class CreateConceptRequest(BaseModel):
    """Request body for POST /concepts — create a new global concept."""

    concept_id: str = Field(
        ...,
        min_length=1,
        max_length=128,
        description="Stable slug identifier (lowercase snake_case).",
    )
    label: str = Field(
        ...,
        min_length=1,
        max_length=255,
        description="Human-readable title surfaced in tutoring prompts.",
    )
    subject: str = Field(
        ...,
        min_length=1,
        max_length=64,
        description="Top-level subject grouping (e.g. 'chemistry').",
    )
    path: str = Field(
        ...,
        min_length=1,
        max_length=1024,
        description="Dot-separated ltree path (e.g. 'chemistry.organic.chirality').",
    )

    @field_validator("path")
    @classmethod
    def _validate_ltree_path(cls, v: str) -> str:
        """Reject paths that Postgres ltree would fail to cast.

        Postgres requires each segment to match ``[A-Za-z0-9_]+`` and start
        with a letter; we additionally enforce lowercase here so every node
        has exactly one canonical representation in the graph.
        """
        if not _LTREE_PATH_RE.match(v):
            raise ValueError(
                "path must be a dot-separated ltree path of lowercase "
                "segments matching [a-z][a-z0-9_]* (e.g. 'chemistry.organic')."
            )
        return v


class PrerequisiteGap(BaseModel):
    """Ancestor concept whose mastery is below the prereq threshold."""

    concept_id: str
    label: str
    path: str
    mastery_score: float = Field(..., ge=0.0, le=1.0, description="Mastery score in [0, 1].")


# ── Flashcard DTOs (tl-y3v) ───────────────────────────────────────────────────


class FlashcardResponse(BaseModel):
    """A single flashcard returned by the listing / generation endpoints."""

    id: UUID
    front: str
    back: str
    concept_id: str | None
    source_session_id: UUID | None
    created_at: datetime
    last_reviewed_at: datetime | None
    due_at: datetime
    sm2_interval_days: int
    sm2_ease_factor: float
    sm2_repetitions: int


class GenerateFlashcardsRequest(BaseModel):
    """Request body for POST /v1/flashcards/generate.

    ``user_id`` is derived from the JWT (never the request body) so one
    student cannot seed cards into another student's deck.  The session ID
    selects the transcript that will be summarised into Q/A pairs.
    """

    session_id: UUID
    max_cards: int = Field(
        default=10,
        ge=1,
        le=30,
        description="Upper bound on Q/A pairs the extractor may return.",
    )


class GenerateFlashcardsResponse(BaseModel):
    """Result of POST /v1/flashcards/generate — the newly inserted cards."""

    session_id: UUID
    created_count: int
    cards: list[FlashcardResponse]


class FlashcardListResponse(BaseModel):
    """Result of GET /v1/flashcards."""

    items: list[FlashcardResponse]
    total: int


class FlashcardReviewRequest(BaseModel):
    """Request body for POST /v1/flashcards/{id}/review."""

    quality: int = Field(..., ge=0, le=5, description="SM-2 quality 0-5 (0=blackout, 5=perfect).")


class FlashcardReviewResponse(BaseModel):
    """Result of reviewing a flashcard — the updated SM-2 state + next due time."""

    id: UUID
    quality: int
    sm2_interval_days: int
    sm2_ease_factor: float
    sm2_repetitions: int
    last_reviewed_at: datetime
    due_at: datetime
