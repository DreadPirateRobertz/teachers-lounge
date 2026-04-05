"""Tests for concept dependency graph — prerequisite mapping, gap detection, remediation paths."""
from dataclasses import dataclass, field
from uuid import UUID, uuid4

import pytest

from app.graph import (
    ADEQUATE_THRESHOLD,
    MASTERY_THRESHOLD,
    _build_prereq_graph,
    _get_all_prerequisites,
    detect_gaps,
    generate_remediation_path,
    get_prerequisite_chain,
)

# ── Lightweight stand-ins (avoid SQLAlchemy instrumentation in unit tests) ───

COURSE_ID = uuid4()


@dataclass
class FakeEdge:
    concept_id: UUID
    prerequisite_id: UUID
    weight: float = 1.0


@dataclass
class FakeConcept:
    id: UUID
    course_id: UUID
    name: str
    description: str = ""
    path: str = ""
    difficulty: float = 0.5
    prerequisites: list[FakeEdge] = field(default_factory=list)
    dependents: list[FakeEdge] = field(default_factory=list)


@dataclass
class FakeMastery:
    user_id: UUID
    concept_id: UUID
    mastery_score: float
    last_reviewed_at: object = None
    next_review_at: object = None
    decay_rate: float = 0.1


def _concept(name: str, prereqs: list | None = None) -> FakeConcept:
    c = FakeConcept(
        id=uuid4(),
        course_id=COURSE_ID,
        name=name,
        path=name.lower().replace(" ", "_"),
    )
    if prereqs:
        for prereq_concept, weight in prereqs:
            c.prerequisites.append(FakeEdge(
                concept_id=c.id,
                prerequisite_id=prereq_concept.id,
                weight=weight,
            ))
    return c


def _mastery(user_id, concept_id, score):
    return FakeMastery(user_id=user_id, concept_id=concept_id, mastery_score=score)


# ── Graph: linear chain A -> B -> C ─────────────────────────────────────────
# C depends on B, B depends on A

@pytest.fixture
def linear_chain():
    a = _concept("Algebra Basics")
    b = _concept("Linear Equations", prereqs=[(a, 1.0)])
    c = _concept("Quadratics", prereqs=[(b, 1.0)])
    return [a, b, c]


# ── Graph: diamond A -> B, A -> C, B+C -> D ─────────────────────────────────

@pytest.fixture
def diamond_graph():
    a = _concept("Arithmetic")
    b = _concept("Fractions", prereqs=[(a, 1.0)])
    c = _concept("Decimals", prereqs=[(a, 1.0)])
    d = _concept("Ratios", prereqs=[(b, 0.8), (c, 0.6)])
    return [a, b, c, d]


# ── Tests: _build_prereq_graph ───────────────────────────────────────────────

class TestBuildPrereqGraph:
    def test_linear_chain(self, linear_chain):
        a, b, c = linear_chain
        graph = _build_prereq_graph(linear_chain)
        assert graph[c.id] == [(b.id, 1.0)]
        assert graph[b.id] == [(a.id, 1.0)]
        assert graph[a.id] == []

    def test_diamond(self, diamond_graph):
        a, b, c, d = diamond_graph
        graph = _build_prereq_graph(diamond_graph)
        assert len(graph[d.id]) == 2
        prereq_ids = {pid for pid, _ in graph[d.id]}
        assert prereq_ids == {b.id, c.id}


# ── Tests: _get_all_prerequisites ────────────────────────────────────────────

class TestGetAllPrerequisites:
    def test_transitive_linear(self, linear_chain):
        a, b, c = linear_chain
        graph = _build_prereq_graph(linear_chain)
        all_prereqs = _get_all_prerequisites(c.id, graph)
        assert set(all_prereqs) == {a.id, b.id}

    def test_no_prereqs(self, linear_chain):
        a, b, c = linear_chain
        graph = _build_prereq_graph(linear_chain)
        assert _get_all_prerequisites(a.id, graph) == []

    def test_diamond_no_duplicates(self, diamond_graph):
        a, b, c, d = diamond_graph
        graph = _build_prereq_graph(diamond_graph)
        all_prereqs = _get_all_prerequisites(d.id, graph)
        # a appears in both paths but should only be listed once
        assert len(all_prereqs) == len(set(all_prereqs))
        assert set(all_prereqs) == {a.id, b.id, c.id}


# ── Tests: detect_gaps ──────────────────────────────────────────────────────

class TestDetectGaps:
    def test_all_mastered_no_gaps(self, linear_chain):
        a, b, c = linear_chain
        user_id = uuid4()
        mastery = {
            a.id: _mastery(user_id, a.id, 0.9),
            b.id: _mastery(user_id, b.id, 0.8),
        }
        gaps = detect_gaps(c.id, linear_chain, mastery)
        assert gaps == []

    def test_single_gap(self, linear_chain):
        a, b, c = linear_chain
        user_id = uuid4()
        mastery = {
            a.id: _mastery(user_id, a.id, 0.9),
            b.id: _mastery(user_id, b.id, 0.3),
        }
        gaps = detect_gaps(c.id, linear_chain, mastery)
        assert len(gaps) == 1
        assert gaps[0]["concept_id"] == b.id
        assert gaps[0]["mastery_score"] == 0.3

    def test_missing_mastery_treated_as_zero(self, linear_chain):
        a, b, c = linear_chain
        gaps = detect_gaps(c.id, linear_chain, mastery={})
        assert len(gaps) == 2
        gap_ids = {g["concept_id"] for g in gaps}
        assert gap_ids == {a.id, b.id}

    def test_diamond_gap_required_by(self, diamond_graph):
        a, b, c, d = diamond_graph
        user_id = uuid4()
        mastery = {
            a.id: _mastery(user_id, a.id, 0.3),  # gap
            b.id: _mastery(user_id, b.id, 0.9),
            c.id: _mastery(user_id, c.id, 0.9),
        }
        gaps = detect_gaps(d.id, diamond_graph, mastery)
        assert len(gaps) == 1
        assert gaps[0]["concept_id"] == a.id
        # a is required by b and c (both in the prereq chain)
        assert set(gaps[0]["required_by"]) == {b.id, c.id}

    def test_custom_threshold(self, linear_chain):
        a, b, c = linear_chain
        user_id = uuid4()
        mastery = {
            a.id: _mastery(user_id, a.id, 0.5),
            b.id: _mastery(user_id, b.id, 0.5),
        }
        # Default threshold 0.7 -> both are gaps
        assert len(detect_gaps(c.id, linear_chain, mastery)) == 2
        # Threshold 0.4 -> no gaps
        assert len(detect_gaps(c.id, linear_chain, mastery, threshold=0.4)) == 0


# ── Tests: generate_remediation_path ─────────────────────────────────────────

class TestRemediationPath:
    def test_no_gaps_empty_path(self, linear_chain):
        a, b, c = linear_chain
        user_id = uuid4()
        mastery = {
            a.id: _mastery(user_id, a.id, 0.9),
            b.id: _mastery(user_id, b.id, 0.8),
        }
        path = generate_remediation_path(c.id, linear_chain, mastery)
        assert path == []

    def test_linear_order(self, linear_chain):
        a, b, c = linear_chain
        path = generate_remediation_path(c.id, linear_chain, mastery={})
        assert len(path) == 2
        # a must come before b (a has no prereqs, b depends on a)
        assert path[0]["concept_id"] == a.id
        assert path[1]["concept_id"] == b.id
        assert path[0]["order"] == 1
        assert path[1]["order"] == 2

    def test_diamond_order(self, diamond_graph):
        a, b, c, d = diamond_graph
        path = generate_remediation_path(d.id, diamond_graph, mastery={})
        assert len(path) == 3  # a, b, c (d is the target, not included)
        # a must come first (no prereqs)
        assert path[0]["concept_id"] == a.id
        # b and c can be in either order (both depend only on a)
        remaining = {path[1]["concept_id"], path[2]["concept_id"]}
        assert remaining == {b.id, c.id}

    def test_partial_mastery_skips_mastered(self, linear_chain):
        a, b, c = linear_chain
        user_id = uuid4()
        mastery = {
            a.id: _mastery(user_id, a.id, 0.9),  # mastered
            b.id: _mastery(user_id, b.id, 0.2),  # gap
        }
        path = generate_remediation_path(c.id, linear_chain, mastery)
        assert len(path) == 1
        assert path[0]["concept_id"] == b.id

    def test_step_has_reason(self, linear_chain):
        a, b, c = linear_chain
        path = generate_remediation_path(c.id, linear_chain, mastery={})
        for step in path:
            assert "reason" in step
            assert "mastery" in step["reason"].lower() or "prerequisite" in step["reason"].lower()


# ── Tests: API endpoint response shapes (unit, no DB) ───────────────────────

class TestConceptResponseModel:
    def test_concept_response_serialization(self):
        from app.models import ConceptResponse
        resp = ConceptResponse(
            id=uuid4(),
            course_id=uuid4(),
            name="Algebra",
            description="Basic algebra",
            path="math.algebra",
            prerequisite_ids=[uuid4()],
        )
        data = resp.model_dump()
        assert data["name"] == "Algebra"
        assert len(data["prerequisite_ids"]) == 1

    def test_gap_detection_response_serialization(self):
        from app.models import GapDetectionResponse, GapInfo
        resp = GapDetectionResponse(
            target_concept_id=uuid4(),
            target_concept_name="Quadratics",
            gaps=[
                GapInfo(
                    concept_id=uuid4(),
                    concept_name="Algebra",
                    mastery_score=0.3,
                    required_by=[uuid4()],
                )
            ],
        )
        data = resp.model_dump()
        assert len(data["gaps"]) == 1

    def test_remediation_path_response_serialization(self):
        from app.models import RemediationPathResponse, RemediationStep
        resp = RemediationPathResponse(
            target_concept_id=uuid4(),
            target_concept_name="Quadratics",
            steps=[
                RemediationStep(
                    order=1,
                    concept_id=uuid4(),
                    concept_name="Algebra",
                    mastery_score=0.2,
                    reason="Direct prerequisite of target (mastery: 20%)",
                )
            ],
        )
        data = resp.model_dump()
        assert data["steps"][0]["order"] == 1


# ── Tests: get_prerequisite_chain ────────────────────────────────────────────

class TestGetPrerequisiteChain:
    def test_linear_chain_returns_all_prereqs(self, linear_chain):
        a, b, c = linear_chain
        mastery = {}
        chain = get_prerequisite_chain(c.id, linear_chain, mastery)
        chain_ids = {entry["concept_id"] for entry in chain}
        assert chain_ids == {a.id, b.id}

    def test_depth_is_1_for_direct_prereq(self, linear_chain):
        a, b, c = linear_chain
        chain = get_prerequisite_chain(c.id, linear_chain, mastery={})
        by_id = {e["concept_id"]: e for e in chain}
        assert by_id[b.id]["depth"] == 1
        assert by_id[a.id]["depth"] == 2

    def test_diamond_all_prereqs_included(self, diamond_graph):
        a, b, c, d = diamond_graph
        chain = get_prerequisite_chain(d.id, diamond_graph, mastery={})
        chain_ids = {entry["concept_id"] for entry in chain}
        assert chain_ids == {a.id, b.id, c.id}

    def test_no_prereqs_returns_empty(self, linear_chain):
        a, b, c = linear_chain
        chain = get_prerequisite_chain(a.id, linear_chain, mastery={})
        assert chain == []

    def test_mastery_score_populated(self, linear_chain):
        a, b, c = linear_chain
        user_id = uuid4()
        mastery = {
            a.id: FakeMastery(user_id, a.id, 0.9),
            b.id: FakeMastery(user_id, b.id, 0.3),
        }
        chain = get_prerequisite_chain(c.id, linear_chain, mastery)
        by_id = {e["concept_id"]: e for e in chain}
        assert by_id[a.id]["mastery_score"] == 0.9
        assert by_id[b.id]["mastery_score"] == 0.3

    def test_mastery_adequate_flag(self, linear_chain):
        a, b, c = linear_chain
        user_id = uuid4()
        mastery = {
            a.id: FakeMastery(user_id, a.id, 0.9),  # above threshold
            b.id: FakeMastery(user_id, b.id, 0.3),  # below threshold
        }
        chain = get_prerequisite_chain(c.id, linear_chain, mastery)
        by_id = {e["concept_id"]: e for e in chain}
        assert by_id[a.id]["mastery_adequate"] is True
        assert by_id[b.id]["mastery_adequate"] is False

    def test_custom_threshold(self, linear_chain):
        a, b, c = linear_chain
        user_id = uuid4()
        mastery = {a.id: FakeMastery(user_id, a.id, 0.5), b.id: FakeMastery(user_id, b.id, 0.5)}
        chain_strict = get_prerequisite_chain(c.id, linear_chain, mastery, threshold=0.9)
        chain_loose = get_prerequisite_chain(c.id, linear_chain, mastery, threshold=0.4)
        assert all(not e["mastery_adequate"] for e in chain_strict)
        assert all(e["mastery_adequate"] for e in chain_loose)

    def test_path_and_difficulty_included(self, linear_chain):
        a, b, c = linear_chain
        chain = get_prerequisite_chain(c.id, linear_chain, mastery={})
        for entry in chain:
            assert "path" in entry
            assert "difficulty" in entry
            assert 0.0 <= entry["difficulty"] <= 1.0


# ── Tests: new response models ────────────────────────────────────────────────

class TestNewResponseModels:
    def test_prerequisite_chain_response_serialization(self):
        from app.models import PrerequisiteChainEntry, PrerequisiteChainResponse
        resp = PrerequisiteChainResponse(
            target_concept_id=uuid4(),
            target_concept_name="Stereochemistry",
            chain=[
                PrerequisiteChainEntry(
                    concept_id=uuid4(),
                    concept_name="Chirality",
                    path="chem.organic.chirality",
                    difficulty=0.7,
                    mastery_score=0.4,
                    mastery_adequate=False,
                    depth=1,
                )
            ],
        )
        data = resp.model_dump()
        assert data["chain"][0]["depth"] == 1
        assert data["chain"][0]["mastery_adequate"] is False

    def test_mastery_update_request_validates_bounds(self):
        from pydantic import ValidationError

        from app.models import MasteryUpdateRequest
        req = MasteryUpdateRequest(mastery_score=0.75)
        assert req.mastery_score == 0.75

        with pytest.raises(ValidationError):
            MasteryUpdateRequest(mastery_score=1.5)

        with pytest.raises(ValidationError):
            MasteryUpdateRequest(mastery_score=-0.1)
