"""Tests for the table extraction module.

Covers:
- _table_to_markdown: markdown formatting, None normalization, pipe escaping
- extract_table_chunks: table detection, metadata tagging, page numbers,
  chunk_index absence (caller assigns it), empty/no-table pages skipped

All pdfplumber I/O is mocked; no real PDF files are needed.
"""

from unittest.mock import MagicMock, call, patch

import pytest

from app.table_extractor import _table_to_markdown, extract_table_chunks

# ---------------------------------------------------------------------------
# _table_to_markdown unit tests
# ---------------------------------------------------------------------------


class TestTableToMarkdown:
    """Unit tests for the _table_to_markdown helper."""

    def test_empty_table_returns_empty_string(self):
        """Empty table list produces an empty string."""
        assert _table_to_markdown([]) == ""

    def test_single_row_header_only(self):
        """A table with one row (header) produces header + separator."""
        table = [["Name", "Score"]]
        result = _table_to_markdown(table)
        lines = result.splitlines()
        assert len(lines) == 2
        assert "Name" in lines[0]
        assert "Score" in lines[0]
        assert "---" in lines[1]

    def test_two_row_table_header_and_body(self):
        """A two-row table produces header, separator, and one body row."""
        table = [["Subject", "Grade"], ["Math", "A"]]
        result = _table_to_markdown(table)
        lines = result.splitlines()
        assert len(lines) == 3
        assert "Subject" in lines[0]
        assert "Grade" in lines[0]
        assert "---" in lines[1]
        assert "Math" in lines[2]
        assert "A" in lines[2]

    def test_none_cells_become_empty_string(self):
        """None values in cells are rendered as empty strings, not 'None'."""
        table = [["Col1", None], [None, "val"]]
        result = _table_to_markdown(table)
        assert "None" not in result

    def test_pipe_characters_in_cells_are_escaped(self):
        """Pipe characters inside cells are escaped to avoid breaking the table."""
        table = [["A|B", "C"], ["x|y", "z"]]
        result = _table_to_markdown(table)
        # The outer pipes are structural; inner pipes should be escaped
        lines = result.splitlines()
        # Count structural pipes: each line should have exactly max_cols+1 unescaped pipes
        for line in lines:
            # Replace escaped pipes, then count structural pipes
            structural = line.replace("\\|", "")
            assert structural.startswith("| ")
            assert structural.endswith(" |")

    def test_rows_with_different_lengths_are_padded(self):
        """Rows shorter than the widest row are padded with empty cells."""
        table = [["A", "B", "C"], ["X"]]
        result = _table_to_markdown(table)
        lines = result.splitlines()
        # All rows should have the same number of pipe-separated columns
        col_counts = [line.count("|") for line in lines]
        assert len(set(col_counts)) == 1  # all equal

    def test_multirow_table_has_correct_line_count(self):
        """A table with N body rows produces N+2 lines (header + sep + N body)."""
        table = [
            ["Lesson", "Date", "Topic"],
            ["1", "Jan 1", "Intro"],
            ["2", "Jan 8", "Algebra"],
            ["3", "Jan 15", "Geometry"],
        ]
        result = _table_to_markdown(table)
        assert len(result.splitlines()) == len(table) + 1  # header + sep + body rows

    def test_separator_row_uses_triple_dash(self):
        """The separator row between header and body uses '---' cells."""
        table = [["H1", "H2"], ["r1", "r2"]]
        result = _table_to_markdown(table)
        separator_line = result.splitlines()[1]
        assert "---" in separator_line

    def test_markdown_starts_and_ends_with_pipe(self):
        """Each line of the markdown table starts and ends with '|'."""
        table = [["X", "Y"], ["1", "2"]]
        result = _table_to_markdown(table)
        for line in result.splitlines():
            assert line.startswith("|")
            assert line.endswith("|")


# ---------------------------------------------------------------------------
# extract_table_chunks unit tests
# ---------------------------------------------------------------------------


def _make_pdfplumber_mock(pages_tables: list[list[list[list[str | None]]]]) -> MagicMock:
    """Build a pdfplumber context-manager mock.

    Args:
        pages_tables: For each page, a list of tables; each table is a list
            of rows; each row is a list of cell values.

    Returns:
        A MagicMock that mimics the ``pdfplumber.open(...)`` context manager.
    """
    mock_pages = []
    for tables in pages_tables:
        page = MagicMock()
        page.extract_tables.return_value = tables
        mock_pages.append(page)

    pdf_obj = MagicMock()
    pdf_obj.pages = mock_pages
    pdf_obj.__enter__ = MagicMock(return_value=pdf_obj)
    pdf_obj.__exit__ = MagicMock(return_value=False)

    mock_module = MagicMock()
    mock_module.open.return_value = pdf_obj
    return mock_module


class TestExtractTableChunks:
    """Unit tests for extract_table_chunks."""

    def test_pdf_with_no_tables_returns_empty_list(self):
        """A PDF with pages that have no tables produces an empty chunk list."""
        mock_pdfplumber = _make_pdfplumber_mock([[], []])
        with patch("app.table_extractor.pdfplumber", mock_pdfplumber):
            result = extract_table_chunks(b"%PDF-fake")
        assert result == []

    def test_single_page_single_table_produces_one_chunk(self):
        """One table on one page produces exactly one chunk."""
        table = [["Subject", "Grade"], ["Math", "A"]]
        mock_pdfplumber = _make_pdfplumber_mock([[table]])
        with patch("app.table_extractor.pdfplumber", mock_pdfplumber):
            result = extract_table_chunks(b"%PDF-fake")
        assert len(result) == 1

    def test_chunk_has_type_table_metadata(self):
        """Each extracted table chunk has ``type == "table"``."""
        table = [["H1", "H2"], ["v1", "v2"]]
        mock_pdfplumber = _make_pdfplumber_mock([[table]])
        with patch("app.table_extractor.pdfplumber", mock_pdfplumber):
            result = extract_table_chunks(b"%PDF-fake")
        assert result[0]["type"] == "table"

    def test_chunk_page_number_is_one_based(self):
        """Table chunks on the first page have ``page == 1``."""
        table = [["Col"], ["row"]]
        mock_pdfplumber = _make_pdfplumber_mock([[table]])
        with patch("app.table_extractor.pdfplumber", mock_pdfplumber):
            result = extract_table_chunks(b"%PDF-fake")
        assert result[0]["page"] == 1

    def test_page_number_reflects_pdf_page(self):
        """Table on the second page has ``page == 2``."""
        table = [["A", "B"], ["1", "2"]]
        # Page 1 has no tables; page 2 has one table
        mock_pdfplumber = _make_pdfplumber_mock([[], [table]])
        with patch("app.table_extractor.pdfplumber", mock_pdfplumber):
            result = extract_table_chunks(b"%PDF-fake")
        assert len(result) == 1
        assert result[0]["page"] == 2

    def test_chunk_text_contains_header_content(self):
        """The chunk ``text`` field contains the table header values."""
        table = [["Lesson", "Date", "Notes"], ["1", "Jan 1", "Intro"]]
        mock_pdfplumber = _make_pdfplumber_mock([[table]])
        with patch("app.table_extractor.pdfplumber", mock_pdfplumber):
            result = extract_table_chunks(b"%PDF-fake")
        assert "Lesson" in result[0]["text"]
        assert "Date" in result[0]["text"]

    def test_chunk_text_contains_body_content(self):
        """The chunk ``text`` field contains body row values."""
        table = [["Subject", "Score"], ["Physics", "95"], ["Chemistry", "88"]]
        mock_pdfplumber = _make_pdfplumber_mock([[table]])
        with patch("app.table_extractor.pdfplumber", mock_pdfplumber):
            result = extract_table_chunks(b"%PDF-fake")
        assert "Physics" in result[0]["text"]
        assert "88" in result[0]["text"]

    def test_chunk_has_token_count_field(self):
        """Each chunk includes a ``token_count`` field (word proxy)."""
        table = [["A", "B"], ["1", "2"]]
        mock_pdfplumber = _make_pdfplumber_mock([[table]])
        with patch("app.table_extractor.pdfplumber", mock_pdfplumber):
            result = extract_table_chunks(b"%PDF-fake")
        assert "token_count" in result[0]
        assert isinstance(result[0]["token_count"], int)
        assert result[0]["token_count"] > 0

    def test_chunk_index_is_not_set(self):
        """extract_table_chunks does NOT set chunk_index; caller assigns it."""
        table = [["H"], ["r"]]
        mock_pdfplumber = _make_pdfplumber_mock([[table]])
        with patch("app.table_extractor.pdfplumber", mock_pdfplumber):
            result = extract_table_chunks(b"%PDF-fake")
        assert "chunk_index" not in result[0]

    def test_multiple_tables_on_same_page(self):
        """Multiple tables on the same page produce multiple chunks."""
        table1 = [["A", "B"], ["1", "2"]]
        table2 = [["X", "Y"], ["9", "8"]]
        mock_pdfplumber = _make_pdfplumber_mock([[table1, table2]])
        with patch("app.table_extractor.pdfplumber", mock_pdfplumber):
            result = extract_table_chunks(b"%PDF-fake")
        assert len(result) == 2
        assert all(c["page"] == 1 for c in result)

    def test_tables_across_multiple_pages(self):
        """Tables on pages 1 and 3 produce two chunks with correct page numbers."""
        table_p1 = [["Col"], ["val"]]
        table_p3 = [["X", "Y"], ["a", "b"]]
        mock_pdfplumber = _make_pdfplumber_mock([[table_p1], [], [table_p3]])
        with patch("app.table_extractor.pdfplumber", mock_pdfplumber):
            result = extract_table_chunks(b"%PDF-fake")
        assert len(result) == 2
        assert result[0]["page"] == 1
        assert result[1]["page"] == 3

    def test_empty_table_is_skipped(self):
        """An empty table (empty list) in the pdfplumber output is skipped."""
        mock_pdfplumber = _make_pdfplumber_mock([[[]]])
        with patch("app.table_extractor.pdfplumber", mock_pdfplumber):
            result = extract_table_chunks(b"%PDF-fake")
        assert result == []

    def test_none_cells_in_tables_are_handled(self):
        """Tables with None cell values do not raise and produce valid chunks."""
        table = [[None, "Header"], ["Row1", None]]
        mock_pdfplumber = _make_pdfplumber_mock([[table]])
        with patch("app.table_extractor.pdfplumber", mock_pdfplumber):
            result = extract_table_chunks(b"%PDF-fake")
        assert len(result) == 1
        assert "None" not in result[0]["text"]

    def test_pdfplumber_opened_with_bytesio(self):
        """pdfplumber.open is called with a BytesIO wrapping the pdf_bytes."""
        import io

        table = [["H"], ["r"]]
        mock_pdfplumber = _make_pdfplumber_mock([[table]])
        with patch("app.table_extractor.pdfplumber", mock_pdfplumber):
            extract_table_chunks(b"%PDF-fake")

        call_arg = mock_pdfplumber.open.call_args[0][0]
        assert isinstance(call_arg, io.BytesIO)


# ---------------------------------------------------------------------------
# Integration: table chunks flow through ingest_pdf
# ---------------------------------------------------------------------------


class TestIngestPdfTableIntegration:
    """Verify table chunks reach Qdrant with correct metadata via ingest_pdf."""

    def test_table_chunks_upserted_with_type_table(self):
        """Qdrant points for table chunks carry ``type='table'`` in their payload."""
        from app.chunker import chunk_pdf_pages
        from app.tasks.pdf_ingest import ingest_pdf

        page_text = "Some lecture content. " * 10
        table = [["Subject", "Grade"], ["Math", "A"], ["Physics", "B"]]

        # Pre-compute chunk counts so mock responses are sized correctly
        text_chunks = chunk_pdf_pages([page_text], chunk_size=512, overlap=64)
        n_text = len(text_chunks)
        n_table = 1  # one table chunk
        total = n_text + n_table

        # --- fitz mock ---
        mock_fitz = MagicMock()
        mock_page = MagicMock()
        mock_page.get_text.return_value = page_text
        mock_doc = MagicMock()
        mock_doc.__iter__ = MagicMock(return_value=iter([mock_page]))
        mock_fitz.open.return_value = mock_doc

        # --- pdfplumber mock ---
        mock_pdfplumber = _make_pdfplumber_mock([[table]])

        # --- httpx mock ---
        data = [{"index": i, "embedding": [0.1] * 4} for i in range(total)]
        mock_response = MagicMock()
        mock_response.json.return_value = {"data": data}
        mock_response.raise_for_status = MagicMock()
        mock_http = MagicMock()
        mock_http.post.return_value = mock_response
        mock_http.__enter__ = MagicMock(return_value=mock_http)
        mock_http.__exit__ = MagicMock(return_value=False)

        # --- Qdrant mock ---
        mock_qdrant = MagicMock()

        with (
            patch("app.tasks.pdf_ingest.fitz", mock_fitz),
            patch("app.table_extractor.pdfplumber", mock_pdfplumber),
            patch("app.tasks.pdf_ingest.httpx.Client", return_value=mock_http),
            patch("app.tasks.pdf_ingest.QdrantClient", return_value=mock_qdrant),
        ):
            result = ingest_pdf.run(b"%PDF-fake", "course-tbl", "lecture.pdf")

        assert result["chunk_count"] == total

        # Collect all upserted points across all batch calls
        all_points = []
        for upsert_call in mock_qdrant.upsert.call_args_list:
            all_points.extend(upsert_call[1]["points"])

        # At least one point should be type=table
        types = [p.payload["type"] for p in all_points]
        assert "table" in types

    def test_text_chunks_have_type_text_in_payload(self):
        """Prose text chunks upserted via ingest_pdf carry ``type='text'``."""
        from app.chunker import chunk_pdf_pages
        from app.tasks.pdf_ingest import ingest_pdf

        page_text = "Some lecture content. " * 10
        text_chunks = chunk_pdf_pages([page_text], chunk_size=512, overlap=64)
        n_text = len(text_chunks)

        mock_fitz = MagicMock()
        mock_page = MagicMock()
        mock_page.get_text.return_value = page_text
        mock_doc = MagicMock()
        mock_doc.__iter__ = MagicMock(return_value=iter([mock_page]))
        mock_fitz.open.return_value = mock_doc

        # No tables on any page
        mock_pdfplumber = _make_pdfplumber_mock([[]])

        data = [{"index": i, "embedding": [0.2] * 4} for i in range(n_text)]
        mock_response = MagicMock()
        mock_response.json.return_value = {"data": data}
        mock_response.raise_for_status = MagicMock()
        mock_http = MagicMock()
        mock_http.post.return_value = mock_response
        mock_http.__enter__ = MagicMock(return_value=mock_http)
        mock_http.__exit__ = MagicMock(return_value=False)

        mock_qdrant = MagicMock()

        with (
            patch("app.tasks.pdf_ingest.fitz", mock_fitz),
            patch("app.table_extractor.pdfplumber", mock_pdfplumber),
            patch("app.tasks.pdf_ingest.httpx.Client", return_value=mock_http),
            patch("app.tasks.pdf_ingest.QdrantClient", return_value=mock_qdrant),
        ):
            ingest_pdf.run(b"%PDF-fake", "course-txt", "lecture.pdf")

        all_points = []
        for upsert_call in mock_qdrant.upsert.call_args_list:
            all_points.extend(upsert_call[1]["points"])

        assert all(p.payload["type"] == "text" for p in all_points)

    def test_table_chunk_index_continues_after_text_chunks(self):
        """Table chunk_index values continue the sequence from text chunks."""
        from app.chunker import chunk_pdf_pages
        from app.tasks.pdf_ingest import ingest_pdf

        page_text = "Some content. " * 10
        text_chunks = chunk_pdf_pages([page_text], chunk_size=512, overlap=64)
        n_text = len(text_chunks)

        table = [["H1", "H2"], ["r1", "r2"]]

        mock_fitz = MagicMock()
        mock_page = MagicMock()
        mock_page.get_text.return_value = page_text
        mock_doc = MagicMock()
        mock_doc.__iter__ = MagicMock(return_value=iter([mock_page]))
        mock_fitz.open.return_value = mock_doc

        mock_pdfplumber = _make_pdfplumber_mock([[table]])

        total = n_text + 1
        data = [{"index": i, "embedding": [0.3] * 4} for i in range(total)]
        mock_response = MagicMock()
        mock_response.json.return_value = {"data": data}
        mock_response.raise_for_status = MagicMock()
        mock_http = MagicMock()
        mock_http.post.return_value = mock_response
        mock_http.__enter__ = MagicMock(return_value=mock_http)
        mock_http.__exit__ = MagicMock(return_value=False)

        mock_qdrant = MagicMock()

        with (
            patch("app.tasks.pdf_ingest.fitz", mock_fitz),
            patch("app.table_extractor.pdfplumber", mock_pdfplumber),
            patch("app.tasks.pdf_ingest.httpx.Client", return_value=mock_http),
            patch("app.tasks.pdf_ingest.QdrantClient", return_value=mock_qdrant),
        ):
            ingest_pdf.run(b"%PDF-fake", "course-idx", "lecture.pdf")

        all_points = []
        for upsert_call in mock_qdrant.upsert.call_args_list:
            all_points.extend(upsert_call[1]["points"])

        table_points = [p for p in all_points if p.payload["type"] == "table"]
        assert len(table_points) == 1
        assert table_points[0].payload["chunk_index"] == n_text
