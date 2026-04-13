"""HTTP-layer and unit tests for session-summary computation.

Covers ``compute_session_summary`` in ``app.summary`` and the
``GET /v1/sessions/{id}/summary`` endpoint wired through ``app.sessions``.

All tests use FastAPI dependency overrides for auth and the database, so
no real Postgres is needed.
"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone
from unittest.mock import AsyncMock, MagicMock
from uuid import UUID, uuid4

import pytest
from httpx import AsyncClient
from httpx._transports.asgi import ASGITransport

from app.auth import JWTClaims, require_auth
from app.database import get_db
from app.main import app
from app.summary import SessionSummary, compute_session_summary

# ── Shared constants ──────────────────────────────────────────────────────────

STUDENT_ID = uuid4()
OTHER_USER_ID = uuid4()
SESSION_ID = uuid4()
NOW = datetime.now(timezone.utc)


# ── Helpers ───────────────────────────────────────────────────────────────────


def _fake_claims(user_id: UUID = STUDENT_ID) -> JWTClaims:
    return JWTClaims(
        user_id=user_id,
        email="student@test.com",
        account_type="standard",
        sub_status="active",
    )


def _fake_session(
    session_id: UUID = SESSION_ID,
    user_id: UUID = STUDENT_ID,
    created_at: datetime | None = None,
    updated_at: datetime | None = None,
) -> MagicMock:
    """Return a MagicMock mimicking a :class:`app.orm.Session` row."""
    created = created_at or (NOW - timedelta(minutes=5))
    row = MagicMock()
    row.id = session_id
    row.user_id = user_id
    row.course_id = None
    row.created_at = created
    row.updated_at = updated_at or created
    return row


def _fake_interaction(
    role: str,
    created_at: datetime,
    response_time_ms: int | None = None,
) -> MagicMock:
    """Return a MagicMock mimicking an :class:`app.orm.Interaction` row."""
    row = MagicMock()
    row.id = uuid4()
    row.session_id = SESSION_ID
    row.user_id = STUDENT_ID
    row.role = role
    row.content = "x"
    row.response_time_ms = response_time_ms
    row.created_at = created_at
    return row


def _db_with_results(*results: list) -> AsyncMock:
    """Build an AsyncMock DB whose ``execute`` returns the given result lists in order.

    Each positional argument is a list that ``result.scalars().all()`` will yield
    for one successive ``db.execute()`` call.
    """
    db = AsyncMock()
    mock_results = []
    for items in results:
        m = MagicMock()
        m.scalars.return_value.all.return_value = items
        mock_results.append(m)
    db.execute = AsyncMock(side_effect=mock_results)
    return db


def _override_auth(user_id: UUID = STUDENT_ID) -> JWTClaims:
    claims = _fake_claims(user_id)
    app.dependency_overrides[require_auth] = lambda: claims
    return claims


def _override_db(db) -> None:
    async def _fake_db():
        yield db

    app.dependency_overrides[get_db] = _fake_db


def _clear_overrides() -> None:
    app.dependency_overrides.pop(require_auth, None)
    app.dependency_overrides.pop(get_db, None)


# ── Unit tests for compute_session_summary ────────────────────────────────────


class TestComputeSessionSummary:
    """Direct unit tests that bypass the HTTP layer."""

    @pytest.mark.asyncio
    async def test_empty_session_zero_counts(self):
        """A session with no interactions yields zero counts and no topics."""
        session = _fake_session()
        db = _db_with_results([], [])

        result = await compute_session_summary(db, session)

        assert isinstance(result, SessionSummary)
        assert result.session_id == SESSION_ID
        assert result.message_count == 0
        assert result.avg_response_time_ms is None
        assert result.topics == []
        assert result.session_duration_seconds >= 0

    @pytest.mark.asyncio
    async def test_counts_messages_and_averages_tutor_latency(self):
        """Averages only tutor interactions with a recorded response_time_ms."""
        session = _fake_session(created_at=NOW - timedelta(minutes=10))
        interactions = [
            _fake_interaction("student", NOW - timedelta(minutes=9)),
            _fake_interaction("tutor", NOW - timedelta(minutes=8), response_time_ms=400),
            _fake_interaction("student", NOW - timedelta(minutes=7)),
            _fake_interaction("tutor", NOW - timedelta(minutes=6), response_time_ms=800),
        ]
        db = _db_with_results(interactions, [])

        result = await compute_session_summary(db, session)

        assert result.message_count == 4
        assert result.avg_response_time_ms == pytest.approx(600.0)

    @pytest.mark.asyncio
    async def test_skips_tutor_interactions_without_latency(self):
        """Tutor rows with response_time_ms=None are excluded from the mean."""
        session = _fake_session()
        interactions = [
            _fake_interaction("tutor", NOW, response_time_ms=None),
            _fake_interaction("tutor", NOW, response_time_ms=300),
        ]
        db = _db_with_results(interactions, [])

        result = await compute_session_summary(db, session)

        assert result.avg_response_time_ms == pytest.approx(300.0)

    @pytest.mark.asyncio
    async def test_duration_spans_created_to_latest_interaction(self):
        """Duration equals latest interaction timestamp minus session created_at."""
        created = NOW - timedelta(minutes=10)
        latest = NOW - timedelta(minutes=2)
        session = _fake_session(created_at=created)
        interactions = [
            _fake_interaction("student", NOW - timedelta(minutes=9)),
            _fake_interaction("tutor", latest, response_time_ms=100),
        ]
        db = _db_with_results(interactions, [])

        result = await compute_session_summary(db, session)

        assert result.session_duration_seconds == pytest.approx(
            (latest - created).total_seconds()
        )

    @pytest.mark.asyncio
    async def test_topics_are_sorted_and_deduplicated(self):
        """Topic list is sorted alphabetically with duplicates collapsed."""
        session = _fake_session()
        interactions = [_fake_interaction("student", NOW)]
        db = _db_with_results(interactions, ["Stereochemistry", "Atomic Structure", "Stereochemistry"])

        result = await compute_session_summary(db, session)

        assert result.topics == ["Atomic Structure", "Stereochemistry"]


# ── HTTP endpoint tests ───────────────────────────────────────────────────────


class TestSessionSummaryEndpoint:
    """Tests for ``GET /v1/sessions/{id}/summary``."""

    def setup_method(self):
        _override_auth()

    def teardown_method(self):
        _clear_overrides()

    @pytest.mark.asyncio
    async def test_happy_path_returns_summary(self, monkeypatch):
        """Owner fetches summary successfully — 200 with expected payload."""
        session = _fake_session()
        db = _db_with_results(
            [_fake_interaction("tutor", NOW, response_time_ms=250)],
            ["Bonding"],
        )
        _override_db(db)

        monkeypatch.setattr("app.sessions.get_session", AsyncMock(return_value=session))

        async with AsyncClient(
            transport=ASGITransport(app=app), base_url="http://test"
        ) as client:
            resp = await client.get(f"/v1/sessions/{SESSION_ID}/summary")

        assert resp.status_code == 200
        body = resp.json()
        assert body["session_id"] == str(SESSION_ID)
        assert body["message_count"] == 1
        assert body["avg_response_time_ms"] == pytest.approx(250.0)
        assert body["topics"] == ["Bonding"]
        assert body["session_duration_seconds"] >= 0

    @pytest.mark.asyncio
    async def test_missing_session_returns_404(self, monkeypatch):
        """Unknown session id returns 404."""
        _override_db(AsyncMock())
        monkeypatch.setattr("app.sessions.get_session", AsyncMock(return_value=None))

        async with AsyncClient(
            transport=ASGITransport(app=app), base_url="http://test"
        ) as client:
            resp = await client.get(f"/v1/sessions/{uuid4()}/summary")

        assert resp.status_code == 404

    @pytest.mark.asyncio
    async def test_other_user_session_returns_403(self, monkeypatch):
        """A session owned by a different user yields 403 Forbidden."""
        other_session = _fake_session(user_id=OTHER_USER_ID)
        _override_db(AsyncMock())
        monkeypatch.setattr(
            "app.sessions.get_session", AsyncMock(return_value=other_session)
        )

        async with AsyncClient(
            transport=ASGITransport(app=app), base_url="http://test"
        ) as client:
            resp = await client.get(f"/v1/sessions/{SESSION_ID}/summary")

        assert resp.status_code == 403

    @pytest.mark.asyncio
    async def test_requires_auth(self):
        """Without the auth override installed the route rejects unauthenticated calls."""
        # Explicitly remove the auth override to simulate an unauthenticated client.
        app.dependency_overrides.pop(require_auth, None)

        async with AsyncClient(
            transport=ASGITransport(app=app), base_url="http://test"
        ) as client:
            resp = await client.get(f"/v1/sessions/{SESSION_ID}/summary")

        assert resp.status_code in (401, 403)
