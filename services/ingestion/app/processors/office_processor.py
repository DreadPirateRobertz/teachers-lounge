import logging

from app.models import IngestJobMessage

logger = logging.getLogger(__name__)


async def process_office(job: IngestJobMessage) -> dict:
    """
    Stub: Office document processing (DOCX, PPTX, XLSX).

    Phase 2 full implementation will:
    - Download from GCS
    - DOCX: python-docx → HTML → hierarchical chunking
    - PPTX: python-pptx → extract slide text + speaker notes → chunking
    - XLSX: openpyxl → Markdown table extraction → chunking
    - Preserve embedded images → store in GCS
    - Generate embeddings → write to Qdrant curriculum collection
    - Update materials table: status=complete, chunk_count=N
    """
    logger.info("office_processor: processing %s (job_id=%s)", job.filename, job.job_id)
    return {"status": "stub", "job_id": str(job.job_id), "processor": "office"}
