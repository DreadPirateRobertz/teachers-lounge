"""Spaced repetition scheduler — SM-2 variant on global concept slugs (tl-5wz).

Exposes two JWT-protected endpoints backed by ``concept_review_schedule``:

* ``POST /spaced-repetition/review-result`` — record a review response and
  advance SM-2 state.
* ``GET  /spaced-repetition/due``          — list concepts currently due.

The current-time source is injected via the ``get_clock`` dependency so tests
can freeze time; see :mod:`app.clock`.
"""

from __future__ import annotations

from datetime import datetime, timedelta

from fastapi import APIRouter, Depends
from sqlalchemy import func, select
from sqlalchemy.ext.asyncio import AsyncSession

from .auth import JWTClaims, require_auth
from .clock import Clock, get_clock
from .database import get_db
from .models import (
    SpacedRepetitionDueItem,
    SpacedRepetitionDueResponse,
    SpacedRepetitionReviewRequest,
    SpacedRepetitionReviewResponse,
)
from .orm import ConceptReviewSchedule
from .srs import sm2_update

router = APIRouter(prefix="/spaced-repetition", tags=["spaced-repetition"])


async def _get_or_create_schedule(
    db: AsyncSession,
    user_id,
    concept_id: str,
) -> ConceptReviewSchedule:
    """Fetch an existing schedule row or create a fresh SM-2 default."""
    result = await db.execute(
        select(ConceptReviewSchedule).where(
            ConceptReviewSchedule.user_id == user_id,
            ConceptReviewSchedule.concept_id == concept_id,
        )
    )
    row = result.scalar_one_or_none()
    if row is None:
        row = ConceptReviewSchedule(
            user_id=user_id,
            concept_id=concept_id,
            ease_factor=2.5,
            interval_days=1,
            repetitions=0,
            last_reviewed_at=None,
            due_at=None,
        )
        db.add(row)
        await db.flush()
    return row


@router.post("/review-result", response_model=SpacedRepetitionReviewResponse)
async def record_review_result(
    body: SpacedRepetitionReviewRequest,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
    clock: Clock = Depends(get_clock),
):
    """Record a review response and advance the SM-2 schedule.

    Quality below 3 resets the repetition counter so the concept reappears
    the following day; passing quality extends the interval by the updated
    ease factor per SM-2.
    """
    schedule = await _get_or_create_schedule(db, user.user_id, body.concept_id)

    new_interval, new_ef, new_reps = sm2_update(
        quality=body.quality,
        ease_factor=schedule.ease_factor,
        interval_days=schedule.interval_days,
        repetitions=schedule.repetitions,
    )
    now = clock()
    next_due = now + timedelta(days=new_interval)

    schedule.ease_factor = new_ef
    schedule.interval_days = new_interval
    schedule.repetitions = new_reps
    schedule.last_reviewed_at = now
    schedule.due_at = next_due

    await db.commit()

    return SpacedRepetitionReviewResponse(
        concept_id=body.concept_id,
        quality=body.quality,
        ease_factor=new_ef,
        interval_days=new_interval,
        repetitions=new_reps,
        last_reviewed_at=now,
        due_at=next_due,
    )


@router.get("/due", response_model=SpacedRepetitionDueResponse)
async def list_due(
    limit: int = 20,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
    clock: Clock = Depends(get_clock),
):
    """List concepts currently due for review, most overdue first.

    A schedule is "due" when ``due_at`` is null (never reviewed) or has
    passed the injected clock's current value. Results are ordered by
    ``due_at`` ascending so the most overdue items appear first.
    """
    now = clock()

    # Composite index ix_crs_user_due covers both the user_id filter and the
    # due_at ordering — the ORDER BY is push-downable without a sort.
    result = await db.execute(
        select(ConceptReviewSchedule)
        .where(
            ConceptReviewSchedule.user_id == user.user_id,
            (ConceptReviewSchedule.due_at.is_(None))
            | (ConceptReviewSchedule.due_at <= now),
        )
        .order_by(ConceptReviewSchedule.due_at.asc().nullsfirst())
        .limit(limit)
    )
    rows = list(result.scalars().all())

    total_result = await db.execute(
        select(func.count())
        .select_from(ConceptReviewSchedule)
        .where(
            ConceptReviewSchedule.user_id == user.user_id,
            (ConceptReviewSchedule.due_at.is_(None))
            | (ConceptReviewSchedule.due_at <= now),
        )
    )
    total_due = total_result.scalar_one()

    items = [
        SpacedRepetitionDueItem(
            concept_id=row.concept_id,
            ease_factor=row.ease_factor,
            interval_days=row.interval_days,
            repetitions=row.repetitions,
            last_reviewed_at=row.last_reviewed_at,
            due_at=row.due_at,
            is_overdue=_is_overdue(row.due_at, now),
        )
        for row in rows
    ]
    return SpacedRepetitionDueResponse(items=items, total_due=total_due)


def _is_overdue(due_at: datetime | None, now: datetime) -> bool:
    """Return True when ``due_at`` is strictly in the past relative to ``now``."""
    return due_at is not None and due_at < now
