"""Tests for the student analytics endpoints.

All database interaction is replaced with AsyncMock fixtures so no real
Postgres instance is required.  JWT auth is overridden where appropriate to
isolate route logic from infrastructure.
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
