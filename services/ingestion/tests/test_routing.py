"""Tests for Pub/Sub message → processor routing.

The routing logic maps MIME types to processor keys via ACCEPTED_MIME_TYPES
and then dispatches via the _ROUTER dict. Tests mock _ROUTER entries so they
verify dispatch without invoking any real processor (GCS, DB, AI calls).
"""
import uuid
from unittest.mock import AsyncMock, patch

import pytest

from app.models import ACCEPTED_MIME_TYPES, IngestJobMessage
from app.processors import route_to_processor


def _make_job(mime_type: str) -> IngestJobMessage:
    """Build a minimal IngestJobMessage for routing tests."""
    return IngestJobMessage(
        job_id=uuid.uuid4(),
        user_id=uuid.uuid4(),
        course_id=uuid.uuid4(),
        material_id=uuid.uuid4(),
        gcs_path="gs://tvtutor-raw-uploads/test/file",
        mime_type=mime_type,
        filename="test_file",
    )


def _mock_router(processor_key: str):
    """Return a patch.dict context that mocks a single _ROUTER entry."""
    mock_fn = AsyncMock(return_value={"status": "complete", "processor": processor_key})
    return patch.dict("app.processors._ROUTER", {processor_key: mock_fn}), mock_fn


# ── Route verification ────────────────────────────────────────────────────────


@pytest.mark.asyncio
async def test_pdf_routed_to_pdf_processor():
    """PDF MIME type must be dispatched via the 'pdf' router entry."""
    ctx, mock_fn = _mock_router("pdf")
    with ctx:
        result = await route_to_processor(_make_job("application/pdf"))
    assert result["processor"] == "pdf"
    mock_fn.assert_awaited_once()


@pytest.mark.asyncio
async def test_docx_routed_to_office_processor():
    """DOCX MIME type must be dispatched via the 'office' router entry."""
    ctx, mock_fn = _mock_router("office")
    mime = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
    with ctx:
        result = await route_to_processor(_make_job(mime))
    assert result["processor"] == "office"
    mock_fn.assert_awaited_once()


@pytest.mark.asyncio
async def test_pptx_routed_to_office_processor():
    """PPTX MIME type must be dispatched via the 'office' router entry."""
    ctx, mock_fn = _mock_router("office")
    mime = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
    with ctx:
        result = await route_to_processor(_make_job(mime))
    assert result["processor"] == "office"


@pytest.mark.asyncio
async def test_xlsx_routed_to_office_processor():
    """XLSX MIME type must be dispatched via the 'office' router entry."""
    ctx, mock_fn = _mock_router("office")
    mime = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
    with ctx:
        result = await route_to_processor(_make_job(mime))
    assert result["processor"] == "office"


@pytest.mark.asyncio
async def test_mp4_routed_to_video_processor():
    """MP4 MIME type must be dispatched via the 'video' router entry."""
    ctx, mock_fn = _mock_router("video")
    with ctx:
        result = await route_to_processor(_make_job("video/mp4"))
    assert result["processor"] == "video"
    mock_fn.assert_awaited_once()


@pytest.mark.asyncio
async def test_mp3_routed_to_video_processor():
    """MP3 MIME type must be dispatched via the 'video' router entry."""
    ctx, mock_fn = _mock_router("video")
    with ctx:
        result = await route_to_processor(_make_job("audio/mpeg"))
    assert result["processor"] == "video"


@pytest.mark.asyncio
async def test_jpeg_routed_to_image_processor():
    """JPEG MIME type must be dispatched via the 'image' router entry."""
    ctx, mock_fn = _mock_router("image")
    with ctx:
        result = await route_to_processor(_make_job("image/jpeg"))
    assert result["processor"] == "image"
    mock_fn.assert_awaited_once()


@pytest.mark.asyncio
async def test_png_routed_to_image_processor():
    """PNG MIME type must be dispatched via the 'image' router entry."""
    ctx, mock_fn = _mock_router("image")
    with ctx:
        result = await route_to_processor(_make_job("image/png"))
    assert result["processor"] == "image"


@pytest.mark.asyncio
async def test_unknown_mime_raises():
    """Unsupported MIME types must raise ValueError."""
    with pytest.raises(ValueError, match="unsupported mime type"):
        await route_to_processor(_make_job("text/plain"))


@pytest.mark.asyncio
async def test_all_accepted_mimes_route_without_error():
    """Every ACCEPTED_MIME_TYPE must dispatch to a processor without raising."""
    mock_fns = {
        key: AsyncMock(return_value={"processor": key, "status": "complete"})
        for key in ("pdf", "office", "video", "image")
    }
    with patch.dict("app.processors._ROUTER", mock_fns):
        for mime in ACCEPTED_MIME_TYPES:
            result = await route_to_processor(_make_job(mime))
            assert result["status"] == "complete", f"expected complete for {mime}"
            assert "processor" in result
