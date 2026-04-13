"""Tests for the BigQuery writer module.

Covers:
  - _get_client: constructs a BigQuery client for the configured project
  - _ensure_table: calls create_table with exists_ok=True
  - write_ragas_scores: inserts one row; logs error on BQ failure
  - write_learning_effectiveness: inserts one row per topic; logs error on failure
"""
import logging
from datetime import date
from unittest.mock import MagicMock, patch

import pytest

from app.bigquery import (
    _ensure_table,
    _get_client,
    write_learning_effectiveness,
    write_ragas_scores,
)


# ---------------------------------------------------------------------------
# _get_client
# ---------------------------------------------------------------------------

class TestGetClient:
    """Unit tests for the BigQuery client factory."""

    def test_returns_client_for_configured_project(self):
        """_get_client passes the configured bigquery_project to the Client constructor."""
        with patch("app.bigquery.bigquery.Client") as mock_cls:
            mock_cls.return_value = MagicMock()
            client = _get_client()

        mock_cls.assert_called_once_with(project="teachers-lounge")
        assert client is mock_cls.return_value


# ---------------------------------------------------------------------------
# _ensure_table
# ---------------------------------------------------------------------------

class TestEnsureTable:
    """Unit tests for the BigQuery table-creation helper."""

    def test_calls_create_table_with_exists_ok_true(self):
        """create_table is called with exists_ok=True so repeated runs are safe."""
        mock_client = MagicMock()

        with patch("app.bigquery.bigquery.Table"), \
             patch("app.bigquery.bigquery.TimePartitioning"):
            _ensure_table(mock_client, "project.dataset.table", [])

        mock_client.create_table.assert_called_once()
        _, kwargs = mock_client.create_table.call_args
        assert kwargs.get("exists_ok") is True


# ---------------------------------------------------------------------------
# write_ragas_scores
# ---------------------------------------------------------------------------

class TestWriteRagasScores:
    """Unit tests for the weekly RAGAS scores BigQuery writer."""

    def _mock_bq_client(self, insert_errors=None):
        """Return a mock BigQuery client.

        Args:
            insert_errors: List of error dicts returned by insert_rows_json.
                Pass [] for success, non-empty list for failure.

        Returns:
            MagicMock acting as a BigQuery client.
        """
        mock_client = MagicMock()
        mock_client.insert_rows_json.return_value = insert_errors or []
        return mock_client

    def test_inserts_one_row_with_correct_week(self):
        """Inserts exactly one row whose week matches the supplied date."""
        mock_client = self._mock_bq_client()

        with patch("app.bigquery._get_client", return_value=mock_client), \
             patch("app.bigquery._ensure_table"):
            write_ragas_scores(
                week=date(2026, 4, 1),
                avg_faithfulness=0.85,
                avg_relevancy=0.80,
                avg_context_precision=0.75,
                avg_context_recall=0.70,
                sample_size=50,
            )

        mock_client.insert_rows_json.assert_called_once()
        rows_arg = mock_client.insert_rows_json.call_args[0][1]
        assert len(rows_arg) == 1
        assert rows_arg[0]["week"] == "2026-04-01"
        assert rows_arg[0]["sample_size"] == 50

    def test_accepts_none_metrics(self):
        """Accepts None metric values without raising (partial evaluation result)."""
        mock_client = self._mock_bq_client()

        with patch("app.bigquery._get_client", return_value=mock_client), \
             patch("app.bigquery._ensure_table"):
            write_ragas_scores(
                week=date(2026, 4, 1),
                avg_faithfulness=None,
                avg_relevancy=None,
                avg_context_precision=None,
                avg_context_recall=None,
                sample_size=0,
            )

        rows_arg = mock_client.insert_rows_json.call_args[0][1]
        assert rows_arg[0]["avg_faithfulness"] is None

    def test_logs_error_on_insert_failure(self, caplog):
        """Logs an error message when BigQuery returns insertion errors."""
        mock_client = self._mock_bq_client(insert_errors=[{"error": "quota exceeded"}])

        with patch("app.bigquery._get_client", return_value=mock_client), \
             patch("app.bigquery._ensure_table"), \
             caplog.at_level(logging.ERROR, logger="app.bigquery"):
            write_ragas_scores(
                week=date(2026, 4, 1),
                avg_faithfulness=None,
                avg_relevancy=None,
                avg_context_precision=None,
                avg_context_recall=None,
                sample_size=0,
            )

        assert any("BigQuery insert errors" in r.message for r in caplog.records)


# ---------------------------------------------------------------------------
# write_learning_effectiveness
# ---------------------------------------------------------------------------

class TestWriteLearningEffectiveness:
    """Unit tests for the weekly learning effectiveness BigQuery writer."""

    def _mock_bq_client(self, insert_errors=None):
        """Return a mock BigQuery client.

        Args:
            insert_errors: Errors returned by insert_rows_json ([] = success).

        Returns:
            MagicMock acting as a BigQuery client.
        """
        mock_client = MagicMock()
        mock_client.insert_rows_json.return_value = insert_errors or []
        return mock_client

    def test_inserts_one_row_per_topic(self):
        """Inserts one BQ row for each topic in the input list."""
        mock_client = self._mock_bq_client()
        rows = [
            {"topic": "Algebra", "effectiveness_score": 0.5, "sample_size": 12},
            {"topic": "Calculus", "effectiveness_score": 0.3, "sample_size": 15},
        ]

        with patch("app.bigquery._get_client", return_value=mock_client), \
             patch("app.bigquery._ensure_table"):
            write_learning_effectiveness(week=date(2026, 4, 1), rows=rows)

        bq_rows = mock_client.insert_rows_json.call_args[0][1]
        assert len(bq_rows) == 2
        assert bq_rows[0]["topic"] == "Algebra"
        assert bq_rows[1]["topic"] == "Calculus"
        assert bq_rows[0]["week"] == "2026-04-01"

    def test_effectiveness_score_preserved(self):
        """The effectiveness_score value in BQ matches the input value."""
        mock_client = self._mock_bq_client()

        with patch("app.bigquery._get_client", return_value=mock_client), \
             patch("app.bigquery._ensure_table"):
            write_learning_effectiveness(
                week=date(2026, 4, 1),
                rows=[{"topic": "X", "effectiveness_score": 0.42, "sample_size": 11}],
            )

        bq_rows = mock_client.insert_rows_json.call_args[0][1]
        assert bq_rows[0]["effectiveness_score"] == pytest.approx(0.42)

    def test_logs_error_on_insert_failure(self, caplog):
        """Logs an error message when BigQuery returns insertion errors."""
        mock_client = self._mock_bq_client(insert_errors=[{"error": "table not found"}])

        with patch("app.bigquery._get_client", return_value=mock_client), \
             patch("app.bigquery._ensure_table"), \
             caplog.at_level(logging.ERROR, logger="app.bigquery"):
            write_learning_effectiveness(
                week=date(2026, 4, 1),
                rows=[{"topic": "X", "effectiveness_score": 0.1, "sample_size": 10}],
            )

        assert any("BigQuery insert errors" in r.message for r in caplog.records)
