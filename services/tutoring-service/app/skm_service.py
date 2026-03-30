"""Business logic for the Student Knowledge Model.

Implements:
  - Exponential decay model (Ebbinghaus-inspired forgetting curve)
  - Confidence scoring based on review count and consistency
  - Spaced repetition scheduling for next_review_at
  - Prerequisite gap detection
"""
import math
from datetime import datetime, timedelta, timezone
from uuid import UUID

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from .skm_orm import Concept, ConceptPrerequisite, StudentConceptMastery

# ── Constants ────────────────────────────────────────────────────────────────

# Base decay half-life in days — mastery drops to 50% after this many days
# without review. Actual half-life scales with review_count.
BASE_HALF_LIFE_DAYS = 7.0

# Each review extends the half-life by this multiplier
HALF_LIFE_GROWTH_FACTOR = 1.5

# Minimum effective mastery score (floor)
MIN_MASTERY = 0.0

# Mastery threshold below which a prerequisite is considered a "gap"
PREREQ_GAP_THRESHOLD = 0.4

# Confidence asymptote approach rate
CONFIDENCE_K = 0.3


def _now() -> datetime:
    return datetime.now(timezone.utc)


# ── Decay Model ──────────────────────────────────────────────────────────────

def compute_half_life(review_count: int, base_decay_rate: float) -> float:
    """Compute the effective half-life in days.

    More reviews → longer half-life (slower forgetting).
    base_decay_rate adjusts per-student per-concept difficulty.
    """
    base = BASE_HALF_LIFE_DAYS / max(base_decay_rate * 20, 0.01)
    return base * (HALF_LIFE_GROWTH_FACTOR ** review_count)


def apply_decay(mastery_score: float, hours_elapsed: float, half_life_days: float) -> float:
    """Apply exponential decay to a mastery score.

    Uses the formula: m(t) = m₀ × 2^(-t / half_life)
    """
    if hours_elapsed <= 0 or half_life_days <= 0:
        return mastery_score
    half_life_hours = half_life_days * 24.0
    decayed = mastery_score * (2.0 ** (-hours_elapsed / half_life_hours))
    return max(decayed, MIN_MASTERY)


def compute_effective_mastery(record: StudentConceptMastery) -> float:
    """Compute the current effective mastery after time-based decay."""
    if record.last_reviewed_at is None:
        return record.mastery_score
    elapsed = (_now() - record.last_reviewed_at).total_seconds() / 3600.0
    half_life = compute_half_life(record.review_count, record.decay_rate)
    return apply_decay(record.mastery_score, elapsed, half_life)


# ── Confidence Scoring ───────────────────────────────────────────────────────

def compute_confidence(review_count: int) -> float:
    """Confidence in the mastery estimate, 0-1.

    Uses 1 - e^(-k * n) — approaches 1.0 as review count grows.
    With k=0.3: 1 review → 0.26, 3 → 0.59, 5 → 0.78, 10 → 0.95
    """
    return 1.0 - math.exp(-CONFIDENCE_K * review_count)


# ── Spaced Repetition Scheduling ─────────────────────────────────────────────

def compute_next_review(
    mastery_score: float, review_count: int, decay_rate: float
) -> datetime:
    """Schedule the next review using spaced repetition.

    Higher mastery + more reviews → longer interval before next review.
    The interval is set so that effective mastery would decay to ~70% of
    current mastery by the review time.
    """
    half_life = compute_half_life(review_count, decay_rate)
    # Solve: 0.7 = 2^(-t / half_life) → t = half_life × log2(1/0.7)
    target_retention = 0.7
    interval_days = half_life * math.log2(1.0 / target_retention)
    # Clamp interval to [1 hour, 180 days]
    interval_hours = max(1.0, min(interval_days * 24.0, 180 * 24.0))
    return _now() + timedelta(hours=interval_hours)


# ── Database Operations ──────────────────────────────────────────────────────

async def get_or_create_mastery(
    db: AsyncSession, user_id: UUID, concept_id: UUID
) -> StudentConceptMastery:
    """Get existing mastery record or create a new one."""
    stmt = select(StudentConceptMastery).where(
        StudentConceptMastery.user_id == user_id,
        StudentConceptMastery.concept_id == concept_id,
    )
    result = await db.execute(stmt)
    record = result.scalar_one_or_none()
    if record is not None:
        return record

    record = StudentConceptMastery(
        user_id=user_id,
        concept_id=concept_id,
        mastery_score=0.0,
        confidence=0.0,
        decay_rate=0.05,
        review_count=0,
    )
    db.add(record)
    await db.flush()
    return record


async def record_mastery_observation(
    db: AsyncSession,
    user_id: UUID,
    concept_id: UUID,
    score: float,
    weight: float = 1.0,
) -> StudentConceptMastery:
    """Record a mastery observation and update the student's mastery state.

    Uses exponential moving average weighted by observation weight:
        new_mastery = α × observed + (1 - α) × old_mastery
    where α = weight / (review_count + weight)

    This gives more weight to early observations (when we know little)
    and smooths out noise as more data accumulates.
    """
    record = await get_or_create_mastery(db, user_id, concept_id)

    # Exponential moving average
    alpha = weight / (record.review_count + weight)
    old_effective = compute_effective_mastery(record)
    new_mastery = alpha * score + (1 - alpha) * old_effective

    # Clamp to [0, 1]
    record.mastery_score = max(0.0, min(1.0, new_mastery))
    record.review_count += 1
    record.confidence = compute_confidence(record.review_count)
    record.last_reviewed_at = _now()
    record.next_review_at = compute_next_review(
        record.mastery_score, record.review_count, record.decay_rate
    )

    # Adjust decay rate: consistent high performance → slower decay
    if score >= 0.8 and record.review_count > 2:
        record.decay_rate = max(0.01, record.decay_rate * 0.95)
    elif score < 0.4:
        record.decay_rate = min(0.2, record.decay_rate * 1.1)

    await db.flush()
    return record


async def get_student_mastery_for_course(
    db: AsyncSession, user_id: UUID, course_id: UUID
) -> list[tuple[StudentConceptMastery, Concept]]:
    """Get all mastery records for a student in a course, joined with concept info."""
    stmt = (
        select(StudentConceptMastery, Concept)
        .join(Concept, StudentConceptMastery.concept_id == Concept.id)
        .where(
            StudentConceptMastery.user_id == user_id,
            Concept.course_id == course_id,
        )
        .order_by(Concept.name)
    )
    result = await db.execute(stmt)
    return list(result.tuples().all())


async def get_student_mastery_single(
    db: AsyncSession, user_id: UUID, concept_id: UUID
) -> tuple[StudentConceptMastery, Concept] | None:
    """Get a single mastery record with concept info."""
    stmt = (
        select(StudentConceptMastery, Concept)
        .join(Concept, StudentConceptMastery.concept_id == Concept.id)
        .where(
            StudentConceptMastery.user_id == user_id,
            StudentConceptMastery.concept_id == concept_id,
        )
    )
    result = await db.execute(stmt)
    row = result.tuples().first()
    return row


async def detect_prerequisite_gaps(
    db: AsyncSession, user_id: UUID, concept_id: UUID
) -> list[dict]:
    """Find prerequisite concepts where the student's mastery is below threshold.

    Returns list of dicts with prerequisite concept info and effective mastery.
    """
    # Get prerequisites for the target concept
    stmt = (
        select(ConceptPrerequisite, Concept)
        .join(Concept, ConceptPrerequisite.prerequisite_id == Concept.id)
        .where(ConceptPrerequisite.concept_id == concept_id)
    )
    result = await db.execute(stmt)
    prereqs = list(result.tuples().all())

    # Get the target concept name
    target_stmt = select(Concept).where(Concept.id == concept_id)
    target_result = await db.execute(target_stmt)
    target_concept = target_result.scalar_one_or_none()

    gaps = []
    for prereq_edge, prereq_concept in prereqs:
        mastery_row = await get_student_mastery_single(db, user_id, prereq_concept.id)
        if mastery_row is None:
            # No mastery record means never studied → gap
            effective = 0.0
        else:
            effective = compute_effective_mastery(mastery_row[0])

        if effective < PREREQ_GAP_THRESHOLD:
            gaps.append({
                "concept_id": prereq_concept.id,
                "concept_name": prereq_concept.name,
                "effective_mastery": round(effective, 4),
                "required_by": concept_id,
                "required_by_name": target_concept.name if target_concept else "Unknown",
            })

    return gaps


async def get_concepts_for_course(
    db: AsyncSession, course_id: UUID
) -> list[Concept]:
    """List all concepts in a course."""
    stmt = select(Concept).where(Concept.course_id == course_id).order_by(Concept.name)
    result = await db.execute(stmt)
    return list(result.scalars().all())


async def create_concept(
    db: AsyncSession, course_id: UUID, name: str, description: str | None = None
) -> Concept:
    """Create a new concept in a course."""
    concept = Concept(course_id=course_id, name=name, description=description)
    db.add(concept)
    await db.flush()
    return concept


async def add_prerequisite(
    db: AsyncSession, concept_id: UUID, prerequisite_id: UUID, weight: float = 1.0
) -> ConceptPrerequisite:
    """Add a prerequisite relationship between two concepts."""
    edge = ConceptPrerequisite(
        concept_id=concept_id, prerequisite_id=prerequisite_id, weight=weight
    )
    db.add(edge)
    await db.flush()
    return edge


async def get_prerequisites(
    db: AsyncSession, concept_id: UUID
) -> list[tuple[ConceptPrerequisite, Concept]]:
    """Get all prerequisites for a concept."""
    stmt = (
        select(ConceptPrerequisite, Concept)
        .join(Concept, ConceptPrerequisite.prerequisite_id == Concept.id)
        .where(ConceptPrerequisite.concept_id == concept_id)
    )
    result = await db.execute(stmt)
    return list(result.tuples().all())
