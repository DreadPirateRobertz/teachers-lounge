"""HTTP-layer tests for the spaced-repetition scheduler (tl-5wz).

Covers both endpoints:
  POST /v1/spaced-repetition/review-result
  GET  /v1/spaced-repetition/due

Authentication, the DB session, and the UTC clock are replaced through
FastAPI ``dependency_overrides`` so no real Postgres or wall-clock time is
required. Freezing the clock is how we assert deterministic ``due_at``
values in the response payloads.
"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone
from unittest.mock import AsyncMock, MagicMock
from uuid import UUID, uuid4

import pytest
from httpx import AsyncClient
from httpx._transports.asgi import ASGITransport

from app.auth import JWTClaims, require_auth
from app.clock import _utc_now, get_clock
from app.database import get_db
from app.main import app
from app.orm import ConceptReviewSchedule

# ── Shared constants ──────────────────────────────────────────────────────────

STUDENT_ID = uuid4()
CONCEPT = "chemistry.organic.stereochemistry"
FROZEN_NOW = datetime(2026, 4, 16, 12, 0, 0, tzinfo=timezone.utc)


# ── Helpers ───────────────────────────────────────────────────────────────────


def _fake_claims(user_id: UUID = STUDENT_ID) -> JWTClaims:
    """Return a minimal JWTClaims for the given user."""
    return JWTClaims(
        user_id=user_id,
        email="student@test.com",
        account_type="standard",
        sub_status="active",
    )


def _override_auth(user_id: UUID = STUDENT_ID) -> JWTClaims:
    """Install a fake JWT claim so protected routes see an authenticated user."""
    claims = _fake_claims(user_id)
    app.dependency_overrides[require_auth] = lambda: claims
    return claims


def _override_db(mock_session) -> None:
    """Install a DB override yielding the supplied mock session."""

    async def _fake_db():
        yield mock_session

    app.dependency_overrides[get_db] = _fake_db


def _override_clock(frozen: datetime = FROZEN_NOW) -> None:
    """Install a clock override returning a fixed UTC datetime."""
    app.dependency_overrides[get_clock] = lambda: (lambda: frozen)


def _clear_overrides() -> None:
    """Remove every dependency override installed by these tests."""
    for dep in (require_auth, get_db, get_clock):
        app.dependency_overrides.pop(dep, None)


def _make_schedule_row(
    concept_id: str = CONCEPT,
    ease_factor: float = 2.5,
    interval_days: int = 6,
    repetitions: int = 2,
    last_reviewed_at: datetime | None = None,
    due_at: datetime | None = None,
) -> MagicMock:
    """Return a MagicMock mimicking a ConceptReviewSchedule ORM row."""
    row = MagicMock(spec=ConceptReviewSchedule)
    row.user_id = STUDENT_ID
    row.concept_id = concept_id
    row.ease_factor = ease_factor
    row.interval_days = interval_days
    row.repetitions = repetitions
    row.last_reviewed_at = last_reviewed_at
    row.due_at = due_at
    return row


def _empty_lookup_result() -> MagicMock:
    """A select() result whose scalar_one_or_none() returns None (no row found)."""
    result = MagicMock()
    result.scalar_one_or_none.return_value = None
    return result


def _single_lookup_result(row) -> MagicMock:
    """A select() result whose scalar_one_or_none() returns the given row."""
    result = MagicMock()
    result.scalar_one_or_none.return_value = row
    return result


def _scalars_list_result(rows) -> MagicMock:
    """A select() result whose scalars().all() returns the given list."""
    result = MagicMock()
    result.scalars.return_value.all.return_value = rows
    return result


def _scalar_count_result(count: int) -> MagicMock:
    """A count() result whose scalar_one() returns the given integer."""
    result = MagicMock()
    result.scalar_one.return_value = count
    return result


# ── Clock dependency ──────────────────────────────────────────────────────────


class TestClockDependency:
    """Direct tests for the injectable clock module."""

    def test_get_clock_returns_utc_now_by_default(self):
        """Production get_clock returns the _utc_now callable."""
        clock = get_clock()
        assert clock is _utc_now

    def test_utc_now_returns_timezone_aware_datetime(self):
        """_utc_now yields a tz-aware UTC datetime."""
        value = _utc_now()
        assert value.tzinfo is not None
        assert value.tzinfo.utcoffset(value) == timedelta(0)


# ── POST /v1/spaced-repetition/review-result ─────────────────────────────────


class TestRecordReviewResult:
    """Tests for POST /v1/spaced-repetition/review-result."""

    def setup_method(self):
        _override_auth()
        _override_clock()

    def teardown_method(self):
        _clear_overrides()

    @pytest.mark.asyncio
    async def test_creates_schedule_on_first_passing_review(self):
        """A first-time passing review (quality=5) creates a row with interval=1."""
        db = AsyncMock()
        db.execute = AsyncMock(return_value=_empty_lookup_result())
        db.add = MagicMock()  # SQLAlchemy add() is sync; keep the mock sync too.
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.post(
                "/v1/spaced-repetition/review-result",
                json={"concept_id": CONCEPT, "quality": 5},
            )

        assert resp.status_code == 200
        body = resp.json()
        # First successful review → interval of 1 day, repetitions advances to 1.
        assert body["concept_id"] == CONCEPT
        assert body["interval_days"] == 1
        assert body["repetitions"] == 1
        # Due date is last_reviewed + interval, computed against the frozen clock.
        expected_due = (FROZEN_NOW + timedelta(days=1)).isoformat().replace("+00:00", "Z")
        parsed_due = datetime.fromisoformat(body["due_at"].replace("Z", "+00:00"))
        assert parsed_due == FROZEN_NOW + timedelta(days=1)
        parsed_reviewed = datetime.fromisoformat(body["last_reviewed_at"].replace("Z", "+00:00"))
        assert parsed_reviewed == FROZEN_NOW
        # Ease factor for quality=5 rises from 2.5 by +0.1 → 2.6.
        assert body["ease_factor"] == pytest.approx(2.6)
        # New row was added and committed.
        db.add.assert_called_once()
        db.commit.assert_awaited_once()
        _ = expected_due  # kept for clarity; assertion already compares parsed value

    @pytest.mark.asyncio
    async def test_failing_review_resets_repetitions_on_existing_row(self):
        """Quality < 3 on an established row restarts from interval=1, reps=0."""
        row = _make_schedule_row(
            ease_factor=2.7, interval_days=10, repetitions=4, due_at=FROZEN_NOW
        )
        db = AsyncMock()
        db.execute = AsyncMock(return_value=_single_lookup_result(row))
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.post(
                "/v1/spaced-repetition/review-result",
                json={"concept_id": CONCEPT, "quality": 1},
            )

        assert resp.status_code == 200
        body = resp.json()
        assert body["interval_days"] == 1
        assert body["repetitions"] == 0
        # SM-2 resets interval/reps but retains the existing ease factor on failure.
        assert body["ease_factor"] == pytest.approx(2.7)
        # Due date lands one day after the frozen clock.
        parsed_due = datetime.fromisoformat(body["due_at"].replace("Z", "+00:00"))
        assert parsed_due == FROZEN_NOW + timedelta(days=1)
        # Row was updated in place, not re-added.
        db.add.assert_not_called()
        db.commit.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_second_passing_review_moves_to_six_day_interval(self):
        """SM-2 second-success path yields interval=6 regardless of existing interval."""
        row = _make_schedule_row(
            ease_factor=2.5, interval_days=1, repetitions=1, due_at=FROZEN_NOW
        )
        db = AsyncMock()
        db.execute = AsyncMock(return_value=_single_lookup_result(row))
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.post(
                "/v1/spaced-repetition/review-result",
                json={"concept_id": CONCEPT, "quality": 4},
            )

        assert resp.status_code == 200
        body = resp.json()
        assert body["interval_days"] == 6
        assert body["repetitions"] == 2
        parsed_due = datetime.fromisoformat(body["due_at"].replace("Z", "+00:00"))
        assert parsed_due == FROZEN_NOW + timedelta(days=6)

    @pytest.mark.asyncio
    async def test_invalid_quality_returns_422(self):
        """Quality outside the 0-5 range is rejected by the Pydantic validator."""
        db = AsyncMock()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.post(
                "/v1/spaced-repetition/review-result",
                json={"concept_id": CONCEPT, "quality": 9},
            )
        assert resp.status_code == 422

    @pytest.mark.asyncio
    async def test_empty_concept_id_returns_422(self):
        """Empty concept_id is rejected by the min_length=1 constraint."""
        db = AsyncMock()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.post(
                "/v1/spaced-repetition/review-result",
                json={"concept_id": "", "quality": 3},
            )
        assert resp.status_code == 422

    @pytest.mark.asyncio
    async def test_requires_authentication(self):
        """Without a valid bearer token the endpoint returns 403/401."""
        # Remove the auth override so the real JWT dependency runs.
        app.dependency_overrides.pop(require_auth, None)
        db = AsyncMock()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.post(
                "/v1/spaced-repetition/review-result",
                json={"concept_id": CONCEPT, "quality": 3},
            )

        # HTTPBearer raises 403 when the header is missing; either indicates
        # the route refused the unauthenticated caller.
        assert resp.status_code in (401, 403)


# ── GET /v1/spaced-repetition/due ─────────────────────────────────────────────


class TestListDue:
    """Tests for GET /v1/spaced-repetition/due."""

    def setup_method(self):
        _override_auth()
        _override_clock()

    def teardown_method(self):
        _clear_overrides()

    @pytest.mark.asyncio
    async def test_empty_due_list(self):
        """When no schedule rows are due the response is empty with total_due=0."""
        db = AsyncMock()
        db.execute = AsyncMock(
            side_effect=[_scalars_list_result([]), _scalar_count_result(0)]
        )
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/spaced-repetition/due")

        assert resp.status_code == 200
        body = resp.json()
        assert body == {"items": [], "total_due": 0}

    @pytest.mark.asyncio
    async def test_overdue_row_flagged_as_overdue(self):
        """A schedule whose due_at is before the frozen clock is flagged overdue."""
        overdue = FROZEN_NOW - timedelta(days=2)
        row = _make_schedule_row(
            interval_days=3,
            repetitions=2,
            last_reviewed_at=overdue - timedelta(days=3),
            due_at=overdue,
        )
        db = AsyncMock()
        db.execute = AsyncMock(
            side_effect=[_scalars_list_result([row]), _scalar_count_result(1)]
        )
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/spaced-repetition/due")

        assert resp.status_code == 200
        body = resp.json()
        assert body["total_due"] == 1
        assert len(body["items"]) == 1
        item = body["items"][0]
        assert item["concept_id"] == CONCEPT
        assert item["is_overdue"] is True
        assert item["repetitions"] == 2

    @pytest.mark.asyncio
    async def test_never_reviewed_row_is_due_not_overdue(self):
        """Rows with NULL due_at are due (new concepts) but not considered overdue."""
        row = _make_schedule_row(
            interval_days=1,
            repetitions=0,
            last_reviewed_at=None,
            due_at=None,
        )
        db = AsyncMock()
        db.execute = AsyncMock(
            side_effect=[_scalars_list_result([row]), _scalar_count_result(1)]
        )
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/spaced-repetition/due")

        assert resp.status_code == 200
        body = resp.json()
        item = body["items"][0]
        assert item["is_overdue"] is False
        assert item["due_at"] is None
        assert item["last_reviewed_at"] is None

    @pytest.mark.asyncio
    async def test_limit_is_passed_through_to_query(self):
        """The limit query parameter is honoured (SQL LIMIT applied by the DB)."""
        rows = [
            _make_schedule_row(concept_id=f"concept-{i}", due_at=None) for i in range(3)
        ]
        db = AsyncMock()
        db.execute = AsyncMock(
            side_effect=[_scalars_list_result(rows), _scalar_count_result(10)]
        )
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/spaced-repetition/due?limit=3")

        assert resp.status_code == 200
        body = resp.json()
        assert len(body["items"]) == 3
        assert body["total_due"] == 10

    @pytest.mark.asyncio
    async def test_clock_override_controls_overdue_classification(self):
        """Swapping the clock to before a row's due_at flips is_overdue to False."""
        due = FROZEN_NOW + timedelta(days=1)  # one day after the default frozen clock
        row = _make_schedule_row(due_at=due, repetitions=1)
        db = AsyncMock()
        db.execute = AsyncMock(
            side_effect=[_scalars_list_result([row]), _scalar_count_result(1)]
        )
        _override_db(db)

        # Default frozen clock is before due_at → not overdue.
        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/spaced-repetition/due")
        body = resp.json()
        assert body["items"][0]["is_overdue"] is False

        # Advance the clock past due_at and re-query → overdue.
        _override_clock(due + timedelta(hours=1))
        db.execute = AsyncMock(
            side_effect=[_scalars_list_result([row]), _scalar_count_result(1)]
        )
        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/spaced-repetition/due")
        body = resp.json()
        assert body["items"][0]["is_overdue"] is True

    @pytest.mark.asyncio
    async def test_requires_authentication(self):
        """The due endpoint refuses callers without a bearer token."""
        app.dependency_overrides.pop(require_auth, None)
        db = AsyncMock()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/spaced-repetition/due")

        assert resp.status_code in (401, 403)
