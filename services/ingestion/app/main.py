import logging
import threading
from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.logging_config import configure_logging
from app.metrics import metrics_app
from app.metrics_middleware import PrometheusMiddleware
from app.routers import ingest
from app.services import embeddings as embeddings_svc
from app.services import qdrant as qdrant_svc
from app.services.db import close_pool
from app.services.pubsub import start_subscriber

configure_logging(service_name="ingestion", log_level="INFO")
logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Manage application lifespan: initialize clients on startup, close on shutdown.

    Args:
        app: The FastAPI application instance.

    Yields:
        Control to the running application between startup and shutdown.
    """
    # Initialize embedding + vector store clients
    embeddings_svc.init_client()
    qdrant_svc.init_client()
    logger.info("embedding and qdrant clients initialized")

    # Start Pub/Sub subscriber in a background thread
    thread = threading.Thread(target=start_subscriber, daemon=True, name="pubsub-subscriber")
    thread.start()
    logger.info("pub/sub subscriber thread started")

    yield

    await qdrant_svc.close_client()
    await close_pool()
    logger.info("qdrant + db pool closed")


app = FastAPI(
    title="TeachersLounge Ingestion Service",
    version="0.1.0",
    description="Accepts course material uploads, stores in GCS, dispatches to processing pipeline.",
    lifespan=lifespan,
)

app.add_middleware(PrometheusMiddleware)
app.mount("/metrics", metrics_app)

app.include_router(ingest.router)


@app.get("/healthz")
async def healthz() -> dict:
    """Return service liveness status.

    Returns:
        Dict with ``status`` key set to ``"ok"``.
    """
    return {"status": "ok"}
