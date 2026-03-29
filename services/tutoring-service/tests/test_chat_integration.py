"""
Integration test: POST /v1/sessions/{id}/messages returns SSE stream.

Uses httpx AsyncClient against the FastAPI app with a mocked AI Gateway.
No real gateway or database required.

The test verifies:
  1. A valid JWT is accepted
  2. The SSE stream contains at least one "delta" event
  3. The stream ends with a "done" event
  4. No "error" events are emitted on the happy path
"""
import json
import time
import uuid
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
import pytest_asyncio
from httpx import ASGITransport, AsyncClient
from jose import jwt

from app.main import app

SECRET = "integration-test-secret"
ALGORITHM = "HS256"


def _make_token(user_id: str) -> str:
    return jwt.encode(
        {
            "uid": user_id,
            "email": "test@example.com",
            "acct": "standard",
            "sub_status": "active",
            "exp": int(time.time()) + 3600,
        },
        SECRET,
        algorithm=ALGORITHM,
    )


def _make_chunk(content: str | None):
    """Build a minimal OpenAI chat completion chunk mock."""
    delta = MagicMock()
    delta.content = content
    choice = MagicMock()
    choice.delta = delta
    chunk = MagicMock()
    chunk.choices = [choice]
    return chunk


@pytest.fixture(autouse=True)
def _patch_jwt_secret(patch_settings):
    patch_settings(jwt_secret=SECRET, jwt_algorithm=ALGORITHM)


@pytest.fixture()
def user_id():
    return str(uuid.uuid4())


@pytest.fixture()
def session_id():
    return str(uuid.uuid4())


@pytest.fixture()
def auth_headers(user_id):
    return {"Authorization": f"Bearer {_make_token(user_id)}"}


@pytest_asyncio.fixture()
async def client():
    async with AsyncClient(
        transport=ASGITransport(app=app), base_url="http://test"
    ) as ac:
        yield ac


@pytest.mark.asyncio
async def test_sse_stream_happy_path(client, auth_headers, user_id, session_id):
    """
    Full SSE round-trip: valid token → session ownership check → stream delta+done.
    DB and gateway are fully mocked.
    """
    # ── mock DB session lookup ─────────────────────────────────────────────────
    fake_session = MagicMock()
    fake_session.user_id = uuid.UUID(user_id)

    # ── mock AI Gateway stream ─────────────────────────────────────────────────
    async def _fake_stream():
        for token in ["Hello", ", ", "student", "!"]:
            yield _make_chunk(token)
        yield _make_chunk(None)   # final chunk with no content

    fake_stream_cm = AsyncMock()
    fake_stream_cm.__aenter__ = AsyncMock(return_value=_fake_stream())
    fake_stream_cm.__aexit__ = AsyncMock(return_value=False)

    fake_completions = AsyncMock()
    fake_completions.create = AsyncMock(return_value=_fake_stream())

    fake_openai = MagicMock()
    fake_openai.chat.completions = fake_completions

    with (
        patch("app.history.get_session", AsyncMock(return_value=fake_session)),
        patch("app.history.get_history", AsyncMock(return_value=[])),
        patch("app.history.append_message", AsyncMock()),
        patch("app.chat.get_gateway_client", return_value=fake_openai),
    ):
        resp = await client.post(
            f"/v1/sessions/{session_id}/messages",
            json={"content": "What is the Pythagorean theorem?"},
            headers=auth_headers,
        )

    assert resp.status_code == 200
    assert "text/event-stream" in resp.headers["content-type"]

    events = [
        json.loads(line[len("data: "):])
        for line in resp.text.splitlines()
        if line.startswith("data: ")
    ]

    types = [e["type"] for e in events]
    assert "delta" in types
    assert types[-1] == "done"
    assert "error" not in types

    delta_content = "".join(e["content"] for e in events if e["type"] == "delta")
    assert delta_content == "Hello, student!"
