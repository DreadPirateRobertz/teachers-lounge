import asyncio
import logging
import tempfile
from pathlib import Path
from uuid import UUID

# google-cloud-storage imported lazily inside functions to avoid
# requiring the package at module import time (speeds up test collection).
from app.config import settings

logger = logging.getLogger(__name__)


def _upload_raw_file_sync(
    data: bytes,
    user_id: UUID,
    course_id: UUID,
    job_id: UUID,
    safe_filename: str,
    content_type: str,
) -> str:
    """Synchronous GCS upload. Called via run_in_executor — never call directly from async code."""
    from google.cloud import storage  # lazy

    client = storage.Client(project=settings.gcp_project)
    bucket = client.bucket(settings.gcs_raw_bucket)

    blob_name = f"{user_id}/{course_id}/{job_id}/{safe_filename}"
    blob = bucket.blob(blob_name)
    blob.metadata = {
        "user_id": str(user_id),
        "course_id": str(course_id),
        "job_id": str(job_id),
    }

    blob.upload_from_string(data, content_type=content_type)
    gcs_path = f"gs://{settings.gcs_raw_bucket}/{blob_name}"
    logger.info("uploaded %s to %s", safe_filename, gcs_path)
    return gcs_path


def _download_file_sync(gcs_path: str, job_id: UUID) -> Path:
    """Synchronous GCS download to a local temp file. Returns the Path."""
    from google.cloud import storage  # lazy

    parts = gcs_path.replace("gs://", "").split("/", 1)
    bucket_name, blob_name = parts[0], parts[1]

    client = storage.Client(project=settings.gcp_project)
    bucket = client.bucket(bucket_name)
    blob = bucket.blob(blob_name)

    suffix = Path(blob_name).suffix or ".bin"
    tmp = tempfile.NamedTemporaryFile(
        delete=False, suffix=suffix, prefix=f"ingest-{job_id}-"
    )
    blob.download_to_filename(tmp.name)
    tmp.close()
    logger.info("downloaded %s → %s", gcs_path, tmp.name)
    return Path(tmp.name)


async def download_file(gcs_path: str, job_id: UUID) -> Path:
    """Download a GCS object to a local temp file without blocking the event loop.

    Returns:
        Path to the downloaded temp file. Caller is responsible for cleanup.
    """
    loop = asyncio.get_running_loop()
    return await loop.run_in_executor(None, _download_file_sync, gcs_path, job_id)


async def upload_raw_file(
    data: bytes,
    user_id: UUID,
    course_id: UUID,
    job_id: UUID,
    filename: str,
    content_type: str,
) -> str:
    """Upload a raw file to GCS without blocking the event loop. Returns the gs:// URI."""
    # Strip directory components to prevent path traversal in the GCS blob name
    safe_filename = Path(filename).name or f"upload-{job_id}"

    loop = asyncio.get_running_loop()
    return await loop.run_in_executor(
        None,
        _upload_raw_file_sync,
        data,
        user_id,
        course_id,
        job_id,
        safe_filename,
        content_type,
    )
