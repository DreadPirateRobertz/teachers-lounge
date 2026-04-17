"""Tests for :mod:`app.concept_graph` and its HTTP endpoints (tl-mhd)."""

from __future__ import annotations

from unittest.mock import AsyncMock, MagicMock, patch
from uuid import uuid4

import pytest
from httpx import AsyncClient
from httpx._transports.asgi import ASGITransport
from pydantic import ValidationError
from sqlalchemy.exc import IntegrityError

from app.auth import JWTClaims, require_auth
from app.concept_graph import (
    ANCESTOR_GAP_THRESHOLD,
    AncestorGap,
    create_concept,
    detect_ancestor_gaps,
    format_gap_note,
    get_ancestors,
    get_concept,
    get_concept_by_label,
    get_descendants,
)
from app.database import get_db
from app.main import app
from app.models import CreateConceptRequest
from app.orm import ConceptGraphNode

# ── Fixtures ──────────────────────────────────────────────────────────────────

STUDENT_ID = uuid4()


def _claims() -> JWTClaims:
    """Minimal authenticated user for tests."""
    return JWTClaims(
        user_id=STUDENT_ID,
        email="student@test.com",
        account_type="standard",
        sub_status="active",
    )


def _node(concept_id: str, label: str, path: str, node_id: int = 0) -> ConceptGraphNode:
    """Detached ConceptGraphNode instance for assembling fake query results."""
    n = ConceptGraphNode(
        concept_id=concept_id, label=label, subject="chemistry", path=path
    )
    n.id = node_id
    return n


def _scalar_result(value):
    """A result object whose ``.scalar_one_or_none()`` returns ``value``."""
    r = MagicMock()
    r.scalar_one_or_none = MagicMock(return_value=value)
    return r


def _mappings_result(rows):
    """A result object whose ``.mappings().all()`` returns ``rows`` (list of dicts)."""
    r = MagicMock()
    mappings = MagicMock()
    mappings.all = MagicMock(return_value=list(rows))
    r.mappings = MagicMock(return_value=mappings)
    return r


# ── Pure helpers ──────────────────────────────────────────────────────────────


def test_format_gap_note_empty_when_no_gaps():
    """No gaps → empty string (caller uses truthiness to skip injection)."""
    assert format_gap_note("Chirality", []) == ""


def test_format_gap_note_singular_phrasing():
    """One gap uses the spec's singular sentence form."""
    gap = AncestorGap(
        concept_id="stereochemistry",
        label="Stereochemistry",
        path="chemistry.organic.stereochemistry",
        mastery_score=0.1,
    )
    note = format_gap_note("Chirality", [gap])
    assert note == (
        "Prerequisite gap detected: student has not mastered Stereochemistry "
        "which is required for Chirality."
    )


def test_format_gap_note_plural_phrasing():
    """Multiple gaps are joined into one compound sentence."""
    gaps = [
        AncestorGap("organic_chem", "Organic Chemistry", "chemistry.organic", 0.1),
        AncestorGap(
            "stereochemistry",
            "Stereochemistry",
            "chemistry.organic.stereochemistry",
            0.2,
        ),
    ]
    note = format_gap_note("Chirality", gaps)
    assert "Organic Chemistry" in note
    assert "Stereochemistry" in note
    assert "required for Chirality" in note
    assert "gaps detected" in note


def test_ancestor_gap_threshold_matches_spec():
    """Spec tl-mhd fixes the gap threshold at 0.4."""
    assert ANCESTOR_GAP_THRESHOLD == 0.4


# ── get_concept / get_concept_by_label ────────────────────────────────────────


@pytest.mark.asyncio
async def test_get_concept_returns_row():
    """Basic pk-by-slug lookup returns the matching node."""
    node = _node("chirality", "Chirality", "chemistry.organic.stereochemistry.chirality")
    session = MagicMock()
    session.execute = AsyncMock(return_value=_scalar_result(node))

    out = await get_concept(session, "chirality")

    assert out is node
    session.execute.assert_awaited_once()


@pytest.mark.asyncio
async def test_get_concept_returns_none_when_missing():
    """Unknown slug → None, not an exception."""
    session = MagicMock()
    session.execute = AsyncMock(return_value=_scalar_result(None))
    assert await get_concept(session, "nope") is None


@pytest.mark.asyncio
async def test_get_concept_by_label_case_insensitive():
    """Label lookup uses ILIKE so UI-displayed names match regardless of case."""
    node = _node("chirality", "Chirality", "chemistry.organic.stereochemistry.chirality")
    session = MagicMock()
    session.execute = AsyncMock(return_value=_scalar_result(node))

    out = await get_concept_by_label(session, "chirality")
    assert out is node


# ── get_ancestors / get_descendants (raw SQL mocked) ──────────────────────────


@pytest.mark.asyncio
async def test_get_ancestors_returns_nodes_in_depth_order():
    """Ancestor query results are rehydrated into ConceptGraphNode rows."""
    rows = [
        {
            "id": 3,
            "concept_id": "organic_chem",
            "label": "Organic Chemistry",
            "subject": "chemistry",
            "path": "chemistry.organic",
        },
        {
            "id": 4,
            "concept_id": "stereochemistry",
            "label": "Stereochemistry",
            "subject": "chemistry",
            "path": "chemistry.organic.stereochemistry",
        },
    ]
    session = MagicMock()
    session.execute = AsyncMock(return_value=_mappings_result(rows))

    result = await get_ancestors(session, "chirality")

    labels = [n.label for n in result]
    assert labels == ["Organic Chemistry", "Stereochemistry"]
    # Rehydrated rows keep their DB primary keys.
    assert [n.id for n in result] == [3, 4]


@pytest.mark.asyncio
async def test_get_descendants_delegates_to_ltree_query():
    """Descendants use the same mapping rehydration as ancestors."""
    rows = [
        {
            "id": 10,
            "concept_id": "chirality",
            "label": "Chirality",
            "subject": "chemistry",
            "path": "chemistry.organic.stereochemistry.chirality",
        }
    ]
    session = MagicMock()
    session.execute = AsyncMock(return_value=_mappings_result(rows))

    result = await get_descendants(session, "stereochemistry")
    assert [n.concept_id for n in result] == ["chirality"]


# ── detect_ancestor_gaps ──────────────────────────────────────────────────────


@pytest.mark.asyncio
async def test_detect_ancestor_gaps_flags_unknown_mastery_as_zero():
    """An ancestor absent from the mastery map defaults to 0.0 → counted as a gap."""
    ancestor = _node("organic_chem", "Organic Chemistry", "chemistry.organic")

    with patch(
        "app.concept_graph.get_ancestors", new=AsyncMock(return_value=[ancestor])
    ):
        gaps = await detect_ancestor_gaps(MagicMock(), "chirality", mastery={})

    assert len(gaps) == 1
    assert gaps[0].mastery_score == 0.0
    assert gaps[0].concept_id == "organic_chem"


@pytest.mark.asyncio
async def test_detect_ancestor_gaps_applies_threshold():
    """Mastery ≥ threshold filters out ancestors; < threshold surfaces them."""
    ancestors = [
        _node("organic_chem", "Organic Chemistry", "chemistry.organic"),
        _node("stereochemistry", "Stereochemistry", "chemistry.organic.stereochemistry"),
    ]
    mastery = {
        "Organic Chemistry": 0.9,  # mastered — no gap
        "Stereochemistry": 0.1,  # gap
    }

    with patch(
        "app.concept_graph.get_ancestors", new=AsyncMock(return_value=ancestors)
    ):
        gaps = await detect_ancestor_gaps(MagicMock(), "chirality", mastery=mastery)

    assert [g.concept_id for g in gaps] == ["stereochemistry"]
    assert gaps[0].mastery_score == pytest.approx(0.1)


@pytest.mark.asyncio
async def test_detect_ancestor_gaps_respects_custom_threshold():
    """Callers can tighten or relax the gap threshold."""
    ancestor = _node("organic_chem", "Organic Chemistry", "chemistry.organic")

    with patch(
        "app.concept_graph.get_ancestors", new=AsyncMock(return_value=[ancestor])
    ):
        # With default threshold 0.4, mastery 0.5 is not a gap.
        no_gap = await detect_ancestor_gaps(
            MagicMock(), "chirality", mastery={"Organic Chemistry": 0.5}
        )
        # Raise threshold to 0.6 → becomes a gap.
        gap = await detect_ancestor_gaps(
            MagicMock(), "chirality", mastery={"Organic Chemistry": 0.5}, threshold=0.6
        )

    assert no_gap == []
    assert len(gap) == 1


# ── create_concept ────────────────────────────────────────────────────────────


@pytest.mark.asyncio
async def test_create_concept_inserts_and_flushes():
    """Creating a concept adds the row and flushes to populate its id."""
    added: list = []
    session = MagicMock()
    session.add = lambda row: added.append(row)
    session.flush = AsyncMock()

    node = await create_concept(
        session,
        concept_id="aldol_condensation",
        label="Aldol Condensation",
        subject="chemistry",
        path="chemistry.organic.advanced.aldol_condensation",
    )

    session.flush.assert_awaited_once()
    assert added == [node]
    assert node.concept_id == "aldol_condensation"


# ── HTTP endpoints ────────────────────────────────────────────────────────────


def _override_db(session):
    async def _db_override():
        yield session

    app.dependency_overrides[get_db] = _db_override


def _override_auth():
    app.dependency_overrides[require_auth] = _claims


def _clear_overrides():
    app.dependency_overrides.pop(get_db, None)
    app.dependency_overrides.pop(require_auth, None)


@pytest.mark.asyncio
async def test_prerequisites_endpoint_returns_ancestors():
    """GET /v1/concepts/{id}/prerequisites returns ltree ancestor rows."""
    target = _node("chirality", "Chirality", "chemistry.organic.stereochemistry.chirality")
    ancestor_rows = [
        {
            "id": 1,
            "concept_id": "organic_chem",
            "label": "Organic Chemistry",
            "subject": "chemistry",
            "path": "chemistry.organic",
        },
        {
            "id": 2,
            "concept_id": "stereochemistry",
            "label": "Stereochemistry",
            "subject": "chemistry",
            "path": "chemistry.organic.stereochemistry",
        },
    ]

    # First execute: resolve target. Second execute: ancestor query.
    execute = AsyncMock(
        side_effect=[
            _scalar_result(target),
            _mappings_result(ancestor_rows),
        ]
    )
    session = MagicMock()
    session.execute = execute
    _override_db(session)
    _override_auth()
    try:
        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://t") as client:
            resp = await client.get("/v1/concepts/chirality/prerequisites")
    finally:
        _clear_overrides()

    assert resp.status_code == 200
    body = resp.json()
    assert [row["concept_id"] for row in body] == ["organic_chem", "stereochemistry"]
    assert [row["label"] for row in body] == ["Organic Chemistry", "Stereochemistry"]


@pytest.mark.asyncio
async def test_prerequisites_endpoint_404s_for_missing_concept():
    """Unknown target concept → 404."""
    session = MagicMock()
    session.execute = AsyncMock(return_value=_scalar_result(None))
    _override_db(session)
    _override_auth()
    try:
        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://t") as client:
            resp = await client.get("/v1/concepts/nope/prerequisites")
    finally:
        _clear_overrides()

    assert resp.status_code == 404


@pytest.mark.asyncio
async def test_dependents_endpoint_returns_descendants():
    """GET /v1/concepts/{id}/dependents returns ltree descendant rows."""
    target = _node("stereochemistry", "Stereochemistry", "chemistry.organic.stereochemistry")
    descendants = [
        {
            "id": 7,
            "concept_id": "chirality",
            "label": "Chirality",
            "subject": "chemistry",
            "path": "chemistry.organic.stereochemistry.chirality",
        }
    ]
    session = MagicMock()
    session.execute = AsyncMock(
        side_effect=[_scalar_result(target), _mappings_result(descendants)]
    )
    _override_db(session)
    _override_auth()
    try:
        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://t") as client:
            resp = await client.get("/v1/concepts/stereochemistry/dependents")
    finally:
        _clear_overrides()

    assert resp.status_code == 200
    assert [r["concept_id"] for r in resp.json()] == ["chirality"]


@pytest.mark.asyncio
async def test_post_concept_inserts_and_returns_201():
    """POST /v1/concepts returns 201 with the inserted row."""
    session = MagicMock()
    session.add = MagicMock()
    session.flush = AsyncMock()
    session.commit = AsyncMock()
    session.rollback = AsyncMock()
    _override_db(session)
    _override_auth()
    try:
        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://t") as client:
            resp = await client.post(
                "/v1/concepts",
                json={
                    "concept_id": "friedel_crafts",
                    "label": "Friedel–Crafts Alkylation",
                    "subject": "chemistry",
                    "path": "chemistry.organic.mechanisms.friedel_crafts",
                },
            )
    finally:
        _clear_overrides()

    assert resp.status_code == 201
    session.commit.assert_awaited_once()
    body = resp.json()
    assert body["concept_id"] == "friedel_crafts"
    assert body["subject"] == "chemistry"


@pytest.mark.asyncio
async def test_post_concept_returns_409_on_duplicate():
    """POST with a duplicate concept_id surfaces a 409 after rollback."""
    session = MagicMock()
    session.add = MagicMock()
    session.flush = AsyncMock(side_effect=IntegrityError("stmt", {}, Exception("dup")))
    session.commit = AsyncMock()
    session.rollback = AsyncMock()
    _override_db(session)
    _override_auth()
    try:
        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://t") as client:
            resp = await client.post(
                "/v1/concepts",
                json={
                    "concept_id": "chirality",
                    "label": "Chirality",
                    "subject": "chemistry",
                    "path": "chemistry.organic.stereochemistry.chirality",
                },
            )
    finally:
        _clear_overrides()

    assert resp.status_code == 409
    session.rollback.assert_awaited_once()
    session.commit.assert_not_called()


# ── CreateConceptRequest ltree path validator ─────────────────────────────────


def test_create_concept_request_accepts_valid_ltree_path():
    """Canonical dot-separated lowercase slugs pass validation."""
    req = CreateConceptRequest(
        concept_id="chirality",
        label="Chirality",
        subject="chemistry",
        path="chemistry.organic.stereochemistry.chirality",
    )
    assert req.path == "chemistry.organic.stereochemistry.chirality"


def test_create_concept_request_accepts_single_segment_path():
    """Root-level paths (no dots) are valid ltree labels."""
    req = CreateConceptRequest(
        concept_id="chemistry", label="Chemistry", subject="chemistry", path="chemistry"
    )
    assert req.path == "chemistry"


@pytest.mark.parametrize(
    "bad_path",
    [
        "Chemistry.Organic",  # uppercase
        "chemistry..organic",  # empty segment
        ".chemistry.organic",  # leading dot
        "chemistry.organic.",  # trailing dot
        "1chemistry.organic",  # leading digit
        "chemistry-organic",  # hyphen (not underscore)
        "chemistry organic",  # space
        "chemistry.organic/stereo",  # slash
    ],
)
def test_create_concept_request_rejects_invalid_ltree_paths(bad_path):
    """Paths that Postgres ltree would reject must fail validation client-side."""
    with pytest.raises(ValidationError):
        CreateConceptRequest(
            concept_id="x", label="X", subject="chemistry", path=bad_path
        )
