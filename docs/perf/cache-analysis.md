# Redis Cache Analysis — tutoring-service

## Current Cache Architecture

The tutoring-service uses Redis for session history caching only.

| Key Pattern | Namespace | TTL | Purpose |
|-------------|-----------|-----|---------|
| `tutoring:session_history:<session_id>` | `session_history` | `SESSION_HISTORY_CACHE_TTL` (config) | Full message history for a session |
| `tutoring:user_sessions:<user_id>` | (internal) | same TTL | LRU list of recent session IDs per user |

## Cache Hit Rate Analysis

### Expected Behavior

- **Cold start** (first message in session): Always miss. No pre-warming.
- **Active session** (student sending follow-up messages): Hit on every message after
  the first. A 5-minute study session generates ~5-10 messages → ~80-90% hit rate
  during the session.
- **Session resume** (returning after >TTL): Miss on first message, then warm.

### Prometheus Metrics (Added in tl-3j2)

```promql
# Cache hit rate (5-minute window)
rate(tl_cache_hits_total{namespace="session_history"}[5m])
  /
(rate(tl_cache_hits_total{namespace="session_history"}[5m])
  + rate(tl_cache_misses_total{namespace="session_history"}[5m]))
```

**Target hit rate**: >70% during active tutoring hours.

**Alert threshold**: <50% hit rate over 15 minutes indicates either:
1. TTL too short (users resuming sessions frequently)
2. Redis eviction under memory pressure (check `maxmemory-policy`)
3. Redis connection failures (all misses)

### Grafana Dashboard Query

```promql
# Absolute hit/miss counts
sum(increase(tl_cache_hits_total[1h])) by (namespace)
sum(increase(tl_cache_misses_total[1h])) by (namespace)

# Hit rate as gauge (0-1)
sum(rate(tl_cache_hits_total[5m])) by (namespace)
  /
(sum(rate(tl_cache_hits_total[5m])) by (namespace)
  + sum(rate(tl_cache_misses_total[5m])) by (namespace))
```

## Cache Gaps — Future Improvements

| Candidate | Benefit | Risk |
|-----------|---------|------|
| Review queue cache (`GET /reviews/queue`) | Avoid DB on rapid refreshes | Stale data (max 30s staleness acceptable) |
| Concept graph cache (`GET /concepts`) | Graph is static per-course | Invalidation on concept add/update |
| Leaderboard cache (gaming-service) | High read fan-out | Already handled by gaming-service |

**Recommendation**: Add review queue caching with 30s TTL in Phase 9. The SQL
optimization (composite index) is sufficient for Phase 8 exit criteria.

## Redis Configuration Recommendations

```yaml
# redis.conf
maxmemory 512mb
maxmemory-policy allkeys-lru   # evict LRU keys when full
hz 20                           # more frequent expiry cleanup
save ""                         # disable persistence (cache only)
```

Monitor `redis_evicted_keys_total` — any evictions under normal load indicate
`maxmemory` needs to be raised or TTLs shortened.
