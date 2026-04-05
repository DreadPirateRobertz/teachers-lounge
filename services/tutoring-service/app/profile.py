"""Learning profile and misconception API routes — JWT-protected.

Exposes the Student Knowledge Model (SKM) adaptive layer via REST:

  GET  /students/me/learning-profile                           — fetch dials
  PATCH /students/me/learning-profile                          — update dials
  GET  /students/me/misconceptions                             — active errors
  POST /students/me/misconceptions/{concept_id}               — log an error
  PATCH /students/me/misconceptions/{misconception_id}/resolve — dismiss error

Each write route owns its own db.commit() — helpers in knowledge_model.py are
commit-free so multiple writes can be batched in a single transaction if needed.
"""
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from .auth import JWTClaims, require_auth
from .database import get_db
from .knowledge_model import (
    get_active_misconceptions,
    get_dials,
    log_misconception,
    resolve_misconception,
    update_learning_profile_dials,
)
from .models import (
    LearningProfileResponse,
    LearningProfileUpdateRequest,
    MisconceptionEntry,
    MisconceptionLogRequest,
)
from .orm import Concept

router = APIRouter(prefix="/students/me", tags=["learning-profile"])


@router.get("/learning-profile", response_model=LearningProfileResponse)
async def get_learning_profile(
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Return the authenticated student's Felder-Silverman learning-style dials.

    Returns all-zero dials for a student who has never interacted with the
    adaptive layer, without creating a profile row.
    """
    dials = await get_dials(db, user.user_id)
    return LearningProfileResponse(user_id=user.user_id, dials=dials)


@router.patch("/learning-profile", response_model=LearningProfileResponse)
async def patch_learning_profile(
    body: LearningProfileUpdateRequest,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Update one or more of the student's Felder-Silverman learning-style dials.

    Only keys present in the request body are updated; other dimensions keep
    their current values.  Unknown dimension keys and values outside [-1, 1]
    are rejected with 422.
    """
    try:
        body.validate_dial_values()
    except ValueError as exc:
        raise HTTPException(status_code=422, detail=str(exc)) from exc

    profile = await update_learning_profile_dials(db, user.user_id, body.dials)
    await db.commit()
    dials = {
        "active_reflective": profile.active_reflective,
        "sensing_intuitive": profile.sensing_intuitive,
        "visual_verbal": profile.visual_verbal,
        "sequential_global": profile.sequential_global,
    }
    return LearningProfileResponse(
        user_id=user.user_id,
        dials=dials,
        updated_at=profile.updated_at,
    )


@router.get("/misconceptions", response_model=list[MisconceptionEntry])
async def list_misconceptions(
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """List the student's active (unresolved) misconceptions with recency weights.

    Misconceptions are ordered by recency_weight descending — the most recently
    observed errors appear first.
    """
    entries = await get_active_misconceptions(db, user.user_id)
    return [MisconceptionEntry(**e) for e in entries]


@router.post(
    "/misconceptions/{concept_id}",
    response_model=MisconceptionEntry,
    status_code=201,
)
async def add_misconception(
    concept_id: UUID,
    body: MisconceptionLogRequest,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Log a detected misconception for the authenticated student.

    Returns 404 if the concept does not exist.  If an identical unresolved
    misconception already exists for this concept, its last_seen_at is
    refreshed rather than creating a duplicate.

    Called by the tutoring agent when it detects that the student holds an
    incorrect belief about a concept.
    """
    concept_result = await db.execute(select(Concept).where(Concept.id == concept_id))
    if concept_result.scalar_one_or_none() is None:
        raise HTTPException(status_code=404, detail="Concept not found")

    m = await log_misconception(db, user.user_id, concept_id, body.description)
    await db.commit()
    return MisconceptionEntry(
        id=m.id,
        concept_id=m.concept_id,
        description=m.description,
        confidence=m.confidence,
        recorded_at=m.recorded_at,
        recency_weight=1.0,
    )


@router.patch("/misconceptions/{misconception_id}/resolve")
async def resolve_student_misconception(
    misconception_id: UUID,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Mark a misconception as resolved so it no longer appears in the active list.

    Returns 404 if the misconception does not exist or belongs to a different student.
    """
    ok = await resolve_misconception(db, misconception_id, user.user_id)
    if not ok:
        raise HTTPException(status_code=404, detail="Misconception not found")
    await db.commit()
    return {"resolved": True}
