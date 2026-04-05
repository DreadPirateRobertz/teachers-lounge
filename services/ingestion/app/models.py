from enum import StrEnum
from uuid import UUID

from pydantic import BaseModel


class ProcessingStatus(StrEnum):
    """Lifecycle states for a material ingestion job."""

    PENDING = "pending"
    PROCESSING = "processing"
    COMPLETE = "complete"
    FAILED = "failed"


class UploadResponse(BaseModel):
    """Response body returned to the client after a successful material upload."""

    job_id: UUID
    material_id: UUID
    status: ProcessingStatus
    gcs_path: str


class IngestJobMessage(BaseModel):
    """Pub/Sub message payload for the ingest-jobs topic."""
    job_id: UUID
    user_id: UUID
    course_id: UUID
    material_id: UUID
    gcs_path: str
    mime_type: str
    filename: str


ACCEPTED_MIME_TYPES: dict[str, str] = {
    "application/pdf": "pdf",
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document": "office",
    "application/vnd.openxmlformats-officedocument.presentationml.presentation": "office",
    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": "office",
    "video/mp4": "video",
    "video/quicktime": "video",
    "audio/mpeg": "video",   # audio routed through video processor (both use transcription)
    "audio/wav": "video",
    "image/jpeg": "image",
    "image/png": "image",
}
