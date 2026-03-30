"""Tests for chat helper functions."""
import os

os.environ.setdefault("DATABASE_URL", "postgresql+asyncpg://test:test@localhost:5432/test")
os.environ.setdefault("AI_GATEWAY_KEY", "test-key")
os.environ.setdefault("JWT_SECRET", "test-secret-that-is-long-enough-for-hs256")

import json
from unittest.mock import MagicMock
from uuid import uuid4

import pytest

from app.chat import FALLBACK_MESSAGE, PROFESSOR_NOVA_SYSTEM_PROMPT, _history_to_messages, _sse


def _mock_interaction(role: str, content: str):
    m = MagicMock()
    m.role = role
    m.content = content
    return m


def test_history_student_maps_to_user():
    interactions = [_mock_interaction("student", "What is gravity?")]
    result = _history_to_messages(interactions)
    assert result == [{"role": "user", "content": "What is gravity?"}]


def test_history_tutor_maps_to_assistant():
    interactions = [_mock_interaction("tutor", "Gravity is a force...")]
    result = _history_to_messages(interactions)
    assert result == [{"role": "assistant", "content": "Gravity is a force..."}]


def test_history_alternating_roles():
    interactions = [
        _mock_interaction("student", "Q1"),
        _mock_interaction("tutor", "A1"),
        _mock_interaction("student", "Q2"),
        _mock_interaction("tutor", "A2"),
    ]
    result = _history_to_messages(interactions)
    assert result[0] == {"role": "user", "content": "Q1"}
    assert result[1] == {"role": "assistant", "content": "A1"}
    assert result[2] == {"role": "user", "content": "Q2"}
    assert result[3] == {"role": "assistant", "content": "A2"}


def test_history_empty():
    assert _history_to_messages([]) == []


@pytest.mark.asyncio
async def test_sse_delta_format():
    msg_id = str(uuid4())
    result = await _sse("delta", content="hello", message_id=msg_id)
    assert result.startswith("data: ")
    assert result.endswith("\n\n")
    payload = json.loads(result[len("data: "):].strip())
    assert payload["type"] == "delta"
    assert payload["content"] == "hello"
    assert payload["message_id"] == msg_id


@pytest.mark.asyncio
async def test_sse_done_format():
    msg_id = str(uuid4())
    result = await _sse("done", message_id=msg_id)
    payload = json.loads(result[len("data: "):].strip())
    assert payload["type"] == "done"
    assert payload["content"] == ""


def test_system_prompt_contains_nova():
    assert "Professor Nova" in PROFESSOR_NOVA_SYSTEM_PROMPT


def test_fallback_message_is_non_empty():
    assert len(FALLBACK_MESSAGE) > 0
