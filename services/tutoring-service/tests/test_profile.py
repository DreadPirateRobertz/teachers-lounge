"""HTTP-layer tests for the learning-profile and misconception routes.

Covers:
  GET  /v1/students/me/learning-profile
  PATCH /v1/students/me/learning-profile
  GET  /v1/students/me/misconceptions
  POST /v1/students/me/misconceptions/{concept_id}
  PATCH /v1/students/me/misconceptions/{misconception_id}/resolve

All tests use FastAPI dependency overrides for auth + DB — no real Postgres needed.
"""
from __future__ import annotations

from datetime import datetime, timedelta, timezone
from unittest.mock import AsyncMock, MagicMock, patch
from uuid import UUID, uuid4

import pytest
from httpx import AsyncClient
from httpx._transports.asgi import ASGITransport

from app.auth import JWTClaims, require_auth
from app.database import get_db
from app.main import app

# ── Shared constants ──────────────────────────────────────────────────────────

STUDENT_ID = uuid4()
CONCEPT_ID = uuid4()
NOW = datetime(2026, 4, 5, 12, 0, 0, tzinfo=timezone.utc)


# ── Helpers ───────────────────────────────────────────────────────────────────

def _fake_claims(user_id: UUID = STUDENT_ID) -> JWTClaims:
    """Return minimal JWTClaims for tests."""
    return JWTClaims(
        user_id=user_id,
        email="student@test.com",
        account_type="standard",
        sub_status="active",
    )


def _make_profile_row(
    active_reflective: float = 0.0,
    sensing_intuitive: float = 0.0,
    visual_verbal: float = -0.5,
    sequential_global: float = 0.3,
) -> MagicMock:
    """Return a MagicMock mimicking a LearningProfile ORM row."""
    row = MagicMock()
    row.user_id = STUDENT_ID
    row.active_reflective = active_reflective
    row.sensing_intuitive = sensing_intuitive
    row.visual_verbal = visual_verbal
    row.sequential_global = sequential_global
    row.updated_at = NOW
    return row


def _make_misconception_row(
    description: str = "Confuses X with Y",
    confidence: float = 0.8,
) -> MagicMock:
    """Return a MagicMock mimicking a Misconception ORM row."""
    row = MagicMock()
    row.id = uuid4()
    row.user_id = STUDENT_ID
    row.concept_id = CONCEPT_ID
    row.description = description
    row.confidence = confidence
    row.recorded_at = NOW - timedelta(days=3)
    row.last_seen_at = NOW - timedelta(days=3)
    row.resolved = False
    return row


async def _fake_db_gen(mock_session):
    """Async generator wrapping a mock session for get_db override."""
    yield mock_session


def _override_auth(user_id: UUID = STUDENT_ID):
    claims = _fake_claims(user_id)
    app.dependency_overrides[require_auth] = lambda: claims
    return claims


def _override_db(mock_session):
    app.dependency_overrides[get_db] = lambda: _fake_db_gen(mock_session)
    return mock_session


def _clear_overrides():
    app.dependency_overrides.pop(require_auth, None)
    app.dependency_overrides.pop(get_db, None)


# ── GET /v1/students/me/learning-profile ─────────────────────────────────────

class TestGetLearningProfile:
    """Tests for GET /v1/students/me/learning-profile."""

    def setup_method(self):
        _override_auth()

    def teardown_method(self):
        _clear_overrides()

    @pytest.mark.asyncio
    async def test_returns_200_with_dials(self):
        """Returns 200 and a dials object with four dimensions."""
        db = AsyncMock()
        profile = _make_profile_row()
        result = MagicMock()
        result.scalar_one_or_none.return_value = profile
        db.execute = AsyncMock(return_value=result)
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/students/me/learning-profile")

        assert resp.status_code == 200
        body = resp.json()
        assert "dials" in body
        dials = body["dials"]
        assert "active_reflective" in dials
        assert "sensing_intuitive" in dials
        assert "visual_verbal" in dials
        assert "sequential_global" in dials

    @pytest.mark.asyncio
    async def test_dials_values_match_profile(self):
        """Dial values in the response match the stored profile row."""
        db = AsyncMock()
        profile = _make_profile_row(visual_verbal=-0.7, sequential_global=0.4)
        result = MagicMock()
        result.scalar_one_or_none.return_value = profile
        db.execute = AsyncMock(return_value=result)
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/students/me/learning-profile")

        dials = resp.json()["dials"]
        assert dials["visual_verbal"] == pytest.approx(-0.7)
        assert dials["sequential_global"] == pytest.approx(0.4)

    @pytest.mark.asyncio
    async def test_returns_zero_dials_when_no_profile(self):
        """Returns all-zero dials when no profile row exists (read-only, no insert)."""
        db = AsyncMock()
        db.add = MagicMock()
        result = MagicMock()
        result.scalar_one_or_none.return_value = None
        db.execute = AsyncMock(return_value=result)
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/students/me/learning-profile")

        assert resp.status_code == 200
        db.add.assert_not_called()  # get_dials never creates a profile row

    @pytest.mark.asyncio
    async def test_unauthenticated_returns_403(self):
        """Without auth the endpoint returns 401 or 403."""
        _clear_overrides()

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/students/me/learning-profile")

        assert resp.status_code in (401, 403)


# ── PATCH /v1/students/me/learning-profile ───────────────────────────────────

class TestPatchLearningProfile:
    """Tests for PATCH /v1/students/me/learning-profile."""

    def setup_method(self):
        _override_auth()

    def teardown_method(self):
        _clear_overrides()

    @pytest.mark.asyncio
    async def test_patch_updates_dials_and_returns_200(self):
        """A valid PATCH returns 200 and the updated dials."""
        db = AsyncMock()
        db.commit = AsyncMock()
        profile = _make_profile_row()
        result = MagicMock()
        result.scalar_one_or_none.return_value = profile
        db.execute = AsyncMock(return_value=result)
        _override_db(db)

        payload = {"dials": {"visual_verbal": -0.8, "active_reflective": 0.2}}

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.patch("/v1/students/me/learning-profile", json=payload)

        assert resp.status_code == 200
        db.commit.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_patch_with_empty_dials_returns_200(self):
        """An empty dials dict is valid — no-op update."""
        db = AsyncMock()
        db.commit = AsyncMock()
        profile = _make_profile_row()
        result = MagicMock()
        result.scalar_one_or_none.return_value = profile
        db.execute = AsyncMock(return_value=result)
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.patch("/v1/students/me/learning-profile", json={"dials": {}})

        assert resp.status_code == 200

    @pytest.mark.asyncio
    async def test_patch_with_out_of_range_dial_returns_422(self):
        """Dial values must be in [-1, 1]; values outside this range fail validation."""
        db = AsyncMock()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.patch(
                "/v1/students/me/learning-profile",
                json={"dials": {"visual_verbal": 2.5}},
            )

        assert resp.status_code == 422

    @pytest.mark.asyncio
    async def test_patch_with_unknown_dial_key_returns_422(self):
        """Unknown dimension keys are rejected with 422 to prevent ORM pollution."""
        db = AsyncMock()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.patch(
                "/v1/students/me/learning-profile",
                json={"dials": {"__class__": 0.5}},
            )

        assert resp.status_code == 422

    @pytest.mark.asyncio
    async def test_unauthenticated_returns_403(self):
        """Without auth the endpoint returns 401 or 403."""
        _clear_overrides()

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.patch(
                "/v1/students/me/learning-profile",
                json={"dials": {}},
            )

        assert resp.status_code in (401, 403)


# ── GET /v1/students/me/misconceptions ───────────────────────────────────────

class TestGetMisconceptions:
    """Tests for GET /v1/students/me/misconceptions."""

    def setup_method(self):
        _override_auth()

    def teardown_method(self):
        _clear_overrides()

    @pytest.mark.asyncio
    async def test_returns_list_of_misconceptions(self):
        """Returns a list of active misconceptions with recency weights."""
        db = AsyncMock()
        m = _make_misconception_row()
        result = MagicMock()
        result.scalars.return_value.all.return_value = [m]
        db.execute = AsyncMock(return_value=result)
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/students/me/misconceptions")

        assert resp.status_code == 200
        body = resp.json()
        assert isinstance(body, list)
        assert len(body) == 1
        assert "description" in body[0]
        assert "recency_weight" in body[0]

    @pytest.mark.asyncio
    async def test_empty_list_when_no_misconceptions(self):
        """Returns empty list when student has no active misconceptions."""
        db = AsyncMock()
        result = MagicMock()
        result.scalars.return_value.all.return_value = []
        db.execute = AsyncMock(return_value=result)
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/students/me/misconceptions")

        assert resp.status_code == 200
        assert resp.json() == []

    @pytest.mark.asyncio
    async def test_unauthenticated_returns_403(self):
        """Without auth the endpoint returns 401 or 403."""
        _clear_overrides()

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/students/me/misconceptions")

        assert resp.status_code in (401, 403)


# ── POST /v1/students/me/misconceptions/{concept_id} ─────────────────────────

class TestLogMisconception:
    """Tests for POST /v1/students/me/misconceptions/{concept_id}."""

    def setup_method(self):
        _override_auth()

    def teardown_method(self):
        _clear_overrides()

    def _build_db(self, concept_exists: bool = True) -> AsyncMock:
        """Build a mock DB for the add_misconception flow.

        Two sequential execute calls:
          1. Concept existence check → row or None
          2. Existing misconception upsert check → None (insert path)
        """
        db = AsyncMock()
        db.add = MagicMock()
        db.commit = AsyncMock()
        db.flush = AsyncMock()

        concept_result = MagicMock()
        concept_result.scalar_one_or_none.return_value = MagicMock() if concept_exists else None

        no_existing = MagicMock()
        no_existing.scalar_one_or_none.return_value = None

        db.execute = AsyncMock(side_effect=[concept_result, no_existing])
        return db

    @pytest.mark.asyncio
    async def test_logs_misconception_returns_201(self):
        """A valid POST creates a misconception and returns 201."""
        db = self._build_db()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.post(
                f"/v1/students/me/misconceptions/{CONCEPT_ID}",
                json={"description": "Confuses gradient with divergence"},
            )

        assert resp.status_code == 201
        db.add.assert_called_once()
        db.commit.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_missing_concept_returns_404(self):
        """POST for a non-existent concept returns 404."""
        db = self._build_db(concept_exists=False)
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.post(
                f"/v1/students/me/misconceptions/{CONCEPT_ID}",
                json={"description": "test"},
            )

        assert resp.status_code == 404

    @pytest.mark.asyncio
    async def test_missing_description_returns_422(self):
        """POST without a description body returns 422."""
        db = AsyncMock()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.post(
                f"/v1/students/me/misconceptions/{CONCEPT_ID}",
                json={},
            )

        assert resp.status_code == 422

    @pytest.mark.asyncio
    async def test_unauthenticated_returns_403(self):
        """Without auth the endpoint returns 401 or 403."""
        _clear_overrides()

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.post(
                f"/v1/students/me/misconceptions/{CONCEPT_ID}",
                json={"description": "test"},
            )

        assert resp.status_code in (401, 403)


# ── PATCH /v1/students/me/misconceptions/{misconception_id}/resolve ───────────

class TestResolveMisconception:
    """Tests for PATCH /v1/students/me/misconceptions/{misconception_id}/resolve."""

    def setup_method(self):
        _override_auth()

    def teardown_method(self):
        _clear_overrides()

    @pytest.mark.asyncio
    async def test_resolve_existing_misconception_returns_200(self):
        """Resolving an existing misconception returns 200."""
        db = AsyncMock()
        db.commit = AsyncMock()
        misc_id = uuid4()
        m = _make_misconception_row()
        m.id = misc_id
        m.resolved = False
        result = MagicMock()
        result.scalar_one_or_none.return_value = m
        db.execute = AsyncMock(return_value=result)
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.patch(
                f"/v1/students/me/misconceptions/{misc_id}/resolve"
            )

        assert resp.status_code == 200
        db.commit.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_resolve_missing_misconception_returns_404(self):
        """Resolving a non-existent misconception returns 404."""
        db = AsyncMock()
        result = MagicMock()
        result.scalar_one_or_none.return_value = None
        db.execute = AsyncMock(return_value=result)
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.patch(
                f"/v1/students/me/misconceptions/{uuid4()}/resolve"
            )

        assert resp.status_code == 404

    @pytest.mark.asyncio
    async def test_unauthenticated_returns_403(self):
        """Without auth the endpoint returns 401 or 403."""
        _clear_overrides()

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.patch(
                f"/v1/students/me/misconceptions/{uuid4()}/resolve"
            )

        assert resp.status_code in (401, 403)
