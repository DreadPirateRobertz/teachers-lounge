import logging

from app.models import ACCEPTED_MIME_TYPES, IngestJobMessage
from app.processors.image_processor import process_image
from app.processors.office_processor import process_office
from app.processors.pdf_processor import process_pdf
from app.processors.video_processor import process_video

logger = logging.getLogger(__name__)

_ROUTER = {
    "pdf": process_pdf,
    "office": process_office,
    "video": process_video,
    "image": process_image,
}


async def route_to_processor(job: IngestJobMessage) -> dict:
    """Dispatch an ingest job to the appropriate processor based on MIME type.

    Args:
        job: The ingest job message containing the file's MIME type and metadata.

    Returns:
        A result dict produced by the selected processor (keys vary by type).

    Raises:
        ValueError: If job.mime_type is not in ACCEPTED_MIME_TYPES.
    """
    processor_key = ACCEPTED_MIME_TYPES.get(job.mime_type)
    if not processor_key:
        raise ValueError(f"unsupported mime type: {job.mime_type}")

    processor = _ROUTER[processor_key]
    logger.info(
        "routing job_id=%s mime=%s → %s", job.job_id, job.mime_type, processor_key
    )
    return await processor(job)
