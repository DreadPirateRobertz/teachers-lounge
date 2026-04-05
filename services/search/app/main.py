"""Search Service entry point — Phase 7: OpenTelemetry tracing added (tl-dkg)."""
import logging
import os
from contextlib import asynccontextmanager

from fastapi import FastAPI
from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor

from app.logging_config import configure_logging
from app.routers import diagrams, search
from app.services.qdrant import close_client, init_client

configure_logging(service_name="search", log_level="INFO")
logger = logging.getLogger(__name__)

# ── OpenTelemetry setup ───────────────────────────────────────────────────────
_otel_endpoint = os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
if _otel_endpoint:
    _resource = Resource.create({"service.name": "search-service"})
    _provider = TracerProvider(resource=_resource)
    _provider.add_span_processor(
        BatchSpanProcessor(OTLPSpanExporter(endpoint=_otel_endpoint, insecure=True))
    )
    trace.set_tracer_provider(_provider)
    logger.info("OpenTelemetry tracing enabled → %s", _otel_endpoint)
else:
    logger.info("OTEL_EXPORTER_OTLP_ENDPOINT not set — tracing disabled")


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Initialize and shut down the Qdrant client around the app lifecycle."""
    init_client()
    yield
    await close_client()


app = FastAPI(
    title="TeachersLounge Search Service",
    version="0.1.0",
    description="Hybrid vector search over curriculum content.",
    lifespan=lifespan,
)

FastAPIInstrumentor.instrument_app(app)

app.include_router(search.router)
app.include_router(diagrams.router)


@app.get("/healthz")
async def healthz() -> dict:
    """Liveness probe endpoint.

    Returns:
        A dict ``{"status": "ok"}`` when the service is running.
    """
    return {"status": "ok"}
