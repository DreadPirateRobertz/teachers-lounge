# Query Analysis — tutoring-service

## Methodology

Queries were analyzed using SQLAlchemy query tracing and logical EXPLAIN ANALYZE
inference based on table structure, index coverage, and row cardinality estimates.
Production EXPLAIN ANALYZE output should be collected from staging after the
index migration runs — run with `SET enable_seqscan = off` to compare index paths.

---

## Slow Query 1: Review Queue (`GET /v1/reviews/queue`)

### Before (full table scan)

```sql
SELECT *
FROM student_concept_mastery
WHERE user_id = $1
ORDER BY next_review_at ASC NULLS FIRST;
```

**EXPLAIN ANALYZE (estimated)**:
```
Seq Scan on student_concept_mastery
  Filter: (user_id = $1)
  Rows removed by filter: ~50,000  (for 1,000-user system with 50 concepts/user)
  Actual rows: 50  (avg per user)
  Sort: next_review_at ASC NULLS FIRST
  Planning time: ~0.5ms
  Execution time: ~15-80ms at scale (grows linearly with total rows)
```

**Problem**: No index on `user_id` alone; Postgres reads all rows and filters.
At 10,000+ total mastery rows the query degrades significantly.

### After (composite index + SQL filter)

```sql
-- Due items (limit pushed to SQL)
SELECT *
FROM student_concept_mastery
WHERE user_id = $1
  AND (next_review_at IS NULL OR next_review_at <= $2)
ORDER BY next_review_at ASC NULLS FIRST
LIMIT 20;

-- Upcoming count (single aggregate)
SELECT COUNT(*)
FROM student_concept_mastery
WHERE user_id = $1
  AND next_review_at > $2
  AND next_review_at <= $3;
```

**EXPLAIN ANALYZE (estimated with ix_scm_user_next_review)**:
```
Index Scan using ix_scm_user_next_review on student_concept_mastery
  Index Cond: (user_id = $1 AND next_review_at <= $2)
  Rows: 20 (LIMIT applied at index level)
  Planning time: ~0.3ms
  Execution time: ~0.5-2ms  ← 10-50× improvement
```

---

## Slow Query 2: Review Stats (`GET /v1/reviews/stats`)

### Before

Python loaded all mastery rows, then used list comprehensions for counts:
```python
mastery_rows = list(result.scalars().all())  # loads 50+ rows per user
due_now = sum(1 for r in mastery_rows if r.next_review_at is None or r.next_review_at <= now)
```

**Cost**: 50+ row fetch + Python iteration per request. Scales with concepts per user.

### After

Single SQL aggregate query:
```sql
SELECT
  COUNT(*) AS total,
  AVG(mastery_score) AS avg_mastery,
  AVG(ease_factor) AS avg_ef,
  COUNT(*) FILTER (WHERE next_review_at IS NULL OR next_review_at <= $1) AS due_now,
  COUNT(*) FILTER (WHERE next_review_at IS NOT NULL AND next_review_at <= $2) AS due_today,
  COUNT(*) FILTER (WHERE next_review_at IS NOT NULL AND next_review_at <= $3) AS due_week
FROM student_concept_mastery
WHERE user_id = $4;
```

**EXPLAIN ANALYZE (estimated)**:
```
Aggregate
  -> Index Only Scan using ix_scm_user_next_review
       Index Cond: (user_id = $4)
       Rows: 50
       Execution time: ~1-3ms  ← data stays in Postgres, no Python iteration
```

---

## Index DDL

```sql
-- Added to student_concept_mastery ORM model (__table_args__)
CREATE INDEX ix_scm_user_next_review
ON student_concept_mastery (user_id, next_review_at);
```

SQLAlchemy creates this on `Base.metadata.create_all()` (dev) and via Alembic
migration in production (add to next migration revision).

**Index size estimate**: ~50 bytes/row × 500K rows = ~25 MB (negligible).

---

## Other Indexes Already Present

| Table | Column | Type | Notes |
|-------|--------|------|-------|
| `concepts` | `course_id` | B-tree | Good — concepts always scoped by course |
| `review_records` | `user_id` | B-tree | Good — but see query 3 below |
| `review_records` | `concept_id` | B-tree | Good |
| `interactions` | `session_id` | B-tree | Good |
| `chat_sessions` | `user_id` | B-tree | Good |

---

## Query 3: ReviewRecord Count (`GET /v1/reviews/stats`)

```sql
SELECT COUNT(id) FROM review_records WHERE user_id = $1;
```

**Status**: OK — `user_id` index exists. At 1,000 reviews/user this is fast.
Consider adding a **materialized running count** to `student_concept_mastery.review_count`
if this query becomes slow (>10K reviews/user). The column already exists in the model.

---

## Production Verification

After deploying, run from a psql session against staging:

```sql
-- Verify index exists
\d student_concept_mastery

-- Force index use and measure
SET enable_seqscan = off;
EXPLAIN (ANALYZE, BUFFERS)
  SELECT COUNT(*) FILTER (WHERE next_review_at <= NOW())
  FROM student_concept_mastery
  WHERE user_id = '<real-user-uuid>';
```

Target: Index Scan, execution time <5ms at p99.
