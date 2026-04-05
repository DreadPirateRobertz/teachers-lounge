"""Tests for the weekly RAGAS evaluation module.

Covers:
  - _build_dataset: correct column mapping from interaction dicts
  - run_ragas_evaluation: no interactions short-circuit, evaluation failure
    graceful return, successful run with BigQuery write and alert logging
"""
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from app.ragas_eval import _build_dataset, run_ragas_evaluation


# ---------------------------------------------------------------------------
# _build_dataset
# ---------------------------------------------------------------------------

class TestBuildDataset:
    def test_dataset_has_required_columns(self):
        """Dataset must have question, answer, contexts, ground_truth columns."""
        interactions = [
            {
                "question": "What is osmosis?",
                "answer": "Osmosis is the movement of water through a membrane.",
                "contexts": ["Water moves through semi-permeable membranes."],
                "ground_truth": "Osmosis is the movement of water through a membrane.",
            }
        ]
        ds = _build_dataset(interactions)
        assert "question" in ds.column_names
        assert "answer" in ds.column_names
        assert "contexts" in ds.column_names
        assert "ground_truth" in ds.column_names

    def test_dataset_length_matches_input(self):
        """Dataset row count matches input list length."""
        interactions = [
            {
                "question": f"q{i}",
                "answer": f"a{i}",
                "contexts": [f"c{i}"],
                "ground_truth": f"g{i}",
            }
            for i in range(5)
        ]
        ds = _build_dataset(interactions)
        assert len(ds) == 5


# ---------------------------------------------------------------------------
# run_ragas_evaluation
# ---------------------------------------------------------------------------

class TestRunRagasEvaluation:
    @pytest.mark.asyncio
    async def test_returns_empty_result_when_no_interactions(self):
        """When no RAG interactions exist, returns all None metrics and sample_size=0."""
        with patch("app.ragas_eval._fetch_rag_interactions", new_callable=AsyncMock) as mock_fetch:
            mock_fetch.return_value = []
            result = await run_ragas_evaluation()

        assert result["sample_size"] == 0
        assert result["faithfulness"] is None

    @pytest.mark.asyncio
    async def test_returns_none_metrics_on_ragas_failure(self):
        """If RAGAS evaluation raises, returns None metrics with correct sample_size."""
        interactions = [
            {
                "question": "q",
                "answer": "a",
                "contexts": ["c"],
                "ground_truth": "g",
                "interaction_id": "x",
                "session_id": "y",
            }
        ]
        with (
            patch("app.ragas_eval._fetch_rag_interactions", new_callable=AsyncMock) as mock_fetch,
            patch("app.ragas_eval.evaluate", side_effect=RuntimeError("ragas failure")),
        ):
            mock_fetch.return_value = interactions
            result = await run_ragas_evaluation()

        assert result["faithfulness"] is None
        assert result["sample_size"] == 1

    @pytest.mark.asyncio
    async def test_successful_run_writes_to_bigquery(self):
        """Happy path: RAGAS scores computed and write_ragas_scores called once."""
        interactions = [
            {
                "question": "q",
                "answer": "a",
                "contexts": ["c"],
                "ground_truth": "g",
                "interaction_id": "x",
                "session_id": "y",
            }
        ]

        mock_scores_df = MagicMock()
        mock_scores_df.mean.return_value = {
            "faithfulness": 0.85,
            "answer_relevancy": 0.80,
            "context_precision": 0.75,
            "context_recall": 0.70,
        }
        mock_result = MagicMock()
        mock_result.to_pandas.return_value = mock_scores_df

        with (
            patch("app.ragas_eval._fetch_rag_interactions", new_callable=AsyncMock) as mock_fetch,
            patch("app.ragas_eval.evaluate", return_value=mock_result),
            patch("app.ragas_eval.write_ragas_scores") as mock_write,
        ):
            mock_fetch.return_value = interactions
            result = await run_ragas_evaluation()

        mock_write.assert_called_once()
        assert result["faithfulness"] == pytest.approx(0.85)
        assert result["sample_size"] == 1

    @pytest.mark.asyncio
    async def test_alerts_logged_when_faithfulness_below_threshold(self, caplog):
        """When avg_faithfulness < threshold, an ERROR log is emitted."""
        import logging

        interactions = [
            {
                "question": "q",
                "answer": "a",
                "contexts": ["c"],
                "ground_truth": "g",
                "interaction_id": "x",
                "session_id": "y",
            }
        ]

        mock_scores_df = MagicMock()
        mock_scores_df.mean.return_value = {
            "faithfulness": 0.50,   # below default 0.7 threshold
            "answer_relevancy": 0.80,
            "context_precision": 0.75,
            "context_recall": 0.70,
        }
        mock_result = MagicMock()
        mock_result.to_pandas.return_value = mock_scores_df

        with (
            patch("app.ragas_eval._fetch_rag_interactions", new_callable=AsyncMock) as mock_fetch,
            patch("app.ragas_eval.evaluate", return_value=mock_result),
            patch("app.ragas_eval.write_ragas_scores"),
            caplog.at_level(logging.ERROR, logger="app.ragas_eval"),
        ):
            mock_fetch.return_value = interactions
            await run_ragas_evaluation()

        assert any("RAGAS ALERT" in r.message for r in caplog.records)
