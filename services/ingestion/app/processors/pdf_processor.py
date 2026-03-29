import logging

from app.models import IngestJobMessage

logger = logging.getLogger(__name__)


async def process_pdf(job: IngestJobMessage) -> dict:
    """
    Stub: PDF processing pipeline.

    Phase 2 full implementation will:
    - Download from GCS
    - Detect digital vs scanned (pdfminer heuristic)
    - Digital: unstructured.io → layout-aware hierarchical chunking
    - Scanned: Google Document AI OCR → chunking
    - Extract figures → store in GCS, create text chunk with caption
    - Preserve tables (Markdown), equations (LaTeX)
    - Generate embeddings → write to Qdrant curriculum collection
    - Update materials table: status=complete, chunk_count=N
    """
    logger.info("pdf_processor: processing %s (job_id=%s)", job.filename, job.job_id)
    return {"status": "stub", "job_id": str(job.job_id), "processor": "pdf"}
