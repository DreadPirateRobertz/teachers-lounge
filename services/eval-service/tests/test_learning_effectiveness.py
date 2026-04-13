"""Tests for the learning effectiveness metric module.

Covers:
  - _fetch_quiz_effectiveness: empty result, normal computation, zero pre_rate skip
  - run_learning_effectiveness: no topics short-circuit, successful path,
    below-threshold flag logging
"""
import logging
from contextlib import asynccontextmanager
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from app.learning_effectiveness import _fetch_quiz_effectiveness, run_learning_effectiveness


# ---------------------------------------------------------------------------
# _fetch_quiz_effectiveness
# ---------------------------------------------------------------------------

class TestFetchQuizEffectiveness:
    """Unit tests for the DB-fetching layer of learning effectiveness."""

    def _mock_session(self, fetchall_result):
        """Return a mock get_session context manager yielding a mock DB.

        Args:
            fetchall_result: List of row objects to return from result.fetchall().

        Returns:
            An async context manager function compatible with get_session usage.
        """
        mock_result = MagicMock()
        mock_result.fetchall.return_value = fetchall_result
        mock_db = AsyncMock()
        mock_db.execute.return_value = mock_result

        @asynccontextmanager
        async def _ctx():
            yield mock_db

        return _ctx

    @pytest.mark.asyncio
    async def test_returns_empty_when_no_rows(self):
        """Returns empty list when DB returns no qualifying rows."""
        ctx = self._mock_session([])
        with patch("app.learning_effectiveness.get_session", ctx):
            result = await _fetch_quiz_effectiveness()
        assert result == []

    @pytest.mark.asyncio
    async def test_computes_effectiveness_score(self):
        """Computes effectiveness = (post_rate - pre_rate) / pre_rate correctly."""
        row = MagicMock()
        row.topic = "Chiral Centers"
        row.pre_correct = 10
        row.pre_total = 20
        row.post_correct = 18
        row.post_total = 20

        ctx = self._mock_session([row])
        with patch("app.learning_effectiveness.get_session", ctx):
            result = await _fetch_quiz_effectiveness()

        assert len(result) == 1
        assert result[0]["topic"] == "Chiral Centers"
        assert result[0]["pre_rate"] == pytest.approx(0.5)
        assert result[0]["post_rate"] == pytest.approx(0.9)
        # effectiveness = (0.9 - 0.5) / 0.5 = 0.8
        assert result[0]["effectiveness_score"] == pytest.approx(0.8)
        assert result[0]["sample_size"] == 20

    @pytest.mark.asyncio
    async def test_skips_rows_with_zero_pre_rate(self):
        """Rows where pre_rate == 0 are excluded (no divide-by-zero in score)."""
        row = MagicMock()
        row.topic = "Brand New Topic"
        row.pre_correct = 0
        row.pre_total = 10
        row.post_correct = 8
        row.post_total = 10

        ctx = self._mock_session([row])
        with patch("app.learning_effectiveness.get_session", ctx):
            result = await _fetch_quiz_effectiveness()

        assert result == []

    @pytest.mark.asyncio
    async def test_filters_zero_pre_rate_but_keeps_valid(self):
        """Only rows with non-zero pre_rate are included in output."""
        row_valid = MagicMock()
        row_valid.topic = "Algebra"
        row_valid.pre_correct = 5
        row_valid.pre_total = 10
        row_valid.post_correct = 9
        row_valid.post_total = 10

        row_zero = MagicMock()
        row_zero.topic = "Zero"
        row_zero.pre_correct = 0
        row_zero.pre_total = 10
        row_zero.post_correct = 5
        row_zero.post_total = 10

        ctx = self._mock_session([row_valid, row_zero])
        with patch("app.learning_effectiveness.get_session", ctx):
            result = await _fetch_quiz_effectiveness()

        assert len(result) == 1
        assert result[0]["topic"] == "Algebra"


# ---------------------------------------------------------------------------
# run_learning_effectiveness
# ---------------------------------------------------------------------------

class TestRunLearningEffectiveness:
    @pytest.mark.asyncio
    async def test_returns_empty_when_no_qualifying_topics(self):
        """When no topics meet the minimum sample threshold, returns empty list."""
        with patch("app.learning_effectiveness._fetch_quiz_effectiveness", new_callable=AsyncMock) as mock_fetch:
            mock_fetch.return_value = []
            result = await run_learning_effectiveness()

        assert result == []

    @pytest.mark.asyncio
    async def test_writes_to_bigquery_on_success(self):
        """BigQuery write is called once with the fetched metrics."""
        metrics = [
            {
                "topic": "Chiral Centers",
                "effectiveness_score": 0.45,
                "sample_size": 15,
                "pre_rate": 0.40,
                "post_rate": 0.58,
            }
        ]

        with (
            patch("app.learning_effectiveness._fetch_quiz_effectiveness", new_callable=AsyncMock) as mock_fetch,
            patch("app.learning_effectiveness.write_learning_effectiveness") as mock_write,
        ):
            mock_fetch.return_value = metrics
            result = await run_learning_effectiveness()

        mock_write.assert_called_once()
        assert len(result) == 1
        assert result[0]["topic"] == "Chiral Centers"

    @pytest.mark.asyncio
    async def test_flags_low_effectiveness_topics(self, caplog):
        """Topics below the threshold emit a WARNING log."""
        metrics = [
            {
                "topic": "Nucleophilic Substitution",
                "effectiveness_score": 0.05,   # below 0.1 threshold
                "sample_size": 12,
                "pre_rate": 0.40,
                "post_rate": 0.42,
            }
        ]

        with (
            patch("app.learning_effectiveness._fetch_quiz_effectiveness", new_callable=AsyncMock) as mock_fetch,
            patch("app.learning_effectiveness.write_learning_effectiveness"),
            caplog.at_level(logging.WARNING, logger="app.learning_effectiveness"),
        ):
            mock_fetch.return_value = metrics
            await run_learning_effectiveness()

        assert any("EFFECTIVENESS FLAG" in r.message for r in caplog.records)

    @pytest.mark.asyncio
    async def test_no_flag_for_high_effectiveness_topics(self, caplog):
        """Topics above the threshold do not emit a WARNING flag."""
        metrics = [
            {
                "topic": "Acid-Base Chemistry",
                "effectiveness_score": 0.35,
                "sample_size": 20,
                "pre_rate": 0.50,
                "post_rate": 0.68,
            }
        ]

        with (
            patch("app.learning_effectiveness._fetch_quiz_effectiveness", new_callable=AsyncMock) as mock_fetch,
            patch("app.learning_effectiveness.write_learning_effectiveness"),
            caplog.at_level(logging.WARNING, logger="app.learning_effectiveness"),
        ):
            mock_fetch.return_value = metrics
            await run_learning_effectiveness()

        flag_records = [r for r in caplog.records if "EFFECTIVENESS FLAG" in r.message]
        assert flag_records == []
