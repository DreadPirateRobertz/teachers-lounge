"""SQLAlchemy ORM models."""
from datetime import datetime, timezone
from uuid import uuid4

from sqlalchemy import DateTime, Enum as SAEnum, Float, ForeignKey, Integer, String, Text
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

    concept: Mapped["Concept"] = relationship(foreign_keys=[concept_id], back_populates="prerequisites")
    prerequisite: Mapped["Concept"] = relationship(foreign_keys=[prerequisite_id], back_populates="dependents")


class StudentConceptMastery(Base):
    """Per-student, per-concept mastery state including SM-2 scheduling fields."""

    __tablename__ = "student_concept_mastery"

    user_id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), primary_key=True)
    concept_id: Mapped[UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("concepts.id", ondelete="CASCADE"), primary_key=True
    )
    mastery_score: Mapped[float] = mapped_column(Float, default=0.0)
    last_reviewed_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    next_review_at: Mapped[datetime | None] = mapped_column(DateTime(timezone=True), nullable=True)
    decay_rate: Mapped[float] = mapped_column(Float, default=0.1)
    review_count: Mapped[int] = mapped_column(Integer, default=0)  # total reviews ever

    # SM-2 scheduling state
    ease_factor: Mapped[float] = mapped_column(Float, default=2.5)
    interval_days: Mapped[int] = mapped_column(Integer, default=1)
    repetitions: Mapped[int] = mapped_column(Integer, default=0)

    concept: Mapped["Concept"] = relationship(lazy="selectin")
    review_records: Mapped[list["ReviewRecord"]] = relationship(
        back_populates="mastery",
        order_by="ReviewRecord.reviewed_at",
        lazy="select",
    )


class ReviewRecord(Base):
    """Audit log of individual review responses for a student + concept."""

    __tablename__ = "review_records"

    id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    user_id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), nullable=False, index=True)
    concept_id: Mapped[UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("concepts.id", ondelete="CASCADE"), nullable=False, index=True
    )
    quality: Mapped[int] = mapped_column(Integer, nullable=False)          # 0-5
    mastery_before: Mapped[float] = mapped_column(Float, nullable=False)
    mastery_after: Mapped[float] = mapped_column(Float, nullable=False)
    interval_after: Mapped[int] = mapped_column(Integer, nullable=False)   # days
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
    __tablename__ = "chat_sessions"

    id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    user_id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), nullable=False, index=True)
    course_id: Mapped[UUID | None] = mapped_column(UUID(as_uuid=True), nullable=True)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=_now)
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=_now, onupdate=_now)

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
    role: Mapped[str] = mapped_column(SAEnum("student", "tutor", name="interaction_role"), nullable=False)
    content: Mapped[str] = mapped_column(Text, nullable=False)
    response_time_ms: Mapped[int | None] = mapped_column(Integer, nullable=True)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=_now)

    session: Mapped["Session"] = relationship(back_populates="interactions")


# ── Flashcard decks + cards ────────────────────────────────────────────────────

class FlashcardDeck(Base):
    """A named collection of flashcards owned by a student."""

    __tablename__ = "flashcard_decks"

    id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    user_id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), nullable=False, index=True)
    name: Mapped[str] = mapped_column(String(255), nullable=False)
    description: Mapped[str] = mapped_column(Text, nullable=False, default="")
    # Optional: deck was created from a specific chat session
    session_id: Mapped[UUID | None] = mapped_column(UUID(as_uuid=True), nullable=True)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=_now)
    updated_at: Mapped[datetime] = mapped_column(
        DateTime(timezone=True), default=_now, onupdate=_now
    )

    cards: Mapped[list["Flashcard"]] = relationship(
        back_populates="deck",
        cascade="all, delete-orphan",
        order_by="Flashcard.created_at",
        lazy="selectin",
    )


class Flashcard(Base):
    """A single question/answer flashcard belonging to a deck."""

    __tablename__ = "flashcards"

    id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), primary_key=True, default=uuid4)
    deck_id: Mapped[UUID] = mapped_column(
        UUID(as_uuid=True), ForeignKey("flashcard_decks.id", ondelete="CASCADE"), nullable=False, index=True
    )
    user_id: Mapped[UUID] = mapped_column(UUID(as_uuid=True), nullable=False)
    front: Mapped[str] = mapped_column(Text, nullable=False)   # question / term
    back: Mapped[str] = mapped_column(Text, nullable=False)    # answer / definition
    # Optional: which tutor interaction generated this card
    source_interaction_id: Mapped[UUID | None] = mapped_column(UUID(as_uuid=True), nullable=True)
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), default=_now)

    deck: Mapped["FlashcardDeck"] = relationship(back_populates="cards")
