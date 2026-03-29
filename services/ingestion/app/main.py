import logging
import threading
from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.routers import ingest
from app.services.db import close_pool
from app.services.pubsub import start_subscriber

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(name)s %(levelname)s %(message)s")
logger = logging.getLogger(__name__)


@asynccontextmanager
async def lifespan(app: FastAPI):
    # Start Pub/Sub subscriber in a background thread
    thread = threading.Thread(target=start_subscriber, daemon=True, name="pubsub-subscriber")
    thread.start()
    logger.info("pub/sub subscriber thread started")

    yield

    await close_pool()
    logger.info("db pool closed")


app = FastAPI(
    title="TeachersLounge Ingestion Service",
    version="0.1.0",
    description="Accepts course material uploads, stores in GCS, dispatches to processing pipeline.",
    lifespan=lifespan,
)

app.include_router(ingest.router)


@app.get("/healthz")
async def healthz() -> dict:
    return {"status": "ok"}
