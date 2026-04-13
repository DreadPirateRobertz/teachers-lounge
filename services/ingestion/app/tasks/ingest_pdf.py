"""Celery-compatible task entry point for PDF ingestion.

Wraps the async :func:`~app.processors.pdf_processor.process_pdf` pipeline
with a synchronous interface suitable for Celery workers or any sync caller.
"""

import asyncio
import logging
from uuid import UUID

from app.models import IngestJobMessage
from app.processors.pdf_processor import process_pdf

logger = logging.getLogger(__name__)


def ingest_pdf(
    job_id: UUID,
    user_id: UUID,
    course_id: UUID,
    material_id: UUID,
    gcs_path: str,
    mime_type: str = "application/pdf",
    filename: str = "upload.pdf",
) -> dict:
    """Run the full PDF ingestion pipeline from a synchronous task context.

    Constructs an :class:`~app.models.IngestJobMessage` and executes the async
    :func:`~app.processors.pdf_processor.process_pdf` pipeline via
    :func:`asyncio.run`, making the function callable from Celery workers or
    any synchronous context.

    Pipeline steps (delegated to the processor):

    1. Download the PDF from GCS.
    2. Detect digital vs. scanned.
    3. Parse with unstructured.io (layout-aware).
    4. Build hierarchical text chunks.
    5. Generate dense embeddings (OpenAI).
    6. Upsert vectors to Qdrant.
    7. Write chunk metadata to Postgres.
    8. Extract figures; generate CLIP embeddings and upsert to Qdrant.
    9. Update material status to ``complete``.

    Args:
        job_id: UUID for this ingestion job, used for tracing and temp-file naming.
        user_id: UUID of the user who uploaded the material.
        course_id: UUID of the course the material belongs to.
        material_id: UUID of the material record in the database.
        gcs_path: Full GCS URI of the uploaded file, e.g. ``gs://bucket/path/to/file.pdf``.
        mime_type: MIME type of the source file; defaults to ``application/pdf``.
        filename: Original filename used for logging; defaults to ``upload.pdf``.

    Returns:
        Result dict from :func:`~app.processors.pdf_processor.process_pdf`::

            {
                "status": "complete",
                "job_id": "<uuid>",
                "chunk_count": <int>,
                "diagram_count": <int>,
                "processor": "pdf",
            }

    Raises:
        FileNotFoundError: Propagated when the GCS object does not exist.
        Exception: Any other pipeline failure (Qdrant upsert, malformed PDF,
            DB write) propagates to the caller for Celery retry handling.
    """
    job = IngestJobMessage(
        job_id=job_id,
        user_id=user_id,
        course_id=course_id,
        material_id=material_id,
        gcs_path=gcs_path,
        mime_type=mime_type,
        filename=filename,
    )
    logger.info(
        "ingest_pdf task: starting job_id=%s file=%s gcs_path=%s",
        job_id,
        filename,
        gcs_path,
    )
    result = asyncio.run(process_pdf(job))
    logger.info("ingest_pdf task: complete job_id=%s chunks=%s", job_id, result.get("chunk_count"))
    return result
