"""Spaced repetition review queue API — JWT-protected."""

from __future__ import annotations

from datetime import datetime, timedelta, timezone
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy import func, select
from sqlalchemy.ext.asyncio import AsyncSession

from .auth import JWTClaims, require_auth
from .database import get_db
from .models import (
    AnswerRequest,
    AnswerResponse,
    ReviewQueueItem,
    ReviewQueueResponse,
    ReviewStatsResponse,
)
from .orm import Concept, ReviewRecord, StudentConceptMastery
from .srs import mastery_after_review, next_review_time, sm2_update

router = APIRouter(prefix="/reviews", tags=["reviews"])


# ── Helpers ───────────────────────────────────────────────────────────────────


async def _get_or_create_mastery(
    db: AsyncSession,
    user_id: UUID,
    concept_id: UUID,
) -> StudentConceptMastery:
    result = await db.execute(
        select(StudentConceptMastery).where(
            StudentConceptMastery.user_id == user_id,
            StudentConceptMastery.concept_id == concept_id,
        )
    )
    row = result.scalar_one_or_none()
    if row is None:
        row = StudentConceptMastery(
            user_id=user_id,
            concept_id=concept_id,
            mastery_score=0.0,
            decay_rate=0.1,
            review_count=0,
            ease_factor=2.5,
            interval_days=1,
            repetitions=0,
        )
        db.add(row)
        await db.flush()
    return row


# ── Endpoints ─────────────────────────────────────────────────────────────────


@router.get("/queue", response_model=ReviewQueueResponse)
async def get_review_queue(
    limit: int = 20,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Return concepts due for review, ordered by urgency (most overdue first)."""
    now = datetime.now(timezone.utc)
    week_later = now + timedelta(days=7)

    # Due: next_review_at IS NULL (never reviewed) or overdue.
    # Uses ix_scm_user_next_review composite index — avoids full user-row scan.
    due_result = await db.execute(
        select(StudentConceptMastery)
        .where(
            StudentConceptMastery.user_id == user.user_id,
            (StudentConceptMastery.next_review_at == None)  # noqa: E711
            | (StudentConceptMastery.next_review_at <= now),
        )
        .order_by(StudentConceptMastery.next_review_at.asc().nullsfirst())
        .limit(limit)
    )
    due = list(due_result.scalars().all())

    # Upcoming: due within the next 7 days (for the "coming up" preview).
    upcoming_result = await db.execute(
        select(func.count())
        .select_from(StudentConceptMastery)
        .where(
            StudentConceptMastery.user_id == user.user_id,
            StudentConceptMastery.next_review_at > now,
            StudentConceptMastery.next_review_at <= week_later,
        )
    )
    upcoming_count = upcoming_result.scalar_one()

    # Total due count (may exceed `limit` — needed for the queue summary).
    total_due_result = await db.execute(
        select(func.count())
        .select_from(StudentConceptMastery)
        .where(
            StudentConceptMastery.user_id == user.user_id,
            (StudentConceptMastery.next_review_at == None)  # noqa: E711
            | (StudentConceptMastery.next_review_at <= now),
        )
    )
    total_due_count = total_due_result.scalar_one()

    items: list[ReviewQueueItem] = []
    for row in due:
        items.append(
            ReviewQueueItem(
                concept_id=row.concept_id,
                concept_name=row.concept.name if row.concept else str(row.concept_id),
                mastery_score=row.mastery_score,
                ease_factor=row.ease_factor,
                interval_days=row.interval_days,
                repetitions=row.repetitions,
                next_review_at=row.next_review_at,
                last_reviewed_at=row.last_reviewed_at,
                is_overdue=row.next_review_at is not None and row.next_review_at < now,
            )
        )

    return ReviewQueueResponse(
        items=items,
        total_due=total_due_count,
        total_upcoming=upcoming_count,
    )


@router.post("/{concept_id}/answer", response_model=AnswerResponse)
async def record_answer(
    concept_id: UUID,
    body: AnswerRequest,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Record a review response and advance the SM-2 schedule."""
    # Verify concept exists
    concept_result = await db.execute(select(Concept).where(Concept.id == concept_id))
    if concept_result.scalar_one_or_none() is None:
        raise HTTPException(status_code=404, detail="Concept not found")

    mastery = await _get_or_create_mastery(db, user.user_id, concept_id)

    mastery_before = mastery.mastery_score
    new_interval, new_ef, new_reps = sm2_update(
        quality=body.quality,
        ease_factor=mastery.ease_factor,
        interval_days=mastery.interval_days,
        repetitions=mastery.repetitions,
    )
    new_mastery = mastery_after_review(mastery_before, body.quality)
    now = datetime.now(timezone.utc)
    new_next = next_review_time(new_interval, now)

    # Persist review record
    record = ReviewRecord(
        user_id=user.user_id,
        concept_id=concept_id,
        quality=body.quality,
        mastery_before=mastery_before,
        mastery_after=new_mastery,
        interval_after=new_interval,
        ease_after=new_ef,
        reviewed_at=now,
    )
    db.add(record)

    # Update mastery state
    mastery.mastery_score = new_mastery
    mastery.ease_factor = new_ef
    mastery.interval_days = new_interval
    mastery.repetitions = new_reps
    mastery.last_reviewed_at = now
    mastery.next_review_at = new_next

    await db.commit()

    return AnswerResponse(
        concept_id=concept_id,
        quality=body.quality,
        mastery_before=mastery_before,
        mastery_after=new_mastery,
        ease_factor=new_ef,
        interval_days=new_interval,
        repetitions=new_reps,
        next_review_at=new_next,
    )


@router.get("/stats", response_model=ReviewStatsResponse)
async def get_review_stats(
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Return aggregate review statistics for the authenticated student."""
    now = datetime.now(timezone.utc)
    today_end = now.replace(hour=23, minute=59, second=59, microsecond=999999)
    week_end = now + timedelta(days=7)

    # All aggregates pushed to Postgres — avoids loading full mastery table into Python.
    # ix_scm_user_next_review covers the user_id filter + next_review_at conditions.
    stats_result = await db.execute(
        select(
            func.count().label("total"),
            func.avg(StudentConceptMastery.mastery_score).label("avg_mastery"),
            func.avg(StudentConceptMastery.ease_factor).label("avg_ef"),
            func.count()
            .filter(
                (StudentConceptMastery.next_review_at == None)  # noqa: E711
                | (StudentConceptMastery.next_review_at <= now)
            )
            .label("due_now"),
            func.count()
            .filter(
                StudentConceptMastery.next_review_at != None,  # noqa: E711
                StudentConceptMastery.next_review_at <= today_end,
            )
            .label("due_today"),
            func.count()
            .filter(
                StudentConceptMastery.next_review_at != None,  # noqa: E711
                StudentConceptMastery.next_review_at <= week_end,
            )
            .label("due_week"),
        ).where(StudentConceptMastery.user_id == user.user_id)
    )
    row = stats_result.one()

    record_count_result = await db.execute(
        select(func.count(ReviewRecord.id)).where(ReviewRecord.user_id == user.user_id)
    )
    total_reviews = record_count_result.scalar_one() or 0

    return ReviewStatsResponse(
        total_concepts_studied=row.total or 0,
        total_reviews=total_reviews,
        due_now=row.due_now or 0,
        due_today=row.due_today or 0,
        due_this_week=row.due_week or 0,
        average_mastery=round(float(row.avg_mastery or 0.0), 4),
        average_ease_factor=round(float(row.avg_ef or 2.5), 4),
    )
