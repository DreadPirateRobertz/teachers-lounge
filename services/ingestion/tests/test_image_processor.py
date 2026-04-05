"""Tests for image OCR processing pipeline using Google Document AI."""
import uuid
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from app.models import IngestJobMessage, ProcessingStatus
from app.processors.image_processor import (
    _average_confidence,
    _extract_layout_text,
    _extract_segments_from_document,
    _run_document_ai,
    process_image,
)


def _make_job(
    mime_type: str = "image/jpeg",
    filename: str = "scan.jpg",
) -> IngestJobMessage:
    """Build a minimal IngestJobMessage for image processor tests."""
    return IngestJobMessage(
        job_id=uuid.uuid4(),
        user_id=uuid.uuid4(),
        course_id=uuid.uuid4(),
        material_id=uuid.uuid4(),
        gcs_path=f"gs://tvtutor-raw-uploads/u/c/j/{filename}",
        mime_type=mime_type,
        filename=filename,
    )


# ── Layout text extraction ────────────────────────────────────────────────────


class TestExtractLayoutText:
    def test_basic_extraction(self):
        """Text is extracted using start/end byte offsets from text_anchor."""
        full_text = "Hello world, this is a test."
        layout = MagicMock()
        seg = MagicMock()
        seg.start_index = 0
        seg.end_index = 5
        layout.text_anchor.text_segments = [seg]
        assert _extract_layout_text(full_text, layout) == "Hello"

    def test_multiple_segments(self):
        """Multiple text_segments are concatenated."""
        full_text = "Hello world"
        layout = MagicMock()
        seg1 = MagicMock()
        seg1.start_index = 0
        seg1.end_index = 5
        seg2 = MagicMock()
        seg2.start_index = 5
        seg2.end_index = 11
        layout.text_anchor.text_segments = [seg1, seg2]
        assert _extract_layout_text(full_text, layout) == "Hello world"

    def test_no_layout_returns_empty(self):
        """None layout returns empty string."""
        assert _extract_layout_text("text", None) == ""

    def test_no_text_anchor_returns_empty(self):
        """Layout with no text_anchor returns empty string."""
        layout = MagicMock()
        layout.text_anchor = None
        assert _extract_layout_text("text", layout) == ""


# ── Average confidence ────────────────────────────────────────────────────────


class TestAverageConfidence:
    def test_empty_segments_returns_one(self):
        """Empty segment list returns full confidence (1.0)."""
        assert _average_confidence([]) == 1.0

    def test_mixed_confidence(self):
        """Mean confidence is computed correctly across segments."""
        segments = [
            {"metadata": {"ocr_confidence": 0.8}},
            {"metadata": {"ocr_confidence": 0.6}},
        ]
        assert _average_confidence(segments) == pytest.approx(0.7)

    def test_missing_metadata_defaults_to_one(self):
        """Segments without ocr_confidence default to 1.0."""
        segments = [{"metadata": {}}, {"metadata": {"ocr_confidence": 0.5}}]
        assert _average_confidence(segments) == pytest.approx(0.75)


# ── Document AI segment extraction ───────────────────────────────────────────


def _make_doc_ai_document(blocks: list[dict]) -> MagicMock:
    """Build a mock Document AI document with the given blocks.

    Args:
        blocks: List of dicts with 'text' and 'confidence' keys.

    Returns:
        Mock Document AI document.
    """
    full_text = " ".join(b["text"] for b in blocks)
    doc = MagicMock()
    doc.text = full_text

    pages = []
    page = MagicMock()
    page_blocks = []
    offset = 0
    for block_data in blocks:
        block = MagicMock()
        block.layout = MagicMock()
        block.layout.confidence = block_data["confidence"]
        seg = MagicMock()
        seg.start_index = offset
        seg.end_index = offset + len(block_data["text"])
        block.layout.text_anchor.text_segments = [seg]
        page_blocks.append(block)
        offset += len(block_data["text"]) + 1  # +1 for space

    page.blocks = page_blocks
    pages.append(page)
    doc.pages = pages
    return doc


class TestExtractSegmentsFromDocument:
    def test_basic_extraction(self):
        """Text blocks with high confidence produce segments without low_confidence flag."""
        doc = _make_doc_ai_document([
            {"text": "This is clear printed text.", "confidence": 0.95},
        ])
        segments = _extract_segments_from_document(doc)
        assert len(segments) == 1
        assert "clear printed text" in segments[0]["text"]
        assert segments[0]["metadata"]["low_confidence"] is False
        assert segments[0]["page"] == 1

    def test_low_confidence_flagged(self):
        """Blocks with confidence below threshold are flagged low_confidence=True."""
        doc = _make_doc_ai_document([
            {"text": "Hard to read handwriting.", "confidence": 0.4},
        ])
        segments = _extract_segments_from_document(doc, threshold=0.7)
        assert segments[0]["metadata"]["low_confidence"] is True

    def test_empty_text_blocks_skipped(self):
        """Blocks with no text after stripping are skipped."""
        doc = MagicMock()
        doc.text = "   "
        page = MagicMock()
        block = MagicMock()
        block.layout = MagicMock()
        block.layout.confidence = 0.9
        seg = MagicMock()
        seg.start_index = 0
        seg.end_index = 3
        block.layout.text_anchor.text_segments = [seg]
        page.blocks = [block]
        doc.pages = [page]

        segments = _extract_segments_from_document(doc)
        assert segments == []


# ── Full pipeline (mocked externals) ─────────────────────────────────────────


class TestProcessImagePipeline:
    @pytest.mark.asyncio
    @patch("app.processors.image_processor.qdrant")
    @patch("app.processors.image_processor.embeddings")
    @patch("app.processors.image_processor.db")
    @patch("app.processors.image_processor.gcs")
    @patch("app.processors.image_processor._run_document_ai")
    async def test_happy_path_jpeg(
        self, mock_docai, mock_gcs, mock_db, mock_embed, mock_qdrant, tmp_path
    ):
        """JPEG job processes through Document AI and writes chunks to Qdrant + DB."""
        job = _make_job("image/jpeg", "notes.jpg")

        fake_image = tmp_path / "notes.jpg"
        fake_image.write_bytes(b"\xff\xd8\xff")  # minimal JPEG header
        mock_gcs.download_file = AsyncMock(return_value=fake_image)

        mock_docai.return_value = [
            {
                "text": "Handwritten equation: E = mc²",
                "chapter": None,
                "section": None,
                "page": 1,
                "content_type": "text",
                "metadata": {"ocr_confidence": 0.88, "low_confidence": False},
            }
        ]
        mock_embed.embed_texts = AsyncMock(return_value=[[0.1] * 1024])
        mock_db.update_material_status = AsyncMock()
        mock_db.insert_chunks = AsyncMock()
        mock_qdrant.upsert_chunks = AsyncMock()

        with patch("app.processors.image_processor.settings") as mock_settings:
            mock_settings.document_ai_ocr_processor_id = "test-ocr-id"
            mock_settings.document_ai_form_processor_id = ""
            mock_settings.document_ai_low_confidence_threshold = 0.7
            mock_settings.document_ai_location = "us"
            mock_settings.gcp_project = "test-project"
            mock_settings.chunk_max_tokens = 512
            mock_settings.chunk_overlap_tokens = 64
            mock_settings.curriculum_collection = "curriculum"

            result = await process_image(job)

        assert result["status"] == "complete"
        assert result["processor"] == "image"
        assert result["chunk_count"] >= 1

        calls = mock_db.update_material_status.call_args_list
        assert calls[0].args == (job.material_id, ProcessingStatus.PROCESSING)
        assert calls[-1].args[1] == ProcessingStatus.COMPLETE

        # Verify low_confidence is forwarded to Qdrant payload
        qdrant_payloads = mock_qdrant.upsert_chunks.call_args.args[2]
        assert "low_confidence" in qdrant_payloads[0]

    @pytest.mark.asyncio
    @patch("app.processors.image_processor.qdrant")
    @patch("app.processors.image_processor.embeddings")
    @patch("app.processors.image_processor.db")
    @patch("app.processors.image_processor.gcs")
    @patch("app.processors.image_processor._run_document_ai")
    async def test_low_confidence_triggers_form_parser_retry(
        self, mock_docai, mock_gcs, mock_db, mock_embed, mock_qdrant, tmp_path
    ):
        """Low-confidence OCR with form processor configured causes retry."""
        job = _make_job("image/jpeg", "handwritten.jpg")

        fake_image = tmp_path / "hw.jpg"
        fake_image.write_bytes(b"\xff\xd8\xff")
        mock_gcs.download_file = AsyncMock(return_value=fake_image)

        # First call (OCR): low confidence. Second call (Form Parser): better result.
        low_conf_seg = {
            "text": "Messy scrawl",
            "chapter": None, "section": None, "page": 1, "content_type": "text",
            "metadata": {"ocr_confidence": 0.3, "low_confidence": True},
        }
        good_seg = {
            "text": "F = ma (Newton second law)",
            "chapter": None, "section": None, "page": 1, "content_type": "text",
            "metadata": {"ocr_confidence": 0.85, "low_confidence": False},
        }
        mock_docai.side_effect = [[low_conf_seg], [good_seg]]
        mock_embed.embed_texts = AsyncMock(return_value=[[0.1] * 1024])
        mock_db.update_material_status = AsyncMock()
        mock_db.insert_chunks = AsyncMock()
        mock_qdrant.upsert_chunks = AsyncMock()

        with patch("app.processors.image_processor.settings") as mock_settings:
            mock_settings.document_ai_ocr_processor_id = "ocr-id"
            mock_settings.document_ai_form_processor_id = "form-id"
            mock_settings.document_ai_low_confidence_threshold = 0.7
            mock_settings.document_ai_location = "us"
            mock_settings.gcp_project = "test-project"
            mock_settings.chunk_max_tokens = 512
            mock_settings.chunk_overlap_tokens = 64
            mock_settings.curriculum_collection = "curriculum"

            result = await process_image(job)

        assert result["status"] == "complete"
        # Both OCR and Form Parser calls should have been made
        assert mock_docai.call_count == 2
        # The second call should use the form processor id
        second_call_args = mock_docai.call_args_list[1].args
        assert second_call_args[2] == "form-id"

    @pytest.mark.asyncio
    @patch("app.processors.image_processor.db")
    @patch("app.processors.image_processor.gcs")
    async def test_missing_processor_id_raises(self, mock_gcs, mock_db, tmp_path):
        """Missing document_ai_ocr_processor_id raises RuntimeError."""
        job = _make_job()
        fake_image = tmp_path / "img.jpg"
        fake_image.write_bytes(b"\xff\xd8\xff")
        mock_gcs.download_file = AsyncMock(return_value=fake_image)
        mock_db.update_material_status = AsyncMock()

        with patch("app.processors.image_processor.settings") as mock_settings:
            mock_settings.document_ai_ocr_processor_id = ""
            mock_settings.document_ai_form_processor_id = ""

            with pytest.raises(RuntimeError, match="document_ai_ocr_processor_id"):
                await process_image(job)

        calls = mock_db.update_material_status.call_args_list
        assert calls[-1].args == (job.material_id, ProcessingStatus.FAILED)

    @pytest.mark.asyncio
    @patch("app.processors.image_processor.db")
    @patch("app.processors.image_processor.gcs")
    async def test_gcs_failure_marks_failed(self, mock_gcs, mock_db):
        """GCS download failure marks the material as FAILED."""
        job = _make_job()
        mock_gcs.download_file = AsyncMock(side_effect=Exception("GCS error"))
        mock_db.update_material_status = AsyncMock()

        with pytest.raises(Exception, match="GCS error"):
            await process_image(job)

        calls = mock_db.update_material_status.call_args_list
        assert calls[-1].args == (job.material_id, ProcessingStatus.FAILED)

    @pytest.mark.asyncio
    @patch("app.processors.image_processor.db")
    @patch("app.processors.image_processor.gcs")
    @patch("app.processors.image_processor._run_document_ai")
    async def test_empty_ocr_result_completes_zero_chunks(
        self, mock_docai, mock_gcs, mock_db, tmp_path
    ):
        """Document AI returning no segments completes with chunk_count=0."""
        job = _make_job()
        fake_image = tmp_path / "blank.jpg"
        fake_image.write_bytes(b"\xff\xd8\xff")
        mock_gcs.download_file = AsyncMock(return_value=fake_image)
        mock_docai.return_value = []
        mock_db.update_material_status = AsyncMock()

        with patch("app.processors.image_processor.settings") as mock_settings:
            mock_settings.document_ai_ocr_processor_id = "ocr-id"
            mock_settings.document_ai_form_processor_id = ""
            mock_settings.document_ai_low_confidence_threshold = 0.7
            mock_settings.document_ai_location = "us"
            mock_settings.gcp_project = "test-project"
            mock_settings.chunk_max_tokens = 512
            mock_settings.chunk_overlap_tokens = 64

            result = await process_image(job)

        assert result["status"] == "complete"
        assert result["chunk_count"] == 0
        calls = mock_db.update_material_status.call_args_list
        assert calls[-1].args[1] == ProcessingStatus.COMPLETE
