"""BigQuery writer for evaluation metrics.

Creates tables on first write (if they do not exist) and appends rows.
Two tables are maintained:

  analytics.ragas_scores
    week DATE, avg_faithfulness FLOAT, avg_relevancy FLOAT,
    avg_context_precision FLOAT, avg_context_recall FLOAT, sample_size INT

  analytics.learning_effectiveness
    week DATE, topic STRING, effectiveness_score FLOAT, sample_size INT
"""
import logging
from datetime import date
from typing import Any

from google.cloud import bigquery

from .config import settings

logger = logging.getLogger(__name__)

_RAGAS_SCHEMA = [
    bigquery.SchemaField("week", "DATE", mode="REQUIRED"),
    bigquery.SchemaField("avg_faithfulness", "FLOAT64", mode="NULLABLE"),
    bigquery.SchemaField("avg_relevancy", "FLOAT64", mode="NULLABLE"),
    bigquery.SchemaField("avg_context_precision", "FLOAT64", mode="NULLABLE"),
    bigquery.SchemaField("avg_context_recall", "FLOAT64", mode="NULLABLE"),
    bigquery.SchemaField("sample_size", "INT64", mode="REQUIRED"),
]

_EFFECTIVENESS_SCHEMA = [
    bigquery.SchemaField("week", "DATE", mode="REQUIRED"),
    bigquery.SchemaField("topic", "STRING", mode="REQUIRED"),
    bigquery.SchemaField("effectiveness_score", "FLOAT64", mode="REQUIRED"),
    bigquery.SchemaField("sample_size", "INT64", mode="REQUIRED"),
]


def _get_client() -> bigquery.Client:
    """Return an authenticated BigQuery client.

    Uses Application Default Credentials (Workload Identity on GKE).

    Returns:
        BigQuery client for the configured project.
    """
    return bigquery.Client(project=settings.bigquery_project)


def _ensure_table(client: bigquery.Client, table_id: str, schema: list[bigquery.SchemaField]) -> None:
    """Create the table if it does not already exist.

    Args:
        client: Authenticated BigQuery client.
        table_id: Fully-qualified table ID (project.dataset.table).
        schema: List of SchemaField definitions.
    """
    table = bigquery.Table(table_id, schema=schema)
    table.time_partitioning = bigquery.TimePartitioning(field="week")
    client.create_table(table, exists_ok=True)
    logger.info("BigQuery table ensured: %s", table_id)


def write_ragas_scores(
    week: date,
    avg_faithfulness: float | None,
    avg_relevancy: float | None,
    avg_context_precision: float | None,
    avg_context_recall: float | None,
    sample_size: int,
) -> None:
    """Append a weekly RAGAS evaluation row to BigQuery.

    Args:
        week: ISO week start date (Sunday).
        avg_faithfulness: Mean RAGAS faithfulness score (0–1), or None if unavailable.
        avg_relevancy: Mean RAGAS answer_relevancy score (0–1), or None if unavailable.
        avg_context_precision: Mean RAGAS context_precision score, or None.
        avg_context_recall: Mean RAGAS context_recall score, or None.
        sample_size: Number of interactions evaluated.
    """
    client = _get_client()
    table_id = f"{settings.bigquery_project}.{settings.bigquery_dataset}.ragas_scores"
    _ensure_table(client, table_id, _RAGAS_SCHEMA)

    rows: list[dict[str, Any]] = [
        {
            "week": week.isoformat(),
            "avg_faithfulness": avg_faithfulness,
            "avg_relevancy": avg_relevancy,
            "avg_context_precision": avg_context_precision,
            "avg_context_recall": avg_context_recall,
            "sample_size": sample_size,
        }
    ]
    errors = client.insert_rows_json(table_id, rows)
    if errors:
        logger.error("BigQuery insert errors (ragas_scores): %s", errors)
    else:
        logger.info(
            "ragas_scores written: week=%s faithfulness=%.3f sample=%d",
            week,
            avg_faithfulness or 0.0,
            sample_size,
        )


def write_learning_effectiveness(
    week: date,
    rows: list[dict[str, Any]],
) -> None:
    """Append weekly learning effectiveness rows to BigQuery.

    Args:
        week: ISO week start date (Sunday).
        rows: List of dicts with keys: topic (str), effectiveness_score (float),
            sample_size (int).
    """
    client = _get_client()
    table_id = f"{settings.bigquery_project}.{settings.bigquery_dataset}.learning_effectiveness"
    _ensure_table(client, table_id, _EFFECTIVENESS_SCHEMA)

    bq_rows = [
        {
            "week": week.isoformat(),
            "topic": r["topic"],
            "effectiveness_score": r["effectiveness_score"],
            "sample_size": r["sample_size"],
        }
        for r in rows
    ]
    errors = client.insert_rows_json(table_id, bq_rows)
    if errors:
        logger.error("BigQuery insert errors (learning_effectiveness): %s", errors)
    else:
        logger.info(
            "learning_effectiveness written: week=%s topics=%d", week, len(bq_rows)
        )
