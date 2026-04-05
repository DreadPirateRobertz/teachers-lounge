"""Concept dependency graph endpoints — JWT-protected."""
from datetime import datetime, timezone
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from .auth import JWTClaims, require_auth
from .database import get_db
from .graph import (
    ADEQUATE_THRESHOLD,
    detect_gaps,
    generate_remediation_path,
    get_concept,
    get_course_concepts,
    get_prerequisite_chain,
    get_student_mastery,
)
from .models import (
    ConceptResponse,
    GapDetectionResponse,
    GapInfo,
    MasteryEntry,
    MasteryUpdateRequest,
    MasteryUpdateResponse,
    PrerequisiteChainEntry,
    PrerequisiteChainResponse,
    RemediationPathResponse,
    RemediationStep,
)
from .orm import StudentConceptMastery
from .srs import mastery_from_retention

router = APIRouter(prefix="/courses/{course_id}/concepts", tags=["concepts"])


@router.get("", response_model=list[ConceptResponse])
async def list_concepts(
    course_id: UUID,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """List all concepts in a course with their prerequisite edges."""
    concepts = await get_course_concepts(db, course_id)
    return [
        ConceptResponse(
            id=c.id,
            course_id=c.course_id,
            name=c.name,
            description=c.description,
            path=c.path,
            prerequisite_ids=[e.prerequisite_id for e in c.prerequisites],
        )
        for c in concepts
    ]


@router.get("/mastery", response_model=list[MasteryEntry])
async def get_mastery(
    course_id: UUID,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Get the student's mastery scores for all concepts in a course."""
    concepts = await get_course_concepts(db, course_id)
    mastery = await get_student_mastery(db, user.user_id, course_id)
    return [
        MasteryEntry(
            concept_id=c.id,
            concept_name=c.name,
            mastery_score=mastery[c.id].mastery_score if c.id in mastery else 0.0,
            last_reviewed_at=mastery[c.id].last_reviewed_at if c.id in mastery else None,
            next_review_at=mastery[c.id].next_review_at if c.id in mastery else None,
        )
        for c in concepts
    ]


@router.get("/{concept_id}/gaps", response_model=GapDetectionResponse)
async def get_gaps(
    course_id: UUID,
    concept_id: UUID,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Detect prerequisite gaps for a target concept."""
    concept = await get_concept(db, concept_id)
    if concept is None or concept.course_id != course_id:
        raise HTTPException(status_code=404, detail="Concept not found")

    concepts = await get_course_concepts(db, course_id)
    mastery = await get_student_mastery(db, user.user_id, course_id)
    gaps = detect_gaps(concept_id, concepts, mastery)

    return GapDetectionResponse(
        target_concept_id=concept_id,
        target_concept_name=concept.name,
        gaps=[GapInfo(**g) for g in gaps],
    )


@router.get("/{concept_id}/remediation", response_model=RemediationPathResponse)
async def get_remediation(
    course_id: UUID,
    concept_id: UUID,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Generate an ordered remediation path for a target concept."""
    concept = await get_concept(db, concept_id)
    if concept is None or concept.course_id != course_id:
        raise HTTPException(status_code=404, detail="Concept not found")

    concepts = await get_course_concepts(db, course_id)
    mastery = await get_student_mastery(db, user.user_id, course_id)
    steps = generate_remediation_path(concept_id, concepts, mastery)

    return RemediationPathResponse(
        target_concept_id=concept_id,
        target_concept_name=concept.name,
        steps=[RemediationStep(**s) for s in steps],
    )


@router.get("/{concept_id}/prerequisites", response_model=PrerequisiteChainResponse)
async def get_prerequisites(
    course_id: UUID,
    concept_id: UUID,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Walk the full prerequisite chain for a concept with the student's mastery scores.

    Returns all transitive prerequisites ordered by depth (direct prerequisites first),
    with a flag indicating whether each prerequisite meets the adequacy threshold.
    """
    concept = await get_concept(db, concept_id)
    if concept is None or concept.course_id != course_id:
        raise HTTPException(status_code=404, detail="Concept not found")

    concepts = await get_course_concepts(db, course_id)
    mastery = await get_student_mastery(db, user.user_id, course_id)
    chain = get_prerequisite_chain(concept_id, concepts, mastery)

    return PrerequisiteChainResponse(
        target_concept_id=concept_id,
        target_concept_name=concept.name,
        chain=[PrerequisiteChainEntry(**entry) for entry in chain],
    )


@router.patch("/mastery/{concept_id}", response_model=MasteryUpdateResponse)
async def update_mastery(
    course_id: UUID,
    concept_id: UUID,
    body: MasteryUpdateRequest,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Update a student's mastery score for a concept after an interaction.

    Applies exponential decay to the stored score before accepting the new value,
    so the effective mastery at update time is visible in the response.
    """
    concept = await get_concept(db, concept_id)
    if concept is None or concept.course_id != course_id:
        raise HTTPException(status_code=404, detail="Concept not found")

    result = await db.execute(
        select(StudentConceptMastery).where(
            StudentConceptMastery.user_id == user.user_id,
            StudentConceptMastery.concept_id == concept_id,
        )
    )
    row = result.scalar_one_or_none()
    if row is None:
        row = StudentConceptMastery(user_id=user.user_id, concept_id=concept_id)
        db.add(row)

    mastery_before_decay = row.mastery_score
    now = datetime.now(timezone.utc)

    # Apply forgetting-curve decay to compute current effective mastery
    if row.last_reviewed_at is not None:
        elapsed_days = (now - row.last_reviewed_at).total_seconds() / 86400
        mastery_after_decay = mastery_from_retention(
            base_mastery=mastery_before_decay,
            elapsed_days=elapsed_days,
            decay_rate=row.decay_rate,
        )
    else:
        mastery_after_decay = mastery_before_decay

    row.mastery_score = body.mastery_score
    row.last_reviewed_at = now
    row.review_count = (row.review_count or 0) + 1

    await db.commit()

    return MasteryUpdateResponse(
        concept_id=concept_id,
        mastery_before_decay=mastery_before_decay,
        mastery_after_decay=mastery_after_decay,
        mastery_updated=body.mastery_score,
        decay_rate=row.decay_rate,
        last_reviewed_at=now,
    )
