"""HTTP-layer tests for the spaced-repetition review queue routes.

Tests all three review endpoints:
  GET  /v1/reviews/queue
  POST /v1/reviews/{concept_id}/answer
  GET  /v1/reviews/stats

Uses FastAPI dependency overrides for auth (require_auth) and database
(get_db) so no real Postgres is needed.

Auth is mocked via app.dependency_overrides[require_auth] → a stub that
returns a fixed JWTClaims for user STUDENT_ID.

DB is mocked via app.dependency_overrides[get_db] → an async generator
yielding an AsyncMock session configured per test.
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
# Computed at runtime so relative offsets (NOW ± N days) are always meaningful.
NOW = datetime.now(timezone.utc)


# ── Helpers ───────────────────────────────────────────────────────────────────


def _fake_claims(user_id: UUID = STUDENT_ID) -> JWTClaims:
    """Return a minimal JWTClaims for the given user."""
    return JWTClaims(
        user_id=user_id,
        email="student@test.com",
        account_type="standard",
        sub_status="active",
    )


def _make_mastery_row(
    concept_id: UUID | None = None,
    mastery_score: float = 0.5,
    ease_factor: float = 2.5,
    interval_days: int = 6,
    repetitions: int = 2,
    next_review_at: datetime | None = None,
    last_reviewed_at: datetime | None = None,
) -> MagicMock:
    """Return a MagicMock mimicking a StudentConceptMastery ORM row."""
    row = MagicMock()
    row.concept_id = concept_id or CONCEPT_ID
    row.user_id = STUDENT_ID
    row.mastery_score = mastery_score
    row.ease_factor = ease_factor
    row.interval_days = interval_days
    row.repetitions = repetitions
    row.next_review_at = next_review_at
    row.last_reviewed_at = last_reviewed_at
    row.decay_rate = 0.1
    row.review_count = 3
    # Eagerly-loaded relationship: concept.name
    row.concept = MagicMock()
    row.concept.name = "Test Concept"
    return row


async def _fake_db_gen(mock_session):
    """Async generator wrapping a mock session for get_db override."""
    yield mock_session


def _override_auth(user_id: UUID = STUDENT_ID):
    """Install auth and return the override so it can be cleaned up."""
    claims = _fake_claims(user_id)
    app.dependency_overrides[require_auth] = lambda: claims
    return claims


def _override_db(mock_session):
    """Install DB override and return the session mock."""

    async def _fake_db():
        yield mock_session

    app.dependency_overrides[get_db] = _fake_db
    return mock_session


def _clear_overrides():
    """Remove all dependency overrides installed by this test module."""
    app.dependency_overrides.pop(require_auth, None)
    app.dependency_overrides.pop(get_db, None)


# ── GET /v1/reviews/queue ─────────────────────────────────────────────────────


class TestReviewQueue:
    """Tests for GET /v1/reviews/queue."""

    def setup_method(self):
        """Install fresh auth + DB overrides before each test."""
        _override_auth()

    def teardown_method(self):
        _clear_overrides()

    @pytest.mark.asyncio
    async def test_empty_queue_returns_zero_counts(self):
        """With no mastery rows the queue is empty and counts are zero."""
        db = AsyncMock()
        due_result = MagicMock()
        due_result.scalars.return_value.all.return_value = []
        count_zero = MagicMock()
        count_zero.scalar_one.return_value = 0
        db.execute = AsyncMock(side_effect=[due_result, count_zero, count_zero])
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/reviews/queue")

        assert resp.status_code == 200
        body = resp.json()
        assert body["items"] == []
        assert body["total_due"] == 0
        assert body["total_upcoming"] == 0

    @pytest.mark.asyncio
    async def test_overdue_concept_appears_in_items(self):
        """A concept whose next_review_at is in the past appears in the queue."""
        overdue_at = NOW - timedelta(days=1)
        row = _make_mastery_row(next_review_at=overdue_at)

        db = AsyncMock()
        result_mock = MagicMock()
        result_mock.scalars.return_value.all.return_value = [row]
        db.execute = AsyncMock(return_value=result_mock)
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/reviews/queue")

        assert resp.status_code == 200
        body = resp.json()
        assert body["total_due"] == 1
        assert len(body["items"]) == 1
        assert body["items"][0]["is_overdue"] is True

    @pytest.mark.asyncio
    async def test_never_reviewed_concept_is_due(self):
        """A concept with null next_review_at (never reviewed) counts as due."""
        row = _make_mastery_row(next_review_at=None)

        db = AsyncMock()
        result_mock = MagicMock()
        result_mock.scalars.return_value.all.return_value = [row]
        db.execute = AsyncMock(return_value=result_mock)
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/reviews/queue")

        assert resp.status_code == 200
        body = resp.json()
        assert body["total_due"] == 1

    @pytest.mark.asyncio
    async def test_future_concept_is_not_due(self):
        """A concept scheduled far in the future does not appear in due items."""
        db = AsyncMock()
        due_result = MagicMock()
        due_result.scalars.return_value.all.return_value = []  # SQL WHERE excludes future row
        count_zero = MagicMock()
        count_zero.scalar_one.return_value = 0
        db.execute = AsyncMock(side_effect=[due_result, count_zero, count_zero])
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/reviews/queue")

        assert resp.status_code == 200
        body = resp.json()
        assert body["total_due"] == 0
        assert body["items"] == []

    @pytest.mark.asyncio
    async def test_upcoming_within_7_days_counted(self):
        """A concept due within 7 days is not in items but is counted as upcoming."""
        db = AsyncMock()
        due_result = MagicMock()
        due_result.scalars.return_value.all.return_value = []  # not yet due
        upcoming_result = MagicMock()
        upcoming_result.scalar_one.return_value = 1  # 1 concept within 7-day window
        total_due_result = MagicMock()
        total_due_result.scalar_one.return_value = 0
        db.execute = AsyncMock(side_effect=[due_result, upcoming_result, total_due_result])
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/reviews/queue")

        assert resp.status_code == 200
        body = resp.json()
        assert body["total_due"] == 0
        assert body["total_upcoming"] == 1
        assert body["items"] == []

    @pytest.mark.asyncio
    async def test_queue_respects_limit_param(self):
        """The limit query parameter caps the items list length."""
        rows = [_make_mastery_row(concept_id=uuid4(), next_review_at=None) for _ in range(5)]

        db = AsyncMock()
        due_result = MagicMock()
        due_result.scalars.return_value.all.return_value = rows[:3]  # SQL LIMIT 3 applied
        upcoming_result = MagicMock()
        upcoming_result.scalar_one.return_value = 0
        total_due_result = MagicMock()
        total_due_result.scalar_one.return_value = 5
        db.execute = AsyncMock(side_effect=[due_result, upcoming_result, total_due_result])
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/reviews/queue?limit=3")

        assert resp.status_code == 200
        assert len(resp.json()["items"]) == 3

    @pytest.mark.asyncio
    async def test_queue_item_shape(self):
        """Each queue item has all required fields with correct types."""
        past = NOW - timedelta(days=2)
        row = _make_mastery_row(next_review_at=past, mastery_score=0.7)

        db = AsyncMock()
        result_mock = MagicMock()
        result_mock.scalars.return_value.all.return_value = [row]
        db.execute = AsyncMock(return_value=result_mock)
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/reviews/queue")

        item = resp.json()["items"][0]
        assert "concept_id" in item
        assert "concept_name" in item
        assert "mastery_score" in item
        assert "ease_factor" in item
        assert "interval_days" in item
        assert "repetitions" in item
        assert "is_overdue" in item

    @pytest.mark.asyncio
    async def test_unauthenticated_returns_403(self):
        """Without auth override the endpoint returns 403."""
        _clear_overrides()  # remove auth override

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/reviews/queue")

        assert resp.status_code in (401, 403)


# ── POST /v1/reviews/{concept_id}/answer ──────────────────────────────────────


class TestRecordAnswer:
    """Tests for POST /v1/reviews/{concept_id}/answer."""

    def setup_method(self):
        _override_auth()

    def teardown_method(self):
        _clear_overrides()

    def _build_db(
        self,
        concept_exists: bool = True,
        existing_mastery: bool = True,
    ) -> AsyncMock:
        """Build a mock DB session for the record_answer flow."""
        db = AsyncMock()
        db.add = MagicMock()
        db.flush = AsyncMock()
        db.commit = AsyncMock()

        # First execute → concept existence check
        concept_mock = MagicMock()
        concept_row = MagicMock() if concept_exists else None
        concept_mock.scalar_one_or_none.return_value = concept_row

        # Second execute → mastery row lookup
        mastery_mock = MagicMock()
        mastery_row = _make_mastery_row() if existing_mastery else None
        mastery_mock.scalar_one_or_none.return_value = mastery_row

        db.execute = AsyncMock(side_effect=[concept_mock, mastery_mock])
        return db

    @pytest.mark.asyncio
    async def test_valid_answer_returns_200(self):
        """A valid quality=5 answer returns 200 with updated scheduling state."""
        db = self._build_db()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.post(
                f"/v1/reviews/{CONCEPT_ID}/answer",
                json={"quality": 5},
            )

        assert resp.status_code == 200
        body = resp.json()
        assert body["quality"] == 5
        assert "mastery_before" in body
        assert "mastery_after" in body
        assert "ease_factor" in body
        assert "interval_days" in body
        assert "repetitions" in body
        assert "next_review_at" in body

    @pytest.mark.asyncio
    async def test_answer_commits_to_db(self):
        """Recording an answer commits the review record and mastery update."""
        db = self._build_db()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            await client.post(
                f"/v1/reviews/{CONCEPT_ID}/answer",
                json={"quality": 4},
            )

        db.add.assert_called_once()
        added = db.add.call_args[0][0]
        from app.orm import ReviewRecord

        assert isinstance(added, ReviewRecord)
        db.commit.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_answer_for_missing_concept_returns_404(self):
        """Answering for a non-existent concept returns 404."""
        db = self._build_db(concept_exists=False)
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.post(
                f"/v1/reviews/{CONCEPT_ID}/answer",
                json={"quality": 3},
            )

        assert resp.status_code == 404

    @pytest.mark.asyncio
    async def test_quality_0_fails_review(self):
        """quality=0 (blackout) still returns 200 but mastery decreases."""
        db = self._build_db()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.post(
                f"/v1/reviews/{CONCEPT_ID}/answer",
                json={"quality": 0},
            )

        assert resp.status_code == 200
        body = resp.json()
        assert body["mastery_after"] < body["mastery_before"]

    @pytest.mark.asyncio
    async def test_quality_5_increases_mastery(self):
        """quality=5 (perfect) increases mastery above the baseline."""
        db = self._build_db()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.post(
                f"/v1/reviews/{CONCEPT_ID}/answer",
                json={"quality": 5},
            )

        assert resp.status_code == 200
        body = resp.json()
        assert body["mastery_after"] > body["mastery_before"]

    @pytest.mark.asyncio
    async def test_quality_above_5_returns_422(self):
        """quality=6 fails Pydantic validation with 422."""
        db = self._build_db()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.post(
                f"/v1/reviews/{CONCEPT_ID}/answer",
                json={"quality": 6},
            )

        assert resp.status_code == 422

    @pytest.mark.asyncio
    async def test_quality_below_0_returns_422(self):
        """quality=-1 fails Pydantic validation with 422."""
        db = self._build_db()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.post(
                f"/v1/reviews/{CONCEPT_ID}/answer",
                json={"quality": -1},
            )

        assert resp.status_code == 422

    @pytest.mark.asyncio
    async def test_missing_quality_returns_422(self):
        """Body without quality field returns 422."""
        db = self._build_db()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.post(
                f"/v1/reviews/{CONCEPT_ID}/answer",
                json={},
            )

        assert resp.status_code == 422

    @pytest.mark.asyncio
    async def test_new_student_mastery_created(self):
        """For a student with no existing mastery row, one is created (flush called)."""
        db = self._build_db(existing_mastery=False)
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.post(
                f"/v1/reviews/{CONCEPT_ID}/answer",
                json={"quality": 3},
            )

        assert resp.status_code == 200
        db.flush.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_concept_id_in_response(self):
        """The response concept_id matches the URL path parameter."""
        db = self._build_db()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.post(
                f"/v1/reviews/{CONCEPT_ID}/answer",
                json={"quality": 4},
            )

        assert resp.status_code == 200
        assert resp.json()["concept_id"] == str(CONCEPT_ID)


# ── GET /v1/reviews/stats ─────────────────────────────────────────────────────


class TestReviewStats:
    """Tests for GET /v1/reviews/stats."""

    def setup_method(self):
        _override_auth()

    def teardown_method(self):
        _clear_overrides()

    def _build_stats_db(
        self,
        mastery_rows: list | None = None,
        total_reviews: int = 0,
    ) -> AsyncMock:
        """Build a DB mock for the stats endpoint (two sequential execute calls)."""
        db = AsyncMock()

        rows = mastery_rows or []
        now = datetime.now(timezone.utc)
        today_end = now.replace(hour=23, minute=59, second=59, microsecond=999999)
        week_end = now + timedelta(days=7)

        total = len(rows)
        avg_mastery = sum(r.mastery_score for r in rows) / total if rows else 0.0
        avg_ef = sum(r.ease_factor for r in rows) / total if rows else 2.5
        due_now = sum(1 for r in rows if r.next_review_at is None or r.next_review_at <= now)
        due_today = sum(
            1 for r in rows if r.next_review_at is not None and r.next_review_at <= today_end
        )
        due_week = sum(
            1 for r in rows if r.next_review_at is not None and r.next_review_at <= week_end
        )

        # First execute → aggregate row (mimics the SQL aggregate query)
        agg_row = MagicMock()
        agg_row.total = total
        agg_row.avg_mastery = avg_mastery
        agg_row.avg_ef = avg_ef
        agg_row.due_now = due_now
        agg_row.due_today = due_today
        agg_row.due_week = due_week

        agg_result = MagicMock()
        agg_result.one.return_value = agg_row

        # Second execute → COUNT(review_records.id)
        count_result = MagicMock()
        count_result.scalar_one.return_value = total_reviews

        db.execute = AsyncMock(side_effect=[agg_result, count_result])
        return db

    @pytest.mark.asyncio
    async def test_stats_shape_with_no_data(self):
        """With no mastery rows the stats are all zero/default."""
        db = self._build_stats_db()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/reviews/stats")

        assert resp.status_code == 200
        body = resp.json()
        assert body["total_concepts_studied"] == 0
        assert body["total_reviews"] == 0
        assert body["due_now"] == 0
        assert body["due_today"] == 0
        assert body["due_this_week"] == 0
        assert body["average_mastery"] == 0.0
        assert body["average_ease_factor"] == 2.5

    @pytest.mark.asyncio
    async def test_stats_counts_studied_concepts(self):
        """total_concepts_studied equals the number of mastery rows."""
        rows = [_make_mastery_row(concept_id=uuid4()) for _ in range(4)]
        db = self._build_stats_db(mastery_rows=rows, total_reviews=12)
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/reviews/stats")

        assert resp.status_code == 200
        body = resp.json()
        assert body["total_concepts_studied"] == 4
        assert body["total_reviews"] == 12

    @pytest.mark.asyncio
    async def test_stats_counts_due_now(self):
        """due_now counts concepts with next_review_at <= now or null."""
        rows = [
            _make_mastery_row(concept_id=uuid4(), next_review_at=None),  # due (null)
            _make_mastery_row(
                concept_id=uuid4(), next_review_at=NOW - timedelta(hours=1)
            ),  # due (past)
            _make_mastery_row(
                concept_id=uuid4(), next_review_at=NOW + timedelta(days=3)
            ),  # not due
        ]
        db = self._build_stats_db(mastery_rows=rows)
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/reviews/stats")

        assert resp.status_code == 200
        assert resp.json()["due_now"] == 2

    @pytest.mark.asyncio
    async def test_stats_average_mastery(self):
        """average_mastery is the mean of all mastery_score values."""
        rows = [
            _make_mastery_row(concept_id=uuid4(), mastery_score=0.8),
            _make_mastery_row(concept_id=uuid4(), mastery_score=0.4),
        ]
        db = self._build_stats_db(mastery_rows=rows)
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/reviews/stats")

        assert resp.status_code == 200
        assert resp.json()["average_mastery"] == pytest.approx(0.6, abs=0.001)

    @pytest.mark.asyncio
    async def test_stats_has_all_required_fields(self):
        """Response contains all documented fields."""
        db = self._build_stats_db()
        _override_db(db)

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/reviews/stats")

        body = resp.json()
        required = {
            "total_concepts_studied",
            "total_reviews",
            "due_now",
            "due_today",
            "due_this_week",
            "average_mastery",
            "average_ease_factor",
        }
        assert required <= set(body.keys())

    @pytest.mark.asyncio
    async def test_unauthenticated_returns_401_or_403(self):
        """Without auth the endpoint rejects the request."""
        _clear_overrides()

        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            resp = await client.get("/v1/reviews/stats")

        assert resp.status_code in (401, 403)
