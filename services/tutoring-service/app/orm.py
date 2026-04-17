"""SQLAlchemy ORM models."""

from datetime import datetime, timezone
from uuid import uuid4

from sqlalchemy import (
    Boolean,
    DateTime,
    Float,
    ForeignKey,
    Index,
    Integer,
    String,
    Text,
    UniqueConstraint,
)
from sqlalchemy import (
    Enum as SAEnum,
)
from sqlalchemy.dialects.postgresql import UUID
from sqlalchemy.orm import Mapped, mapped_column, relationship

from .database import Base


def _now() -> datetime:
    return datetime.now(timezone.utc)


# ── Concept graph + mastery (shared with concept-dependency and SRS) ──────────


class Concept(Base):
    """A knowledge concept in a course, with ltree path for hierarchy."""

    __tablename__ = "concepts"

    id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    course_id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), nullable=False, index=True)
    name: Mapped[str] = mapped_column(String(255), nullable=False)
    description: Mapped[str] = mapped_column(Text, nullable=False, default="")
    path: Mapped[str] = mapped_column(Text, nullable=False)  # ltree stored as text via asyncpg
    difficulty: Mapped[float] = mapped_column(Float, default=0.5)  # 0.0=easy, 1.0=very hard

    prerequisites: Mapped[list["ConceptPrerequisite"]] = relationship(
        foreign_keys="ConceptPrerequisite.concept_id",
        back_populates="concept",
        lazy="selectin",
    )
    dependents: Mapped[list["ConceptPrerequisite"]] = relationship(
        foreign_keys="ConceptPrerequisite.prerequisite_id",
        back_populates="prerequisite",
        lazy="selectin",
    )


class ConceptPrerequisite(Base):
    """Directed edge: concept_id requires prerequisite_id, with optional weight."""

    __tablename__ = "concept_prerequisites"

    concept_id: Mapped[UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("concepts.id", ondelete="CASCADE"), primary_key=True
    )
    prerequisite_id: Mapped[UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("concepts.id", ondelete="CASCADE"), primary_key=True
    )
    weight: Mapped[float] = mapped_column(Float, default=1.0)

    concept: Mapped["Concept"] = relationship(
        foreign_keys=[concept_id], back_populates="prerequisites"
    )
    prerequisite: Mapped["Concept"] = relationship(
        foreign_keys=[prerequisite_id], back_populates="dependents"
    )


class StudentConceptMastery(Base):
    """Per-student, per-concept mastery state including SM-2 scheduling fields."""

    __tablename__ = "student_concept_mastery"
    __table_args__ = (
        # Composite index for review queue: filter by user, sort by next_review_at.
        # Powers: GET /reviews/queue and GET /reviews/stats (no full-scan per user).
        Index("ix_scm_user_next_review", "user_id", "next_review_at"),
    )

    user_id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), primary_key=True)
    concept_id: Mapped[UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("concepts.id", ondelete="CASCADE"), primary_key=True
    )
    mastery_score: Mapped[float] = mapped_column(Float, default=0.0)
    last_reviewed_at: Mapped[datetime | None] = mapped_column(
        DateTime(timezone=True), nullable=True
    )
    next_review_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    decay_rate: Mapped[float] = mapped_column(Float, default=0.1)
    review_count: Mapped[int] = mapped_column(Integer, default=0)  # total reviews ever

    # SM-2 scheduling state
    ease_factor: Mapped[float] = mapped_column(Float, default=2.5)
    interval_days: Mapped[int] = mapped_column(Integer, default=1)
    repetitions: Mapped[int] = mapped_column(Integer, default=0)

    concept: Mapped["Concept"] = relationship(lazy="selectin")
    review_records: Mapped[list["ReviewRecord"]] = relationship(
        primaryjoin="and_(StudentConceptMastery.user_id == ReviewRecord.user_id, "
        "StudentConceptMastery.concept_id == ReviewRecord.concept_id)",
        foreign_keys="[ReviewRecord.user_id, ReviewRecord.concept_id]",
        back_populates="mastery",
        order_by="ReviewRecord.reviewed_at",
        lazy="select",
        overlaps="mastery",
    )


class ReviewRecord(Base):
    """Audit log of individual review responses for a student + concept."""

    __tablename__ = "review_records"

    id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    user_id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), nullable=False, index=True)
    concept_id: Mapped[UUID] = mapped_column(
        UUID(as_uuid=True),
        ForeignKey("concepts.id", ondelete="CASCADE"),
        nullable=False,
        index=True,
    )
    quality: Mapped[int] = mapped_column(Integer, nullable=False)  # 0-5
    mastery_before: Mapped[float] = mapped_column(Float, nullable=False)
    mastery_after: Mapped[float] = mapped_column(Float, nullable=False)
    interval_after: Mapped[int] = mapped_column(Integer, nullable=False)  # days
    ease_after: Mapped[float] = mapped_column(Float, nullable=False)
    reviewed_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=_now)

    mastery: Mapped["StudentConceptMastery"] = relationship(
        foreign_keys=[user_id, concept_id],
        primaryjoin="and_(ReviewRecord.user_id == StudentConceptMastery.user_id, "
        "ReviewRecord.concept_id == StudentConceptMastery.concept_id)",
        back_populates="review_records",
        overlaps="review_records",
    )


# ── Chat sessions + interactions ──────────────────────────────────────────────


class Session(Base):
    """ORM model for the chat_sessions table."""

    __tablename__ = "chat_sessions"

    id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    user_id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), nullable=False, index=True)
    course_id: Mapped[UUID | None] = mapped_column(UUID(as_uuid=True), nullable=True)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=_now)
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=_now, onupdate=_now
    )

    interactions: Mapped[list["Interaction"]] = relationship(
        back_populates="session",
        order_by="Interaction.created_at",
        lazy="selectin",
    )


class Interaction(Base):
    """Mirrors the interactions table in the full schema (Phase 1 subset)."""

    __tablename__ = "interactions"

    id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    session_id: Mapped[UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("chat_sessions.id", ondelete="CASCADE"), index=True
    )
    user_id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), nullable=False)
    role: Mapped[str] = mapped_column(
        SAEnum("student", "tutor", name="interaction_role"), nullable=False
    )
    content: Mapped[str] = mapped_column(Text, nullable=False)
    response_time_ms: Mapped[int | None] = mapped_column(Integer, nullable=True)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=_now)

    session: Mapped["Session"] = relationship(back_populates="interactions")


class InteractionQuality(Base):
    """LLM judge scores for sampled tutor interactions — written nightly by eval CronJob.

    Each row records a Claude Haiku evaluation of one tutor response on three
    dimensions (1–5 scale each):
      - directness:  did the response directly address the student's question?
      - pace:        was it appropriately paced for the student's level?
      - grounding:   was it grounded in the source material?

    The composite judge_score is the average of the three dimension scores.
    """

    __tablename__ = "interaction_quality"
    __table_args__ = (
        UniqueConstraint("interaction_id", name="uq_interaction_quality_interaction"),
    )

    id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    interaction_id: Mapped[UUID] = mapped_column(
        UUID(as_uuid=True),
        ForeignKey("interactions.id", ondelete="CASCADE"),
        nullable=False,
        index=True,
    )
    judge_score: Mapped[int] = mapped_column(Integer, nullable=False)  # 1–5 composite
    judge_reasoning: Mapped[str] = mapped_column(Text, nullable=False)
    score_directness: Mapped[int | None] = mapped_column(Integer, nullable=True)  # 1–5
    score_pace: Mapped[int | None] = mapped_column(Integer, nullable=True)  # 1–5
    score_grounding: Mapped[int | None] = mapped_column(Integer, nullable=True)  # 1–5
    judged_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=_now)
    judge_model: Mapped[str] = mapped_column(
        String(64), nullable=False, default="claude-haiku-4-5-20251001"
    )


# ── Student Knowledge Model — learning profiles + misconceptions ───────────────


class LearningProfile(Base):
    """Per-student Felder-Silverman learning-style dials stored in Postgres.

    Four bipolar dimensions in [-1.0, 1.0]:
      active_reflective:  -1=active,     +1=reflective
      sensing_intuitive:  -1=sensing,    +1=intuitive
      visual_verbal:      -1=visual,     +1=verbal
      sequential_global:  -1=sequential, +1=global
    """

    __tablename__ = "learning_profiles"

    user_id: Mapped[UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("users.id", ondelete="CASCADE"), primary_key=True
    )
    active_reflective: Mapped[float] = mapped_column(Float, default=0.0)
    sensing_intuitive: Mapped[float] = mapped_column(Float, default=0.0)
    visual_verbal: Mapped[float] = mapped_column(Float, default=0.0)
    sequential_global: Mapped[float] = mapped_column(Float, default=0.0)
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=_now, onupdate=_now
    )


class ExplanationPreference(Base):
    """Log of which explanation types helped a student understand a concept.

    Accumulated from interaction signals; used by the tutor to personalise
    future explanations for the same concept.
    """

    __tablename__ = "explanation_preferences"

    id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    user_id: Mapped[UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("users.id", ondelete="CASCADE"), nullable=False, index=True
    )
    concept_id: Mapped[UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("concepts.id", ondelete="CASCADE"), nullable=False
    )
    explanation_type: Mapped[str] = mapped_column(String(50), nullable=False)
    helpful: Mapped[bool] = mapped_column(Boolean, nullable=False)
    recorded_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=_now)


class Misconception(Base):
    """Tracked student error with recency-weighted confidence.

    confidence decays over time in application logic (exponential decay).
    resolved=True dismisses the entry from the active misconceptions list.
    """

    __tablename__ = "misconceptions"

    id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    user_id: Mapped[UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("users.id", ondelete="CASCADE"), nullable=False, index=True
    )
    concept_id: Mapped[UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("concepts.id", ondelete="CASCADE"), nullable=False
    )
    description: Mapped[str] = mapped_column(Text, nullable=False)
    confidence: Mapped[float] = mapped_column(Float, default=1.0)
    recorded_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=_now)
    last_seen_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=_now)
    resolved: Mapped[bool] = mapped_column(Boolean, default=False)


# ── Global concept knowledge graph — Postgres ltree (tl-mhd) ──────────────────


class ConceptGraphNode(Base):
    """Single node in the global ltree-backed concept graph.

    The ``path`` column is stored using Postgres' ``ltree`` type — declared
    here as ``Text`` because SQLAlchemy lacks a first-class ltree mapping.
    Ancestor / descendant queries use raw ``text()`` statements from
    :mod:`app.concept_graph`.
    """

    __tablename__ = "concept_graph"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    concept_id: Mapped[str] = mapped_column(Text, nullable=False, unique=True)
    label: Mapped[str] = mapped_column(Text, nullable=False)
    subject: Mapped[str] = mapped_column(Text, nullable=False, index=True)
    path: Mapped[str] = mapped_column(Text, nullable=False)
