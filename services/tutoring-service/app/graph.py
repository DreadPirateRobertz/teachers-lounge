"""Concept dependency graph — prerequisite mapping, gap detection, remediation paths."""
from collections import defaultdict, deque
from uuid import UUID

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from .orm import Concept, ConceptPrerequisite, StudentConceptMastery

MASTERY_THRESHOLD = 0.7  # below this a prerequisite is considered a "gap"


async def get_course_concepts(db: AsyncSession, course_id: UUID) -> list[Concept]:
    result = await db.execute(
        select(Concept).where(Concept.course_id == course_id).order_by(Concept.name)
    )
    return list(result.scalars().all())


async def get_concept(db: AsyncSession, concept_id: UUID) -> Concept | None:
    result = await db.execute(select(Concept).where(Concept.id == concept_id))
    return result.scalar_one_or_none()


async def get_student_mastery(
    db: AsyncSession, user_id: UUID, course_id: UUID
) -> dict[UUID, StudentConceptMastery]:
    result = await db.execute(
        select(StudentConceptMastery)
        .join(Concept, Concept.id == StudentConceptMastery.concept_id)
        .where(StudentConceptMastery.user_id == user_id, Concept.course_id == course_id)
    )
    rows = result.scalars().all()
    return {row.concept_id: row for row in rows}


def _build_prereq_graph(concepts: list[Concept]) -> dict[UUID, list[tuple[UUID, float]]]:
    """Build adjacency list: concept_id -> [(prerequisite_id, weight), ...]"""
    graph: dict[UUID, list[tuple[UUID, float]]] = defaultdict(list)
    for c in concepts:
        graph.setdefault(c.id, [])
        for edge in c.prerequisites:
            graph[c.id].append((edge.prerequisite_id, edge.weight))
    return graph


def _get_all_prerequisites(
    concept_id: UUID,
    graph: dict[UUID, list[tuple[UUID, float]]],
) -> list[UUID]:
    """BFS to collect all transitive prerequisites of a concept."""
    visited: set[UUID] = set()
    queue = deque([concept_id])
    result: list[UUID] = []
    while queue:
        cid = queue.popleft()
        for prereq_id, _ in graph.get(cid, []):
            if prereq_id not in visited:
                visited.add(prereq_id)
                result.append(prereq_id)
                queue.append(prereq_id)
    return result


def detect_gaps(
    target_concept_id: UUID,
    concepts: list[Concept],
    mastery: dict[UUID, StudentConceptMastery],
    threshold: float = MASTERY_THRESHOLD,
) -> list[dict]:
    """Find prerequisite concepts where the student's mastery is below threshold.

    Returns a list of gap dicts: {concept_id, concept_name, mastery_score, required_by}.
    """
    graph = _build_prereq_graph(concepts)
    concept_map = {c.id: c for c in concepts}
    all_prereqs = _get_all_prerequisites(target_concept_id, graph)

    # Build reverse mapping: prerequisite -> which concepts directly need it
    direct_dependents: dict[UUID, list[UUID]] = defaultdict(list)
    for cid, edges in graph.items():
        for prereq_id, _ in edges:
            direct_dependents[prereq_id].append(cid)

    gaps = []
    for prereq_id in all_prereqs:
        score = mastery[prereq_id].mastery_score if prereq_id in mastery else 0.0
        if score < threshold:
            concept = concept_map.get(prereq_id)
            if concept:
                # required_by: direct dependents that are also in the prereq chain or the target
                required_by = [
                    d for d in direct_dependents[prereq_id]
                    if d == target_concept_id or d in set(all_prereqs)
                ]
                gaps.append({
                    "concept_id": prereq_id,
                    "concept_name": concept.name,
                    "mastery_score": score,
                    "required_by": required_by,
                })

    return gaps


def generate_remediation_path(
    target_concept_id: UUID,
    concepts: list[Concept],
    mastery: dict[UUID, StudentConceptMastery],
    threshold: float = MASTERY_THRESHOLD,
) -> list[dict]:
    """Generate a topologically-sorted study plan for unmastered prerequisites.

    Returns steps ordered so that each concept's prerequisites come before it.
    Only includes concepts below the mastery threshold.
    """
    graph = _build_prereq_graph(concepts)
    concept_map = {c.id: c for c in concepts}
    all_prereqs = _get_all_prerequisites(target_concept_id, graph)

    # Filter to only unmastered prerequisites
    unmastered = set()
    for prereq_id in all_prereqs:
        score = mastery[prereq_id].mastery_score if prereq_id in mastery else 0.0
        if score < threshold:
            unmastered.add(prereq_id)

    if not unmastered:
        return []

    # Topological sort (Kahn's algorithm) restricted to the unmastered subgraph
    in_degree: dict[UUID, int] = defaultdict(int)
    subgraph: dict[UUID, list[UUID]] = defaultdict(list)

    for cid in unmastered:
        in_degree.setdefault(cid, 0)
        for prereq_id, _ in graph.get(cid, []):
            if prereq_id in unmastered:
                subgraph[prereq_id].append(cid)
                in_degree[cid] += 1

    queue = deque([cid for cid in unmastered if in_degree[cid] == 0])
    order: list[UUID] = []
    while queue:
        cid = queue.popleft()
        order.append(cid)
        for dependent in subgraph[cid]:
            in_degree[dependent] -= 1
            if in_degree[dependent] == 0:
                queue.append(dependent)

    # Build steps
    steps = []
    for i, cid in enumerate(order):
        concept = concept_map.get(cid)
        if not concept:
            continue
        score = mastery[cid].mastery_score if cid in mastery else 0.0
        # Determine reason
        direct_prereqs_of_target = {p for p, _ in graph.get(target_concept_id, [])}
        if cid in direct_prereqs_of_target:
            reason = f"Direct prerequisite of target (mastery: {score:.0%})"
        else:
            reason = f"Transitive prerequisite (mastery: {score:.0%})"
        steps.append({
            "order": i + 1,
            "concept_id": cid,
            "concept_name": concept.name,
            "mastery_score": score,
            "reason": reason,
        })

    return steps
