"""Prometheus metrics for the search service."""

from prometheus_client import Histogram, make_asgi_app

search_query_duration_seconds = Histogram(
    "search_query_duration_seconds",
    "Latency of /search queries against Qdrant in seconds.",
    labelnames=["query_type", "status"],
    buckets=[0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0],
)

metrics_app = make_asgi_app()
