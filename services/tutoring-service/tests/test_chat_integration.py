"""
Integration tests: chat endpoints return correct streaming responses.

Uses httpx AsyncClient against the FastAPI app with a mocked AI Gateway.
No real gateway or database required.

Covers:
  POST /v1/sessions/{id}/messages — SSE stream (delta + done events)
  POST /v1/sessions/{id}/messages — RAG path: sources event emitted
  POST /v1/chat                   — plain-text stream, happy path + 401
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


# ── SSE stream — no course (base Professor Nova, no RAG) ────────────────────

@pytest.mark.asyncio
async def test_sse_stream_happy_path(client, auth_headers, user_id, session_id):
    """
    Full SSE round-trip: valid token → session ownership check → stream delta+done.
    Session has no course_id so RAG is skipped. DB and gateway are fully mocked.
    """
    fake_session = MagicMock()
    fake_session.user_id = uuid.UUID(user_id)
    fake_session.course_id = None  # no course → no RAG

    async def _fake_stream():
        for token in ["Hello", ", ", "student", "!"]:
            yield _make_chunk(token)
        yield _make_chunk(None)

    fake_completions = AsyncMock()
    fake_completions.create = AsyncMock(return_value=_fake_stream())

    fake_openai = MagicMock()
    fake_openai.chat.completions = fake_completions

    with (
        patch("app.chat.get_session", AsyncMock(return_value=fake_session)),
        patch("app.chat.get_history", AsyncMock(return_value=[])),
        patch("app.chat.append_message", AsyncMock()),
        patch("app.chat.get_gateway_client", return_value=fake_openai),
        patch("app.chat.get_dials", AsyncMock(return_value={
            "active_reflective": 0.0,
            "sensing_intuitive": 0.0,
            "visual_verbal": 0.0,
            "sequential_global": 0.0,
        })),
        patch("app.chat.update_learning_profile_dials", AsyncMock()),
        patch("app.chat.get_due_review_prompt", AsyncMock(return_value=None)),
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
    assert "sources" not in types  # no course_id → no sources event

    delta_content = "".join(e["content"] for e in events if e["type"] == "delta")
    assert delta_content == "Hello, student!"


# ── SSE stream — with course_id → RAG path with sources event ───────────────

@pytest.mark.asyncio
async def test_sse_stream_rag_emits_sources_event(client, auth_headers, user_id, session_id):
    """
    Session has a course_id — agentic RAG runs and a 'sources' event is emitted
    before 'done' when chunks are retrieved.
    """
    from app.search_client import SearchResult

    fake_session = MagicMock()
    fake_session.user_id = uuid.UUID(user_id)
    fake_session.course_id = uuid.uuid4()  # triggers RAG

    fake_chunk = SearchResult(
        chunk_id=str(uuid.uuid4()),
        material_id=str(uuid.uuid4()),
        course_id=str(fake_session.course_id),
        content="A chiral center is a carbon atom with four different substituents.",
        score=0.91,
        chapter="Chapter 5",
        section="5.3",
        page=87,
    )

    async def _fake_stream():
        for token in ["Chiral", " centers", " are", "..."]:
            yield _make_chunk(token)
        yield _make_chunk(None)

    fake_completions = AsyncMock()
    fake_completions.create = AsyncMock(return_value=_fake_stream())

    fake_openai = MagicMock()
    fake_openai.chat.completions = fake_completions

    with (
        patch("app.chat.get_session", AsyncMock(return_value=fake_session)),
        patch("app.chat.get_history", AsyncMock(return_value=[])),
        patch("app.chat.append_message", AsyncMock()),
        patch("app.chat.get_gateway_client", return_value=fake_openai),
        patch("app.chat.build_rag_context", AsyncMock(return_value=("system prompt", [fake_chunk]))),
        patch("app.chat.get_dials", AsyncMock(return_value={
            "active_reflective": 0.0,
            "sensing_intuitive": 0.0,
            "visual_verbal": 0.0,
            "sequential_global": 0.0,
        })),
        patch("app.chat.update_learning_profile_dials", AsyncMock()),
        patch("app.chat.get_due_review_prompt", AsyncMock(return_value=None)),
    ):
        resp = await client.post(
            f"/v1/sessions/{session_id}/messages",
            json={"content": "What is a chiral center?"},
            headers=auth_headers,
        )

    assert resp.status_code == 200
    events = [
        json.loads(line[len("data: "):])
        for line in resp.text.splitlines()
        if line.startswith("data: ")
    ]

    types = [e["type"] for e in events]
    assert "delta" in types
    assert "sources" in types
    assert types[-1] == "done"

    sources_event = next(e for e in events if e["type"] == "sources")
    assert isinstance(sources_event["sources"], list)
    assert len(sources_event["sources"]) == 1
    src = sources_event["sources"][0]
    assert src["chapter"] == "Chapter 5"
    assert src["section"] == "5.3"
    assert src["page"] == 87


@pytest.mark.asyncio
async def test_sse_stream_no_sources_event_when_chunks_empty(
    client, auth_headers, user_id, session_id
):
    """When RAG returns no chunks (e.g. not yet indexed), no sources event is emitted."""
    fake_session = MagicMock()
    fake_session.user_id = uuid.UUID(user_id)
    fake_session.course_id = uuid.uuid4()

    async def _fake_stream():
        yield _make_chunk("Answer from general knowledge.")
        yield _make_chunk(None)

    fake_completions = AsyncMock()
    fake_completions.create = AsyncMock(return_value=_fake_stream())

    fake_openai = MagicMock()
    fake_openai.chat.completions = fake_completions

    with (
        patch("app.chat.get_session", AsyncMock(return_value=fake_session)),
        patch("app.chat.get_history", AsyncMock(return_value=[])),
        patch("app.chat.append_message", AsyncMock()),
        patch("app.chat.get_gateway_client", return_value=fake_openai),
        patch("app.chat.build_rag_context", AsyncMock(return_value=("system prompt", []))),
        patch("app.chat.get_dials", AsyncMock(return_value={
            "active_reflective": 0.0,
            "sensing_intuitive": 0.0,
            "visual_verbal": 0.0,
            "sequential_global": 0.0,
        })),
        patch("app.chat.update_learning_profile_dials", AsyncMock()),
        patch("app.chat.get_due_review_prompt", AsyncMock(return_value=None)),
    ):
        resp = await client.post(
            f"/v1/sessions/{session_id}/messages",
            json={"content": "What is osmosis?"},
            headers=auth_headers,
        )

    events = [
        json.loads(line[len("data: "):])
        for line in resp.text.splitlines()
        if line.startswith("data: ")
    ]
    assert "sources" not in [e["type"] for e in events]
    assert events[-1]["type"] == "done"


# ── SSE stream — review_reminder event ──────────────────────────────────────

@pytest.mark.asyncio
async def test_sse_stream_emits_review_reminder_when_concepts_due(
    client, auth_headers, user_id, session_id
):
    """When get_due_review_prompt returns a prompt, a review_reminder event is emitted
    before the done event so the client can surface it to the student."""
    fake_session = MagicMock()
    fake_session.user_id = uuid.UUID(user_id)
    fake_session.course_id = None

    async def _fake_stream():
        yield _make_chunk("Great question.")
        yield _make_chunk(None)

    fake_completions = AsyncMock()
    fake_completions.create = AsyncMock(return_value=_fake_stream())

    fake_openai = MagicMock()
    fake_openai.chat.completions = fake_completions

    reminder_text = "You have 2 concepts due for review: Derivatives and Integrals."

    with (
        patch("app.chat.get_session", AsyncMock(return_value=fake_session)),
        patch("app.chat.get_history", AsyncMock(return_value=[])),
        patch("app.chat.append_message", AsyncMock()),
        patch("app.chat.get_gateway_client", return_value=fake_openai),
        patch("app.chat.get_dials", AsyncMock(return_value={
            "active_reflective": 0.0,
            "sensing_intuitive": 0.0,
            "visual_verbal": 0.0,
            "sequential_global": 0.0,
        })),
        patch("app.chat.update_learning_profile_dials", AsyncMock()),
        patch("app.chat.get_due_review_prompt", AsyncMock(return_value=reminder_text)),
    ):
        resp = await client.post(
            f"/v1/sessions/{session_id}/messages",
            json={"content": "Explain derivatives."},
            headers=auth_headers,
        )

    assert resp.status_code == 200
    events = [
        json.loads(line[len("data: "):])
        for line in resp.text.splitlines()
        if line.startswith("data: ")
    ]

    types = [e["type"] for e in events]
    assert "review_reminder" in types
    assert types[-1] == "done"
    # review_reminder must come before done
    assert types.index("review_reminder") < types.index("done")

    reminder_event = next(e for e in events if e["type"] == "review_reminder")
    assert reminder_event["content"] == reminder_text


# ── POST /v1/chat (stateless plain-text stream) ──────────────────────────────

@pytest.mark.asyncio
async def test_simple_chat_happy_path(client, auth_headers):
    """POST /v1/chat with valid JWT + messages array → plain-text chunked stream."""
    async def _fake_stream():
        for token in ["Photosynthesis", " is", " the", " process"]:
            yield _make_chunk(token)
        yield _make_chunk(None)

    fake_completions = AsyncMock()
    fake_completions.create = AsyncMock(return_value=_fake_stream())

    fake_openai = MagicMock()
    fake_openai.chat.completions = fake_completions

    with patch("app.chat_simple.get_gateway_client", return_value=fake_openai):
        resp = await client.post(
            "/v1/chat",
            json={"messages": [{"role": "user", "content": "What is photosynthesis?"}]},
            headers=auth_headers,
        )

    assert resp.status_code == 200
    assert "text/plain" in resp.headers["content-type"]
    assert resp.text == "Photosynthesis is the process"


@pytest.mark.asyncio
async def test_simple_chat_requires_auth(client):
    """POST /v1/chat without a Bearer token → 403 (HTTPBearer auto_error=True)."""
    resp = await client.post(
        "/v1/chat",
        json={"messages": [{"role": "user", "content": "Hello"}]},
    )
    assert resp.status_code == 403
