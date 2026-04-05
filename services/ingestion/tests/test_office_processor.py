"""Tests for services/ingestion/app/processors/office_processor.py.

Covers shared helpers (flush_segments, _table_to_markdown) and all three
format extractors (DOCX, PPTX, XLSX) plus the top-level process_office
pipeline.  All external dependencies (GCS, DB, Qdrant, OpenAI, python-docx,
python-pptx, openpyxl) are mocked — no real services are required.
"""
from unittest.mock import AsyncMock, MagicMock, call, patch
from uuid import UUID, uuid4

import pytest

from app.models import ProcessingStatus
from app.processors.common import flush_segments, make_chunk
from app.processors.office_processor import (
    _extract_docx_chunks,
    _extract_pptx_chunks,
    _extract_xlsx_chunks,
    _table_to_markdown,
    process_office,
)

MATERIAL_ID = uuid4()
COURSE_ID = uuid4()
JOB_ID = uuid4()

DOCX_MIME = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
PPTX_MIME = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
XLSX_MIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"


def _make_job(mime=DOCX_MIME):
    job = MagicMock()
    job.job_id = JOB_ID
    job.material_id = MATERIAL_ID
    job.course_id = COURSE_ID
    job.gcs_path = "gs://bucket/doc.docx"
    job.filename = "doc.docx"
    job.mime_type = mime
    return job


# ── flush_segments (shared helper) ───────────────────────────────────────────


class TestFlushSegments:
    def test_empty_segments_returns_empty_list(self):
        """No segments → no chunks."""
        assert flush_segments([], MATERIAL_ID, COURSE_ID, 100, 20) == []

    def test_single_segment_produces_one_chunk(self):
        """A single segment that fits within max_chars becomes one chunk."""
        segs = [{"text": "Hello world", "chapter": "C1", "section": "S1",
                 "page": 1, "content_type": "text"}]
        chunks = flush_segments(segs, MATERIAL_ID, COURSE_ID, 100, 20)
        assert len(chunks) == 1
        assert chunks[0]["content"] == "Hello world"
        assert chunks[0]["chapter"] == "C1"

    def test_overflow_splits_into_multiple_chunks(self):
        """Segments exceeding max_chars are split into separate chunks."""
        segs = [
            {"text": "A" * 60, "chapter": None, "section": None, "page": None, "content_type": "text"},
            {"text": "B" * 60, "chapter": None, "section": None, "page": None, "content_type": "text"},
        ]
        chunks = flush_segments(segs, MATERIAL_ID, COURSE_ID, max_chars=80, overlap_chars=10)
        assert len(chunks) == 2

    def test_overlap_carries_tail_into_next_chunk(self):
        """The tail of the previous chunk's buffer appears at the start of the next."""
        tail = "overlap text"
        filler = "X" * 80
        # Put `tail` at the END of the first buffer so the overlap algorithm
        # (which takes the last segments that fit within overlap_chars) picks it up.
        segs = [
            {"text": filler, "chapter": None, "section": None, "page": None, "content_type": "text"},
            {"text": tail, "chapter": None, "section": None, "page": None, "content_type": "text"},
            {"text": "new content", "chapter": None, "section": None, "page": None, "content_type": "text"},
        ]
        # max_chars large enough that filler+tail fit, but adding 'new content' overflows
        max_chars = len(filler) + len(tail) + 1
        chunks = flush_segments(segs, MATERIAL_ID, COURSE_ID, max_chars=max_chars, overlap_chars=len(tail) + 5)
        assert len(chunks) >= 2
        # The tail text should appear in the second chunk as overlap context
        assert tail in chunks[1]["content"]

    def test_chunk_ids_are_unique(self):
        """Each chunk produced gets a distinct UUID."""
        segs = [
            {"text": "X" * 60, "chapter": None, "section": None, "page": None, "content_type": "text"},
            {"text": "Y" * 60, "chapter": None, "section": None, "page": None, "content_type": "text"},
        ]
        chunks = flush_segments(segs, MATERIAL_ID, COURSE_ID, max_chars=70, overlap_chars=5)
        ids = [c["id"] for c in chunks]
        assert len(ids) == len(set(ids))


# ── _table_to_markdown ────────────────────────────────────────────────────────


class TestTableToMarkdown:
    def _make_table(self, rows_data: list[list[str]]):
        """Build a mock docx Table from a list of row data."""
        mock_rows = []
        for row_data in rows_data:
            cells = [MagicMock(text=cell) for cell in row_data]
            mock_row = MagicMock()
            mock_row.cells = cells
            mock_rows.append(mock_row)
        table = MagicMock()
        table.rows = mock_rows
        return table

    def test_two_row_table_produces_header_sep_body(self):
        """A two-row table produces header, separator, and one body row."""
        table = self._make_table([["Name", "Age"], ["Alice", "30"]])
        md = _table_to_markdown(table)
        lines = md.split("\n")
        assert lines[0] == "| Name | Age |"
        assert "---" in lines[1]
        assert lines[2] == "| Alice | 30 |"

    def test_empty_table_returns_empty_string(self):
        """A table with no rows returns an empty string."""
        table = MagicMock()
        table.rows = []
        assert _table_to_markdown(table) == ""

    def test_single_row_table_has_no_body(self):
        """A header-only table has no body lines."""
        table = self._make_table([["Col1", "Col2"]])
        md = _table_to_markdown(table)
        lines = md.split("\n")
        assert len(lines) == 2  # header + separator only


# ── _extract_docx_chunks ──────────────────────────────────────────────────────


class TestExtractDocxChunks:
    def _make_paragraph(self, text: str, style_name: str):
        para = MagicMock()
        para.text = text
        para.style.name = style_name
        return para

    def test_heading1_sets_chapter(self, tmp_path):
        """Heading 1 style sets the chapter for subsequent segments."""
        docx_file = tmp_path / "doc.docx"
        docx_file.write_bytes(b"")

        h1 = self._make_paragraph("Chapter One", "Heading 1")
        body = self._make_paragraph("Body text here.", "Normal")

        with patch("app.processors.office_processor.Document"), \
             patch("app.processors.office_processor._iter_block_items", return_value=[h1, body]), \
             patch("app.processors.office_processor.settings") as mock_settings:
            mock_settings.chunk_max_tokens = 512
            mock_settings.chunk_overlap_tokens = 64
            chunks = _extract_docx_chunks(docx_file, MATERIAL_ID, COURSE_ID)

        assert len(chunks) == 1
        assert chunks[0]["chapter"] == "Chapter One"
        assert "Body text here." in chunks[0]["content"]

    def test_heading2_sets_section(self, tmp_path):
        """Heading 2 style sets the section for subsequent segments."""
        docx_file = tmp_path / "doc.docx"
        docx_file.write_bytes(b"")

        h2 = self._make_paragraph("Section A", "Heading 2")
        body = self._make_paragraph("Section content.", "Normal")

        with patch("app.processors.office_processor.Document"), \
             patch("app.processors.office_processor._iter_block_items", return_value=[h2, body]), \
             patch("app.processors.office_processor.settings") as mock_settings:
            mock_settings.chunk_max_tokens = 512
            mock_settings.chunk_overlap_tokens = 64
            chunks = _extract_docx_chunks(docx_file, MATERIAL_ID, COURSE_ID)

        assert chunks[0]["section"] == "Section A"

    def test_table_block_produces_table_content_type(self, tmp_path):
        """A DocxTable block is converted to Markdown with content_type='table'."""
        from docx.table import Table as DocxTable

        docx_file = tmp_path / "doc.docx"
        docx_file.write_bytes(b"")

        mock_table = MagicMock(spec=DocxTable)

        with patch("app.processors.office_processor.Document"), \
             patch("app.processors.office_processor._iter_block_items", return_value=[mock_table]), \
             patch("app.processors.office_processor._table_to_markdown", return_value="| H1 |\n| --- |\n| V1 |"), \
             patch("app.processors.office_processor.settings") as mock_settings:
            mock_settings.chunk_max_tokens = 512
            mock_settings.chunk_overlap_tokens = 64
            chunks = _extract_docx_chunks(docx_file, MATERIAL_ID, COURSE_ID)

        assert len(chunks) == 1
        assert chunks[0]["content_type"] == "table"

    def test_empty_paragraphs_skipped(self, tmp_path):
        """Empty paragraphs (whitespace only) do not produce chunks."""
        docx_file = tmp_path / "doc.docx"
        docx_file.write_bytes(b"")

        blank = self._make_paragraph("   ", "Normal")

        with patch("app.processors.office_processor.Document"), \
             patch("app.processors.office_processor._iter_block_items", return_value=[blank]), \
             patch("app.processors.office_processor.settings") as mock_settings:
            mock_settings.chunk_max_tokens = 512
            mock_settings.chunk_overlap_tokens = 64
            chunks = _extract_docx_chunks(docx_file, MATERIAL_ID, COURSE_ID)

        assert chunks == []


# ── _extract_pptx_chunks ──────────────────────────────────────────────────────


class TestExtractPptxChunks:
    def _make_slide(self, title: str, body: str, notes: str = ""):
        """Build a mock PPTX slide with title, body, and optional notes."""
        title_shape = MagicMock()
        title_shape.has_text_frame = True
        title_shape.text_frame.text = title
        title_ph = MagicMock()
        title_ph.idx = 0
        title_shape.placeholder_format = title_ph

        body_shape = MagicMock()
        body_shape.has_text_frame = True
        body_shape.text_frame.text = body
        body_shape.placeholder_format = None

        slide = MagicMock()
        slide.shapes = [title_shape, body_shape] if body else [title_shape]

        if notes:
            slide.has_notes_slide = True
            slide.notes_slide.notes_text_frame.text = notes
        else:
            slide.has_notes_slide = False

        return slide

    def test_slide_title_becomes_section(self, tmp_path):
        """The slide title shape (placeholder idx=0) becomes the section."""
        pptx_file = tmp_path / "deck.pptx"
        pptx_file.write_bytes(b"")

        slide = self._make_slide("Slide One", "Some body text.")
        mock_prs = MagicMock()
        mock_prs.core_properties.title = "My Presentation"
        mock_prs.slides = [slide]

        with patch("app.processors.office_processor.Presentation", return_value=mock_prs), \
             patch("app.processors.office_processor.settings") as mock_settings:
            mock_settings.chunk_max_tokens = 512
            mock_settings.chunk_overlap_tokens = 64
            chunks = _extract_pptx_chunks(pptx_file, MATERIAL_ID, COURSE_ID)

        assert len(chunks) >= 1
        assert chunks[0]["section"] == "Slide One"
        assert chunks[0]["chapter"] == "My Presentation"

    def test_speaker_notes_included_in_chunk(self, tmp_path):
        """Speaker notes are appended to the slide content with [Speaker Notes] prefix."""
        pptx_file = tmp_path / "deck.pptx"
        pptx_file.write_bytes(b"")

        slide = self._make_slide("Title", "Body.", notes="This is a note.")
        mock_prs = MagicMock()
        mock_prs.core_properties.title = ""
        mock_prs.slides = [slide]

        with patch("app.processors.office_processor.Presentation", return_value=mock_prs), \
             patch("app.processors.office_processor.settings") as mock_settings:
            mock_settings.chunk_max_tokens = 512
            mock_settings.chunk_overlap_tokens = 64
            chunks = _extract_pptx_chunks(pptx_file, MATERIAL_ID, COURSE_ID)

        combined = " ".join(c["content"] for c in chunks)
        assert "[Speaker Notes]" in combined
        assert "This is a note." in combined

    def test_empty_slide_skipped(self, tmp_path):
        """Slides with no text content produce no segments."""
        pptx_file = tmp_path / "deck.pptx"
        pptx_file.write_bytes(b"")

        empty_shape = MagicMock()
        empty_shape.has_text_frame = False
        slide = MagicMock()
        slide.shapes = [empty_shape]
        slide.has_notes_slide = False

        mock_prs = MagicMock()
        mock_prs.core_properties.title = ""
        mock_prs.slides = [slide]

        with patch("app.processors.office_processor.Presentation", return_value=mock_prs), \
             patch("app.processors.office_processor.settings") as mock_settings:
            mock_settings.chunk_max_tokens = 512
            mock_settings.chunk_overlap_tokens = 64
            chunks = _extract_pptx_chunks(pptx_file, MATERIAL_ID, COURSE_ID)

        assert chunks == []

    def test_slide_page_number_is_one_indexed(self, tmp_path):
        """Slide page numbers start at 1."""
        pptx_file = tmp_path / "deck.pptx"
        pptx_file.write_bytes(b"")

        slide = self._make_slide("First Slide", "Content.")
        mock_prs = MagicMock()
        mock_prs.core_properties.title = ""
        mock_prs.slides = [slide]

        with patch("app.processors.office_processor.Presentation", return_value=mock_prs), \
             patch("app.processors.office_processor.settings") as mock_settings:
            mock_settings.chunk_max_tokens = 512
            mock_settings.chunk_overlap_tokens = 64
            chunks = _extract_pptx_chunks(pptx_file, MATERIAL_ID, COURSE_ID)

        assert chunks[0]["page"] == 1


# ── _extract_xlsx_chunks ──────────────────────────────────────────────────────


class TestExtractXlsxChunks:
    def _make_workbook(self, sheets: dict[str, list[list]]):
        """Build a mock openpyxl workbook from {sheet_name: [[row cells], ...]}."""
        mock_wb = MagicMock()
        mock_wb.properties.title = "Test Workbook"
        mock_wb.sheetnames = list(sheets.keys())

        def get_sheet(name):
            ws = MagicMock()
            rows_data = sheets[name]
            ws.iter_rows.return_value = [tuple(r) for r in rows_data]
            return ws

        mock_wb.__getitem__ = MagicMock(side_effect=get_sheet)
        return mock_wb

    def test_sheet_name_becomes_section(self, tmp_path):
        """Each worksheet's name is used as the section in produced chunks."""
        xlsx_file = tmp_path / "book.xlsx"
        xlsx_file.write_bytes(b"")

        mock_wb = self._make_workbook({
            "Grades": [["Student", "Score"], ["Alice", "95"]],
        })

        with patch("app.processors.office_processor.openpyxl") as mock_openpyxl, \
             patch("app.processors.office_processor.settings") as mock_settings:
            mock_openpyxl.load_workbook.return_value = mock_wb
            mock_settings.chunk_max_tokens = 512
            mock_settings.chunk_overlap_tokens = 64
            chunks = _extract_xlsx_chunks(xlsx_file, MATERIAL_ID, COURSE_ID)

        assert any(c["section"] == "Grades" for c in chunks)

    def test_workbook_title_becomes_chapter(self, tmp_path):
        """The workbook's core properties title becomes the chapter."""
        xlsx_file = tmp_path / "book.xlsx"
        xlsx_file.write_bytes(b"")

        mock_wb = self._make_workbook({
            "Sheet1": [["A", "B"], ["1", "2"]],
        })
        mock_wb.properties.title = "Course Data"

        with patch("app.processors.office_processor.openpyxl") as mock_openpyxl, \
             patch("app.processors.office_processor.settings") as mock_settings:
            mock_openpyxl.load_workbook.return_value = mock_wb
            mock_settings.chunk_max_tokens = 512
            mock_settings.chunk_overlap_tokens = 64
            chunks = _extract_xlsx_chunks(xlsx_file, MATERIAL_ID, COURSE_ID)

        assert all(c["chapter"] == "Course Data" for c in chunks)

    def test_empty_sheet_produces_no_chunks(self, tmp_path):
        """A worksheet with no data rows is skipped entirely."""
        xlsx_file = tmp_path / "book.xlsx"
        xlsx_file.write_bytes(b"")

        mock_wb = self._make_workbook({"EmptySheet": []})

        with patch("app.processors.office_processor.openpyxl") as mock_openpyxl, \
             patch("app.processors.office_processor.settings") as mock_settings:
            mock_openpyxl.load_workbook.return_value = mock_wb
            mock_settings.chunk_max_tokens = 512
            mock_settings.chunk_overlap_tokens = 64
            chunks = _extract_xlsx_chunks(xlsx_file, MATERIAL_ID, COURSE_ID)

        assert chunks == []

    def test_large_sheet_splits_at_max_chars(self, tmp_path):
        """Sheets exceeding max_chars are split across multiple table chunks."""
        xlsx_file = tmp_path / "book.xlsx"
        xlsx_file.write_bytes(b"")

        # Header + 10 data rows, each ~30 chars
        rows = [["Column A", "Column B"]] + [[f"Value{i}A", f"Value{i}B"] for i in range(10)]
        mock_wb = self._make_workbook({"Data": rows})

        with patch("app.processors.office_processor.openpyxl") as mock_openpyxl, \
             patch("app.processors.office_processor.settings") as mock_settings:
            mock_openpyxl.load_workbook.return_value = mock_wb
            mock_settings.chunk_max_tokens = 10  # 40 chars max — forces splits
            mock_settings.chunk_overlap_tokens = 0
            chunks = _extract_xlsx_chunks(xlsx_file, MATERIAL_ID, COURSE_ID)

        assert len(chunks) > 1


# ── process_office pipeline ───────────────────────────────────────────────────


class TestProcessOfficePipeline:
    @pytest.mark.asyncio
    async def test_happy_path_docx(self, tmp_path):
        """DOCX file: downloads, extracts chunks, embeds, returns complete status."""
        local_file = tmp_path / "doc.docx"
        local_file.write_bytes(b"PK")  # minimal zip header

        with patch("app.processors.office_processor.db") as mock_db, \
             patch("app.processors.office_processor.download_from_gcs", return_value=local_file), \
             patch("app.processors.office_processor.embed_and_store", new_callable=AsyncMock, return_value=5) as mock_store, \
             patch("app.processors.office_processor._extract_docx_chunks", return_value=[{"id": uuid4()}] * 5):
            mock_db.update_material_status = AsyncMock()

            job = _make_job(mime=DOCX_MIME)
            result = await process_office(job)

        assert result["status"] == "complete"
        assert result["chunk_count"] == 5
        assert result["processor"] == "office"

    @pytest.mark.asyncio
    async def test_happy_path_pptx(self, tmp_path):
        """PPTX file routes to the PPTX extractor."""
        local_file = tmp_path / "deck.pptx"
        local_file.write_bytes(b"PK")

        with patch("app.processors.office_processor.db") as mock_db, \
             patch("app.processors.office_processor.download_from_gcs", return_value=local_file), \
             patch("app.processors.office_processor.embed_and_store", new_callable=AsyncMock, return_value=3), \
             patch("app.processors.office_processor._extract_pptx_chunks", return_value=[{"id": uuid4()}] * 3) as mock_pptx:
            mock_db.update_material_status = AsyncMock()

            job = _make_job(mime=PPTX_MIME)
            await process_office(job)

        mock_pptx.assert_called_once()

    @pytest.mark.asyncio
    async def test_happy_path_xlsx(self, tmp_path):
        """XLSX file routes to the XLSX extractor."""
        local_file = tmp_path / "book.xlsx"
        local_file.write_bytes(b"PK")

        with patch("app.processors.office_processor.db") as mock_db, \
             patch("app.processors.office_processor.download_from_gcs", return_value=local_file), \
             patch("app.processors.office_processor.embed_and_store", new_callable=AsyncMock, return_value=2), \
             patch("app.processors.office_processor._extract_xlsx_chunks", return_value=[{"id": uuid4()}] * 2) as mock_xlsx:
            mock_db.update_material_status = AsyncMock()

            job = _make_job(mime=XLSX_MIME)
            await process_office(job)

        mock_xlsx.assert_called_once()

    @pytest.mark.asyncio
    async def test_unsupported_mime_raises_and_marks_failed(self, tmp_path):
        """Unsupported MIME type raises ValueError and marks material FAILED."""
        local_file = tmp_path / "unknown.bin"
        local_file.write_bytes(b"data")

        with patch("app.processors.office_processor.db") as mock_db, \
             patch("app.processors.office_processor.download_from_gcs", return_value=local_file):
            mock_db.update_material_status = AsyncMock()

            job = _make_job(mime="application/octet-stream")
            with pytest.raises(ValueError, match="unsupported office MIME type"):
                await process_office(job)

        mock_db.update_material_status.assert_any_call(MATERIAL_ID, ProcessingStatus.FAILED)

    @pytest.mark.asyncio
    async def test_exception_cleans_up_temp_file(self, tmp_path):
        """If extraction fails, the temporary local file is deleted."""
        local_file = tmp_path / "doc.docx"
        local_file.write_bytes(b"PK")

        with patch("app.processors.office_processor.db") as mock_db, \
             patch("app.processors.office_processor.download_from_gcs", return_value=local_file), \
             patch("app.processors.office_processor._extract_docx_chunks", side_effect=RuntimeError("parse error")):
            mock_db.update_material_status = AsyncMock()

            job = _make_job(mime=DOCX_MIME)
            with pytest.raises(RuntimeError):
                await process_office(job)

        assert not local_file.exists()
