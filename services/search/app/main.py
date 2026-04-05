import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.routers import diagrams, search
from app.services.qdrant import close_client, init_client

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(name)s %(levelname)s %(message)s")


@asynccontextmanager
async def lifespan(app: FastAPI):
    init_client()
    yield
    await close_client()


app = FastAPI(
    title="TeachersLounge Search Service",
    version="0.1.0",
    description="Hybrid vector search over curriculum content.",
    lifespan=lifespan,
)

app.include_router(search.router)
app.include_router(diagrams.router)


@app.get("/healthz")
async def healthz() -> dict:
    return {"status": "ok"}
