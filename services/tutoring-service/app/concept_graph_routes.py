"""HTTP endpoints for the global concept knowledge graph (tl-mhd).

Mounted under ``/v1/concepts`` (alongside the course-scoped graph at
``/v1/courses/{course_id}/concepts`` — the two serve different purposes:
this one is the subject-wide catalog used by prerequisite-aware search).
"""

from __future__ import annotations

from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.exc import IntegrityError
from sqlalchemy.ext.asyncio import AsyncSession

from .auth import JWTClaims, require_auth
from .concept_graph import create_concept, get_ancestors, get_concept, get_descendants
from .database import get_db
from .models import ConceptGraphNodeResponse, CreateConceptRequest

router = APIRouter(prefix="/concepts", tags=["concept-graph"])


def _to_response(node) -> ConceptGraphNodeResponse:
    """Project an ORM row to its wire representation."""
    return ConceptGraphNodeResponse(
        concept_id=node.concept_id,
        label=node.label,
        subject=node.subject,
        path=node.path,
    )


@router.get("/{concept_id}/prerequisites", response_model=list[ConceptGraphNodeResponse])
async def list_prerequisites(
    concept_id: str,
    db: AsyncSession = Depends(get_db),
    _: JWTClaims = Depends(require_auth),
):
    """Return ancestor concepts — the prerequisites — of ``concept_id``.

    The ancestor relation is derived from ltree paths: any row whose ``path``
    is a prefix of the target's ``path`` is considered a prerequisite.
    """
    target = await get_concept(db, concept_id)
    if target is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Concept not found")
    ancestors = await get_ancestors(db, concept_id)
    return [_to_response(n) for n in ancestors]


@router.get("/{concept_id}/dependents", response_model=list[ConceptGraphNodeResponse])
async def list_dependents(
    concept_id: str,
    db: AsyncSession = Depends(get_db),
    _: JWTClaims = Depends(require_auth),
):
    """Return descendant concepts — the dependents — of ``concept_id``."""
    target = await get_concept(db, concept_id)
    if target is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Concept not found")
    descendants = await get_descendants(db, concept_id)
    return [_to_response(n) for n in descendants]


@router.post(
    "",
    response_model=ConceptGraphNodeResponse,
    status_code=status.HTTP_201_CREATED,
)
async def add_concept(
    body: CreateConceptRequest,
    db: AsyncSession = Depends(get_db),
    _: JWTClaims = Depends(require_auth),
):
    """Insert a new concept into the global graph.

    Returns 409 if ``concept_id`` is already present.
    """
    try:
        node = await create_concept(
            db,
            concept_id=body.concept_id,
            label=body.label,
            subject=body.subject,
            path=body.path,
        )
        await db.commit()
    except IntegrityError as exc:
        await db.rollback()
        raise HTTPException(
            status_code=status.HTTP_409_CONFLICT,
            detail=f"concept_id {body.concept_id!r} already exists",
        ) from exc
    return _to_response(node)
