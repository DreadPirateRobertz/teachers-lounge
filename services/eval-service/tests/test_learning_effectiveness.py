"""Tests for the learning effectiveness metric module.

Covers:
  - run_learning_effectiveness: no topics short-circuit, successful path,
    below-threshold flag logging
"""
import logging
from unittest.mock import AsyncMock, patch

import pytest

from app.learning_effectiveness import run_learning_effectiveness


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
