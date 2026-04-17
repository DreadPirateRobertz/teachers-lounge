"""Global concept knowledge graph (tl-mhd).

Provides ltree-based ancestor / descendant lookups against the
``concept_graph`` table and a mastery-aware prerequisite-gap detector used by
the RAG context builder. Ancestors in the ltree ``path`` are treated as
prerequisites; descendants are dependents.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Mapping

from sqlalchemy import select, text
from sqlalchemy.ext.asyncio import AsyncSession

from .orm import ConceptGraphNode

# Default mastery threshold below which an ancestor counts as a "gap" for
# prerequisite-aware tutoring. Matches the spec on tl-mhd.
ANCESTOR_GAP_THRESHOLD = 0.4


async def get_concept(db: AsyncSession, concept_id: str) -> ConceptGraphNode | None:
    """Fetch a single concept by its public slug.

    Args:
        db: Open async SQLAlchemy session.
        concept_id: Slug identifier (e.g. ``"chirality"``).

    Returns:
        The matching :class:`ConceptGraphNode`, or ``None`` if not found.
    """
    result = await db.execute(
        select(ConceptGraphNode).where(ConceptGraphNode.concept_id == concept_id)
    )
    return result.scalar_one_or_none()


async def get_concept_by_label(
    db: AsyncSession, label: str
) -> ConceptGraphNode | None:
    """Fetch a concept by case-insensitive label match.

    The RAG agent resolves the per-course ``Concept.name`` into a global
    :class:`ConceptGraphNode` through this lookup; labels are the most
    stable join key between the two representations.
    """
    result = await db.execute(
        select(ConceptGraphNode).where(ConceptGraphNode.label.ilike(label))
    )
    return result.scalar_one_or_none()


async def get_ancestors(db: AsyncSession, concept_id: str) -> list[ConceptGraphNode]:
    """Return ancestor concepts (prerequisites) of ``concept_id``.

    Uses Postgres ltree ``@>`` to find every row whose path is a strict
    ancestor of the target's path. The target itself is excluded. Results
    are ordered by path depth (shallowest → deepest), matching the natural
    "study this before that" ordering.

    Args:
        db: Open async SQLAlchemy session.
        concept_id: Slug of the target concept.

    Returns:
        List of ancestor :class:`ConceptGraphNode` rows, or ``[]`` if the
        target has no ancestors or does not exist.
    """
    stmt = text(
        """
        SELECT id, concept_id, label, subject, path::text AS path
          FROM concept_graph
         WHERE path @> (
             SELECT path FROM concept_graph WHERE concept_id = :concept_id
         )
           AND concept_id <> :concept_id
         ORDER BY nlevel(path), path
        """
    )
    result = await db.execute(stmt, {"concept_id": concept_id})
    return [_row_to_node(row) for row in result.mappings().all()]


async def get_descendants(db: AsyncSession, concept_id: str) -> list[ConceptGraphNode]:
    """Return descendant concepts (dependents) of ``concept_id``.

    Uses Postgres ltree ``<@`` to find every row whose path is a strict
    descendant of the target's path.

    Args:
        db: Open async SQLAlchemy session.
        concept_id: Slug of the target concept.

    Returns:
        List of descendant :class:`ConceptGraphNode` rows ordered by depth.
    """
    stmt = text(
        """
        SELECT id, concept_id, label, subject, path::text AS path
          FROM concept_graph
         WHERE path <@ (
             SELECT path FROM concept_graph WHERE concept_id = :concept_id
         )
           AND concept_id <> :concept_id
         ORDER BY nlevel(path), path
        """
    )
    result = await db.execute(stmt, {"concept_id": concept_id})
    return [_row_to_node(row) for row in result.mappings().all()]


async def create_concept(
    db: AsyncSession,
    *,
    concept_id: str,
    label: str,
    subject: str,
    path: str,
) -> ConceptGraphNode:
    """Insert a new concept into ``concept_graph``.

    Uniqueness is enforced at the DB level on ``concept_id``; callers should
    surface a 409 when the DB raises :class:`IntegrityError`.

    Args:
        db:         Open async SQLAlchemy session.
        concept_id: Stable public slug.
        label:      Human-readable title.
        subject:    Top-level subject grouping (e.g. ``"chemistry"``).
        path:       Dot-separated ltree path.

    Returns:
        The newly-inserted :class:`ConceptGraphNode` (with ``id`` populated).
    """
    node = ConceptGraphNode(concept_id=concept_id, label=label, subject=subject, path=path)
    db.add(node)
    await db.flush()
    return node


@dataclass(frozen=True)
class AncestorGap:
    """An ancestor concept where the student's mastery is below threshold.

    Attributes:
        concept_id: Slug of the ancestor concept.
        label: Human-readable label.
        path: ltree path.
        mastery_score: The student's current mastery in [0, 1].
    """

    concept_id: str
    label: str
    path: str
    mastery_score: float


async def detect_ancestor_gaps(
    db: AsyncSession,
    target_concept_id: str,
    mastery: Mapping[str, float],
    threshold: float = ANCESTOR_GAP_THRESHOLD,
) -> list[AncestorGap]:
    """Find ancestors of ``target_concept_id`` whose mastery is below ``threshold``.

    The mastery store is supplied by the caller as a ``{label: score}``
    mapping — labels are the stable join key between per-course
    :class:`~app.orm.Concept` rows (``Concept.name``) and global
    :class:`~app.orm.ConceptGraphNode` rows (``ConceptGraphNode.label``).
    Ancestors absent from the mapping are treated as mastery 0.0, so an
    unseen prerequisite surfaces as a gap — the conservative default for
    prereq-aware tutoring.

    Args:
        db:                Open async SQLAlchemy session.
        target_concept_id: Slug of the concept the student is currently working on.
        mastery:           Mapping of concept label → mastery score in [0, 1].
        threshold:         Gap threshold; defaults to :data:`ANCESTOR_GAP_THRESHOLD`.

    Returns:
        Ancestor gaps in depth order (shallowest first). Empty if the target
        has no ancestors below the threshold.
    """
    ancestors = await get_ancestors(db, target_concept_id)
    gaps: list[AncestorGap] = []
    for node in ancestors:
        score = float(mastery.get(node.label, 0.0))
        if score < threshold:
            gaps.append(
                AncestorGap(
                    concept_id=node.concept_id,
                    label=node.label,
                    path=node.path,
                    mastery_score=score,
                )
            )
    return gaps


def format_gap_note(target_label: str, gaps: list[AncestorGap]) -> str:
    """Render a concise prerequisite-gap note for the RAG system prompt.

    The spec on tl-mhd calls for a sentence of the form::

        Prerequisite gap detected: student has not mastered <ancestor>
        which is required for <target>.

    When multiple gaps are present, they are joined into a single note so
    the tutor can address them in order.

    Args:
        target_label: Label of the concept the student is asking about.
        gaps:         Ancestors below the mastery threshold.

    Returns:
        The formatted note, or an empty string if there are no gaps.
    """
    if not gaps:
        return ""
    if len(gaps) == 1:
        g = gaps[0]
        return (
            f"Prerequisite gap detected: student has not mastered {g.label} "
            f"which is required for {target_label}."
        )
    names = ", ".join(g.label for g in gaps)
    return (
        f"Prerequisite gaps detected: student has not mastered {names} "
        f"which are required for {target_label}."
    )


def _row_to_node(row) -> ConceptGraphNode:
    """Hydrate a ``row.mappings()`` result into an unattached ORM instance.

    We use raw text queries for ltree ancestor/descendant lookups because
    SQLAlchemy lacks first-class ltree operators; this helper packages the
    rows back into ORM shape so callers don't see two types.
    """
    node = ConceptGraphNode(
        concept_id=row["concept_id"],
        label=row["label"],
        subject=row["subject"],
        path=row["path"],
    )
    # id is server-generated; set it directly to avoid persistence confusion.
    node.id = row["id"]
    return node
