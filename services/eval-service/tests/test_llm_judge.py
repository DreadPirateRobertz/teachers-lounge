"""Tests for the nightly LLM judge module.

Covers:
  - _sample_interactions: DB result mapping, empty result
  - _persist_judge_result: execute+commit called, composite score calculation
  - _call_judge: happy path JSON parsing, malformed JSON graceful return
  - run_llm_judge: missing API key short-circuit, no interactions short-circuit,
    successful judge write path
"""
import json
from contextlib import asynccontextmanager
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from app.llm_judge import _call_judge, _persist_judge_result, _sample_interactions, run_llm_judge


# ---------------------------------------------------------------------------
# _sample_interactions
# ---------------------------------------------------------------------------

def _make_session_ctx(fetchall_result):
    """Build an async context manager that yields a mock DB session.

    Args:
        fetchall_result: List of row objects returned by result.fetchall().

    Returns:
        An async context manager function.
    """
    mock_result = MagicMock()
    mock_result.fetchall.return_value = fetchall_result
    mock_db = AsyncMock()
    mock_db.execute.return_value = mock_result

    @asynccontextmanager
    async def _ctx():
        yield mock_db

    return _ctx


class TestSampleInteractions:
    """Unit tests for the _sample_interactions DB query helper."""

    @pytest.mark.asyncio
    async def test_returns_interaction_dicts(self):
        """Maps DB rows to dicts with interaction_id, question, answer."""
        row = MagicMock()
        row.interaction_id = "abc-123"
        row.question = "What is osmosis?"
        row.answer = "Water moves through membranes."

        ctx = _make_session_ctx([row])
        with patch("app.llm_judge.get_session", ctx):
            result = await _sample_interactions(5)

        assert len(result) == 1
        assert result[0]["interaction_id"] == "abc-123"
        assert result[0]["question"] == "What is osmosis?"
        assert result[0]["answer"] == "Water moves through membranes."

    @pytest.mark.asyncio
    async def test_returns_empty_when_no_rows(self):
        """Returns empty list when no unjudged interactions exist."""
        ctx = _make_session_ctx([])
        with patch("app.llm_judge.get_session", ctx):
            result = await _sample_interactions(10)

        assert result == []


# ---------------------------------------------------------------------------
# _persist_judge_result
# ---------------------------------------------------------------------------

class TestPersistJudgeResult:
    """Unit tests for the _persist_judge_result DB write helper."""

    @pytest.mark.asyncio
    async def test_executes_insert_and_commits(self):
        """Calls db.execute and db.commit for a valid scores dict."""
        mock_db = AsyncMock()

        @asynccontextmanager
        async def mock_session():
            yield mock_db

        scores = {"directness": 4, "pace": 3, "grounding": 5, "reasoning": "Good."}
        with patch("app.llm_judge.get_session", mock_session):
            await _persist_judge_result("test-uuid", scores)

        mock_db.execute.assert_awaited_once()
        mock_db.commit.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_computes_composite_as_rounded_average(self):
        """Composite score is round((directness + pace + grounding) / 3)."""
        mock_db = AsyncMock()
        captured: dict = {}

        async def capture_execute(sql, params):
            captured.update(params)

        mock_db.execute.side_effect = capture_execute

        @asynccontextmanager
        async def mock_session():
            yield mock_db

        # round((3 + 4 + 5) / 3) = round(4.0) = 4
        scores = {"directness": 3, "pace": 4, "grounding": 5, "reasoning": "Test."}
        with patch("app.llm_judge.get_session", mock_session):
            await _persist_judge_result("test-uuid", scores)

        assert captured["judge_score"] == 4

    @pytest.mark.asyncio
    async def test_handles_missing_score_keys(self):
        """Missing score keys default to 0 without raising."""
        mock_db = AsyncMock()

        @asynccontextmanager
        async def mock_session():
            yield mock_db

        with patch("app.llm_judge.get_session", mock_session):
            # Should not raise even with empty scores
            await _persist_judge_result("test-uuid", {})

        mock_db.commit.assert_awaited_once()


# ---------------------------------------------------------------------------
# _call_judge
# ---------------------------------------------------------------------------

class TestCallJudge:
    def test_returns_parsed_dict_on_valid_json(self):
        """Happy path: Haiku returns well-formed JSON with all expected keys."""
        fake_response = json.dumps(
            {"directness": 4, "pace": 3, "grounding": 5, "reasoning": "Clear and grounded."}
        )
        mock_message = MagicMock()
        mock_message.content = [MagicMock(text=fake_response)]

        with patch("app.llm_judge.anthropic.Anthropic") as mock_cls:
            mock_client = mock_cls.return_value
            mock_client.messages.create.return_value = mock_message

            result = _call_judge(
                question="What is a chiral center?",
                answer="A chiral center is a carbon bonded to four different groups.",
            )

        assert result is not None
        assert result["directness"] == 4
        assert result["pace"] == 3
        assert result["grounding"] == 5
        assert "reasoning" in result

    def test_returns_none_on_malformed_json(self):
        """If Haiku returns non-JSON, _call_judge returns None without raising."""
        mock_message = MagicMock()
        mock_message.content = [MagicMock(text="Sorry, I cannot rate this.")]

        with patch("app.llm_judge.anthropic.Anthropic") as mock_cls:
            mock_client = mock_cls.return_value
            mock_client.messages.create.return_value = mock_message

            result = _call_judge(question="q", answer="a")

        assert result is None

    def test_returns_none_on_api_error(self):
        """Network/API errors are caught and return None."""
        with patch("app.llm_judge.anthropic.Anthropic") as mock_cls:
            mock_client = mock_cls.return_value
            mock_client.messages.create.side_effect = Exception("connection timeout")

            result = _call_judge(question="q", answer="a")

        assert result is None


# ---------------------------------------------------------------------------
# run_llm_judge
# ---------------------------------------------------------------------------

class TestRunLlmJudge:
    @pytest.mark.asyncio
    async def test_returns_zero_when_api_key_missing(self, monkeypatch):
        """When ANTHROPIC_API_KEY is empty, run_llm_judge exits early with 0."""
        monkeypatch.setattr("app.llm_judge.settings.anthropic_api_key", "")
        result = await run_llm_judge()
        assert result == 0

    @pytest.mark.asyncio
    async def test_returns_zero_when_no_interactions(self, monkeypatch):
        """When no unjudged interactions exist, returns 0."""
        monkeypatch.setattr("app.llm_judge.settings.anthropic_api_key", "test-key")

        with patch("app.llm_judge._sample_interactions", new_callable=AsyncMock) as mock_sample:
            mock_sample.return_value = []
            result = await run_llm_judge()

        assert result == 0

    @pytest.mark.asyncio
    async def test_judges_all_sampled_interactions(self, monkeypatch):
        """With valid interactions and successful judge, returns judged count."""
        monkeypatch.setattr("app.llm_judge.settings.anthropic_api_key", "test-key")

        interactions = [
            {"interaction_id": "aaa", "question": "q1", "answer": "a1"},
            {"interaction_id": "bbb", "question": "q2", "answer": "a2"},
        ]

        fake_scores = {"directness": 4, "pace": 4, "grounding": 4, "reasoning": "Good."}

        with (
            patch("app.llm_judge._sample_interactions", new_callable=AsyncMock) as mock_sample,
            patch("app.llm_judge._call_judge", return_value=fake_scores),
            patch("app.llm_judge._persist_judge_result", new_callable=AsyncMock) as mock_persist,
        ):
            mock_sample.return_value = interactions
            result = await run_llm_judge()

        assert result == 2
        assert mock_persist.call_count == 2

    @pytest.mark.asyncio
    async def test_skips_interaction_when_judge_returns_none(self, monkeypatch):
        """If _call_judge returns None for an interaction, it is skipped (not persisted)."""
        monkeypatch.setattr("app.llm_judge.settings.anthropic_api_key", "test-key")

        interactions = [{"interaction_id": "aaa", "question": "q", "answer": "a"}]

        with (
            patch("app.llm_judge._sample_interactions", new_callable=AsyncMock) as mock_sample,
            patch("app.llm_judge._call_judge", return_value=None),
            patch("app.llm_judge._persist_judge_result", new_callable=AsyncMock) as mock_persist,
        ):
            mock_sample.return_value = interactions
            result = await run_llm_judge()

        assert result == 0
        mock_persist.assert_not_called()
