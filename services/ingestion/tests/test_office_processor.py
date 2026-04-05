"""Tests for Office document processing pipeline: DOCX, PPTX, XLSX."""
import io
import uuid
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from app.models import IngestJobMessage, ProcessingStatus
from app.processors.office_processor import (
    _cell_to_str,
    _docx_table_to_markdown,
    _extract_docx_segments,
    _extract_pptx_segments,
    _extract_xlsx_segments,
    _rows_to_markdown,
    _seg,
    process_office,
)


def _make_job(mime_type: str, filename: str = "test.docx") -> IngestJobMessage:
    """Build a minimal IngestJobMessage for office processor tests."""
    return IngestJobMessage(
        job_id=uuid.uuid4(),
        user_id=uuid.uuid4(),
        course_id=uuid.uuid4(),
        material_id=uuid.uuid4(),
        gcs_path=f"gs://tvtutor-raw-uploads/u/c/j/{filename}",
        mime_type=mime_type,
        filename=filename,
    )


# ── Markdown table helpers ────────────────────────────────────────────────────


class TestRowsToMarkdown:
    def test_empty_returns_empty(self):
        """Empty row list returns empty string."""
        assert _rows_to_markdown([]) == ""

    def test_header_only(self):
        """Single-row table produces header + separator, no body."""
        result = _rows_to_markdown([["Name", "Score"]])
        assert "| Name | Score |" in result
        assert "| --- | --- |" in result

    def test_header_and_rows(self):
        """Multi-row table produces header, separator, and body rows."""
        rows = [["A", "B"], ["1", "2"], ["3", "4"]]
        result = _rows_to_markdown(rows)
        assert "| A | B |" in result
        assert "| --- | --- |" in result
        assert "| 1 | 2 |" in result
        assert "| 3 | 4 |" in result

    def test_unequal_row_lengths_padded(self):
        """Rows shorter than the widest row are padded with empty cells."""
        rows = [["A", "B", "C"], ["1"]]
        result = _rows_to_markdown(rows)
        assert result.count("|") > 0
        # Row 2 should be padded to 3 columns
        assert "| 1 |  |  |" in result


class TestCellToStr:
    def test_none_returns_empty(self):
        """None cell value returns empty string."""
        cell = MagicMock()
        cell.value = None
        assert _cell_to_str(cell) == ""

    def test_string_value(self):
        """String cell value is returned as-is (stripped)."""
        cell = MagicMock()
        cell.value = "  hello  "
        assert _cell_to_str(cell) == "hello"

    def test_numeric_value(self):
        """Numeric cell value is stringified."""
        cell = MagicMock()
        cell.value = 42
        assert _cell_to_str(cell) == "42"


# ── Segment builder ───────────────────────────────────────────────────────────


class TestSeg:
    def test_all_fields(self):
        """_seg builds a dict with all required keys."""
        s = _seg("text", chapter="Ch1", section="S1", page=2, content_type="table")
        assert s["text"] == "text"
        assert s["chapter"] == "Ch1"
        assert s["section"] == "S1"
        assert s["page"] == 2
        assert s["content_type"] == "table"
        assert s["metadata"] == {}

    def test_metadata_forwarded(self):
        """Custom metadata is stored under 'metadata' key."""
        s = _seg("text", chapter=None, section=None, page=None,
                 content_type="text", metadata={"notes": True})
        assert s["metadata"] == {"notes": True}


# ── DOCX extraction ───────────────────────────────────────────────────────────


def _build_docx(paragraphs: list[tuple[str, str]]) -> Path:
    """Create a minimal DOCX in memory and write to a temp file.

    Args:
        paragraphs: List of (text, style_name) tuples.

    Returns:
        Path to the created temp file.
    """
    from docx import Document

    doc = Document()
    for text, style in paragraphs:
        para = doc.add_paragraph(text, style=style)
    buf = io.BytesIO()
    doc.save(buf)
    buf.seek(0)

    import tempfile
    tmp = tempfile.NamedTemporaryFile(delete=False, suffix=".docx")
    tmp.write(buf.read())
    tmp.close()
    return Path(tmp.name)


class TestExtractDocxSegments:
    def test_plain_paragraph(self):
        """Plain body text becomes a single 'text' segment."""
        path = _build_docx([("Hello world", "Normal")])
        try:
            segments = _extract_docx_segments(path)
            assert len(segments) == 1
            assert segments[0]["text"] == "Hello world"
            assert segments[0]["content_type"] == "text"
        finally:
            path.unlink(missing_ok=True)

    def test_heading1_sets_chapter(self):
        """Heading 1 sets 'chapter' on subsequent paragraphs."""
        path = _build_docx([
            ("Introduction", "Heading 1"),
            ("First paragraph.", "Normal"),
        ])
        try:
            segments = _extract_docx_segments(path)
            assert len(segments) == 1
            assert segments[0]["chapter"] == "Introduction"
        finally:
            path.unlink(missing_ok=True)

    def test_heading2_sets_section(self):
        """Heading 2 sets 'section' on subsequent paragraphs."""
        path = _build_docx([
            ("Chapter One", "Heading 1"),
            ("Section A", "Heading 2"),
            ("Body text.", "Normal"),
        ])
        try:
            segments = _extract_docx_segments(path)
            assert len(segments) == 1
            assert segments[0]["chapter"] == "Chapter One"
            assert segments[0]["section"] == "Section A"
        finally:
            path.unlink(missing_ok=True)

    def test_empty_paragraphs_skipped(self):
        """Empty paragraphs produce no segments."""
        path = _build_docx([("", "Normal"), ("Real text", "Normal")])
        try:
            segments = _extract_docx_segments(path)
            texts = [s["text"] for s in segments]
            assert "" not in texts
            assert "Real text" in texts
        finally:
            path.unlink(missing_ok=True)


# ── DOCX table → Markdown ─────────────────────────────────────────────────────


class TestDocxTableToMarkdown:
    def _build_table(self, rows: list[list[str]]):
        """Build a mock python-docx Table from a list of string rows."""
        table = MagicMock()
        mock_rows = []
        for row_data in rows:
            mock_row = MagicMock()
            mock_row.cells = [MagicMock(text=cell) for cell in row_data]
            mock_rows.append(mock_row)
        table.rows = mock_rows
        return table

    def test_basic_table(self):
        """Two-column table is converted to Markdown with header and separator."""
        table = self._build_table([["Col1", "Col2"], ["A", "B"]])
        md = _docx_table_to_markdown(table)
        assert "| Col1 | Col2 |" in md
        assert "| --- | --- |" in md
        assert "| A | B |" in md

    def test_empty_table_returns_empty(self):
        """Table with no rows returns empty string."""
        table = MagicMock()
        table.rows = []
        assert _docx_table_to_markdown(table) == ""


# ── PPTX extraction ───────────────────────────────────────────────────────────


def _build_pptx(slides: list[dict]) -> Path:
    """Create a minimal PPTX in memory.

    Args:
        slides: List of dicts with optional 'title', 'body' (list of str), 'notes'.

    Returns:
        Path to the created temp file.
    """
    from pptx import Presentation
    from pptx.util import Inches

    prs = Presentation()
    blank_layout = prs.slide_layouts[6]  # blank layout

    for slide_data in slides:
        slide = prs.slides.add_slide(blank_layout)

        if slide_data.get("title"):
            txBox = slide.shapes.add_textbox(Inches(0), Inches(0), Inches(8), Inches(1))
            tf = txBox.text_frame
            tf.text = slide_data["title"]
            # Mark the first shape as title by monkeypatching
            slide.shapes._spTree = slide.shapes._spTree  # no-op; title detection uses shapes.title

        for body_text in slide_data.get("body", []):
            txBox = slide.shapes.add_textbox(Inches(0), Inches(1.5), Inches(8), Inches(1))
            txBox.text_frame.text = body_text

        if slide_data.get("notes"):
            slide.notes_slide.notes_text_frame.text = slide_data["notes"]

    buf = io.BytesIO()
    prs.save(buf)
    buf.seek(0)

    import tempfile
    tmp = tempfile.NamedTemporaryFile(delete=False, suffix=".pptx")
    tmp.write(buf.read())
    tmp.close()
    return Path(tmp.name)


class TestExtractPptxSegments:
    def test_slide_with_body(self):
        """Slide body text appears as a segment with the slide number as page."""
        path = _build_pptx([{"body": ["Slide body content"]}])
        try:
            segments = _extract_pptx_segments(path)
            texts = [s["text"] for s in segments]
            assert any("Slide body content" in t for t in texts)
            assert all(s["page"] == 1 for s in segments)
        finally:
            path.unlink(missing_ok=True)

    def test_speaker_notes_tagged(self):
        """Speaker notes appear as a segment tagged with [Speaker Notes]."""
        path = _build_pptx([{"body": ["Content"], "notes": "This is a note"}])
        try:
            segments = _extract_pptx_segments(path)
            notes_segs = [s for s in segments if "[Speaker Notes]" in s["text"]]
            assert len(notes_segs) == 1
            assert "This is a note" in notes_segs[0]["text"]
            assert notes_segs[0]["metadata"].get("notes") is True
        finally:
            path.unlink(missing_ok=True)

    def test_multiple_slides_have_distinct_page_numbers(self):
        """Each slide's segments carry the correct slide number as 'page'."""
        path = _build_pptx([
            {"body": ["Slide 1 content"]},
            {"body": ["Slide 2 content"]},
        ])
        try:
            segments = _extract_pptx_segments(path)
            pages = sorted({s["page"] for s in segments})
            assert 1 in pages
            assert 2 in pages
        finally:
            path.unlink(missing_ok=True)

    def test_empty_presentation_produces_no_segments(self):
        """An empty PPTX produces no segments."""
        path = _build_pptx([])
        try:
            segments = _extract_pptx_segments(path)
            assert segments == []
        finally:
            path.unlink(missing_ok=True)


# ── XLSX extraction ───────────────────────────────────────────────────────────


def _build_xlsx(sheets: dict[str, list[list]]) -> Path:
    """Create a minimal XLSX in memory.

    Args:
        sheets: Dict mapping sheet name to list of rows (lists of cell values).

    Returns:
        Path to the created temp file.
    """
    import openpyxl

    wb = openpyxl.Workbook()
    first = True
    for sheet_name, rows in sheets.items():
        if first:
            ws = wb.active
            ws.title = sheet_name
            first = False
        else:
            ws = wb.create_sheet(sheet_name)
        for row in rows:
            ws.append(row)

    buf = io.BytesIO()
    wb.save(buf)
    buf.seek(0)

    import tempfile
    tmp = tempfile.NamedTemporaryFile(delete=False, suffix=".xlsx")
    tmp.write(buf.read())
    tmp.close()
    return Path(tmp.name)


class TestExtractXlsxSegments:
    def test_single_sheet_produces_one_segment(self):
        """A workbook with one non-empty sheet produces one table segment."""
        path = _build_xlsx({"Grades": [["Student", "Score"], ["Alice", "95"]]})
        try:
            segments = _extract_xlsx_segments(path)
            assert len(segments) == 1
            assert segments[0]["chapter"] == "Grades"
            assert segments[0]["content_type"] == "table"
            assert "Student" in segments[0]["text"]
            assert "Alice" in segments[0]["text"]
        finally:
            path.unlink(missing_ok=True)

    def test_multiple_sheets(self):
        """Each non-empty sheet produces one segment."""
        path = _build_xlsx({
            "Sheet1": [["A", "B"], ["1", "2"]],
            "Sheet2": [["X", "Y"], ["3", "4"]],
        })
        try:
            segments = _extract_xlsx_segments(path)
            assert len(segments) == 2
            chapters = {s["chapter"] for s in segments}
            assert "Sheet1" in chapters
            assert "Sheet2" in chapters
        finally:
            path.unlink(missing_ok=True)

    def test_empty_sheet_skipped(self):
        """Sheets with no data produce no segments."""
        path = _build_xlsx({"EmptySheet": []})
        try:
            segments = _extract_xlsx_segments(path)
            assert segments == []
        finally:
            path.unlink(missing_ok=True)

    def test_all_none_rows_skipped(self):
        """Rows where every cell is None are omitted from the segment."""
        path = _build_xlsx({"Data": [["A", "B"], [None, None], ["1", "2"]]})
        try:
            segments = _extract_xlsx_segments(path)
            assert len(segments) == 1
            # The None row should not appear in the markdown
            assert "None" not in segments[0]["text"]
        finally:
            path.unlink(missing_ok=True)


# ── Full pipeline (mocked externals) ─────────────────────────────────────────


DOCX_MIME = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
PPTX_MIME = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
XLSX_MIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"


class TestProcessOfficePipeline:
    @pytest.mark.asyncio
    @patch("app.processors.office_processor.qdrant")
    @patch("app.processors.office_processor.embeddings")
    @patch("app.processors.office_processor.db")
    @patch("app.processors.office_processor.gcs")
    async def test_docx_happy_path(self, mock_gcs, mock_db, mock_embed, mock_qdrant):
        """DOCX file is processed end-to-end and returns complete status."""
        job = _make_job(DOCX_MIME, "essay.docx")

        path = _build_docx([("Introduction", "Heading 1"), ("This is the body.", "Normal")])
        mock_gcs.download_file = AsyncMock(return_value=path)
        mock_embed.embed_texts = AsyncMock(return_value=[[0.1] * 1024])
        mock_db.update_material_status = AsyncMock()
        mock_db.insert_chunks = AsyncMock()
        mock_qdrant.upsert_chunks = AsyncMock()

        result = await process_office(job)

        assert result["status"] == "complete"
        assert result["processor"] == "office"
        assert result["chunk_count"] >= 1
        mock_qdrant.upsert_chunks.assert_awaited_once()
        mock_db.insert_chunks.assert_awaited_once()

        # Verify status transitions: PROCESSING → COMPLETE
        calls = mock_db.update_material_status.call_args_list
        assert calls[0].args == (job.material_id, ProcessingStatus.PROCESSING)
        assert calls[-1].args[1] == ProcessingStatus.COMPLETE

    @pytest.mark.asyncio
    @patch("app.processors.office_processor.qdrant")
    @patch("app.processors.office_processor.embeddings")
    @patch("app.processors.office_processor.db")
    @patch("app.processors.office_processor.gcs")
    async def test_pptx_happy_path(self, mock_gcs, mock_db, mock_embed, mock_qdrant):
        """PPTX file is processed end-to-end and returns complete status."""
        job = _make_job(PPTX_MIME, "lecture.pptx")

        path = _build_pptx([{"body": ["AI is transforming education."]}])
        mock_gcs.download_file = AsyncMock(return_value=path)
        mock_embed.embed_texts = AsyncMock(return_value=[[0.1] * 1024])
        mock_db.update_material_status = AsyncMock()
        mock_db.insert_chunks = AsyncMock()
        mock_qdrant.upsert_chunks = AsyncMock()

        result = await process_office(job)

        assert result["status"] == "complete"
        assert result["processor"] == "office"
        assert result["chunk_count"] >= 1

    @pytest.mark.asyncio
    @patch("app.processors.office_processor.qdrant")
    @patch("app.processors.office_processor.embeddings")
    @patch("app.processors.office_processor.db")
    @patch("app.processors.office_processor.gcs")
    async def test_xlsx_happy_path(self, mock_gcs, mock_db, mock_embed, mock_qdrant):
        """XLSX file is processed end-to-end and returns complete status."""
        job = _make_job(XLSX_MIME, "grades.xlsx")

        path = _build_xlsx({"Scores": [["Name", "Grade"], ["Bob", "88"]]})
        mock_gcs.download_file = AsyncMock(return_value=path)
        mock_embed.embed_texts = AsyncMock(return_value=[[0.1] * 1024])
        mock_db.update_material_status = AsyncMock()
        mock_db.insert_chunks = AsyncMock()
        mock_qdrant.upsert_chunks = AsyncMock()

        result = await process_office(job)

        assert result["status"] == "complete"
        assert result["processor"] == "office"

    @pytest.mark.asyncio
    @patch("app.processors.office_processor.db")
    @patch("app.processors.office_processor.gcs")
    async def test_gcs_failure_marks_material_failed(self, mock_gcs, mock_db):
        """GCS download failure marks the material as FAILED."""
        job = _make_job(DOCX_MIME)
        mock_gcs.download_file = AsyncMock(side_effect=Exception("GCS unavailable"))
        mock_db.update_material_status = AsyncMock()

        with pytest.raises(Exception, match="GCS unavailable"):
            await process_office(job)

        calls = mock_db.update_material_status.call_args_list
        assert calls[-1].args == (job.material_id, ProcessingStatus.FAILED)

    @pytest.mark.asyncio
    @patch("app.processors.office_processor.db")
    @patch("app.processors.office_processor.gcs")
    async def test_unsupported_mime_raises(self, mock_gcs, mock_db):
        """An unexpected MIME type (not in _MIME_TO_FORMAT) raises ValueError."""
        job = _make_job("application/pdf")  # PDF is not handled by office processor
        mock_db.update_material_status = AsyncMock()
        # We need to make GCS return something so the error comes from the handler
        import tempfile
        tmp = Path(tempfile.mktemp(suffix=".pdf"))
        tmp.write_bytes(b"fake")
        mock_gcs.download_file = AsyncMock(return_value=tmp)

        with pytest.raises(ValueError, match="unexpected mime type"):
            await process_office(job)
        tmp.unlink(missing_ok=True)
