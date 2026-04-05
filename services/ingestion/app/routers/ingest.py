import uuid
from pathlib import Path
from typing import Annotated

from fastapi import APIRouter, Depends, UploadFile, HTTPException, status

from app.auth import require_auth
from app.config import settings
from app.models import (
    ACCEPTED_MIME_TYPES,
    IngestJobMessage,
    MaterialStatusResponse,
    ProcessingStatus,
    UploadResponse,
)
from app.services import db, gcs, pubsub

router = APIRouter(prefix="/v1/ingest", tags=["ingest"])


@router.post("/upload", response_model=UploadResponse, status_code=status.HTTP_202_ACCEPTED)
async def upload_file(
    file: UploadFile,
    course_id: uuid.UUID,
    user_id: Annotated[uuid.UUID, Depends(require_auth)],
) -> UploadResponse:
    """
    Accept a course material upload, store in GCS, enqueue for processing.

    - Validates file type against allowlist
    - Enforces max upload size
    - Stores raw file in gs://tvtutor-raw-uploads/{user_id}/{course_id}/{job_id}/{filename}
    - Inserts materials row (status=pending)
    - Publishes IngestJobMessage to ingest-jobs Pub/Sub topic
    - Returns job_id for status polling
    """
    # Validate MIME type
    content_type = file.content_type or ""
    if content_type not in ACCEPTED_MIME_TYPES:
        raise HTTPException(
            status_code=status.HTTP_415_UNSUPPORTED_MEDIA_TYPE,
            detail=f"unsupported file type: {content_type!r}. "
                   f"Accepted: {sorted(ACCEPTED_MIME_TYPES)}",
        )

    # Read and enforce size limit
    data = await file.read()
    if len(data) > settings.max_upload_bytes:
        raise HTTPException(
            status_code=status.HTTP_413_REQUEST_ENTITY_TOO_LARGE,
            detail=f"file exceeds {settings.max_upload_bytes // (1024 * 1024)} MB limit",
        )

    job_id = uuid.uuid4()
    material_id = uuid.uuid4()
    # Strip directory components — prevents path traversal in GCS blob names
    filename = Path(file.filename or f"upload-{job_id}").name or f"upload-{job_id}"

    # Store in GCS (runs in thread pool — does not block the event loop)
    gcs_path = await gcs.upload_raw_file(
        data=data,
        user_id=user_id,
        course_id=course_id,
        job_id=job_id,
        filename=filename,
        content_type=content_type,
    )

    # Write to Postgres
    await db.create_material(
        material_id=material_id,
        course_id=course_id,
        user_id=user_id,
        filename=filename,
        gcs_path=gcs_path,
        file_type=ACCEPTED_MIME_TYPES[content_type],
    )

    # Publish to Pub/Sub (runs in thread pool — does not block the event loop)
    await pubsub.publish_ingest_job(
        IngestJobMessage(
            job_id=job_id,
            user_id=user_id,
            course_id=course_id,
            material_id=material_id,
            gcs_path=gcs_path,
            mime_type=content_type,
            filename=filename,
        )
    )

    return UploadResponse(
        job_id=job_id,
        material_id=material_id,
        status=ProcessingStatus.PENDING,
        gcs_path=gcs_path,
    )


@router.get(
    "/{material_id}/status",
    response_model=MaterialStatusResponse,
    summary="Poll processing status for an uploaded material",
)
async def get_material_status(
    material_id: uuid.UUID,
    user_id: Annotated[uuid.UUID, Depends(require_auth)],
) -> MaterialStatusResponse:
    """Return the current processing status and chunk count for a material.

    The upload endpoint returns a ``material_id``; clients poll this endpoint
    to track progress from ``pending`` → ``processing`` → ``complete``
    (or ``failed``).

    Args:
        material_id: UUID of the material returned by the upload endpoint.
        user_id: Authenticated caller's UUID (validated JWT).

    Returns:
        MaterialStatusResponse with current status and chunk count.

    Raises:
        HTTPException: 404 when no material with that ID exists.
    """
    row = await db.get_material_status(material_id)
    if row is None:
        raise HTTPException(
            status_code=status.HTTP_404_NOT_FOUND,
            detail="material not found",
        )
    return MaterialStatusResponse(
        material_id=material_id,
        processing_status=ProcessingStatus(row["processing_status"]),
        chunk_count=row["chunk_count"],
    )
