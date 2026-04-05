"""Tests for FERPA audit log writer.

Uses an in-memory mock SQLAlchemy session so no real Postgres is needed.
"""
import pytest
from unittest.mock import AsyncMock, MagicMock, patch
from uuid import uuid4

from app.audit import (
    ACTION_READ_INTERACTIONS,
    ACTION_READ_PROFILE,
    write_audit_log,
)


# ── helpers ──────────────────────────────────────────────────────────────────

def _make_db(execute_raises=None):
    """Return a mock AsyncSession that records calls."""
    db = AsyncMock()
    if execute_raises:
        db.execute.side_effect = execute_raises
    else:
        db.execute.return_value = MagicMock()
    return db


# ── happy-path tests ──────────────────────────────────────────────────────────

@pytest.mark.asyncio
async def test_write_audit_log_executes_insert():
    """write_audit_log issues exactly one INSERT and one COMMIT."""
    db = _make_db()
    user_id = uuid4()

    await write_audit_log(
        db,
        accessor_id=user_id,
        student_id=user_id,
        action=ACTION_READ_INTERACTIONS,
        data_accessed="chat_session:abc",
        purpose="user_request",
        ip_address="127.0.0.1",
    )

    db.execute.assert_awaited_once()
    db.commit.assert_awaited_once()


@pytest.mark.asyncio
async def test_write_audit_log_accepts_none_ids():
    """write_audit_log handles None accessor_id / student_id without error."""
    db = _make_db()

    await write_audit_log(
        db,
        accessor_id=None,
        student_id=None,
        action=ACTION_READ_PROFILE,
        data_accessed="profile",
        purpose="system",
        ip_address="",
    )

    db.execute.assert_awaited_once()
    db.commit.assert_awaited_once()


@pytest.mark.asyncio
async def test_write_audit_log_passes_correct_params():
    """Verify the bound parameters passed to execute contain expected values."""
    db = _make_db()
    accessor_id = uuid4()
    student_id = uuid4()

    await write_audit_log(
        db,
        accessor_id=accessor_id,
        student_id=student_id,
        action=ACTION_READ_INTERACTIONS,
        data_accessed="chat_session:xyz",
        purpose="user_request",
        ip_address="10.0.0.1",
    )

    # Extract the params dict passed to db.execute
    _, kwargs = db.execute.await_args
    params = db.execute.await_args[0][1]  # positional dict

    assert params["action"] == ACTION_READ_INTERACTIONS
    assert params["data_accessed"] == "chat_session:xyz"
    assert params["purpose"] == "user_request"
    assert params["accessor_id"] == str(accessor_id)
    assert params["student_id"] == str(student_id)
    assert params["ip_address"] == "10.0.0.1"


# ── error-resilience tests ────────────────────────────────────────────────────

@pytest.mark.asyncio
async def test_write_audit_log_swallows_db_error():
    """write_audit_log must not raise even when the DB insert fails."""
    db = _make_db(execute_raises=Exception("connection reset"))

    # Should not raise — audit failures are non-fatal
    await write_audit_log(
        db,
        accessor_id=uuid4(),
        student_id=uuid4(),
        action=ACTION_READ_INTERACTIONS,
        data_accessed="chat_session:xyz",
        purpose="user_request",
    )


@pytest.mark.asyncio
async def test_write_audit_log_swallows_commit_error():
    """write_audit_log must not raise when commit fails."""
    db = _make_db()
    db.commit.side_effect = Exception("deadlock")

    await write_audit_log(
        db,
        accessor_id=uuid4(),
        student_id=uuid4(),
        action=ACTION_READ_INTERACTIONS,
        data_accessed="chat_session:xyz",
        purpose="user_request",
    )


@pytest.mark.asyncio
async def test_write_audit_log_empty_ip_becomes_none():
    """Empty ip_address string is converted to None for the INET cast."""
    db = _make_db()

    await write_audit_log(
        db,
        accessor_id=uuid4(),
        student_id=uuid4(),
        action=ACTION_READ_INTERACTIONS,
        data_accessed="chat_session:xyz",
        purpose="user_request",
        ip_address="",
    )

    params = db.execute.await_args[0][1]
    assert params["ip_address"] is None
