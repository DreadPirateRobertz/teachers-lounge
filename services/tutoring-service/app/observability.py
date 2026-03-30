"""Observability module — OpenTelemetry tracing + Prometheus metrics for tutoring-service."""

import os
import time
from typing import Callable

from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.semconv.resource import ResourceAttributes
from prometheus_client import (
    CONTENT_TYPE_LATEST,
    Counter,
    Gauge,
    Histogram,
    generate_latest,
)
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import Response

# ── Prometheus metrics ────────────────────────────────────────────────────────

REQUEST_DURATION = Histogram(
    "http_server_request_duration_seconds",
    "HTTP request duration in seconds",
    ["http_request_method", "http_route", "http_response_status_code"],
    buckets=(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10),
)

ACTIVE_REQUESTS = Gauge(
    "http_server_active_requests",
    "Number of active HTTP requests",
    ["http_request_method"],
)

DB_QUERY_DURATION = Histogram(
    "db_query_duration_seconds",
    "Database query duration in seconds",
    ["operation"],
    buckets=(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5),
)

REQUEST_COUNT = Counter(
    "http_server_requests_total",
    "Total HTTP requests",
    ["http_request_method", "http_route", "http_response_status_code"],
)


def record_db_query(operation: str, duration: float) -> None:
    """Record a database query duration metric."""
    DB_QUERY_DURATION.labels(operation=operation).observe(duration)


# ── OpenTelemetry tracing ─────────────────────────────────────────────────────

_tracer: trace.Tracer | None = None


def init_tracer(service_name: str = "tutoring-service") -> None:
    """Initialize the OpenTelemetry tracer with OTLP gRPC exporter."""
    global _tracer

    endpoint = os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")

    resource = Resource.create({ResourceAttributes.SERVICE_NAME: service_name})
    provider = TracerProvider(resource=resource)
    exporter = OTLPSpanExporter(endpoint=endpoint, insecure=True)
    provider.add_span_processor(BatchSpanProcessor(exporter))
    trace.set_tracer_provider(provider)
    _tracer = trace.get_tracer(service_name)


def get_tracer() -> trace.Tracer:
    """Get the configured tracer, falling back to a no-op tracer."""
    return _tracer or trace.get_tracer("tutoring-service")


# ── Metrics endpoint ──────────────────────────────────────────────────────────


async def metrics_endpoint(request: Request) -> Response:
    """Serve Prometheus metrics at /metrics."""
    return Response(content=generate_latest(), media_type=CONTENT_TYPE_LATEST)


# ── ASGI Middleware ───────────────────────────────────────────────────────────


class ObservabilityMiddleware(BaseHTTPMiddleware):
    """Records HTTP metrics and creates trace spans for each request."""

    async def dispatch(self, request: Request, call_next: Callable) -> Response:
        method = request.method
        path = request.url.path

        # Skip metrics endpoint to avoid recursion
        if path == "/metrics":
            return await call_next(request)

        tracer = get_tracer()
        with tracer.start_as_current_span(
            f"{method} {path}",
            attributes={"http.request.method": method, "url.path": path},
        ) as span:
            ACTIVE_REQUESTS.labels(http_request_method=method).inc()
            start = time.monotonic()

            try:
                response = await call_next(request)
            except Exception:
                ACTIVE_REQUESTS.labels(http_request_method=method).dec()
                raise

            duration = time.monotonic() - start
            status = str(response.status_code)

            ACTIVE_REQUESTS.labels(http_request_method=method).dec()
            REQUEST_DURATION.labels(
                http_request_method=method,
                http_route=path,
                http_response_status_code=status,
            ).observe(duration)
            REQUEST_COUNT.labels(
                http_request_method=method,
                http_route=path,
                http_response_status_code=status,
            ).inc()

            span.set_attribute("http.response.status_code", response.status_code)
            return response
