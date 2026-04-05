"""Tests for the nightly analytics CronJob pipeline.

All external I/O (Postgres, BigQuery, Redis, Qdrant, Anthropic, OpenAI) is
replaced with mocks so the test suite runs without any infrastructure.

Coverage targets:
- anonymize_user_id: determinism, salt isolation
- add_laplace_noise: output contract (non-negative for counts)
- _aggregate_topic_difficulty: k-anonymity suppression, DP noise, grouping
- _aggregate_engagement: empty-data handling, k-anonymity suppression
- _aggregate_learning_curves: cohort grouping and k-anonymity
- _generate_insight: Anthropic call forwarding
- _embed_text: OpenAI call forwarding
- _cache_top_insights: Redis key format and TTL
- run_pipeline: full orchestration happy-path; skips bad insight gracefully
"""
from __future__ import annotations

import json
from datetime import datetime, timezone
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from app.cron_job import (
    _aggregate_engagement,
    _aggregate_learning_curves,
    _aggregate_topic_difficulty,
    _cache_top_insights,
    _embed_text,
    _generate_insight,
    add_laplace_noise,
    anonymize_user_id,
    run_pipeline,
)

# ── Fixtures and helpers ──────────────────────────────────────────────────────

SALT = "2026-04-05"
UID_A = "user-aaa"
UID_B = "user-bbb"
UID_C = "user-ccc"
RUN_DATE = "2026-04-05"


def _ts(hour: int = 10) -> datetime:
    """Create a UTC datetime for a given hour on the test date."""
    return datetime(2026, 4, 5, hour, 0, 0, tzinfo=timezone.utc)


def _make_rows(
    user_ids: list[str],
    topic: str,
    is_correct: bool,
    session_id: str = "s1",
) -> list[dict]:
    """Build minimal interaction/quiz rows for the given users.

    Args:
        user_ids: List of user UUID strings.
        topic: Topic name.
        is_correct: Whether each answer is correct.
        session_id: Session identifier.

    Returns:
        List of row dicts compatible with ``_aggregate_topic_difficulty``.
    """
    return [
        {
            "user_id": uid,
            "session_id": session_id,
            "role": "student",
            "created_at": _ts(i),
            "topic": topic,
            "is_correct": is_correct,
        }
        for i, uid in enumerate(user_ids)
    ]


# ── anonymize_user_id ─────────────────────────────────────────────────────────


class TestAnonymizeUserId:
    """Tests for anonymize_user_id."""

    def test_deterministic(self):
        """Same inputs always produce the same output."""
        assert anonymize_user_id(UID_A, SALT) == anonymize_user_id(UID_A, SALT)

    def test_salt_isolation(self):
        """Different salts produce different pseudonyms for the same user."""
        assert anonymize_user_id(UID_A, "2026-04-05") != anonymize_user_id(UID_A, "2026-04-06")

    def test_user_isolation(self):
        """Different users produce different pseudonyms with the same salt."""
        assert anonymize_user_id(UID_A, SALT) != anonymize_user_id(UID_B, SALT)

    def test_hex_output(self):
        """Output is a 64-character hex string (SHA-256 digest)."""
        result = anonymize_user_id(UID_A, SALT)
        assert len(result) == 64
        assert all(c in "0123456789abcdef" for c in result)


# ── add_laplace_noise ─────────────────────────────────────────────────────────


class TestAddLaplaceNoise:
    """Tests for add_laplace_noise."""

    def test_non_negative(self):
        """Result is always >= 0 (counts cannot be negative)."""
        for _ in range(200):
            assert add_laplace_noise(0.0, scale=5.0) >= 0.0

    def test_preserves_large_value(self):
        """For very large values the noise is relatively small on average."""
        results = [add_laplace_noise(10_000.0, scale=1.0) for _ in range(100)]
        avg = sum(results) / len(results)
        # mean should be close to 10000 (Laplace mean = 0 shift)
        assert 9900 < avg < 10100


# ── _aggregate_topic_difficulty ───────────────────────────────────────────────


class TestAggregateTopicDifficulty:
    """Tests for _aggregate_topic_difficulty."""

    def test_k_anonymity_suppression(self):
        """Topics with fewer than k students are suppressed."""
        rows = _make_rows([UID_A, UID_B], "algebra", False)  # 2 students < k=10
        result = _aggregate_topic_difficulty(rows, SALT, k=10, noise_scale=0.0, run_date=RUN_DATE)
        assert result == []

    def test_k_anonymity_passes(self):
        """Topics with >= k students are exported."""
        many_users = [f"user-{i}" for i in range(12)]
        rows = _make_rows(many_users, "calculus", False)
        result = _aggregate_topic_difficulty(rows, SALT, k=10, noise_scale=0.0, run_date=RUN_DATE)
        assert len(result) == 1
        assert result[0]["topic"] == "calculus"

    def test_error_rate_all_incorrect(self):
        """100% wrong answers → error_rate == 1.0."""
        many_users = [f"user-{i}" for i in range(11)]
        rows = _make_rows(many_users, "geometry", is_correct=False)
        result = _aggregate_topic_difficulty(rows, SALT, k=10, noise_scale=0.0, run_date=RUN_DATE)
        assert result[0]["error_rate"] == 1.0

    def test_error_rate_all_correct(self):
        """100% correct answers → error_rate == 0.0."""
        many_users = [f"user-{i}" for i in range(11)]
        rows = _make_rows(many_users, "geometry", is_correct=True)
        result = _aggregate_topic_difficulty(rows, SALT, k=10, noise_scale=0.0, run_date=RUN_DATE)
        assert result[0]["error_rate"] == 0.0

    def test_rows_with_no_topic_skipped(self):
        """Rows with no topic are ignored."""
        rows = [{"user_id": UID_A, "session_id": "s1", "role": "student",
                 "created_at": _ts(), "topic": None, "is_correct": False}]
        result = _aggregate_topic_difficulty(rows, SALT, k=1, noise_scale=0.0, run_date=RUN_DATE)
        assert result == []

    def test_run_date_in_output(self):
        """run_date is propagated to each output row."""
        many_users = [f"user-{i}" for i in range(11)]
        rows = _make_rows(many_users, "physics", False)
        result = _aggregate_topic_difficulty(rows, SALT, k=10, noise_scale=0.0, run_date="2099-01-01")
        assert result[0]["run_date"] == "2099-01-01"


# ── _aggregate_engagement ─────────────────────────────────────────────────────


class TestAggregateEngagement:
    """Tests for _aggregate_engagement."""

    def test_empty_rows_returns_empty(self):
        """No rows → empty output."""
        result = _aggregate_engagement([], SALT, k=1, noise_scale=0.0, run_date=RUN_DATE)
        assert result == []

    def test_k_anonymity_suppression(self):
        """Fewer than k students → empty output."""
        rows = _make_rows([UID_A], "topic", False)
        result = _aggregate_engagement(rows, SALT, k=10, noise_scale=0.0, run_date=RUN_DATE)
        assert result == []

    def test_k_anonymity_passes(self):
        """Enough students → single engagement row."""
        users = [f"user-{i}" for i in range(11)]
        rows = _make_rows(users, "topic", False)
        result = _aggregate_engagement(rows, SALT, k=10, noise_scale=0.0, run_date=RUN_DATE)
        assert len(result) == 1
        assert result[0]["run_date"] == RUN_DATE
        assert result[0]["avg_session_length_min"] >= 0.0


# ── _aggregate_learning_curves ────────────────────────────────────────────────


class TestAggregateLearningCurves:
    """Tests for _aggregate_learning_curves."""

    def _mastery_rows(self, user_ids: list[str], topic: str, score: float) -> list[dict]:
        """Build mastery rows for the given users.

        Args:
            user_ids: List of user ID strings.
            topic: Topic name.
            score: Mastery score to assign.

        Returns:
            List of mastery row dicts.
        """
        return [
            {"user_id": uid, "topic": topic, "mastery_score": score, "created_at": _ts(i)}
            for i, uid in enumerate(user_ids)
        ]

    def test_k_anonymity_suppression(self):
        """Fewer than k students → no rows."""
        rows = self._mastery_rows([UID_A], "trigonometry", 0.7)
        result = _aggregate_learning_curves(rows, SALT, k=5, noise_scale=0.0, run_date=RUN_DATE)
        assert result == []

    def test_aggregation(self):
        """Avg mastery_score computed correctly over cohort."""
        users = [f"u{i}" for i in range(6)]
        rows = self._mastery_rows(users, "trigonometry", 0.8)
        result = _aggregate_learning_curves(rows, SALT, k=5, noise_scale=0.0, run_date=RUN_DATE)
        assert len(result) == 1
        assert result[0]["avg_mastery_score"] == pytest.approx(0.8, abs=1e-4)


# ── _generate_insight ─────────────────────────────────────────────────────────


class TestGenerateInsight:
    """Tests for _generate_insight."""

    @pytest.mark.asyncio
    async def test_returns_text(self):
        """Returns stripped text from the Anthropic response."""
        mock_client = AsyncMock()
        mock_message = MagicMock()
        mock_message.content = [MagicMock(text="  Students confuse X with Y.  ")]
        mock_client.messages.create.return_value = mock_message

        result = await _generate_insight(mock_client, "algebra", 0.6, 50)
        assert result == "Students confuse X with Y."

    @pytest.mark.asyncio
    async def test_uses_haiku_model(self):
        """Always calls Claude Haiku (claude-haiku-4-5-20251001)."""
        mock_client = AsyncMock()
        mock_message = MagicMock()
        mock_message.content = [MagicMock(text="Insight.")]
        mock_client.messages.create.return_value = mock_message

        await _generate_insight(mock_client, "algebra", 0.6, 50)
        call_kwargs = mock_client.messages.create.call_args.kwargs
        assert call_kwargs["model"] == "claude-haiku-4-5-20251001"


# ── _embed_text ───────────────────────────────────────────────────────────────


class TestEmbedText:
    """Tests for _embed_text."""

    @pytest.mark.asyncio
    async def test_returns_vector(self):
        """Returns the embedding vector from the OpenAI response."""
        mock_client = AsyncMock()
        mock_response = MagicMock()
        mock_response.data = [MagicMock(embedding=[0.1, 0.2, 0.3])]
        mock_client.embeddings.create.return_value = mock_response

        result = await _embed_text(mock_client, "some insight text")
        assert result == [0.1, 0.2, 0.3]

    @pytest.mark.asyncio
    async def test_uses_large_model(self):
        """Always uses text-embedding-3-large."""
        mock_client = AsyncMock()
        mock_response = MagicMock()
        mock_response.data = [MagicMock(embedding=[0.0])]
        mock_client.embeddings.create.return_value = mock_response

        await _embed_text(mock_client, "text")
        call_kwargs = mock_client.embeddings.create.call_args.kwargs
        assert call_kwargs["model"] == "text-embedding-3-large"


# ── _cache_top_insights ───────────────────────────────────────────────────────


class TestCacheTopInsights:
    """Tests for _cache_top_insights."""

    @pytest.mark.asyncio
    async def test_sets_correct_key_and_ttl(self):
        """Writes JSON to cache:insight:{topic} with the given TTL."""
        mock_redis = AsyncMock()
        await _cache_top_insights(
            mock_redis,
            {"calculus": ["Insight A", "Insight B"]},
            ttl=21600,
        )
        mock_redis.set.assert_called_once_with(
            "cache:insight:calculus",
            json.dumps(["Insight A", "Insight B"]),
            ex=21600,
        )

    @pytest.mark.asyncio
    async def test_top_3_only(self):
        """Only the first 3 insights are cached when more are provided."""
        mock_redis = AsyncMock()
        insights = [f"Insight {i}" for i in range(10)]
        await _cache_top_insights(mock_redis, {"topic": insights}, ttl=100)
        call_args = mock_redis.set.call_args
        cached = json.loads(call_args[0][1])
        assert len(cached) == 3


# ── run_pipeline (integration smoke test) ────────────────────────────────────


class TestRunPipeline:
    """Smoke tests for the full run_pipeline orchestration."""

    def _make_db(self, interaction_rows: list[dict], mastery_rows: list[dict]) -> AsyncMock:
        """Build an async DB mock with two execute() side-effects.

        Args:
            interaction_rows: Rows returned for the interactions query.
            mastery_rows: Rows returned for the mastery query.

        Returns:
            AsyncMock SQLAlchemy session.
        """
        def _result(rows):
            mock = MagicMock()
            mock.mappings.return_value.all.return_value = rows
            return mock

        db = AsyncMock()
        db.execute.side_effect = [_result(interaction_rows), _result(mastery_rows)]
        return db

    @pytest.mark.asyncio
    async def test_happy_path_no_high_error_topics(self):
        """Pipeline runs without errors when no topics exceed error threshold."""
        users = [f"user-{i}" for i in range(11)]
        rows = _make_rows(users, "easy_topic", is_correct=True)

        db = self._make_db(rows, [])
        bq = MagicMock()
        bq.list_tables.return_value = []
        bq.insert_rows_json.return_value = []
        redis = AsyncMock()
        qdrant = MagicMock()
        qdrant.get_collections.return_value = MagicMock(collections=[])
        anthropic_c = AsyncMock()
        openai_c = AsyncMock()

        await run_pipeline(db, bq, redis, qdrant, anthropic_c, openai_c)

        # No insights → Anthropic not called
        anthropic_c.messages.create.assert_not_called()

    @pytest.mark.asyncio
    async def test_high_error_rate_triggers_insight(self):
        """Topics with error_rate > 0.4 trigger Claude Haiku insight generation."""
        users = [f"user-{i}" for i in range(15)]
        rows = _make_rows(users, "hard_topic", is_correct=False)

        db = self._make_db(rows, [])
        bq = MagicMock()
        bq.list_tables.return_value = []
        bq.insert_rows_json.return_value = []
        redis = AsyncMock()
        qdrant = MagicMock()
        qdrant.get_collections.return_value = MagicMock(collections=[])

        mock_message = MagicMock()
        mock_message.content = [MagicMock(text="Students struggle here.")]
        anthropic_c = AsyncMock()
        anthropic_c.messages.create.return_value = mock_message

        mock_embed_response = MagicMock()
        mock_embed_response.data = [MagicMock(embedding=[0.1] * 3072)]
        openai_c = AsyncMock()
        openai_c.embeddings.create.return_value = mock_embed_response

        await run_pipeline(db, bq, redis, qdrant, anthropic_c, openai_c)

        anthropic_c.messages.create.assert_called_once()
        openai_c.embeddings.create.assert_called_once()
        redis.set.assert_called_once()

    @pytest.mark.asyncio
    async def test_insight_error_does_not_abort_pipeline(self):
        """A failure in insight generation is logged but does not crash the job."""
        users = [f"user-{i}" for i in range(15)]
        rows = _make_rows(users, "hard_topic", is_correct=False)

        db = self._make_db(rows, [])
        bq = MagicMock()
        bq.list_tables.return_value = []
        bq.insert_rows_json.return_value = []
        redis = AsyncMock()
        qdrant = MagicMock()
        qdrant.get_collections.return_value = MagicMock(collections=[])

        anthropic_c = AsyncMock()
        anthropic_c.messages.create.side_effect = RuntimeError("API down")
        openai_c = AsyncMock()

        # Should not raise — error is caught and logged per pipeline contract
        await run_pipeline(db, bq, redis, qdrant, anthropic_c, openai_c)

        # Redis cache not set because insight generation failed
        redis.set.assert_not_called()
