"""Prometheus metrics for the analytics-service."""

from prometheus_client import Counter, Histogram, make_asgi_app

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

metrics_app = make_asgi_app()
