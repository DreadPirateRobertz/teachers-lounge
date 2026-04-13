"""Session summary computation — aggregates per-session metrics from interactions.

Produces a ``SessionSummary`` for a given chat session:

- ``message_count``            total interactions (student + tutor)
- ``avg_response_time_ms``     mean ``response_time_ms`` across tutor
                               interactions where it is recorded; ``None``
                               if no tutor response has a recorded latency
- ``session_duration_seconds`` seconds between session ``created_at`` and
                               the latest interaction ``created_at`` (or
                               the session ``updated_at`` if the session
                               has no interactions yet)
- ``topics``                   concept names whose ``last_reviewed_at``
                               falls within the session window, sorted
                               alphabetically.  A pragmatic proxy for
                               "topics covered" while per-interaction
                               topic tagging is not yet implemented.

The data is read-only — callers should mount this via the existing
sessions router after performing an authorisation check.
"""

from __future__ import annotations

from datetime import datetime, timezone
from uuid import UUID

from pydantic import BaseModel, Field
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from .orm import Concept, Interaction, Session, StudentConceptMastery


class SessionSummary(BaseModel):
    """Response model for ``GET /v1/sessions/{id}/summary``."""

    session_id: UUID
    topics: list[str] = Field(
        default_factory=list,
        description=(
            "Concept names touched during the session window (based on "
            "StudentConceptMastery.last_reviewed_at). Sorted alphabetically."
        ),
    )
    message_count: int = Field(..., ge=0)
    avg_response_time_ms: float | None = Field(
        default=None,
        description="Mean response_time_ms across tutor interactions, or None if unavailable.",
    )
    session_duration_seconds: float = Field(..., ge=0)


async def compute_session_summary(db: AsyncSession, session: Session) -> SessionSummary:
    """Build a :class:`SessionSummary` for the given ORM ``session`` row.

    The caller is expected to have loaded and authorised ``session`` already
    — this function performs no ownership check. It issues two additional
    queries: one over the session's interactions and one over concepts the
    student touched during the session window.

    Args:
        db:      Active async SQLAlchemy session.
        session: The :class:`Session` ORM row to summarise.

    Returns:
        A fully populated :class:`SessionSummary`.
    """
    # ── Interactions (message_count, avg response time, latest timestamp) ──
    interactions_result = await db.execute(
        select(Interaction).where(Interaction.session_id == session.id)
    )
    interactions = list(interactions_result.scalars().all())

    message_count = len(interactions)

    tutor_latencies = [
        i.response_time_ms
        for i in interactions
        if i.role == "tutor" and i.response_time_ms is not None
    ]
    avg_response_time_ms: float | None = (
        sum(tutor_latencies) / len(tutor_latencies) if tutor_latencies else None
    )

    latest_ts = (
        max(i.created_at for i in interactions) if interactions else session.updated_at
    )
    session_duration_seconds = max(
        0.0, (latest_ts - session.created_at).total_seconds()
    )

    # ── Topics covered (proxy: concepts touched in the session window) ─────
    window_end = latest_ts if interactions else _now()
    concept_result = await db.execute(
        select(Concept.name)
        .join(StudentConceptMastery, StudentConceptMastery.concept_id == Concept.id)
        .where(StudentConceptMastery.user_id == session.user_id)
        .where(StudentConceptMastery.last_reviewed_at.is_not(None))
        .where(StudentConceptMastery.last_reviewed_at >= session.created_at)
        .where(StudentConceptMastery.last_reviewed_at <= window_end)
    )
    topics = sorted({row for row in concept_result.scalars().all()})

    return SessionSummary(
        session_id=session.id,
        topics=topics,
        message_count=message_count,
        avg_response_time_ms=avg_response_time_ms,
        session_duration_seconds=session_duration_seconds,
    )


def _now() -> datetime:
    """Indirection over ``datetime.now`` to keep the module trivially mockable."""
    return datetime.now(timezone.utc)
