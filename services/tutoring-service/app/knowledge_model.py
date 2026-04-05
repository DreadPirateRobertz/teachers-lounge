"""Student Knowledge Model — learning profiles, misconceptions, proactive SRS prompts.

Core async functions for the SKM adaptive layer:

  Learning profile (Felder-Silverman dials):
    get_or_create_learning_profile  — fetch or insert a LearningProfile row
    update_learning_profile_dials   — merge new dial values and commit
    get_dials                       — return the current dials as a plain dict

  Explanation preferences:
    log_explanation_preference      — record whether an explanation type helped
    get_explanation_preferences     — retrieve preference history for a concept

  Misconceptions:
    log_misconception               — record a detected student error
    get_active_misconceptions       — list unresolved errors with recency weights
    resolve_misconception           — mark an error as corrected

  Proactive SRS:
    get_due_review_prompt           — generate a nudge string when reviews are due
"""
from __future__ import annotations

import math
from datetime import datetime, timezone
from uuid import UUID

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from .orm import (
    ExplanationPreference,
    LearningProfile,
    Misconception,
    StudentConceptMastery,
)
from .style_detector import DEFAULT_DIALS

# Days over which misconception confidence decays to ~37 % (1/e).
_MISCONCEPTION_DECAY_DAYS: float = 30.0


# ── Learning profile ──────────────────────────────────────────────────────────

async def get_or_create_learning_profile(
    db: AsyncSession,
    user_id: UUID,
) -> LearningProfile:
    """Fetch the student's LearningProfile row, creating it if absent.

    Args:
        db:      Async SQLAlchemy session.
        user_id: UUID of the student.

    Returns:
        The existing or newly-created LearningProfile row.
    """
    result = await db.execute(
        select(LearningProfile).where(LearningProfile.user_id == user_id)
    )
    profile = result.scalar_one_or_none()
    if profile is None:
        profile = LearningProfile(
            user_id=user_id,
            active_reflective=0.0,
            sensing_intuitive=0.0,
            visual_verbal=0.0,
            sequential_global=0.0,
        )
        db.add(profile)
        await db.flush()
    return profile


async def update_learning_profile_dials(
    db: AsyncSession,
    user_id: UUID,
    dials: dict[str, float],
) -> LearningProfile:
    """Merge new Felder-Silverman dial values into the student's profile.

    Only keys present in ``dials`` are updated; other dimensions are unchanged.
    Creates the profile row if it does not yet exist.

    Args:
        db:      Async SQLAlchemy session.
        user_id: UUID of the student.
        dials:   Partial or full dict of dimension → value in [-1, 1].

    Returns:
        The updated LearningProfile row (not yet committed — caller may batch).
    """
    profile = await get_or_create_learning_profile(db, user_id)
    for dimension, value in dials.items():
        if hasattr(profile, dimension):
            setattr(profile, dimension, float(value))
    profile.updated_at = datetime.now(timezone.utc)
    await db.commit()
    return profile


async def get_dials(
    db: AsyncSession,
    user_id: UUID,
) -> dict[str, float]:
    """Return the student's current Felder-Silverman dials as a plain dict.

    Falls back to DEFAULT_DIALS (all zeros) when no profile row exists,
    without creating one — callers may check the result before writing.

    Args:
        db:      Async SQLAlchemy session.
        user_id: UUID of the student.

    Returns:
        Dict mapping each of the four dimension names to a float in [-1, 1].
    """
    result = await db.execute(
        select(LearningProfile).where(LearningProfile.user_id == user_id)
    )
    profile = result.scalar_one_or_none()
    if profile is None:
        return dict(DEFAULT_DIALS)
    return {
        "active_reflective": profile.active_reflective,
        "sensing_intuitive": profile.sensing_intuitive,
        "visual_verbal": profile.visual_verbal,
        "sequential_global": profile.sequential_global,
    }


# ── Explanation preferences ───────────────────────────────────────────────────

async def log_explanation_preference(
    db: AsyncSession,
    user_id: UUID,
    concept_id: UUID,
    explanation_type: str,
    helpful: bool,
) -> ExplanationPreference:
    """Record whether an explanation type helped a student understand a concept.

    Args:
        db:               Async SQLAlchemy session.
        user_id:          UUID of the student.
        concept_id:       UUID of the concept being explained.
        explanation_type: Category string, e.g. 'visual', 'example', 'derivation'.
        helpful:          True if the explanation improved understanding.

    Returns:
        The newly-created ExplanationPreference row.
    """
    pref = ExplanationPreference(
        user_id=user_id,
        concept_id=concept_id,
        explanation_type=explanation_type,
        helpful=helpful,
    )
    db.add(pref)
    await db.commit()
    return pref


async def get_explanation_preferences(
    db: AsyncSession,
    user_id: UUID,
    concept_id: UUID,
) -> list[dict]:
    """Retrieve explanation preference history for a student and concept.

    Args:
        db:         Async SQLAlchemy session.
        user_id:    UUID of the student.
        concept_id: UUID of the concept.

    Returns:
        List of dicts with keys: explanation_type, helpful, recorded_at.
        Most recent entries first.
    """
    result = await db.execute(
        select(ExplanationPreference)
        .where(
            ExplanationPreference.user_id == user_id,
            ExplanationPreference.concept_id == concept_id,
        )
        .order_by(ExplanationPreference.recorded_at.desc())
    )
    rows = result.scalars().all()
    return [
        {
            "explanation_type": r.explanation_type,
            "helpful": r.helpful,
            "recorded_at": r.recorded_at,
        }
        for r in rows
    ]


# ── Misconceptions ────────────────────────────────────────────────────────────

async def log_misconception(
    db: AsyncSession,
    user_id: UUID,
    concept_id: UUID,
    description: str,
) -> Misconception:
    """Record a detected student misconception with full initial confidence.

    Args:
        db:          Async SQLAlchemy session.
        user_id:     UUID of the student.
        concept_id:  UUID of the concept related to the error.
        description: Human-readable description of the misconception.

    Returns:
        The newly-created Misconception row.
    """
    now = datetime.now(timezone.utc)
    m = Misconception(
        user_id=user_id,
        concept_id=concept_id,
        description=description,
        confidence=1.0,
        recorded_at=now,
        last_seen_at=now,
        resolved=False,
    )
    db.add(m)
    await db.commit()
    return m


async def get_active_misconceptions(
    db: AsyncSession,
    user_id: UUID,
    decay_days: float = _MISCONCEPTION_DECAY_DAYS,
) -> list[dict]:
    """Return unresolved misconceptions for a student with recency weighting.

    Recency weight is computed as R = e^(-elapsed_days / decay_days), giving
    each misconception a weight in (0, 1] that decreases exponentially with age.

    Args:
        db:         Async SQLAlchemy session.
        user_id:    UUID of the student.
        decay_days: Half-life decay constant in days (default 30 ≈ 1/e in 30 days).

    Returns:
        List of dicts ordered by recency_weight descending, each containing:
        id, concept_id, description, confidence, recorded_at, recency_weight.
    """
    result = await db.execute(
        select(Misconception)
        .where(
            Misconception.user_id == user_id,
            Misconception.resolved.is_(False),
        )
        .order_by(Misconception.last_seen_at.desc())
    )
    rows = result.scalars().all()
    now = datetime.now(timezone.utc)

    entries = []
    for m in rows:
        elapsed = (now - m.last_seen_at).total_seconds() / 86400
        weight = math.exp(-elapsed / max(decay_days, 0.001))
        entries.append({
            "id": m.id,
            "concept_id": m.concept_id,
            "description": m.description,
            "confidence": m.confidence,
            "recorded_at": m.recorded_at,
            "recency_weight": round(weight, 4),
        })

    entries.sort(key=lambda e: e["recency_weight"], reverse=True)
    return entries


async def resolve_misconception(
    db: AsyncSession,
    misconception_id: UUID,
    user_id: UUID,
) -> bool:
    """Mark a misconception as resolved (no longer surfaced in active list).

    Args:
        db:               Async SQLAlchemy session.
        misconception_id: UUID of the misconception to resolve.
        user_id:          UUID of the student (ownership check).

    Returns:
        True if the misconception was found and marked resolved; False otherwise.
    """
    result = await db.execute(
        select(Misconception).where(
            Misconception.id == misconception_id,
            Misconception.user_id == user_id,
        )
    )
    m = result.scalar_one_or_none()
    if m is None:
        return False
    m.resolved = True
    await db.commit()
    return True


# ── Proactive SRS prompts ─────────────────────────────────────────────────────

async def get_due_review_prompt(
    db: AsyncSession,
    user_id: UUID,
    limit: int = 3,
) -> str | None:
    """Generate a nudge string listing concepts due for spaced-repetition review.

    Called during the chat stream post-processing phase.  When at least one
    concept is overdue the returned string can be appended to the tutor response
    or emitted as a ``review_reminder`` SSE event.

    Args:
        db:      Async SQLAlchemy session.
        user_id: UUID of the student.
        limit:   Maximum number of concept names to include in the prompt.

    Returns:
        A human-readable nudge string if any concepts are due, otherwise None.
    """
    now = datetime.now(timezone.utc)
    result = await db.execute(
        select(StudentConceptMastery)
        .where(
            StudentConceptMastery.user_id == user_id,
            StudentConceptMastery.next_review_at <= now,
        )
        .order_by(StudentConceptMastery.next_review_at.asc())
        .limit(limit)
    )
    due_rows = result.scalars().all()

    if not due_rows:
        return None

    names = [
        row.concept.name if row.concept else str(row.concept_id)
        for row in due_rows
    ]
    if len(names) == 1:
        concepts_str = names[0]
    else:
        concepts_str = ", ".join(names[:-1]) + f" and {names[-1]}"

    return (
        f"📚 Review reminder: you have {len(due_rows)} concept"
        f"{'s' if len(due_rows) > 1 else ''} due for review "
        f"({concepts_str}). Visit the review queue when you're ready."
    )
