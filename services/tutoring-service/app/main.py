"""Tutoring Service — Phase 1

FastAPI application entry point.

Phase 1 capabilities:
- Create and manage chat sessions
- Stream AI responses via SSE (Professor Nova, no RAG)
- Persist conversation history in Postgres
"""
import logging

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from .chat import router as chat_router
from .chat_simple import router as chat_simple_router
from .config import settings
from .database import Base, engine
from .observability import ObservabilityMiddleware, init_tracer, metrics_endpoint
from .sessions import router as sessions_router

logging.basicConfig(level=settings.log_level.upper())
logger = logging.getLogger(__name__)

app = FastAPI(
    title="TeachersLounge — Tutoring Service",
    version="0.1.0",
    description="Phase 1: basic chat with Professor Nova, streaming SSE, conversation history.",
)

app.add_middleware(ObservabilityMiddleware)
app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.allowed_origins.split(","),
    allow_methods=["GET", "POST"],
    allow_headers=["Authorization", "Content-Type"],
)

app.add_api_route("/metrics", metrics_endpoint, methods=["GET"], tags=["ops"])
app.include_router(sessions_router, prefix="/v1")
app.include_router(chat_router, prefix="/v1")
app.include_router(chat_simple_router, prefix="/v1")


@app.on_event("startup")
async def on_startup():
    # Initialize OpenTelemetry tracing
    try:
        init_tracer("tutoring-service")
        logger.info("OpenTelemetry tracing initialized")
    except Exception as e:
        logger.warning("Failed to init tracer, continuing without tracing: %s", e)

    # Create tables if they don't exist (dev only — production uses Alembic migrations)
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    logger.info("Tutoring service started")


@app.get("/health", tags=["ops"])
async def health():
    return {"status": "ok"}


@app.get("/health/readiness", tags=["ops"])
async def readiness():
    # Phase 2+: check DB connection and AI gateway reachability
    return {"status": "ready"}
