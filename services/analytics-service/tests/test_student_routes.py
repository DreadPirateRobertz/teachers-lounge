"""Tests for the student analytics endpoints.

All database interaction is replaced with AsyncMock fixtures so no real
Postgres instance is required.  JWT auth is overridden where appropriate to
isolate route logic from infrastructure.

Covers:
  - GET /v1/analytics/student/{id}/overview
  - GET /v1/analytics/student/{id}/quiz-breakdown
  - GET /v1/analytics/student/{id}/activity
  - GET /v1/analytics/student/{id}/mastery
  - GET /v1/analytics/student/{id}/upcoming-reviews
  - Unit tests for helper functions: _check_self_or_raise, _mastery_level,
    _review_priority, _review_interval
"""
from datetime import date, timedelta
from unittest.mock import AsyncMock, MagicMock

import pytest
from fastapi.testclient import TestClient

from app.auth import require_auth
from app.database import get_db
from app.main import app
from tests.conftest import (
    TEST_USER_ID,
    OTHER_USER_ID,
    auth_headers,
    make_db_override,
)

# ── Helpers ──────────────────────────────────────────────────────────────────

def _mapping_result(rows: list[dict]):
    """Build a mock SQLAlchemy result whose .mappings().all() returns rows.

    Args:
        rows: List of dicts representing database rows.

    Returns:
        A MagicMock that chains .mappings().all() to return the given rows.
    """
    result = MagicMock()
    result.mappings.return_value.all.return_value = rows
    return result


def _mapping_first(row: dict | None):
    """Build a mock SQLAlchemy result whose .mappings().first() returns row.

    Args:
        row: A dict representing a single database row, or None.

    Returns:
        A MagicMock that chains .mappings().first() to return the given row.
    """
    result = MagicMock()
    result.mappings.return_value.first.return_value = row
    return result


# ── /overview ────────────────────────────────────────────────────────────────

class TestOverview:
    """Tests for GET /v1/analytics/student/{user_id}/overview."""

    def _setup(self, gaming_row, session_row):
        """Override dependencies and return a configured TestClient.

        Args:
            gaming_row: Mock result for the gaming_profiles query.
            session_row: Mock result for the interactions query.

        Returns:
            TestClient with DB and auth overrides applied.
        """
        app.dependency_overrides[get_db] = make_db_override([gaming_row, session_row])
        app.dependency_overrides[require_auth] = lambda: TEST_USER_ID
        return TestClient(app)

    def teardown_method(self):
        """Remove all dependency overrides after each test."""
        app.dependency_overrides.clear()

    def test_overview_with_gaming_profile(self):
        """Returns correct stats when a gaming_profiles row exists."""
        gp = _mapping_first({
            "level": 5, "xp": 1200, "current_streak": 3, "longest_streak": 10,
            "total_questions": 40, "correct_answers": 32,
            "bosses_defeated": 2, "gems": 150,
        })
        sr = _mapping_first({"total_sessions": 8, "total_messages": 60})
        client = self._setup(gp, sr)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/overview",
            headers=auth_headers(),
        )

        assert resp.status_code == 200
        data = resp.json()
        assert data["level"] == 5
        assert data["xp"] == 1200
        assert data["current_streak"] == 3
        assert data["longest_streak"] == 10
        assert data["total_questions"] == 40
        assert data["correct_answers"] == 32
        assert data["accuracy_pct"] == 80.0
        assert data["bosses_defeated"] == 2
        assert data["gems"] == 150
        assert data["total_sessions"] == 8
        assert data["total_messages"] == 60

    def test_overview_no_gaming_profile_returns_defaults(self):
        """Returns zeroed defaults when no gaming_profiles row exists."""
        gp = _mapping_first(None)
        sr = _mapping_first({"total_sessions": 0, "total_messages": 0})
        client = self._setup(gp, sr)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/overview",
            headers=auth_headers(),
        )

        assert resp.status_code == 200
        data = resp.json()
        assert data["level"] == 1
        assert data["xp"] == 0
        assert data["accuracy_pct"] == 0.0

    def test_overview_accuracy_zero_when_no_questions(self):
        """accuracy_pct is 0.0 when total_questions is 0 (no division by zero)."""
        gp = _mapping_first({
            "level": 1, "xp": 0, "current_streak": 0, "longest_streak": 0,
            "total_questions": 0, "correct_answers": 0,
            "bosses_defeated": 0, "gems": 0,
        })
        sr = _mapping_first({"total_sessions": 0, "total_messages": 0})
        client = self._setup(gp, sr)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/overview",
            headers=auth_headers(),
        )

        assert resp.status_code == 200
        assert resp.json()["accuracy_pct"] == 0.0

    def test_overview_forbidden_for_other_user(self):
        """Returns 403 when the caller requests another user's overview."""
        app.dependency_overrides[get_db] = make_db_override([])
        app.dependency_overrides[require_auth] = lambda: OTHER_USER_ID
        client = TestClient(app)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/overview",
            headers=auth_headers(OTHER_USER_ID),
        )

        assert resp.status_code == 403

    def test_overview_requires_auth(self):
        """Returns 403/401 when no Authorization header is present."""
        app.dependency_overrides[get_db] = make_db_override([])
        # Remove auth override so real require_auth runs
        app.dependency_overrides.pop(require_auth, None)
        client = TestClient(app)

        resp = client.get(f"/v1/analytics/student/{TEST_USER_ID}/overview")

        assert resp.status_code in (401, 403)


# ── /quiz-breakdown ──────────────────────────────────────────────────────────

class TestQuizBreakdown:
    """Tests for GET /v1/analytics/student/{user_id}/quiz-breakdown."""

    def _setup(self, rows):
        """Override dependencies and return a configured TestClient.

        Args:
            rows: List of dicts to return from the quiz_results aggregate query.

        Returns:
            TestClient with DB and auth overrides applied.
        """
        app.dependency_overrides[get_db] = make_db_override([_mapping_result(rows)])
        app.dependency_overrides[require_auth] = lambda: TEST_USER_ID
        return TestClient(app)

    def teardown_method(self):
        """Remove all dependency overrides after each test."""
        app.dependency_overrides.clear()

    def test_quiz_breakdown_with_data(self):
        """Returns correct per-topic accuracy stats."""
        rows = [
            {"topic": "Algebra", "total": 20, "correct": 15},
            {"topic": "Calculus", "total": 10, "correct": 6},
        ]
        client = self._setup(rows)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/quiz-breakdown",
            headers=auth_headers(),
        )

        assert resp.status_code == 200
        topics = resp.json()["topics"]
        assert len(topics) == 2
        assert topics[0]["topic"] == "Algebra"
        assert topics[0]["total"] == 20
        assert topics[0]["correct"] == 15
        assert topics[0]["accuracy_pct"] == 75.0
        assert topics[1]["accuracy_pct"] == 60.0

    def test_quiz_breakdown_empty_when_no_results(self):
        """Returns an empty topics list when the student has no quiz data."""
        client = self._setup([])

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/quiz-breakdown",
            headers=auth_headers(),
        )

        assert resp.status_code == 200
        assert resp.json()["topics"] == []

    def test_quiz_breakdown_forbidden_for_other_user(self):
        """Returns 403 when caller requests another user's quiz breakdown."""
        app.dependency_overrides[get_db] = make_db_override([])
        app.dependency_overrides[require_auth] = lambda: OTHER_USER_ID
        client = TestClient(app)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/quiz-breakdown",
            headers=auth_headers(OTHER_USER_ID),
        )

        assert resp.status_code == 403

    def test_quiz_breakdown_accuracy_rounds_to_one_decimal(self):
        """accuracy_pct is rounded to one decimal place."""
        rows = [{"topic": "Physics", "total": 3, "correct": 2}]
        client = self._setup(rows)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/quiz-breakdown",
            headers=auth_headers(),
        )

        assert resp.json()["topics"][0]["accuracy_pct"] == 66.7

    def test_quiz_breakdown_requires_auth(self):
        """Returns 401/403 when no Authorization header is sent."""
        app.dependency_overrides[get_db] = make_db_override([])
        app.dependency_overrides.pop(require_auth, None)
        client = TestClient(app)

        resp = client.get(f"/v1/analytics/student/{TEST_USER_ID}/quiz-breakdown")

        assert resp.status_code in (401, 403)


# ── /activity ────────────────────────────────────────────────────────────────

class TestActivity:
    """Tests for GET /v1/analytics/student/{user_id}/activity."""

    def _setup(self, rows):
        """Override dependencies and return a configured TestClient.

        Args:
            rows: List of dicts with 'day' and 'messages' keys to simulate
                the interactions aggregate query result.

        Returns:
            TestClient with DB and auth overrides applied.
        """
        app.dependency_overrides[get_db] = make_db_override([_mapping_result(rows)])
        app.dependency_overrides[require_auth] = lambda: TEST_USER_ID
        return TestClient(app)

    def teardown_method(self):
        """Remove all dependency overrides after each test."""
        app.dependency_overrides.clear()

    def test_activity_returns_30_days(self):
        """Response always contains exactly 30 DayActivity entries."""
        client = self._setup([])

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/activity",
            headers=auth_headers(),
        )

        assert resp.status_code == 200
        assert len(resp.json()["days"]) == 30

    def test_activity_zero_fills_missing_days(self):
        """Days with no interactions are included with messages=0."""
        client = self._setup([])

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/activity",
            headers=auth_headers(),
        )

        data = resp.json()["days"]
        assert all(d["messages"] == 0 for d in data)

    def test_activity_populates_active_days(self):
        """Days present in the DB result have their message count set correctly."""
        today = date.today()
        active_day = str(today - timedelta(days=5))
        rows = [{"day": active_day, "messages": 7}]
        client = self._setup(rows)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/activity",
            headers=auth_headers(),
        )

        days = {d["date"]: d["messages"] for d in resp.json()["days"]}
        assert days[active_day] == 7

    def test_activity_days_are_ordered_oldest_first(self):
        """The 30 days are ordered from oldest to most recent."""
        client = self._setup([])

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/activity",
            headers=auth_headers(),
        )

        dates = [d["date"] for d in resp.json()["days"]]
        assert dates == sorted(dates)

    def test_activity_forbidden_for_other_user(self):
        """Returns 403 when caller requests another user's activity."""
        app.dependency_overrides[get_db] = make_db_override([])
        app.dependency_overrides[require_auth] = lambda: OTHER_USER_ID
        client = TestClient(app)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/activity",
            headers=auth_headers(OTHER_USER_ID),
        )

        assert resp.status_code == 403

    def test_activity_requires_auth(self):
        """Returns 401/403 when no Authorization header is sent."""
        app.dependency_overrides[get_db] = make_db_override([])
        app.dependency_overrides.pop(require_auth, None)
        client = TestClient(app)

        resp = client.get(f"/v1/analytics/student/{TEST_USER_ID}/activity")

        assert resp.status_code in (401, 403)

    def test_activity_multiple_active_days(self):
        """Multiple active days are all reflected in the response."""
        today = date.today()
        rows = [
            {"day": str(today - timedelta(days=2)), "messages": 3},
            {"day": str(today - timedelta(days=1)), "messages": 5},
        ]
        client = self._setup(rows)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/activity",
            headers=auth_headers(),
        )

        days = {d["date"]: d["messages"] for d in resp.json()["days"]}
        assert days[str(today - timedelta(days=2))] == 3
        assert days[str(today - timedelta(days=1))] == 5


# ── _check_self_or_raise unit test ───────────────────────────────────────────

class TestCheckSelfOrRaise:
    """Unit tests for the _check_self_or_raise helper."""

    def test_same_user_does_not_raise(self):
        """No exception raised when requesting_user_id == target_user_id."""
        from app.routes.student import _check_self_or_raise
        # Should not raise
        _check_self_or_raise(TEST_USER_ID, TEST_USER_ID)

    def test_different_user_raises_403(self):
        """HTTPException 403 raised when IDs differ."""
        from fastapi import HTTPException
        from app.routes.student import _check_self_or_raise
        with pytest.raises(HTTPException) as exc_info:
            _check_self_or_raise(OTHER_USER_ID, TEST_USER_ID)
        assert exc_info.value.status_code == 403


# ── /mastery ─────────────────────────────────────────────────────────────────

class TestMastery:
    """Tests for GET /v1/analytics/student/{user_id}/mastery."""

    def _setup(self, rows):
        """Override dependencies and return a configured TestClient.

        Args:
            rows: List of dicts to return from the quiz_results aggregate query.

        Returns:
            TestClient with DB and auth overrides applied.
        """
        app.dependency_overrides[get_db] = make_db_override([_mapping_result(rows)])
        app.dependency_overrides[require_auth] = lambda: TEST_USER_ID
        return TestClient(app)

    def teardown_method(self):
        """Remove all dependency overrides after each test."""
        app.dependency_overrides.clear()

    def test_mastery_with_data_returns_concepts(self):
        """Returns one ConceptMastery entry per topic row."""
        rows = [
            {"topic": "Algebra", "total": 10, "correct": 9},
            {"topic": "Calculus", "total": 10, "correct": 6},
            {"topic": "Geometry", "total": 10, "correct": 4},
        ]
        client = self._setup(rows)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/mastery",
            headers=auth_headers(),
        )

        assert resp.status_code == 200
        concepts = resp.json()["concepts"]
        assert len(concepts) == 3

    def test_mastery_level_mastered(self):
        """Accuracy >= 90% maps to 'mastered'."""
        rows = [{"topic": "Algebra", "total": 10, "correct": 9}]
        client = self._setup(rows)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/mastery",
            headers=auth_headers(),
        )

        concept = resp.json()["concepts"][0]
        assert concept["mastery_level"] == "mastered"
        assert concept["accuracy_pct"] == 90.0

    def test_mastery_level_strong(self):
        """70% ≤ accuracy < 90% maps to 'strong'."""
        rows = [{"topic": "Calculus", "total": 10, "correct": 8}]
        client = self._setup(rows)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/mastery",
            headers=auth_headers(),
        )

        assert resp.json()["concepts"][0]["mastery_level"] == "strong"

    def test_mastery_level_developing(self):
        """50% ≤ accuracy < 70% maps to 'developing'."""
        rows = [{"topic": "Physics", "total": 10, "correct": 6}]
        client = self._setup(rows)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/mastery",
            headers=auth_headers(),
        )

        assert resp.json()["concepts"][0]["mastery_level"] == "developing"

    def test_mastery_level_weak(self):
        """Accuracy < 50% maps to 'weak'."""
        rows = [{"topic": "Chemistry", "total": 10, "correct": 4}]
        client = self._setup(rows)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/mastery",
            headers=auth_headers(),
        )

        assert resp.json()["concepts"][0]["mastery_level"] == "weak"

    def test_mastery_empty_when_no_quiz_data(self):
        """Returns empty list when the student has no quiz_results."""
        client = self._setup([])

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/mastery",
            headers=auth_headers(),
        )

        assert resp.status_code == 200
        assert resp.json()["concepts"] == []

    def test_mastery_forbidden_for_other_user(self):
        """Returns 403 when caller requests another user's mastery data."""
        app.dependency_overrides[get_db] = make_db_override([])
        app.dependency_overrides[require_auth] = lambda: OTHER_USER_ID
        client = TestClient(app)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/mastery",
            headers=auth_headers(OTHER_USER_ID),
        )

        assert resp.status_code == 403


# ── /upcoming-reviews ─────────────────────────────────────────────────────────

class TestUpcomingReviews:
    """Tests for GET /v1/analytics/student/{user_id}/upcoming-reviews."""

    def _setup(self, rows):
        """Override dependencies and return a configured TestClient.

        Args:
            rows: List of dicts with topic, total, correct, last_attempted keys.

        Returns:
            TestClient with DB and auth overrides applied.
        """
        app.dependency_overrides[get_db] = make_db_override([_mapping_result(rows)])
        app.dependency_overrides[require_auth] = lambda: TEST_USER_ID
        return TestClient(app)

    def teardown_method(self):
        """Remove all dependency overrides after each test."""
        app.dependency_overrides.clear()

    def test_upcoming_reviews_empty_when_no_data(self):
        """Returns empty list when the student has no quiz_results."""
        client = self._setup([])

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/upcoming-reviews",
            headers=auth_headers(),
        )

        assert resp.status_code == 200
        assert resp.json()["reviews"] == []

    def test_upcoming_reviews_overdue_gets_urgent(self):
        """A review that is past due is marked 'urgent'."""
        past_date = date.today() - timedelta(days=10)
        rows = [{"topic": "Algebra", "total": 10, "correct": 4, "last_attempted": past_date}]
        client = self._setup(rows)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/upcoming-reviews",
            headers=auth_headers(),
        )

        review = resp.json()["reviews"][0]
        assert review["priority"] == "urgent"
        assert review["days_overdue"] >= 0

    def test_upcoming_reviews_due_today_gets_urgent(self):
        """A concept due exactly today is marked 'urgent'."""
        # weak accuracy → 1-day interval; last_attempted = yesterday
        yesterday = date.today() - timedelta(days=1)
        rows = [{"topic": "Physics", "total": 10, "correct": 3, "last_attempted": yesterday}]
        client = self._setup(rows)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/upcoming-reviews",
            headers=auth_headers(),
        )

        review = resp.json()["reviews"][0]
        assert review["priority"] == "urgent"
        assert review["days_overdue"] == 0

    def test_upcoming_reviews_soon_due_within_3_days(self):
        """A review due within 3 days is marked 'soon'."""
        # mastered accuracy → 14-day interval; last_attempted = 13 days ago → due tomorrow
        last = date.today() - timedelta(days=13)
        rows = [{"topic": "Calculus", "total": 10, "correct": 10, "last_attempted": last}]
        client = self._setup(rows)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/upcoming-reviews",
            headers=auth_headers(),
        )

        review = resp.json()["reviews"][0]
        assert review["priority"] == "soon"

    def test_upcoming_reviews_ordered_most_urgent_first(self):
        """Reviews are sorted with the most overdue entry first."""
        today = date.today()
        rows = [
            # Due in 6 days (upcoming)
            {"topic": "Chemistry", "total": 10, "correct": 10, "last_attempted": today - timedelta(days=8)},
            # Overdue by 5 days (urgent)
            {"topic": "Algebra", "total": 10, "correct": 3, "last_attempted": today - timedelta(days=6)},
        ]
        client = self._setup(rows)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/upcoming-reviews",
            headers=auth_headers(),
        )

        reviews = resp.json()["reviews"]
        assert reviews[0]["concept"] == "Algebra"
        assert reviews[1]["concept"] == "Chemistry"

    def test_upcoming_reviews_forbidden_for_other_user(self):
        """Returns 403 when caller requests another user's reviews."""
        app.dependency_overrides[get_db] = make_db_override([])
        app.dependency_overrides[require_auth] = lambda: OTHER_USER_ID
        client = TestClient(app)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/upcoming-reviews",
            headers=auth_headers(OTHER_USER_ID),
        )

        assert resp.status_code == 403

    def test_upcoming_reviews_upcoming_priority(self):
        """A review due 4+ days away is marked 'upcoming'."""
        # mastered → 14-day interval; last_attempted 4 days ago → due in 10 days
        last = date.today() - timedelta(days=4)
        rows = [{"topic": "Biology", "total": 10, "correct": 10, "last_attempted": last}]
        client = self._setup(rows)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/upcoming-reviews",
            headers=auth_headers(),
        )

        review = resp.json()["reviews"][0]
        assert review["priority"] == "upcoming"
        assert review["days_overdue"] < -3

    def test_upcoming_reviews_last_attempted_none_uses_today(self):
        """When last_attempted is None the review interval is measured from today."""
        rows = [{"topic": "Chemistry", "total": 10, "correct": 3, "last_attempted": None}]
        client = self._setup(rows)

        resp = client.get(
            f"/v1/analytics/student/{TEST_USER_ID}/upcoming-reviews",
            headers=auth_headers(),
        )

        # weak mastery → 1-day interval from today → due tomorrow → days_overdue = -1 ("soon")
        review = resp.json()["reviews"][0]
        assert review["concept"] == "Chemistry"
        assert review["days_overdue"] == -1
        assert review["priority"] == "soon"


# ── Helper unit tests ─────────────────────────────────────────────────────────

class TestMasteryLevel:
    """Unit tests for the _mastery_level helper function."""

    def test_below_50_is_weak(self):
        """Accuracy below 50% returns 'weak'."""
        from app.routes.student import _mastery_level
        assert _mastery_level(0.0) == "weak"
        assert _mastery_level(49.9) == "weak"

    def test_50_to_69_is_developing(self):
        """Accuracy in [50, 70) returns 'developing'."""
        from app.routes.student import _mastery_level
        assert _mastery_level(50.0) == "developing"
        assert _mastery_level(69.9) == "developing"

    def test_70_to_89_is_strong(self):
        """Accuracy in [70, 90) returns 'strong'."""
        from app.routes.student import _mastery_level
        assert _mastery_level(70.0) == "strong"
        assert _mastery_level(89.9) == "strong"

    def test_90_and_above_is_mastered(self):
        """Accuracy >= 90% returns 'mastered'."""
        from app.routes.student import _mastery_level
        assert _mastery_level(90.0) == "mastered"
        assert _mastery_level(100.0) == "mastered"


class TestReviewPriority:
    """Unit tests for the _review_priority helper function."""

    def test_positive_days_overdue_is_urgent(self):
        """Past-due reviews (days_overdue > 0) are 'urgent'."""
        from app.routes.student import _review_priority
        assert _review_priority(1) == "urgent"
        assert _review_priority(10) == "urgent"

    def test_zero_days_overdue_is_urgent(self):
        """Due-today reviews (days_overdue == 0) are 'urgent'."""
        from app.routes.student import _review_priority
        assert _review_priority(0) == "urgent"

    def test_minus_1_to_minus_3_is_soon(self):
        """Reviews due within 3 days are 'soon'."""
        from app.routes.student import _review_priority
        assert _review_priority(-1) == "soon"
        assert _review_priority(-3) == "soon"

    def test_minus_4_and_beyond_is_upcoming(self):
        """Reviews due 4+ days away are 'upcoming'."""
        from app.routes.student import _review_priority
        assert _review_priority(-4) == "upcoming"
        assert _review_priority(-30) == "upcoming"


class TestReviewInterval:
    """Unit tests for the _review_interval helper function."""

    def test_weak_interval_is_1(self):
        """'weak' mastery level returns a 1-day interval."""
        from app.routes.student import _review_interval
        assert _review_interval("weak") == 1

    def test_developing_interval_is_3(self):
        """'developing' mastery level returns a 3-day interval."""
        from app.routes.student import _review_interval
        assert _review_interval("developing") == 3

    def test_strong_interval_is_7(self):
        """'strong' mastery level returns a 7-day interval."""
        from app.routes.student import _review_interval
        assert _review_interval("strong") == 7

    def test_mastered_interval_is_14(self):
        """'mastered' mastery level returns a 14-day interval."""
        from app.routes.student import _review_interval
        assert _review_interval("mastered") == 14

    def test_unknown_level_defaults_to_1(self):
        """Unknown mastery levels default to a 1-day interval."""
        from app.routes.student import _review_interval
        assert _review_interval("unknown") == 1
