"""Starlette middleware that records HTTP RED metrics for analytics-service."""

import time

from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request

from .metrics import http_request_duration_seconds, http_requests_total


class PrometheusMiddleware(BaseHTTPMiddleware):
    """Records http_request_duration_seconds and http_requests_total per request.

    Args:
        app: The ASGI application to wrap.
    """

    async def dispatch(self, request: Request, call_next):
        """Dispatch the request, recording latency and count metrics.

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
