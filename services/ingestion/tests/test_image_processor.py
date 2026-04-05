"""Tests for services/ingestion/app/processors/image_processor.py.

Covers:
- _extract_layout_text — normal segments, empty anchor, multiple segments
- _build_image_chunks — delegates to flush_segments with correct segment shape
- _ocr_with_document_ai — paragraph extraction, fallback to full text, empty doc
- process_image — happy path, missing config raises ValueError,
  exception → FAILED status, temp file cleanup, empty OCR result
"""
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock, patch
from uuid import uuid4

import pytest

from app.models import ProcessingStatus
from app.processors.image_processor import (
    _build_image_chunks,
    _extract_layout_text,
    process_image,
)

MATERIAL_ID = uuid4()
COURSE_ID = uuid4()
JOB_ID = uuid4()


def _make_job(mime="image/jpeg"):
    job = MagicMock()
    job.job_id = JOB_ID
    job.material_id = MATERIAL_ID
    job.course_id = COURSE_ID
    job.gcs_path = "gs://bucket/image.jpg"
    job.filename = "image.jpg"
    job.mime_type = mime
    return job


# ── _extract_layout_text ─────────────────────────────────────────────────────


class TestExtractLayoutText:
    def test_single_segment_returns_slice(self):
        """Single TextAnchor segment returns the correct substring."""
        segment = MagicMock()
        segment.start_index = 0
        segment.end_index = 5

        layout = MagicMock()
        layout.text_anchor.text_segments = [segment]

        result = _extract_layout_text("Hello, world!", layout)
        assert result == "Hello"

    def test_multiple_segments_concatenated(self):
        """Multiple TextAnchor segments are joined without separator."""
        seg1 = MagicMock()
        seg1.start_index = 0
        seg1.end_index = 5
        seg2 = MagicMock()
        seg2.start_index = 7
        seg2.end_index = 12

        layout = MagicMock()
        layout.text_anchor.text_segments = [seg1, seg2]

        result = _extract_layout_text("Hello, world!", layout)
        assert result == "Helloworld"

    def test_empty_text_anchor_returns_empty_string(self):
        """Missing text_anchor or empty segments returns empty string."""
        layout_no_anchor = MagicMock()
        layout_no_anchor.text_anchor = None
        assert _extract_layout_text("Hello", layout_no_anchor) == ""

        layout_empty_segs = MagicMock()
        layout_empty_segs.text_anchor.text_segments = []
        assert _extract_layout_text("Hello", layout_empty_segs) == ""


# ── _build_image_chunks ───────────────────────────────────────────────────────


class TestBuildImageChunks:
    def test_empty_blocks_returns_empty_list(self):
        """No text blocks → no chunks."""
        with patch("app.processors.image_processor.settings") as mock_settings:
            mock_settings.chunk_max_tokens = 512
            mock_settings.chunk_overlap_tokens = 64
            chunks = _build_image_chunks([], MATERIAL_ID, COURSE_ID)
        assert chunks == []

    def test_whitespace_only_blocks_filtered(self):
        """Blocks containing only whitespace are excluded from segments."""
        with patch("app.processors.image_processor.settings") as mock_settings, \
             patch("app.processors.image_processor.flush_segments", return_value=[]) as mock_flush:
            mock_settings.chunk_max_tokens = 512
            mock_settings.chunk_overlap_tokens = 64
            _build_image_chunks(["  ", "\t", "real text"], MATERIAL_ID, COURSE_ID)

        call_segs = mock_flush.call_args[0][0]
        assert len(call_segs) == 1
        assert call_segs[0]["text"] == "real text"

    def test_segments_have_correct_shape(self):
        """Each segment has the expected keys with None hierarchy fields."""
        with patch("app.processors.image_processor.settings") as mock_settings, \
             patch("app.processors.image_processor.flush_segments", return_value=[]) as mock_flush:
            mock_settings.chunk_max_tokens = 512
            mock_settings.chunk_overlap_tokens = 64
            _build_image_chunks(["Block one", "Block two"], MATERIAL_ID, COURSE_ID)

        segs = mock_flush.call_args[0][0]
        for seg in segs:
            assert seg["chapter"] is None
            assert seg["section"] is None
            assert seg["page"] is None
            assert seg["content_type"] == "text"

    def test_max_chars_passed_correctly(self):
        """max_chars and overlap_chars are derived from settings and passed to flush_segments."""
        with patch("app.processors.image_processor.settings") as mock_settings, \
             patch("app.processors.image_processor.flush_segments", return_value=[]) as mock_flush:
            mock_settings.chunk_max_tokens = 256
            mock_settings.chunk_overlap_tokens = 32
            _build_image_chunks(["text"], MATERIAL_ID, COURSE_ID)

        _, _, _, max_c, overlap_c = mock_flush.call_args[0]
        assert max_c == 256 * 4
        assert overlap_c == 32 * 4


# ── _ocr_with_document_ai ─────────────────────────────────────────────────────


class TestOcrWithDocumentAi:
    def _make_paragraph(self, full_text, start, end):
        """Helper: build a mock paragraph layout element."""
        seg = MagicMock()
        seg.start_index = start
        seg.end_index = end

        layout = MagicMock()
        layout.text_anchor.text_segments = [seg]

        para = MagicMock()
        para.layout = layout
        return para

    def test_paragraphs_extracted_in_order(self, tmp_path):
        """Paragraph-level text blocks are returned in page order."""
        from app.processors.image_processor import _ocr_with_document_ai

        full_text = "First paragraph. Second paragraph."
        para1 = self._make_paragraph(full_text, 0, 17)
        para2 = self._make_paragraph(full_text, 17, 34)

        page = MagicMock()
        page.paragraphs = [para1, para2]

        document = MagicMock()
        document.text = full_text
        document.pages = [page]

        mock_response = MagicMock()
        mock_response.document = document
        mock_client = MagicMock()
        mock_client.process_document.return_value = mock_response

        mock_dai = MagicMock()
        mock_dai.DocumentProcessorServiceClient.return_value = mock_client
        mock_dai.ProcessRequest.return_value = MagicMock()
        mock_dai.RawDocument.return_value = MagicMock()

        image_file = tmp_path / "image.jpg"
        image_file.write_bytes(b"\xff\xd8\xff")  # minimal JPEG header

        mock_google_cloud = MagicMock()
        mock_google_cloud.documentai = mock_dai
        with patch("app.processors.image_processor.settings") as mock_settings, \
             patch.dict("sys.modules", {"google.cloud": mock_google_cloud, "google.cloud.documentai": mock_dai}):
            mock_settings.document_ai_processor_name = "projects/p/locations/l/processors/id"
            blocks = _ocr_with_document_ai(image_file, "image/jpeg")

        assert blocks == ["First paragraph.", "Second paragraph."]

    def test_empty_document_text_returns_empty(self, tmp_path):
        """Document with no text returns an empty list."""
        from app.processors.image_processor import _ocr_with_document_ai

        document = MagicMock()
        document.text = ""
        document.pages = []

        mock_response = MagicMock()
        mock_response.document = document
        mock_client = MagicMock()
        mock_client.process_document.return_value = mock_response

        mock_dai = MagicMock()
        mock_dai.DocumentProcessorServiceClient.return_value = mock_client
        mock_dai.ProcessRequest.return_value = MagicMock()
        mock_dai.RawDocument.return_value = MagicMock()

        image_file = tmp_path / "blank.jpg"
        image_file.write_bytes(b"\xff\xd8\xff")

        mock_google_cloud = MagicMock()
        mock_google_cloud.documentai = mock_dai
        with patch("app.processors.image_processor.settings") as mock_settings, \
             patch.dict("sys.modules", {"google.cloud": mock_google_cloud, "google.cloud.documentai": mock_dai}):
            mock_settings.document_ai_processor_name = "projects/p/locations/l/processors/id"
            blocks = _ocr_with_document_ai(image_file, "image/jpeg")

        assert blocks == []

    def test_fallback_to_full_text_when_no_paragraphs(self, tmp_path):
        """When pages have no paragraphs, full document text is returned as one block."""
        from app.processors.image_processor import _ocr_with_document_ai

        full_text = "  Some raw OCR text.  "
        page = MagicMock()
        page.paragraphs = []

        document = MagicMock()
        document.text = full_text
        document.pages = [page]

        mock_response = MagicMock()
        mock_response.document = document
        mock_client = MagicMock()
        mock_client.process_document.return_value = mock_response

        mock_dai = MagicMock()
        mock_dai.DocumentProcessorServiceClient.return_value = mock_client
        mock_dai.ProcessRequest.return_value = MagicMock()
        mock_dai.RawDocument.return_value = MagicMock()

        image_file = tmp_path / "image.png"
        image_file.write_bytes(b"\x89PNG")

        mock_google_cloud = MagicMock()
        mock_google_cloud.documentai = mock_dai
        with patch("app.processors.image_processor.settings") as mock_settings, \
             patch.dict("sys.modules", {"google.cloud": mock_google_cloud, "google.cloud.documentai": mock_dai}):
            mock_settings.document_ai_processor_name = "projects/p/locations/l/processors/id"
            blocks = _ocr_with_document_ai(image_file, "image/png")

        assert blocks == ["Some raw OCR text."]


# ── process_image pipeline ────────────────────────────────────────────────────


class TestProcessImagePipeline:
    @pytest.mark.asyncio
    async def test_happy_path_jpeg(self, tmp_path):
        """Full pipeline: downloads, OCRs, chunks, embeds, returns complete status."""
        local_file = tmp_path / "image.jpg"
        local_file.write_bytes(b"\xff\xd8\xff")

        with patch("app.processors.image_processor.db") as mock_db, \
             patch("app.processors.image_processor.download_from_gcs", return_value=local_file), \
             patch("app.processors.image_processor.embed_and_store", new_callable=AsyncMock, return_value=2), \
             patch("app.processors.image_processor.settings") as mock_settings, \
             patch("app.processors.image_processor._ocr_with_document_ai", return_value=["Block one.", "Block two."]):
            mock_db.update_material_status = AsyncMock()
            mock_settings.document_ai_processor_name = "projects/p/locations/l/processors/id"
            mock_settings.chunk_max_tokens = 512
            mock_settings.chunk_overlap_tokens = 64

            job = _make_job(mime="image/jpeg")
            result = await process_image(job)

        assert result["status"] == "complete"
        assert result["chunk_count"] == 2
        assert result["processor"] == "image"

    @pytest.mark.asyncio
    async def test_missing_processor_name_raises_value_error(self):
        """Unset document_ai_processor_name raises ValueError before any GCS call."""
        with patch("app.processors.image_processor.db") as mock_db, \
             patch("app.processors.image_processor.download_from_gcs") as mock_dl, \
             patch("app.processors.image_processor.settings") as mock_settings:
            mock_db.update_material_status = AsyncMock()
            mock_settings.document_ai_processor_name = ""

            job = _make_job()
            with pytest.raises(ValueError, match="document_ai_processor_name must be set"):
                await process_image(job)

        mock_dl.assert_not_called()

    @pytest.mark.asyncio
    async def test_exception_marks_failed_and_cleans_up(self, tmp_path):
        """Exception during OCR marks material FAILED and removes temp file."""
        local_file = tmp_path / "image.jpg"
        local_file.write_bytes(b"\xff\xd8\xff")

        with patch("app.processors.image_processor.db") as mock_db, \
             patch("app.processors.image_processor.download_from_gcs", return_value=local_file), \
             patch("app.processors.image_processor.settings") as mock_settings, \
             patch("app.processors.image_processor._ocr_with_document_ai", side_effect=RuntimeError("API error")):
            mock_db.update_material_status = AsyncMock()
            mock_settings.document_ai_processor_name = "projects/p/locations/l/processors/id"
            mock_settings.chunk_max_tokens = 512
            mock_settings.chunk_overlap_tokens = 64

            job = _make_job()
            with pytest.raises(RuntimeError, match="API error"):
                await process_image(job)

        mock_db.update_material_status.assert_any_call(MATERIAL_ID, ProcessingStatus.FAILED)
        assert not local_file.exists()

    @pytest.mark.asyncio
    async def test_empty_ocr_result_stores_zero_chunks(self, tmp_path):
        """Empty OCR output calls embed_and_store with an empty list → 0 chunks."""
        local_file = tmp_path / "image.jpg"
        local_file.write_bytes(b"\xff\xd8\xff")

        with patch("app.processors.image_processor.db") as mock_db, \
             patch("app.processors.image_processor.download_from_gcs", return_value=local_file), \
             patch("app.processors.image_processor.embed_and_store", new_callable=AsyncMock, return_value=0) as mock_store, \
             patch("app.processors.image_processor.settings") as mock_settings, \
             patch("app.processors.image_processor._ocr_with_document_ai", return_value=[]):
            mock_db.update_material_status = AsyncMock()
            mock_settings.document_ai_processor_name = "projects/p/locations/l/processors/id"
            mock_settings.chunk_max_tokens = 512
            mock_settings.chunk_overlap_tokens = 64

            job = _make_job()
            result = await process_image(job)

        assert result["chunk_count"] == 0
        mock_store.assert_called_once()
        stored_chunks = mock_store.call_args[0][0]
        assert stored_chunks == []

    @pytest.mark.asyncio
    async def test_png_mime_type_passed_to_dai(self, tmp_path):
        """PNG MIME type is correctly forwarded to Document AI."""
        local_file = tmp_path / "image.png"
        local_file.write_bytes(b"\x89PNG")

        with patch("app.processors.image_processor.db") as mock_db, \
             patch("app.processors.image_processor.download_from_gcs", return_value=local_file), \
             patch("app.processors.image_processor.embed_and_store", new_callable=AsyncMock, return_value=1), \
             patch("app.processors.image_processor.settings") as mock_settings, \
             patch("app.processors.image_processor._ocr_with_document_ai", return_value=["Text."]) as mock_ocr:
            mock_db.update_material_status = AsyncMock()
            mock_settings.document_ai_processor_name = "projects/p/locations/l/processors/id"
            mock_settings.chunk_max_tokens = 512
            mock_settings.chunk_overlap_tokens = 64

            job = _make_job(mime="image/png")
            await process_image(job)

        _, called_mime = mock_ocr.call_args[0]
        assert called_mime == "image/png"
