"""Tests for the nightly LLM judge module.

Covers:
  - _call_judge: happy path JSON parsing, malformed JSON graceful return
  - run_llm_judge: missing API key short-circuit, no interactions short-circuit,
    successful judge write path
"""
import json
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from app.llm_judge import _call_judge, run_llm_judge


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
