import uuid
from typing import Annotated

from fastapi import APIRouter, Depends, HTTPException, UploadFile, status
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer

from app.config import settings
from app.models import ACCEPTED_MIME_TYPES, IngestJobMessage, ProcessingStatus, UploadResponse
from app.services import db, gcs, pubsub

router = APIRouter(prefix="/v1/ingest", tags=["ingest"])
bearer = HTTPBearer()


def _get_user_id(
    credentials: Annotated[HTTPAuthorizationCredentials, Depends(bearer)],
) -> uuid.UUID:
    """
    Stub auth: extract user_id from Bearer token.
    Phase 2 full implementation: validate JWT, extract sub claim.
    """
    try:
        return uuid.UUID(credentials.credentials)
    except ValueError:
        # In production this will be a real JWT decode
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="invalid token")


@router.post("/upload", response_model=UploadResponse, status_code=status.HTTP_202_ACCEPTED)
async def upload_file(
    file: UploadFile,
    course_id: uuid.UUID,
    user_id: Annotated[uuid.UUID, Depends(_get_user_id)],
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
    filename = file.filename or f"upload-{job_id}"

    # Store in GCS
    gcs_path = gcs.upload_raw_file(
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

    # Publish to Pub/Sub
    pubsub.publish_ingest_job(
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
