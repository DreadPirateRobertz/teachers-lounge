# Performance Analysis — Phase 8 Exit Criteria

**Target**: <2s TTFB for chat endpoint (p95), <5s for upload (p95).

## Load Test Suite

| Test Script | Service | Peak VUs | Primary SLO |
|-------------|---------|----------|-------------|
| `auth-flow.js` | user-service (8080) | 1000 | login p99 <500ms |
| `chat-rag-flow.js` | tutoring-service (8000) | 200 | chat TTFB p95 <2s |
| `tutoring-service.js` | tutoring-service (8000) | 150 | streaming p95 <5s |
| `ingestion-service.js` | ingestion-service (8082) | 100 | upload p95 <5s |
| `search-service.js` | search-service (8001) | 200 | search p95 <2s |
| `gaming-service.js` | gaming-service (8083) | 150 | leaderboard p99 <500ms |
| `boss-battle.js` | gaming-service (8083) | 500 | attack p99 <300ms |
| `frontend.js` | Next.js (3000) | 150 | SSR p99 <2s |

### Running Tests

```bash
# Single service (staging)
k6 run tests/load/chat-rag-flow.js \
  --env BASE_URL=https://api-staging.teacherslounge.app \
  --env AUTH_TOKEN=$STAGING_TOKEN

# Upload test (requires course_id with staging data)
k6 run tests/load/ingestion-service.js \
  --env BASE_URL=https://api-staging.teacherslounge.app \
  --env AUTH_TOKEN=$STAGING_TOKEN \
  --env COURSE_ID=$STAGING_COURSE_ID

# Full suite via k6 Cloud
k6 cloud tests/load/chat-rag-flow.js
```

## Identified Bottlenecks

### 1. Review Queue — Full Table Scan (FIXED)

**Severity**: HIGH  
**Service**: tutoring-service  
**Endpoint**: `GET /v1/reviews/queue`, `GET /v1/reviews/stats`

**Before**: Both endpoints loaded _all_ `student_concept_mastery` rows for a user
into Python memory, then filtered with list comprehensions. At 100+ concepts/user
this was a sequential scan with O(n) data transfer.

**After**: SQL `WHERE` clauses push date comparisons into Postgres; `func.count().filter()`
aggregates compute all stats in a single query. Estimated improvement: **10-50×**
at p95 for users with >50 tracked concepts.

**Index added**: `ix_scm_user_next_review (user_id, next_review_at)` on
`student_concept_mastery` — covers both the queue filter and stats aggregates.

See [query-analysis.md](query-analysis.md) for full EXPLAIN ANALYZE output.

### 2. Chat TTFB — AI Gateway / LLM Latency

**Severity**: MEDIUM  
**Service**: tutoring-service  
**Endpoint**: `POST /v1/sessions/{id}/messages`

The 2s TTFB budget is tight for a full RAG pipeline:
embedding → Qdrant dual-search → rerank → LLM first token.

**Mitigations already in place**:
- Redis session history cache (`tl_cache_hits_total` counter — see [cache-analysis.md](cache-analysis.md))
- Parallel dense + sparse search via `asyncio.gather`
- `pool_pre_ping=True` on SQLAlchemy engine (avoids cold-connection overhead)

**Remaining opportunities**:
- Qdrant HNSW `ef` parameter tuning (lower ef = faster at cost of recall)
- Reranker batch size (reduce Qdrant candidates before rerank)
- AI Gateway semantic cache for identical or near-identical questions

### 3. Upload — GCS Throughput

**Severity**: LOW-MEDIUM  
**Service**: ingestion-service  
**Endpoint**: `POST /v1/ingest/upload`

Upload latency is dominated by GCS write time (network-bound). The endpoint
reads the entire file into memory before writing to GCS, which limits
concurrent upload throughput.

**Mitigation**: GCS upload runs in the thread pool (`asyncio` executor), so it
does not block the event loop. The 5s p95 budget accounts for GCS write time
on typical PDF sizes (<10 MB).

**Watch**: If p95 upload latency exceeds 5s at 50+ VUs, enable GCS resumable
uploads with chunked streaming to avoid memory pressure.

### 4. Redis Cache Hit Rate

**Severity**: LOW  
**Service**: tutoring-service

Cache hit rate for session history is tracked via `tl_cache_hits_total` and
`tl_cache_misses_total` Prometheus counters (added in this PR).

Expected hit rate: **~70-80%** for active sessions (5-minute conversation
window within the TTL). Cold-start sessions (first message) always miss.

Grafana query to monitor:
```promql
rate(tl_cache_hits_total[5m]) /
(rate(tl_cache_hits_total[5m]) + rate(tl_cache_misses_total[5m]))
```

See [cache-analysis.md](cache-analysis.md) for full analysis.
