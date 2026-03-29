"""Tests for Pub/Sub message → processor routing."""
import uuid

import pytest

from app.models import ACCEPTED_MIME_TYPES, IngestJobMessage
from app.processors import route_to_processor


def _make_job(mime_type: str) -> IngestJobMessage:
    return IngestJobMessage(
        job_id=uuid.uuid4(),
        user_id=uuid.uuid4(),
        course_id=uuid.uuid4(),
        material_id=uuid.uuid4(),
        gcs_path=f"gs://tvtutor-raw-uploads/test/file",
        mime_type=mime_type,
        filename="test_file",
    )


@pytest.mark.asyncio
async def test_pdf_routed_to_pdf_processor():
    result = await route_to_processor(_make_job("application/pdf"))
    assert result["processor"] == "pdf"
    assert result["status"] == "stub"


@pytest.mark.asyncio
async def test_docx_routed_to_office_processor():
    mime = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
    result = await route_to_processor(_make_job(mime))
    assert result["processor"] == "office"


@pytest.mark.asyncio
async def test_pptx_routed_to_office_processor():
    mime = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
    result = await route_to_processor(_make_job(mime))
    assert result["processor"] == "office"


@pytest.mark.asyncio
async def test_mp4_routed_to_video_processor():
    result = await route_to_processor(_make_job("video/mp4"))
    assert result["processor"] == "video"


@pytest.mark.asyncio
async def test_mp3_routed_to_video_processor():
    result = await route_to_processor(_make_job("audio/mpeg"))
    assert result["processor"] == "video"


@pytest.mark.asyncio
async def test_jpeg_routed_to_image_processor():
    result = await route_to_processor(_make_job("image/jpeg"))
    assert result["processor"] == "image"


@pytest.mark.asyncio
async def test_png_routed_to_image_processor():
    result = await route_to_processor(_make_job("image/png"))
    assert result["processor"] == "image"


@pytest.mark.asyncio
async def test_unknown_mime_raises():
    with pytest.raises(ValueError, match="unsupported mime type"):
        await route_to_processor(_make_job("text/plain"))


@pytest.mark.asyncio
async def test_all_accepted_mimes_route_without_error():
    """Every ACCEPTED_MIME_TYPE must route to a processor without raising."""
    for mime in ACCEPTED_MIME_TYPES:
        result = await route_to_processor(_make_job(mime))
        assert result["status"] == "stub"
        assert "processor" in result
