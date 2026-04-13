"""Tests for app/sessions.py — FastAPI endpoint handlers.

Uses httpx AsyncClient with ASGITransport and mocked dependencies.
No live database, Redis, or auth service required.

Covers:
  POST /v1/sessions              — happy path, creates session
  GET  /v1/sessions/{id}         — cache hit, cache miss, 404 (not found),
                                   403 (wrong owner), and empty history
"""
from __future__ import annotations

import time
import uuid
from datetime import datetime, timezone
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
import pytest_asyncio
from httpx import ASGITransport, AsyncClient
from jose import jwt

from app.database import get_db
from app.main import app

SECRET = "sessions-test-secret"
ALGORITHM = "HS256"


# ── Auth helpers ──────────────────────────────────────────────────────────────


def _make_token(user_id: str) -> str:
    return jwt.encode(
        {
            "aud": "teacherslounge-services",
            "uid": user_id,
            "email": "test@example.com",
            "acct": "standard",
            "sub_status": "active",
            "exp": int(time.time()) + 3600,
        },
        SECRET,
        algorithm=ALGORITHM,
    )


@pytest.fixture(autouse=True)
def _patch_jwt_secret(patch_settings):
    patch_settings(jwt_secret=SECRET, jwt_algorithm=ALGORITHM)


@pytest.fixture()
def user_id() -> str:
    return str(uuid.uuid4())


@pytest.fixture()
def session_id() -> uuid.UUID:
    return uuid.uuid4()


@pytest.fixture()
def auth_headers(user_id: str) -> dict:
    return {"Authorization": f"Bearer {_make_token(user_id)}"}


@pytest_asyncio.fixture()
async def client():
    async with AsyncClient(
        transport=ASGITransport(app=app), base_url="http://test"
    ) as ac:
        yield ac


# ── POST /v1/sessions ─────────────────────────────────────────────────────────


@pytest.mark.asyncio
async def test_new_session_returns_201(client, auth_headers, user_id):
    """POST /v1/sessions creates a new session and returns 201."""
    fake_session = MagicMock()
    fake_session.id = uuid.uuid4()
    fake_session.user_id = uuid.UUID(user_id)
    fake_session.course_id = None
    fake_session.created_at = datetime.now(timezone.utc)

    with patch("app.sessions.create_session", AsyncMock(return_value=fake_session)):
        resp = await client.post(
            "/v1/sessions",
            json={},
            headers=auth_headers,
        )

    assert resp.status_code == 201
    data = resp.json()
    assert "session_id" in data
    assert data["message_count"] == 0


@pytest.mark.asyncio
async def test_new_session_with_course_id(client, auth_headers, user_id):
    """POST /v1/sessions passes course_id when provided."""
    course_id = str(uuid.uuid4())
    fake_session = MagicMock()
    fake_session.id = uuid.uuid4()
    fake_session.user_id = uuid.UUID(user_id)
    fake_session.course_id = uuid.UUID(course_id)
    fake_session.created_at = datetime.now(timezone.utc)

    create_mock = AsyncMock(return_value=fake_session)

    with patch("app.sessions.create_session", create_mock):
        resp = await client.post(
            "/v1/sessions",
            json={"course_id": course_id},
            headers=auth_headers,
        )

    assert resp.status_code == 201
    _, kwargs = create_mock.call_args
    assert kwargs.get("course_id") == uuid.UUID(course_id)


@pytest.mark.asyncio
async def test_new_session_requires_auth(client):
    """POST /v1/sessions without a token returns 401/403."""
    resp = await client.post("/v1/sessions", json={})
    assert resp.status_code in (401, 403)


# ── GET /v1/sessions/{session_id} ─────────────────────────────────────────────


@pytest.mark.asyncio
async def test_get_session_history_not_found_returns_404(
    client, auth_headers, user_id, session_id
):
    """GET /v1/sessions/{id} returns 404 when session does not exist."""
    with patch("app.sessions.get_session", AsyncMock(return_value=None)):
        resp = await client.get(
            f"/v1/sessions/{session_id}",
            headers=auth_headers,
        )

    assert resp.status_code == 404


@pytest.mark.asyncio
async def test_get_session_history_wrong_owner_returns_403(
    client, auth_headers, session_id
):
    """GET /v1/sessions/{id} returns 403 when session belongs to another user."""
    other_user_id = uuid.uuid4()  # not the requesting user
    fake_session = MagicMock()
    fake_session.user_id = other_user_id

    with patch("app.sessions.get_session", AsyncMock(return_value=fake_session)):
        resp = await client.get(
            f"/v1/sessions/{session_id}",
            headers=auth_headers,
        )

    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_get_session_history_cache_hit(client, auth_headers, user_id, session_id):
    """GET /v1/sessions/{id} returns cached history without hitting Postgres."""
    fake_session = MagicMock()
    fake_session.user_id = uuid.UUID(user_id)

    cached_messages = [
        {
            "id": str(uuid.uuid4()),
            "session_id": str(session_id),
            "role": "student",
            "content": "What is gravity?",
            "created_at": "2024-01-01T00:00:00+00:00",
        }
    ]

    with (
        patch("app.sessions.get_session", AsyncMock(return_value=fake_session)),
        patch("app.sessions.get_cached_history", AsyncMock(return_value=cached_messages)),
    ):
        resp = await client.get(
            f"/v1/sessions/{session_id}",
            headers=auth_headers,
        )

    assert resp.status_code == 200
    data = resp.json()
    assert len(data["messages"]) == 1
    assert data["messages"][0]["content"] == "What is gravity?"


@pytest.mark.asyncio
async def test_get_session_history_cache_miss_loads_from_db(
    client, auth_headers, user_id, session_id
):
    """GET /v1/sessions/{id} on cache miss loads from Postgres and populates cache."""
    fake_session = MagicMock()
    fake_session.user_id = uuid.UUID(user_id)

    fake_interaction = MagicMock()
    fake_interaction.id = uuid.uuid4()
    fake_interaction.session_id = session_id
    fake_interaction.role = "student"
    fake_interaction.content = "Hello tutor"
    fake_interaction.created_at = datetime.now(timezone.utc)

    set_cache_mock = AsyncMock()

    with (
        patch("app.sessions.get_session", AsyncMock(return_value=fake_session)),
        patch("app.sessions.get_cached_history", AsyncMock(return_value=None)),
        patch("app.sessions.get_history", AsyncMock(return_value=[fake_interaction])),
        patch("app.sessions.write_audit_log", AsyncMock()),
        patch("app.sessions.set_cached_history", set_cache_mock),
    ):
        resp = await client.get(
            f"/v1/sessions/{session_id}",
            headers=auth_headers,
        )

    assert resp.status_code == 200
    data = resp.json()
    assert len(data["messages"]) == 1
    assert data["messages"][0]["content"] == "Hello tutor"
    # Cache should be populated after a miss.
    set_cache_mock.assert_awaited_once()


@pytest.mark.asyncio
async def test_get_session_history_empty_session(
    client, auth_headers, user_id, session_id
):
    """GET /v1/sessions/{id} for an empty session returns an empty messages list."""
    fake_session = MagicMock()
    fake_session.user_id = uuid.UUID(user_id)

    with (
        patch("app.sessions.get_session", AsyncMock(return_value=fake_session)),
        patch("app.sessions.get_cached_history", AsyncMock(return_value=None)),
        patch("app.sessions.get_history", AsyncMock(return_value=[])),
        patch("app.sessions.write_audit_log", AsyncMock()),
        patch("app.sessions.set_cached_history", AsyncMock()),
    ):
        resp = await client.get(
            f"/v1/sessions/{session_id}",
            headers=auth_headers,
        )

    assert resp.status_code == 200
    data = resp.json()
    assert data["messages"] == []
