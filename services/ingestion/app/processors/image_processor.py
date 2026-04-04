import logging

from app.models import IngestJobMessage

logger = logging.getLogger(__name__)


async def process_image(job: IngestJobMessage) -> dict:
    """
    Stub: Image processing pipeline (JPG, PNG).

    Phase 2 full implementation will:
    - Download from GCS
    - Google Document AI: OCR for scanned notes / handwriting
    - Handwriting model if handwriting detected
    - Extract text → chunking
    - Store original image in GCS processed bucket
    - Generate text embedding → write to Qdrant curriculum collection
    - Phase 6+: also generate CLIP embedding → diagrams collection
    - Update materials table: status=complete, chunk_count=N
    """
    logger.info("image_processor: processing %s (job_id=%s)", job.filename, job.job_id)
    return {"status": "stub", "job_id": str(job.job_id), "processor": "image"}
