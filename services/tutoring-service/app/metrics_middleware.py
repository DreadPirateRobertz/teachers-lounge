"""Starlette middleware that records HTTP RED metrics for every request."""

import time

from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request

from .metrics import http_request_duration_seconds, http_requests_total


class PrometheusMiddleware(BaseHTTPMiddleware):
    """Records http_request_duration_seconds and http_requests_total for each request.

    Mounts before routing so all requests — including 404s — are captured.
    The path label uses the raw URL path; for high-cardinality routes (e.g.
    /v1/sessions/{id}) callers should normalise the label separately if needed.
    """

    async def dispatch(self, request: Request, call_next):
        """Instrument the request and record metrics after the response.

        Args:
            request: Incoming Starlette request.
            call_next: Next middleware or route handler.

        Returns:
            The HTTP response from downstream.
        """
        start = time.perf_counter()
        response = await call_next(request)
        duration = time.perf_counter() - start

        labels = {
            "method": request.method,
            "path": request.url.path,
            "status": str(response.status_code),
        }

        http_request_duration_seconds.labels(**labels).observe(duration)
        http_requests_total.labels(**labels).inc()

        return response
