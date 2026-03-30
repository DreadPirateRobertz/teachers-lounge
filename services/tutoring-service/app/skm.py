"""SKM (Student Knowledge Model) API endpoints — JWT-protected."""
from uuid import UUID

from fastapi import APIRouter, Depends, HTTPException
from sqlalchemy.ext.asyncio import AsyncSession

from .auth import JWTClaims, require_auth
from .database import get_db
from .skm_models import (
    ConceptCreate,
    ConceptResponse,
    MasteryRecordRequest,
    MasteryResponse,
    MasterySummary,
    PrerequisiteCreate,
    PrerequisiteGap,
    PrerequisiteResponse,
)
from .skm_service import (
    add_prerequisite,
    compute_effective_mastery,
    create_concept,
    detect_prerequisite_gaps,
    get_concepts_for_course,
    get_prerequisites,
    get_student_mastery_for_course,
    get_student_mastery_single,
    record_mastery_observation,
)

router = APIRouter(prefix="/skm", tags=["skm"])


def _mastery_to_response(record, concept) -> MasteryResponse:
    effective = compute_effective_mastery(record)
    return MasteryResponse(
        user_id=record.user_id,
        concept_id=record.concept_id,
        concept_name=concept.name,
        mastery_score=round(record.mastery_score, 4),
        confidence=round(record.confidence, 4),
        decay_rate=round(record.decay_rate, 4),
        review_count=record.review_count,
        effective_mastery=round(effective, 4),
        last_reviewed_at=record.last_reviewed_at,
        next_review_at=record.next_review_at,
    )


# ── Concept CRUD ─────────────────────────────────────────────────────────────


@router.post("/concepts", response_model=ConceptResponse, status_code=201)
async def create_concept_endpoint(
    body: ConceptCreate,
    db: AsyncSession = Depends(get_db),
    _user: JWTClaims = Depends(require_auth),
):
    """Create a new concept in a course."""
    concept = await create_concept(db, body.course_id, body.name, body.description)
    await db.commit()
    return ConceptResponse(
        id=concept.id,
        course_id=concept.course_id,
        name=concept.name,
        description=concept.description,
        created_at=concept.created_at,
    )


@router.get("/concepts/{course_id}", response_model=list[ConceptResponse])
async def list_concepts(
    course_id: UUID,
    db: AsyncSession = Depends(get_db),
    _user: JWTClaims = Depends(require_auth),
):
    """List all concepts in a course."""
    concepts = await get_concepts_for_course(db, course_id)
    return [
        ConceptResponse(
            id=c.id,
            course_id=c.course_id,
            name=c.name,
            description=c.description,
            created_at=c.created_at,
        )
        for c in concepts
    ]


# ── Prerequisites ────────────────────────────────────────────────────────────


@router.post("/prerequisites", response_model=PrerequisiteResponse, status_code=201)
async def add_prerequisite_endpoint(
    body: PrerequisiteCreate,
    db: AsyncSession = Depends(get_db),
    _user: JWTClaims = Depends(require_auth),
):
    """Add a prerequisite relationship between two concepts."""
    edge = await add_prerequisite(db, body.concept_id, body.prerequisite_id, body.weight)
    await db.commit()
    return PrerequisiteResponse(
        concept_id=edge.concept_id,
        prerequisite_id=edge.prerequisite_id,
        weight=edge.weight,
    )


@router.get(
    "/prerequisites/{concept_id}", response_model=list[PrerequisiteResponse]
)
async def list_prerequisites(
    concept_id: UUID,
    db: AsyncSession = Depends(get_db),
    _user: JWTClaims = Depends(require_auth),
):
    """List all prerequisites for a concept."""
    prereqs = await get_prerequisites(db, concept_id)
    return [
        PrerequisiteResponse(
            concept_id=edge.concept_id,
            prerequisite_id=edge.prerequisite_id,
            weight=edge.weight,
        )
        for edge, _concept in prereqs
    ]


# ── Mastery Tracking ─────────────────────────────────────────────────────────


@router.post("/mastery", response_model=MasteryResponse)
async def record_mastery(
    body: MasteryRecordRequest,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Record a mastery observation for the authenticated student.

    Updates the student's mastery score using exponential moving average,
    recalculates confidence and schedules the next review.
    """
    record = await record_mastery_observation(
        db, user.user_id, body.concept_id, body.score, body.weight
    )
    await db.commit()

    # Re-fetch with concept info for response
    row = await get_student_mastery_single(db, user.user_id, body.concept_id)
    if row is None:
        raise HTTPException(status_code=500, detail="Mastery record not found after update")
    return _mastery_to_response(row[0], row[1])


@router.get("/mastery/{course_id}", response_model=list[MasteryResponse])
async def get_course_mastery(
    course_id: UUID,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Get all mastery records for the authenticated student in a course."""
    rows = await get_student_mastery_for_course(db, user.user_id, course_id)
    return [_mastery_to_response(record, concept) for record, concept in rows]


@router.get("/mastery/{course_id}/summary", response_model=MasterySummary)
async def get_mastery_summary(
    course_id: UUID,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Get an aggregated mastery summary for a student in a course."""
    rows = await get_student_mastery_for_course(db, user.user_id, course_id)
    responses = [_mastery_to_response(record, concept) for record, concept in rows]

    if not responses:
        return MasterySummary(
            user_id=user.user_id,
            course_id=course_id,
            concept_count=0,
            average_mastery=0.0,
            average_effective_mastery=0.0,
            weak_concepts=[],
            strong_concepts=[],
        )

    avg_mastery = sum(r.mastery_score for r in responses) / len(responses)
    avg_effective = sum(r.effective_mastery for r in responses) / len(responses)

    return MasterySummary(
        user_id=user.user_id,
        course_id=course_id,
        concept_count=len(responses),
        average_mastery=round(avg_mastery, 4),
        average_effective_mastery=round(avg_effective, 4),
        weak_concepts=[r for r in responses if r.effective_mastery < 0.4],
        strong_concepts=[r for r in responses if r.effective_mastery > 0.7],
    )


# ── Prerequisite Gaps ────────────────────────────────────────────────────────


@router.get("/gaps/{concept_id}", response_model=list[PrerequisiteGap])
async def get_prerequisite_gaps(
    concept_id: UUID,
    db: AsyncSession = Depends(get_db),
    user: JWTClaims = Depends(require_auth),
):
    """Detect prerequisite gaps for a concept.

    Returns prerequisites where the student's effective mastery is below
    the gap threshold (40%).
    """
    gaps = await detect_prerequisite_gaps(db, user.user_id, concept_id)
    return [PrerequisiteGap(**g) for g in gaps]
