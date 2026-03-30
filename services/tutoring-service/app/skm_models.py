"""Pydantic request/response DTOs for the Student Knowledge Model."""
from datetime import datetime
from uuid import UUID

from pydantic import BaseModel, Field


# ── Concept DTOs ─────────────────────────────────────────────────────────────

class ConceptCreate(BaseModel):
    course_id: UUID
    name: str = Field(..., max_length=255)
    description: str | None = None


class ConceptResponse(BaseModel):
    id: UUID
    course_id: UUID
    name: str
    description: str | None
    created_at: datetime


class PrerequisiteCreate(BaseModel):
    concept_id: UUID
    prerequisite_id: UUID
    weight: float = Field(default=1.0, ge=0.0, le=1.0)


class PrerequisiteResponse(BaseModel):
    concept_id: UUID
    prerequisite_id: UUID
    weight: float


# ── Mastery DTOs ─────────────────────────────────────────────────────────────

class MasteryRecordRequest(BaseModel):
    """Record a mastery observation (e.g. quiz result, interaction quality)."""
    concept_id: UUID
    score: float = Field(..., ge=0.0, le=1.0, description="Observed performance 0-1")
    weight: float = Field(
        default=1.0, ge=0.0, le=5.0,
        description="Relative importance of this observation (default 1.0)",
    )


class MasteryResponse(BaseModel):
    user_id: UUID
    concept_id: UUID
    concept_name: str
    mastery_score: float
    confidence: float
    decay_rate: float
    review_count: int
    effective_mastery: float = Field(
        description="Mastery after applying time-based decay"
    )
    last_reviewed_at: datetime | None
    next_review_at: datetime | None


class MasterySummary(BaseModel):
    """Aggregated mastery summary for a student in a course."""
    user_id: UUID
    course_id: UUID
    concept_count: int
    average_mastery: float
    average_effective_mastery: float
    weak_concepts: list[MasteryResponse] = Field(
        description="Concepts with effective mastery below 0.4"
    )
    strong_concepts: list[MasteryResponse] = Field(
        description="Concepts with effective mastery above 0.7"
    )


class PrerequisiteGap(BaseModel):
    """A prerequisite the student hasn't mastered."""
    concept_id: UUID
    concept_name: str
    effective_mastery: float
    required_by: UUID
    required_by_name: str
