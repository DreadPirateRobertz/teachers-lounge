"""Nightly analytics CronJob for TeachersLounge.

Runs at 2am UTC every night (schedule: '0 2 * * *') as a GKE CronJob.

Pipeline steps:
1. QUERY — fetch last 24h of interactions and quiz_results from Postgres.
2. ANONYMIZE — replace user_id with SHA256(user_id + daily_salt); enforce
   k-anonymity (suppress topics with < k unique students); add Laplace noise
   to aggregate counts for differential privacy.
3. AGGREGATE — write four summary tables to BigQuery:
   topic_difficulty, explanation_effectiveness, learning_curves,
   engagement_metrics.
4. GENERATE INSIGHTS — for topics with error_rate > 0.4, call Claude Haiku
   to produce a 1-2 sentence insight; embed with text-embedding-3-large and
   upsert into Qdrant 'insights' collection.
5. CACHE — top 3 insights per topic → Redis key ``cache:insight:{topic}``
   with 6h TTL.
6. AUDIT — write a structured audit log entry on completion.
"""
from __future__ import annotations

import asyncio
import hashlib
import json
import logging
import os
import secrets
import sys
from datetime import datetime, timezone
from typing import Any

import anthropic
import numpy as np
import redis.asyncio as aioredis
from google.cloud import bigquery
from openai import AsyncOpenAI
from qdrant_client import QdrantClient
from qdrant_client.models import Distance, PointStruct, VectorParams
from sqlalchemy import text
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from .config import settings

logging.basicConfig(
    level=settings.log_level.upper(),
    format="%(asctime)s %(levelname)s %(name)s %(message)s",
)
logger = logging.getLogger(__name__)

# ── Constants ─────────────────────────────────────────────────────────────────

HIGH_ERROR_RATE_THRESHOLD = 0.4
TOP_INSIGHTS_PER_TOPIC = 3


# ── Anonymisation helpers ─────────────────────────────────────────────────────


def _daily_salt() -> str:
    """Return a deterministic daily salt based on UTC date.

    The salt is the ISO date string (YYYY-MM-DD) so that the same user maps
    to the same pseudonym within one day's run but rotates across days.

    Returns:
        A string like ``"2026-04-05"`` used as the HMAC salt.
    """
    return datetime.now(timezone.utc).strftime("%Y-%m-%d")


def anonymize_user_id(user_id: str, salt: str) -> str:
    """Pseudonymize a user ID with a daily salt using SHA-256.

    Args:
        user_id: Original UUID string from Postgres.
        salt: Daily salt (e.g. ``"2026-04-05"``).

    Returns:
        Hex-encoded SHA-256 digest of ``user_id + salt``.
    """
    return hashlib.sha256(f"{user_id}{salt}".encode()).hexdigest()


def add_laplace_noise(value: float, scale: float) -> float:
    """Add Laplace noise to a numeric value for differential privacy.

    Args:
        value: The true aggregate count or ratio.
        scale: Laplace distribution scale parameter (sensitivity / epsilon).

    Returns:
        Noised value; never negative for counts (clipped at 0.0).
    """
    noise = np.random.laplace(loc=0.0, scale=scale)
    return max(0.0, value + noise)


# ── BigQuery helpers ──────────────────────────────────────────────────────────


def _bq_client() -> bigquery.Client:
    """Construct an authenticated BigQuery client.

    Returns:
        A ``google.cloud.bigquery.Client`` using ADC or Workload Identity.
    """
    return bigquery.Client(project=settings.gcp_project)


def _ensure_bq_tables(client: bigquery.Client) -> None:
    """Create BigQuery tables if they do not yet exist.

    All tables are in ``settings.bigquery_dataset``. Schema is defined inline
    so the CronJob is self-bootstrapping in a fresh dataset.

    Args:
        client: Authenticated BigQuery client.
    """
    dataset_ref = bigquery.DatasetReference(settings.gcp_project, settings.bigquery_dataset)

    schemas: dict[str, list[bigquery.SchemaField]] = {
        "topic_difficulty": [
            bigquery.SchemaField("run_date", "DATE"),
            bigquery.SchemaField("topic", "STRING"),
            bigquery.SchemaField("error_rate", "FLOAT64"),
            bigquery.SchemaField("avg_time_to_correct", "FLOAT64"),
            bigquery.SchemaField("common_wrong_answers", "STRING"),  # JSON
            bigquery.SchemaField("sample_size", "INT64"),
        ],
        "explanation_effectiveness": [
            bigquery.SchemaField("run_date", "DATE"),
            bigquery.SchemaField("topic", "STRING"),
            bigquery.SchemaField("style", "STRING"),
            bigquery.SchemaField("success_rate", "FLOAT64"),
            bigquery.SchemaField("sample_size", "INT64"),
        ],
        "learning_curves": [
            bigquery.SchemaField("run_date", "DATE"),
            bigquery.SchemaField("cohort_week", "STRING"),
            bigquery.SchemaField("topic", "STRING"),
            bigquery.SchemaField("avg_mastery_score", "FLOAT64"),
            bigquery.SchemaField("sample_size", "INT64"),
        ],
        "engagement_metrics": [
            bigquery.SchemaField("run_date", "DATE"),
            bigquery.SchemaField("avg_session_length_min", "FLOAT64"),
            bigquery.SchemaField("avg_questions_per_session", "FLOAT64"),
            bigquery.SchemaField("day7_retention_rate", "FLOAT64"),
            bigquery.SchemaField("active_students", "INT64"),
        ],
    }

    existing = {t.table_id for t in client.list_tables(dataset_ref)}
    for table_name, schema in schemas.items():
        if table_name not in existing:
            table_ref = dataset_ref.table(table_name)
            table = bigquery.Table(table_ref, schema=schema)
            client.create_table(table)
            logger.info("Created BigQuery table: %s.%s", settings.bigquery_dataset, table_name)


def _stream_rows(client: bigquery.Client, table_name: str, rows: list[dict[str, Any]]) -> None:
    """Streaming-insert rows into a BigQuery table.

    Args:
        client: Authenticated BigQuery client.
        table_name: Table name within ``settings.bigquery_dataset``.
        rows: List of dicts matching the table schema.

    Raises:
        RuntimeError: If any rows fail to insert.
    """
    if not rows:
        return
    table_ref = f"{settings.gcp_project}.{settings.bigquery_dataset}.{table_name}"
    errors = client.insert_rows_json(table_ref, rows)
    if errors:
        raise RuntimeError(f"BigQuery insert errors for {table_name}: {errors}")
    logger.info("Inserted %d rows into %s", len(rows), table_name)


# ── Postgres query ─────────────────────────────────────────────────────────────


async def _fetch_interactions(db: AsyncSession, since: datetime) -> list[dict[str, Any]]:
    """Fetch all interactions and joined quiz_results from the past window.

    Args:
        db: Async SQLAlchemy session.
        since: UTC datetime marking the start of the 24-hour window.

    Returns:
        List of row dicts with keys: user_id, session_id, topic,
        is_correct, role, created_at.
    """
    result = await db.execute(
        text("""
            SELECT
                i.user_id,
                i.session_id,
                i.role,
                i.created_at,
                qr.topic,
                qr.is_correct
            FROM interactions i
            LEFT JOIN quiz_results qr
                ON qr.user_id = i.user_id
                AND qr.created_at::date = i.created_at::date
            WHERE i.created_at >= :since
            ORDER BY i.user_id, i.session_id, i.created_at
        """),
        {"since": since},
    )
    return [dict(r) for r in result.mappings().all()]


async def _fetch_mastery(db: AsyncSession, since: datetime) -> list[dict[str, Any]]:
    """Fetch student concept mastery rows for learning curve aggregation.

    Args:
        db: Async SQLAlchemy session.
        since: UTC datetime marking the start of the 24-hour window.

    Returns:
        List of row dicts with keys: user_id, topic, mastery_score,
        created_at.
    """
    result = await db.execute(
        text("""
            SELECT user_id, topic, mastery_score, created_at
            FROM student_concept_mastery
            WHERE created_at >= :since
        """),
        {"since": since},
    )
    return [dict(r) for r in result.mappings().all()]


# ── Aggregation logic ─────────────────────────────────────────────────────────


def _aggregate_topic_difficulty(
    rows: list[dict[str, Any]],
    salt: str,
    k: int,
    noise_scale: float,
    run_date: str,
) -> list[dict[str, Any]]:
    """Aggregate anonymized topic difficulty metrics.

    Enforces k-anonymity: topics with fewer than ``k`` unique anonymized
    students are suppressed entirely. Adds Laplace noise to sample_size.

    Args:
        rows: Raw interaction/quiz rows from Postgres.
        salt: Daily anonymization salt.
        k: Minimum distinct students required to export a topic.
        noise_scale: Laplace noise scale for differential privacy.
        run_date: ISO date string for the BigQuery partition column.

    Returns:
        List of dicts ready for BigQuery streaming insert.
    """
    from collections import defaultdict

    topic_data: dict[str, dict[str, Any]] = defaultdict(
        lambda: {"students": set(), "total": 0, "incorrect": 0, "wrong_answers": []}
    )

    for row in rows:
        topic = row.get("topic")
        if not topic or row.get("is_correct") is None:
            continue
        anon_uid = anonymize_user_id(str(row["user_id"]), salt)
        td = topic_data[topic]
        td["students"].add(anon_uid)
        td["total"] += 1
        if not row["is_correct"]:
            td["incorrect"] += 1

    output = []
    for topic, td in topic_data.items():
        if len(td["students"]) < k:
            logger.debug("Suppressing topic %r: only %d students (k=%d)", topic, len(td["students"]), k)
            continue
        total = td["total"]
        error_rate = td["incorrect"] / total if total > 0 else 0.0
        noised_sample = int(add_laplace_noise(float(len(td["students"])), noise_scale))
        output.append({
            "run_date": run_date,
            "topic": topic,
            "error_rate": round(error_rate, 4),
            "avg_time_to_correct": None,  # not tracked yet — Phase 8 extension
            "common_wrong_answers": json.dumps([]),
            "sample_size": max(0, noised_sample),
        })
    return output


def _aggregate_engagement(
    rows: list[dict[str, Any]],
    salt: str,
    k: int,
    noise_scale: float,
    run_date: str,
) -> list[dict[str, Any]]:
    """Aggregate anonymized engagement metrics for the day.

    Args:
        rows: Raw interaction rows from Postgres.
        salt: Daily anonymization salt.
        k: Minimum distinct students required to export metrics.
        noise_scale: Laplace noise scale.
        run_date: ISO date string.

    Returns:
        A single-element list ready for BigQuery insert, or empty list if
        fewer than k students interacted.
    """
    from collections import defaultdict

    session_lengths: dict[str, list[datetime]] = defaultdict(list)
    students: set[str] = set()
    questions_per_session: dict[str, int] = defaultdict(int)

    for row in rows:
        anon = anonymize_user_id(str(row["user_id"]), salt)
        students.add(anon)
        sid = str(row["session_id"])
        ts = row.get("created_at")
        if ts:
            session_lengths[sid].append(ts)
        if row.get("topic") and row.get("is_correct") is not None:
            questions_per_session[sid] += 1

    if len(students) < k:
        logger.debug("Suppressing engagement metrics: only %d students (k=%d)", len(students), k)
        return []

    durations = []
    for timestamps in session_lengths.values():
        if len(timestamps) >= 2:
            dur = (max(timestamps) - min(timestamps)).total_seconds() / 60.0
            durations.append(dur)

    avg_session = round(float(np.mean(durations)) if durations else 0.0, 2)
    avg_questions = round(
        float(np.mean(list(questions_per_session.values()))) if questions_per_session else 0.0, 2
    )

    noised_students = int(add_laplace_noise(float(len(students)), noise_scale))
    return [{
        "run_date": run_date,
        "avg_session_length_min": avg_session,
        "avg_questions_per_session": avg_questions,
        "day7_retention_rate": None,  # requires 7-day lookback — future enhancement
        "active_students": max(0, noised_students),
    }]


def _aggregate_learning_curves(
    mastery_rows: list[dict[str, Any]],
    salt: str,
    k: int,
    noise_scale: float,
    run_date: str,
) -> list[dict[str, Any]]:
    """Aggregate learning curve data by cohort_week and topic.

    Args:
        mastery_rows: Rows from student_concept_mastery.
        salt: Daily anonymization salt.
        k: Minimum distinct students required to export a cohort-topic.
        noise_scale: Laplace noise scale.
        run_date: ISO date string.

    Returns:
        List of dicts ready for BigQuery insert.
    """
    from collections import defaultdict

    # cohort_week = ISO year-week of the interaction (e.g. "2026-W14")
    bucket: dict[tuple[str, str], dict[str, Any]] = defaultdict(
        lambda: {"students": set(), "scores": []}
    )

    for row in mastery_rows:
        ts = row.get("created_at")
        if not ts:
            continue
        cohort_week = ts.strftime("%Y-W%W")
        topic = str(row.get("topic", ""))
        anon = anonymize_user_id(str(row["user_id"]), salt)
        b = bucket[(cohort_week, topic)]
        b["students"].add(anon)
        score = row.get("mastery_score")
        if score is not None:
            b["scores"].append(float(score))

    output = []
    for (cohort_week, topic), b in bucket.items():
        if len(b["students"]) < k:
            continue
        avg_mastery = round(float(np.mean(b["scores"])) if b["scores"] else 0.0, 4)
        noised_sample = int(add_laplace_noise(float(len(b["students"])), noise_scale))
        output.append({
            "run_date": run_date,
            "cohort_week": cohort_week,
            "topic": topic,
            "avg_mastery_score": avg_mastery,
            "sample_size": max(0, noised_sample),
        })
    return output


# ── Insight generation ────────────────────────────────────────────────────────


async def _generate_insight(
    client: anthropic.AsyncAnthropic,
    topic: str,
    error_rate: float,
    sample_size: int,
) -> str:
    """Call Claude Haiku to generate a 1-2 sentence teaching insight.

    Args:
        client: Async Anthropic client.
        topic: Topic name (e.g. ``"quadratic equations"``).
        error_rate: Fraction of incorrect answers for this topic.
        sample_size: Number of students in the cohort.

    Returns:
        A 1-2 sentence insight suitable for display to teachers.
    """
    prompt = (
        f"Topic: {topic}\n"
        f"Error rate: {error_rate:.0%} of {sample_size} students answered incorrectly.\n\n"
        "Write exactly 1-2 sentences of actionable pedagogical insight for a teacher. "
        "Be specific about what students struggle with and what teaching approach helps. "
        "Do not include any student names or identifying information."
    )
    message = await client.messages.create(
        model="claude-haiku-4-5-20251001",
        max_tokens=150,
        messages=[{"role": "user", "content": prompt}],
    )
    return message.content[0].text.strip()


async def _embed_text(openai_client: AsyncOpenAI, text_to_embed: str) -> list[float]:
    """Embed text using OpenAI text-embedding-3-large.

    Args:
        openai_client: Async OpenAI client.
        text_to_embed: The text to embed.

    Returns:
        List of 3072 floats representing the embedding vector.
    """
    response = await openai_client.embeddings.create(
        model="text-embedding-3-large",
        input=text_to_embed,
    )
    return response.data[0].embedding


def _ensure_qdrant_collection(qdrant: QdrantClient) -> None:
    """Create the insights Qdrant collection if it does not exist.

    Args:
        qdrant: Qdrant client instance.
    """
    existing = {c.name for c in qdrant.get_collections().collections}
    if settings.insights_collection not in existing:
        qdrant.create_collection(
            collection_name=settings.insights_collection,
            vectors_config=VectorParams(
                size=settings.insight_vector_dim,
                distance=Distance.COSINE,
            ),
        )
        logger.info("Created Qdrant collection: %s", settings.insights_collection)


def _upsert_insight(
    qdrant: QdrantClient,
    topic: str,
    insight_text: str,
    vector: list[float],
    run_date: str,
) -> None:
    """Upsert an insight embedding into Qdrant.

    Uses a deterministic point ID derived from topic + run_date so re-running
    the CronJob on the same day updates existing points rather than creating
    duplicates.

    Args:
        qdrant: Qdrant client instance.
        topic: Topic name (used as metadata and for ID derivation).
        insight_text: The generated insight text.
        vector: 3072-dimensional embedding vector.
        run_date: ISO date string (``"YYYY-MM-DD"``).
    """
    point_id = int(hashlib.sha256(f"{topic}:{run_date}".encode()).hexdigest()[:16], 16) % (2**63)
    qdrant.upsert(
        collection_name=settings.insights_collection,
        points=[
            PointStruct(
                id=point_id,
                vector=vector,
                payload={
                    "topic": topic,
                    "insight": insight_text,
                    "run_date": run_date,
                },
            )
        ],
    )
    logger.debug("Upserted insight for topic %r (point_id=%d)", topic, point_id)


# ── Redis caching ─────────────────────────────────────────────────────────────


async def _cache_top_insights(
    redis_client: aioredis.Redis,
    topic_insights: dict[str, list[str]],
    ttl: int,
) -> None:
    """Cache the top N insights per topic in Redis.

    Key format: ``cache:insight:{topic}``
    Value: JSON array of insight strings.

    Args:
        redis_client: Async Redis client.
        topic_insights: Mapping of topic → list of insight strings (ordered
            by error_rate descending — caller is responsible for ordering).
        ttl: TTL in seconds.
    """
    for topic, insights in topic_insights.items():
        key = f"cache:insight:{topic}"
        value = json.dumps(insights[:TOP_INSIGHTS_PER_TOPIC])
        await redis_client.set(key, value, ex=ttl)
        logger.debug("Cached %d insight(s) for topic %r (TTL=%ds)", len(insights[:TOP_INSIGHTS_PER_TOPIC]), topic, ttl)


# ── Audit log ─────────────────────────────────────────────────────────────────


def _write_audit_log(run_date: str, topics_exported: int, insights_generated: int) -> None:
    """Write a structured audit log entry for compliance (§12.1).

    All BigQuery data is anonymized. This log records run metadata only —
    no PII is included.

    Args:
        run_date: ISO date string for the CronJob run.
        topics_exported: Number of topics that passed k-anonymity and were
            exported to BigQuery.
        insights_generated: Number of Claude Haiku insights generated and
            embedded.
    """
    entry = {
        "event": "analytics_cronjob_run",
        "run_date": run_date,
        "topics_exported": topics_exported,
        "insights_generated": insights_generated,
        "k_anonymity_threshold": settings.k_anonymity_threshold,
        "dp_noise_scale": settings.dp_noise_scale,
        "pii_in_bigquery": False,
    }
    logger.info("AUDIT: %s", json.dumps(entry))


# ── Main pipeline ─────────────────────────────────────────────────────────────


async def run_pipeline(
    db: AsyncSession,
    bq_client: bigquery.Client,
    redis_client: aioredis.Redis,
    qdrant_client: QdrantClient,
    anthropic_client: anthropic.AsyncAnthropic,
    openai_client: AsyncOpenAI,
) -> None:
    """Execute the full nightly analytics pipeline.

    This is the top-level coroutine called by the CronJob entrypoint. All
    external clients are passed in to make the function fully testable.

    Args:
        db: Async SQLAlchemy session (read-only, Postgres).
        bq_client: Authenticated Google BigQuery client.
        redis_client: Async Redis client.
        qdrant_client: Qdrant client for the insights collection.
        anthropic_client: Async Anthropic client for Claude Haiku.
        openai_client: Async OpenAI client for text-embedding-3-large.
    """
    from datetime import timedelta

    now = datetime.now(timezone.utc)
    since = now - timedelta(hours=24)
    run_date = now.strftime("%Y-%m-%d")
    salt = _daily_salt()
    k = settings.k_anonymity_threshold
    noise = settings.dp_noise_scale

    logger.info("Starting analytics pipeline run for %s (since=%s)", run_date, since.isoformat())

    # ── Step 1: Fetch ─────────────────────────────────────────────────────────
    rows = await _fetch_interactions(db, since)
    mastery_rows = await _fetch_mastery(db, since)
    logger.info("Fetched %d interaction rows, %d mastery rows", len(rows), len(mastery_rows))

    # ── Step 2 & 3: Anonymize + Aggregate → BigQuery ──────────────────────────
    _ensure_bq_tables(bq_client)

    difficulty_rows = _aggregate_topic_difficulty(rows, salt, k, noise, run_date)
    engagement_rows = _aggregate_engagement(rows, salt, k, noise, run_date)
    curve_rows = _aggregate_learning_curves(mastery_rows, salt, k, noise, run_date)

    _stream_rows(bq_client, "topic_difficulty", difficulty_rows)
    _stream_rows(bq_client, "engagement_metrics", engagement_rows)
    _stream_rows(bq_client, "learning_curves", curve_rows)
    # explanation_effectiveness requires learning-style data — deferred to Phase 8 extension
    logger.info(
        "BigQuery: %d topic_difficulty, %d engagement, %d learning_curve rows written",
        len(difficulty_rows), len(engagement_rows), len(curve_rows),
    )

    # ── Step 4: Generate insights for high-error-rate topics ──────────────────
    _ensure_qdrant_collection(qdrant_client)

    high_error_topics = [
        r for r in difficulty_rows if r["error_rate"] > HIGH_ERROR_RATE_THRESHOLD
    ]
    # Sort descending by error_rate so top insights are first
    high_error_topics.sort(key=lambda r: r["error_rate"], reverse=True)

    topic_insights: dict[str, list[str]] = {}
    insights_generated = 0

    for topic_row in high_error_topics:
        topic = topic_row["topic"]
        try:
            insight_text = await _generate_insight(
                anthropic_client,
                topic,
                topic_row["error_rate"],
                topic_row["sample_size"],
            )
            vector = await _embed_text(openai_client, insight_text)
            _upsert_insight(qdrant_client, topic, insight_text, vector, run_date)
            topic_insights.setdefault(topic, []).append(insight_text)
            insights_generated += 1
            logger.info("Generated insight for topic %r (error_rate=%.2f)", topic, topic_row["error_rate"])
        except Exception:
            logger.exception("Failed to generate/embed insight for topic %r — skipping", topic)

    # ── Step 5: Cache top insights per topic ──────────────────────────────────
    if topic_insights:
        await _cache_top_insights(redis_client, topic_insights, settings.redis_insight_ttl)
        logger.info("Cached insights for %d topic(s) in Redis", len(topic_insights))

    # ── Step 6: Audit log ─────────────────────────────────────────────────────
    _write_audit_log(run_date, len(difficulty_rows), insights_generated)
    logger.info(
        "Analytics pipeline complete: %d topics exported, %d insights generated",
        len(difficulty_rows), insights_generated,
    )


async def _main() -> None:
    """Async entrypoint for the CronJob container.

    Initialises all external clients, runs the pipeline, then tears down
    connections cleanly. Exits with code 1 on any unhandled error so the
    CronJob shows as Failed in GKE and triggers alerting.
    """
    engine = create_async_engine(
        settings.database_url,
        pool_size=2,
        max_overflow=0,
        pool_pre_ping=True,
    )
    session_factory = async_sessionmaker(engine, expire_on_commit=False)

    redis_client = aioredis.from_url(settings.redis_url, decode_responses=True)
    qdrant_client = QdrantClient(
        host=settings.qdrant_host,
        port=settings.qdrant_port,
        api_key=settings.qdrant_api_key,
    )
    anthropic_client = anthropic.AsyncAnthropic(api_key=settings.anthropic_api_key)
    openai_client = AsyncOpenAI(api_key=settings.openai_api_key)
    bq = _bq_client()

    try:
        async with session_factory() as db:
            await run_pipeline(db, bq, redis_client, qdrant_client, anthropic_client, openai_client)
    finally:
        await redis_client.aclose()
        await engine.dispose()


def main() -> None:
    """Synchronous entrypoint called by the CronJob container CMD.

    Raises:
        SystemExit: Exit code 1 on pipeline failure, 0 on success.
    """
    try:
        asyncio.run(_main())
    except Exception:
        logger.exception("Analytics CronJob failed")
        sys.exit(1)


if __name__ == "__main__":
    main()
