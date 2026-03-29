import logging
from uuid import UUID

from google.cloud import storage

from app.config import settings

logger = logging.getLogger(__name__)


def upload_raw_file(
    data: bytes,
    user_id: UUID,
    course_id: UUID,
    job_id: UUID,
    filename: str,
    content_type: str,
) -> str:
    """Upload a raw file to GCS. Returns the gs:// URI."""
    client = storage.Client(project=settings.gcp_project)
    bucket = client.bucket(settings.gcs_raw_bucket)

    blob_name = f"{user_id}/{course_id}/{job_id}/{filename}"
    blob = bucket.blob(blob_name)
    blob.metadata = {
        "user_id": str(user_id),
        "course_id": str(course_id),
        "job_id": str(job_id),
    }

    blob.upload_from_string(data, content_type=content_type)
    gcs_path = f"gs://{settings.gcs_raw_bucket}/{blob_name}"
    logger.info("uploaded %s to %s", filename, gcs_path)
    return gcs_path
