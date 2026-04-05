"""Prometheus metrics for the tutoring-service.

Exposes a /metrics endpoint via prometheus_client and defines custom
histograms and counters for the RAG pipeline, SSE streaming, and
session lifecycle per the Phase 8 spec.
"""

from prometheus_client import Counter, Histogram, make_asgi_app

# ── Session metrics ──────────────────────────────────────────────────────────

session_duration_seconds = Histogram(
    "tl_session_duration_seconds",
    "Total tutoring session duration in seconds.",
    buckets=[5, 15, 30, 60, 120, 300, 600, 1800],
)

# ── RAG pipeline latency ─────────────────────────────────────────────────────

rag_latency_seconds = Histogram(
    "tl_rag_latency_seconds",
    "Latency of each RAG pipeline step in seconds.",
    labelnames=["step"],
    buckets=[0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0],
)

# ── Token usage ──────────────────────────────────────────────────────────────

tokens_used_total = Counter(
    "tl_tokens_used_total",
    "Total tokens consumed by model, labeled by model name and token type.",
    labelnames=["model", "token_type"],
)

# ── Streaming TTFB ───────────────────────────────────────────────────────────

stream_ttfb_seconds = Histogram(
    "tl_stream_ttfb_seconds",
    "Time-to-first-byte for SSE streaming responses in seconds.",
    buckets=[0.1, 0.25, 0.5, 1.0, 1.5, 2.0, 3.0, 5.0],
)

# ── Standard HTTP RED metrics ─────────────────────────────────────────────────

http_request_duration_seconds = Histogram(
    "http_request_duration_seconds",
    "HTTP request latency in seconds.",
    labelnames=["method", "path", "status"],
    buckets=[0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0],
)

http_requests_total = Counter(
    "http_requests_total",
    "Total HTTP requests.",
    labelnames=["method", "path", "status"],
)

# ── ASGI metrics app (mount at /metrics) ─────────────────────────────────────

metrics_app = make_asgi_app()
