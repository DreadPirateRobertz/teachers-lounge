"""Tests for Pydantic models and SSE helpers."""
import os

os.environ.setdefault("DATABASE_URL", "postgresql+asyncpg://test:test@localhost:5432/test")
os.environ.setdefault("AI_GATEWAY_KEY", "test-key")
os.environ.setdefault("JWT_SECRET", "test-secret-that-is-long-enough-for-hs256")

import json
from uuid import uuid4

import pytest

from app.models import MessageRequest, Role, SSEEvent


def test_message_request_max_length():
    # Max-length message should be valid
    MessageRequest(content="x" * 8000)


def test_message_request_exceeds_max_length():
    with pytest.raises(Exception):
        MessageRequest(content="x" * 8001)


def test_role_enum_values():
    assert Role.student == "student"
    assert Role.tutor == "tutor"


def test_sse_event_serializes():
    msg_id = str(uuid4())
    event = SSEEvent(type="delta", content="hello", message_id=msg_id)
    data = json.loads(event.model_dump_json())
    assert data["type"] == "delta"
    assert data["content"] == "hello"
    assert data["message_id"] == msg_id


def test_sse_event_done():
    msg_id = str(uuid4())
    event = SSEEvent(type="done", content="", message_id=msg_id)
    assert event.type == "done"
    assert event.content == ""
