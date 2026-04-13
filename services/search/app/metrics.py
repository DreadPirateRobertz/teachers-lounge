"""Prometheus metrics for the search service (tl-he3).

Defines the ``search_query_duration_seconds`` histogram and an async
context manager (:func:`observe_query`) that records an observation with
the correct ``query_type`` / ``status`` labels whether the wrapped block
succeeds or raises.

The histogram is defined once at module import time and lives in the
default prometheus_client REGISTRY, which is what ``prometheus_client.
generate_latest()`` renders on the ``/metrics`` endpoint in ``main.py``.
"""
from __future__ import annotations

import time
from contextlib import asynccontextmanager
from typing import AsyncIterator, Literal

from prometheus_client import Counter, Histogram

# Buckets chosen per tl-he3 spec: cover expected sub-10ms p50 up through
# a 2.5s outlier ceiling — anything slower than that is a timeout bug.
SEARCH_QUERY_DURATION_BUCKETS: tuple[float, ...] = (
    0.01,
    0.05,
    0.1,
    0.25,
    0.5,
    1.0,
    2.5,
)

QueryType = Literal["semantic", "keyword", "hybrid"]
QueryStatus = Literal["success", "error"]


SEARCH_QUERY_DURATION = Histogram(
    "search_query_duration_seconds",
    "Search query wall-clock duration in seconds, observed at the Qdrant "
    "RPC call site (semantic/keyword) and at the hybrid router (hybrid).",
    labelnames=("query_type", "status"),
    buckets=SEARCH_QUERY_DURATION_BUCKETS,
)


@asynccontextmanager
async def observe_query(query_type: QueryType) -> AsyncIterator[None]:
    """Record a single observation in :data:`SEARCH_QUERY_DURATION`.

    Runs ``perf_counter`` around the ``async with`` body. If the body
    raises, the observation is still recorded with ``status="error"``
    before the exception propagates — so failed queries do not silently
    vanish from the histogram.

    Args:
        query_type: ``"semantic"``, ``"keyword"``, or ``"hybrid"``.

    Yields:
        ``None`` — the context manager exists only for the side-effect
        of recording timing.
    """
    status: QueryStatus = "success"
    start = time.perf_counter()
    try:
        yield
    except BaseException:
        status = "error"
        raise
    finally:
        SEARCH_QUERY_DURATION.labels(
            query_type=query_type, status=status
        ).observe(time.perf_counter() - start)


# ── Query expansion (tl-afb) ─────────────────────────────────────────────────
# Counter of short-query expansion outcomes. ``outcome`` values:
#   - "expanded"   — AI gateway returned a non-empty rewrite that was used
#   - "passthrough_long"    — query was not short, no expansion attempted
#   - "passthrough_nocontext" — no context turns supplied, no expansion attempted
#   - "fallback_blank"      — gateway returned blank output, raw query used
#   - "fallback_error"      — gateway call raised, raw query used
QUERY_EXPANSION_OUTCOMES = Counter(
    "search_query_expansion_total",
    "Short-query expansion attempts by outcome (tl-afb).",
    labelnames=("outcome",),
)
