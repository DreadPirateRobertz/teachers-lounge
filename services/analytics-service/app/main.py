"""Analytics Service entry point.

FastAPI application serving student learning analytics.  Reads from Postgres
tables: ``gaming_profiles``, ``quiz_results``, and ``interactions``.

All endpoints require a valid JWT (Bearer token) issued by the user-service.
"""
import logging

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from .config import settings
from .metrics import metrics_app
from .metrics_middleware import PrometheusMiddleware
from .routes.student import router as student_router

logging.basicConfig(level=settings.log_level.upper())
logger = logging.getLogger(__name__)

app = FastAPI(
    title="TeachersLounge — Analytics Service",
    version="0.1.0",
    description="Student learning analytics: XP progression, quiz breakdown, activity history.",
)

app.add_middleware(PrometheusMiddleware)
app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.allowed_origins.split(","),
    allow_methods=["GET"],
    allow_headers=["Authorization", "Content-Type"],
)

app.mount("/metrics", metrics_app)

app.include_router(student_router)


@app.get("/health", tags=["ops"])
async def health() -> dict[str, str]:
    """Liveness probe endpoint.

    Returns:
        A dict ``{"status": "ok"}`` when the service is running.
    """
    return {"status": "ok"}
