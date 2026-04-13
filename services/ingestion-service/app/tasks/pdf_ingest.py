"""Celery task for the PDF text extraction and RAG ingestion pipeline.

Workflow per task invocation:
1. Extract raw text per page from PDF bytes using PyMuPDF (fitz).
2. Chunk text into 512-token segments with 64-token overlap via tiktoken.
3. Embed each chunk via the AI gateway (POST /embeddings).
4. Upsert embeddings into the Qdrant ``curriculum`` collection.

Retry policy: up to 3 retries with exponential backoff (base 60 s, cap 300 s).
"""

import logging
import os
from uuid import uuid4

import fitz  # PyMuPDF
import httpx
from celery import Celery
from qdrant_client import QdrantClient
from qdrant_client.models import PointStruct

from app.chunker import chunk_pdf_pages

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Celery application
# ---------------------------------------------------------------------------

_BROKER_URL = os.environ.get("CELERY_BROKER_URL", "redis://localhost:6379/0")
_RESULT_BACKEND = os.environ.get("CELERY_RESULT_BACKEND", "redis://localhost:6379/0")

celery_app = Celery(
    "ingestion_service",
    broker=_BROKER_URL,
    backend=_RESULT_BACKEND,
)
celery_app.conf.task_serializer = "json"
celery_app.conf.result_serializer = "json"
celery_app.conf.accept_content = ["json"]

# ---------------------------------------------------------------------------
# Environment-driven configuration
# ---------------------------------------------------------------------------

_AI_GATEWAY_URL: str = os.environ.get(
    "AI_GATEWAY_URL",
    "http://ai-gateway.ai-gateway.svc.cluster.local:8000",
)
_EMBEDDING_MODEL: str = os.environ.get("EMBEDDING_MODEL", "text-embedding-3-small")
_QDRANT_HOST: str = os.environ.get("QDRANT_HOST", "qdrant.qdrant.svc.cluster.local")
_QDRANT_PORT: int = int(os.environ.get("QDRANT_PORT", "6333"))
_QDRANT_API_KEY: str | None = os.environ.get("QDRANT_API_KEY")
_CURRICULUM_COLLECTION: str = os.environ.get("CURRICULUM_COLLECTION", "curriculum")
_CHUNK_SIZE: int = int(os.environ.get("CHUNK_SIZE", "512"))
_CHUNK_OVERLAP: int = int(os.environ.get("CHUNK_OVERLAP", "64"))
_EMBED_TIMEOUT: float = float(os.environ.get("EMBED_TIMEOUT_SECONDS", "60"))
_EMBED_BATCH_SIZE: int = int(os.environ.get("EMBED_BATCH_SIZE", "100"))


# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------


def _extract_pages(pdf_bytes: bytes) -> list[str]:
    """Extract raw text per page from PDF bytes using PyMuPDF.

    Each item in the returned list corresponds to one PDF page (1-indexed
    by the caller — indices here are 0-based, page numbers assigned outside).

    Args:
        pdf_bytes: Raw PDF file content.

    Returns:
        List of page text strings in document order.
    """
    doc = fitz.open(stream=pdf_bytes, filetype="pdf")
    try:
        return [page.get_text() for page in doc]
    finally:
        doc.close()


def _embed_texts(texts: list[str]) -> list[list[float]]:
    """Embed a batch of texts via the AI gateway /embeddings endpoint.

    Batches requests according to ``_EMBED_BATCH_SIZE`` to respect gateway
    payload limits. Input order is preserved in the output.

    Args:
        texts: Non-empty list of text strings to embed.

    Returns:
        List of embedding vectors (same length as ``texts``).

    Raises:
        httpx.HTTPStatusError: If the gateway returns a non-2xx status.
    """
    all_embeddings: list[list[float]] = []
    url = f"{_AI_GATEWAY_URL}/embeddings"

    with httpx.Client(timeout=_EMBED_TIMEOUT) as client:
        for i in range(0, len(texts), _EMBED_BATCH_SIZE):
            batch = texts[i : i + _EMBED_BATCH_SIZE]
            response = client.post(
                url,
                json={"model": _EMBEDDING_MODEL, "input": batch},
            )
            response.raise_for_status()
            data = response.json()
            batch_embeddings = [
                item["embedding"]
                for item in sorted(data["data"], key=lambda x: x["index"])
            ]
            all_embeddings.extend(batch_embeddings)

    return all_embeddings


def _upsert_to_qdrant(
    chunks: list[dict],
    vectors: list[list[float]],
    course_id: str,
    source_pdf: str,
) -> None:
    """Upsert chunk vectors into the Qdrant curriculum collection.

    Each point is assigned a fresh UUID. The payload stores the fields
    required for downstream retrieval and attribution:
    course_id, page, chunk_index, source_pdf, and the raw text.

    Upserts are issued in batches of 100 to avoid oversized requests.

    Args:
        chunks: Chunk dicts produced by ``chunk_pdf_pages``.
        vectors: Embedding vectors; must be the same length as ``chunks``.
        course_id: Identifier for the owning course.
        source_pdf: Original PDF path or filename.
    """
    client = QdrantClient(
        host=_QDRANT_HOST,
        port=_QDRANT_PORT,
        api_key=_QDRANT_API_KEY,
    )
    points = [
        PointStruct(
            id=str(uuid4()),
            vector=vector,
            payload={
                "course_id": course_id,
                "page": chunk["page"],
                "chunk_index": chunk["chunk_index"],
                "source_pdf": source_pdf,
                "text": chunk["text"],
            },
        )
        for chunk, vector in zip(chunks, vectors)
    ]

    batch_size = 100
    for i in range(0, len(points), batch_size):
        client.upsert(
            collection_name=_CURRICULUM_COLLECTION,
            points=points[i : i + batch_size],
        )

    logger.info(
        "upserted %d chunks | source_pdf=%s course_id=%s",
        len(points),
        source_pdf,
        course_id,
    )


# ---------------------------------------------------------------------------
# Celery task
# ---------------------------------------------------------------------------


@celery_app.task(
    bind=True,
    max_retries=3,
    default_retry_delay=60,
    autoretry_for=(Exception,),
    retry_backoff=True,
    retry_backoff_max=300,
    retry_jitter=True,
)
def ingest_pdf(self, pdf_bytes: bytes, course_id: str, source_pdf: str) -> dict:
    """Extract, chunk, embed, and upsert a PDF document.

    This is a Celery task. On failure it retries up to 3 times with
    exponential backoff starting at 60 s, capped at 300 s, with jitter.

    Args:
        self: Celery task instance; used to access retry state and request context.
        pdf_bytes: Raw bytes of the PDF file to ingest.
        course_id: Course identifier associated with this document.
        source_pdf: Original PDF filename or URI; stored in each Qdrant point.

    Returns:
        Dict with keys: chunk_count (int), course_id (str), source_pdf (str).
    """
    logger.info(
        "ingest_pdf start | source_pdf=%s course_id=%s attempt=%d",
        source_pdf,
        course_id,
        self.request.retries,
    )

    # Step 1: Extract text per page
    pages = _extract_pages(pdf_bytes)
    logger.info("ingest_pdf: extracted %d pages | source_pdf=%s", len(pages), source_pdf)

    # Step 2: Chunk
    chunks = chunk_pdf_pages(pages, chunk_size=_CHUNK_SIZE, overlap=_CHUNK_OVERLAP)
    logger.info("ingest_pdf: produced %d chunks | source_pdf=%s", len(chunks), source_pdf)

    if not chunks:
        logger.warning("ingest_pdf: no chunks produced | source_pdf=%s", source_pdf)
        return {"chunk_count": 0, "course_id": course_id, "source_pdf": source_pdf}

    # Step 3: Embed
    texts = [c["text"] for c in chunks]
    vectors = _embed_texts(texts)

    # Step 4: Upsert to Qdrant
    _upsert_to_qdrant(chunks, vectors, course_id, source_pdf)

    result = {"chunk_count": len(chunks), "course_id": course_id, "source_pdf": source_pdf}
    logger.info("ingest_pdf complete | %s", result)
    return result
