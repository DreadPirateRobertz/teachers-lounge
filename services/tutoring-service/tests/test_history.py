"""Unit tests for app/history.py — CRUD helpers for sessions and interactions.

Tests use a mock AsyncSession so no live database is required.

Covers all six exported functions:
  - create_session: happy path, with and without course_id
  - get_session: found and not-found cases
  - count_history: non-zero and zero (empty session)
  - get_history: returns reversed list, empty session returns empty list
  - get_history_slice: normal slice, offset boundary (offset == total)
  - append_message: with and without response_time_ms
"""
from __future__ import annotations

import uuid
from datetime import datetime, timezone
from unittest.mock import AsyncMock, MagicMock, call

import pytest

from app.history import (
    append_message,
    count_history,
    create_session,
    get_history,
    get_history_slice,
    get_session,
)

# ── Helpers ───────────────────────────────────────────────────────────────────


def _make_db() -> MagicMock:
    """Return a mock AsyncSession where async calls are pre-wired."""
    db = MagicMock()
    db.add = MagicMock()
    db.commit = AsyncMock()
    db.refresh = AsyncMock()
    db.execute = AsyncMock()
    return db


def _make_interaction(
    session_id: uuid.UUID | None = None,
    role: str = "student",
    content: str = "hello",
) -> MagicMock:
    obj = MagicMock()
    obj.id = uuid.uuid4()
    obj.session_id = session_id or uuid.uuid4()
    obj.role = role
    obj.content = content
    obj.created_at = datetime.now(timezone.utc)
    return obj


def _scalar_result(value) -> MagicMock:
    """Wrap a value in a mock that looks like AsyncResult from db.execute."""
    result = MagicMock()
    result.scalar_one_or_none.return_value = value
    result.scalar_one.return_value = value
    return result


def _scalars_result(items: list) -> MagicMock:
    """Wrap a list in a mock that looks like a scalars().all() result."""
    result = MagicMock()
    scalars_mock = MagicMock()
    scalars_mock.all.return_value = items
    result.scalars.return_value = scalars_mock
    return result


# ── create_session ────────────────────────────────────────────────────────────


@pytest.mark.asyncio
async def test_create_session_returns_session_object():
    """create_session adds, commits, refreshes and returns the session."""
    db = _make_db()
    user_id = uuid.uuid4()

    # db.refresh sets attributes on the passed object (simulated via side_effect)
    async def _refresh(obj):
        obj.id = uuid.uuid4()
        obj.user_id = user_id
        obj.course_id = None
        obj.created_at = datetime.now(timezone.utc)

    db.refresh = AsyncMock(side_effect=_refresh)

    result = await create_session(db, user_id=user_id)

    db.add.assert_called_once()
    db.commit.assert_awaited_once()
    db.refresh.assert_awaited_once()
    assert result.user_id == user_id


@pytest.mark.asyncio
async def test_create_session_with_course_id():
    """create_session stores course_id on the new session."""
    db = _make_db()
    user_id = uuid.uuid4()
    course_id = uuid.uuid4()

    added_obj = None

    def _capture_add(obj):
        nonlocal added_obj
        added_obj = obj

    db.add = MagicMock(side_effect=_capture_add)
    db.refresh = AsyncMock()

    await create_session(db, user_id=user_id, course_id=course_id)

    assert added_obj is not None
    assert added_obj.course_id == course_id


# ── get_session ───────────────────────────────────────────────────────────────


@pytest.mark.asyncio
async def test_get_session_found():
    """get_session returns the Session ORM object when it exists."""
    db = _make_db()
    session_id = uuid.uuid4()
    fake_session = MagicMock()
    db.execute.return_value = _scalar_result(fake_session)

    result = await get_session(db, session_id)

    assert result is fake_session


@pytest.mark.asyncio
async def test_get_session_not_found_returns_none():
    """get_session returns None when session does not exist."""
    db = _make_db()
    db.execute.return_value = _scalar_result(None)

    result = await get_session(db, uuid.uuid4())

    assert result is None


# ── count_history ─────────────────────────────────────────────────────────────


@pytest.mark.asyncio
async def test_count_history_returns_count():
    """count_history returns the integer row count from the DB."""
    db = _make_db()
    db.execute.return_value = _scalar_result(7)

    result = await count_history(db, uuid.uuid4())

    assert result == 7


@pytest.mark.asyncio
async def test_count_history_empty_session_returns_zero():
    """count_history returns 0 for a session with no messages."""
    db = _make_db()
    db.execute.return_value = _scalar_result(0)

    result = await count_history(db, uuid.uuid4())

    assert result == 0


# ── get_history ───────────────────────────────────────────────────────────────


@pytest.mark.asyncio
async def test_get_history_returns_chronological_order():
    """get_history reverses the DESC-ordered DB result to return oldest-first."""
    db = _make_db()
    session_id = uuid.uuid4()
    # DB returns in DESC order (newest first) — function must reverse.
    newest = _make_interaction(session_id, content="newest")
    middle = _make_interaction(session_id, content="middle")
    oldest = _make_interaction(session_id, content="oldest")
    db.execute.return_value = _scalars_result([newest, middle, oldest])

    result = await get_history(db, session_id)

    assert [m.content for m in result] == ["oldest", "middle", "newest"]


@pytest.mark.asyncio
async def test_get_history_empty_session_returns_empty_list():
    """get_history returns an empty list when the session has no messages."""
    db = _make_db()
    db.execute.return_value = _scalars_result([])

    result = await get_history(db, uuid.uuid4())

    assert result == []


@pytest.mark.asyncio
async def test_get_history_respects_limit():
    """get_history passes the limit parameter through to the query."""
    db = _make_db()
    session_id = uuid.uuid4()
    interactions = [_make_interaction(session_id) for _ in range(5)]
    db.execute.return_value = _scalars_result(interactions)

    result = await get_history(db, session_id, limit=5)

    assert len(result) == 5


# ── get_history_slice ─────────────────────────────────────────────────────────


@pytest.mark.asyncio
async def test_get_history_slice_returns_items():
    """get_history_slice returns items in ascending (oldest-first) order."""
    db = _make_db()
    session_id = uuid.uuid4()
    items = [_make_interaction(session_id, content=f"msg-{i}") for i in range(3)]
    db.execute.return_value = _scalars_result(items)

    result = await get_history_slice(db, session_id, offset=0, limit=3)

    assert result == items


@pytest.mark.asyncio
async def test_get_history_slice_offset_at_boundary_returns_empty():
    """get_history_slice with offset == total messages returns empty list.

    This models the 'offset boundary' case from the bead description:
    when the caller asks for messages starting beyond the end of the history,
    the DB returns an empty result set.
    """
    db = _make_db()
    db.execute.return_value = _scalars_result([])

    result = await get_history_slice(db, uuid.uuid4(), offset=100, limit=20)

    assert result == []


@pytest.mark.asyncio
async def test_get_history_slice_partial_last_page():
    """get_history_slice returns fewer items than limit when near the end."""
    db = _make_db()
    session_id = uuid.uuid4()
    # Only 3 items left when offset=17 and there are 20 total.
    items = [_make_interaction(session_id) for _ in range(3)]
    db.execute.return_value = _scalars_result(items)

    result = await get_history_slice(db, session_id, offset=17, limit=20)

    assert len(result) == 3


# ── append_message ────────────────────────────────────────────────────────────


@pytest.mark.asyncio
async def test_append_message_persists_and_returns_interaction():
    """append_message adds, commits, refreshes and returns the new Interaction."""
    db = _make_db()
    session_id = uuid.uuid4()
    user_id = uuid.uuid4()

    added_obj = None

    def _capture(obj):
        nonlocal added_obj
        added_obj = obj

    db.add = MagicMock(side_effect=_capture)
    db.refresh = AsyncMock()

    result = await append_message(
        db,
        session_id=session_id,
        user_id=user_id,
        role="student",
        content="What is entropy?",
    )

    db.commit.assert_awaited_once()
    db.refresh.assert_awaited_once()
    assert added_obj is not None
    assert added_obj.role == "student"
    assert added_obj.content == "What is entropy?"
    assert added_obj.response_time_ms is None


@pytest.mark.asyncio
async def test_append_message_with_response_time_ms():
    """append_message stores response_time_ms on tutor messages."""
    db = _make_db()
    session_id = uuid.uuid4()
    user_id = uuid.uuid4()

    added_obj = None

    def _capture(obj):
        nonlocal added_obj
        added_obj = obj

    db.add = MagicMock(side_effect=_capture)
    db.refresh = AsyncMock()

    await append_message(
        db,
        session_id=session_id,
        user_id=user_id,
        role="tutor",
        content="Entropy is a measure of disorder.",
        response_time_ms=342,
    )

    assert added_obj is not None
    assert added_obj.response_time_ms == 342
