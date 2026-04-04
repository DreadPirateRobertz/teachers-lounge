"""Concept dependency graph endpoints — JWT-protected."""
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.ext.asyncio import AsyncSession

from .auth import JWTClaims, require_auth
from .database import get_db
from .graph import (
    detect_gaps,
    generate_remediation_path,
    get_concept,
    get_course_concepts,
    get_student_mastery,
)
from .models import (
    ConceptResponse,
    GapDetectionResponse,
    GapInfo,
    MasteryEntry,
    RemediationPathResponse,
    RemediationStep,
)

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
