"""Tutoring Service — Phase 1 + Phase 8 Observability.

FastAPI application entry point.

Phase 1 capabilities:
- Create and manage chat sessions
- Stream AI responses via SSE (Professor Nova, no RAG)
- Persist conversation history in Postgres

Phase 8 additions:
- Prometheus metrics at /metrics (RED metrics + RAG-specific histograms)
- OpenTelemetry distributed tracing exported to Google Cloud Trace
"""

import logging
import os

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor

from .cache import close_cache, init_cache
from .chat import router as chat_router
from .chat_simple import router as chat_simple_router
from .concepts import router as concepts_router
from .config import settings
from .database import Base, engine
from .health import router as health_router
from .logging_config import configure_logging
from .metrics import metrics_app
from .metrics_middleware import PrometheusMiddleware
from .profile import router as profile_router
from .quiz import router as quiz_router
from .reviews import router as reviews_router
from .sessions import router as sessions_router

configure_logging(service_name="tutoring-service", log_level=settings.log_level)
logger = logging.getLogger(__name__)

# ── OpenTelemetry setup ───────────────────────────────────────────────────────
# Exports traces to Google Cloud Trace via the OTLP gRPC endpoint.
# OTEL_EXPORTER_OTLP_ENDPOINT defaults to the GKE DaemonSet collector.

_otel_endpoint = os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

if _otel_endpoint:
    _resource = Resource.create({"service.name": "tutoring-service"})
    _provider = TracerProvider(resource=_resource)
    _provider.add_span_processor(
        BatchSpanProcessor(OTLPSpanExporter(endpoint=_otel_endpoint, insecure=True))
    )
    trace.set_tracer_provider(_provider)
    logger.info("OpenTelemetry tracing enabled", extra={"endpoint": _otel_endpoint})
else:
    logger.info("OTEL_EXPORTER_OTLP_ENDPOINT not set — tracing disabled")

app = FastAPI(
    title="TeachersLounge — Tutoring Service",
    version="0.1.0",
    description="Phase 1: basic chat with Professor Nova, streaming SSE, conversation history.",
)

# Instrument FastAPI for auto-span creation on each route.
FastAPIInstrumentor.instrument_app(app)

app.add_middleware(PrometheusMiddleware)
app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.allowed_origins.split(","),
    allow_methods=["GET", "POST"],
    allow_headers=["Authorization", "Content-Type"],
)

app.mount("/metrics", metrics_app)

app.include_router(health_router)
app.include_router(sessions_router, prefix="/v1")
app.include_router(chat_router, prefix="/v1")
app.include_router(chat_simple_router, prefix="/v1")
app.include_router(reviews_router, prefix="/v1")
app.include_router(concepts_router, prefix="/v1")
app.include_router(quiz_router, prefix="/v1")
app.include_router(profile_router, prefix="/v1")


@app.on_event("startup")
async def on_startup():
    """Initialise database tables and Redis cache on application startup."""
    # Create tables if they don't exist (dev only — production uses Alembic migrations)
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    await init_cache()
    logger.info("Tutoring service started")


@app.on_event("shutdown")
async def on_shutdown():
    """Close Redis cache connection on application shutdown."""
    await close_cache()


