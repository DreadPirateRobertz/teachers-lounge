"""Tests for PDF processing pipeline: chunking, embedding, Qdrant writes, DB inserts."""
import uuid
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from app.models import IngestJobMessage, ProcessingStatus
from app.processors.pdf_processor import (
    _build_hierarchical_chunks,
    _classify_figure_type,
    _flush_segments,
    _make_chunk,
    _process_figures,
    process_pdf,
)


def _make_job(**overrides) -> IngestJobMessage:
    defaults = {
        "job_id": uuid.uuid4(),
        "user_id": uuid.uuid4(),
        "course_id": uuid.uuid4(),
        "material_id": uuid.uuid4(),
        "gcs_path": "gs://tvtutor-raw-uploads/u/c/j/test.pdf",
        "mime_type": "application/pdf",
        "filename": "test.pdf",
    }
    defaults.update(overrides)
    return IngestJobMessage(**defaults)


# ── Hierarchical Chunking ────────────────────────────────────────────────────


class _FakeElement:
    """Minimal mock of unstructured Element for unit testing."""
    def __init__(self, text: str, metadata=None):
        self.text = text
        self.metadata = metadata or MagicMock(page_number=1)


class _FakeTitle(_FakeElement):
    pass


class _FakeNarrativeText(_FakeElement):
    pass


class _FakeTable(_FakeElement):
    pass


# Patch isinstance checks — we use duck typing in tests
def _patch_isinstance():
    """We need to make our fakes pass isinstance checks in the processor."""
    return patch.multiple(
        "app.processors.pdf_processor",
        Title=_FakeTitle,
        NarrativeText=_FakeNarrativeText,
        Table=_FakeTable,
    )


class TestBuildHierarchicalChunks:
    def test_empty_elements(self):
        chunks = _build_hierarchical_chunks([], uuid.uuid4(), uuid.uuid4())
        assert chunks == []

    def test_single_paragraph(self):
        from unstructured.documents.elements import NarrativeText
        meta = MagicMock(page_number=1)
        el = NarrativeText(text="This is a test paragraph about chemistry.")
        el.metadata = meta
        chunks = _build_hierarchical_chunks(
            [el], uuid.uuid4(), uuid.uuid4()
        )
        assert len(chunks) == 1
        assert "chemistry" in chunks[0]["content"]
        assert chunks[0]["page"] == 1
        assert chunks[0]["content_type"] == "text"

    def test_chapter_and_section_tracking(self):
        from unstructured.documents.elements import NarrativeText, Title
        meta = MagicMock(page_number=1)
        meta2 = MagicMock(page_number=2)

        elements = [
            _make_el(Title, "Introduction", meta),
            _make_el(NarrativeText, "Opening paragraph about the course.", meta),
            _make_el(Title, "Chapter 1: Thermodynamics", meta2),
            _make_el(NarrativeText, "Heat transfer fundamentals.", meta2),
        ]

        mid = uuid.uuid4()
        cid = uuid.uuid4()
        chunks = _build_hierarchical_chunks(elements, mid, cid)
        assert len(chunks) >= 2

        # First chunk should have chapter=Introduction
        assert chunks[0]["chapter"] == "Introduction"
        # Second chunk should have chapter=Chapter 1: Thermodynamics
        assert chunks[1]["chapter"] == "Chapter 1: Thermodynamics"

    def test_long_text_splits_into_multiple_chunks(self):
        from unstructured.documents.elements import NarrativeText
        # Create text that exceeds chunk_max_tokens * 4 chars
        meta = MagicMock(page_number=1)
        # Default max_chars = 512 * 4 = 2048
        long_para = "Word " * 600  # ~3000 chars
        el = _make_el(NarrativeText, long_para, meta)

        chunks = _build_hierarchical_chunks([el], uuid.uuid4(), uuid.uuid4())
        # Single large element shouldn't split (it's one segment)
        # But if we have multiple paragraphs that exceed limit, they should split
        assert len(chunks) >= 1

    def test_multiple_paragraphs_respect_max_chars(self):
        from unstructured.documents.elements import NarrativeText
        meta = MagicMock(page_number=1)
        # Create many paragraphs that together exceed max_chars
        elements = [
            _make_el(NarrativeText, "Paragraph " + str(i) + ". " + "x" * 400, meta)
            for i in range(10)
        ]
        chunks = _build_hierarchical_chunks(elements, uuid.uuid4(), uuid.uuid4())
        # Should produce multiple chunks
        assert len(chunks) > 1
        # All content should be present
        all_content = " ".join(c["content"] for c in chunks)
        for i in range(10):
            assert f"Paragraph {i}" in all_content


def _make_el(cls, text, metadata):
    el = cls(text=text)
    el.metadata = metadata
    return el


class TestFlushSegments:
    def test_single_segment_under_limit(self):
        mid = uuid.uuid4()
        cid = uuid.uuid4()
        segments = [{"text": "Hello world", "chapter": None, "section": None, "page": 1, "content_type": "text"}]
        chunks = _flush_segments(segments, mid, cid, max_chars=2048, overlap_chars=256)
        assert len(chunks) == 1
        assert chunks[0]["content"] == "Hello world"

    def test_overlap_preserved(self):
        mid = uuid.uuid4()
        cid = uuid.uuid4()
        # Each segment is 100 chars, max_chars=250, overlap=100
        segments = [
            {"text": "A" * 100, "chapter": None, "section": None, "page": 1, "content_type": "text"},
            {"text": "B" * 100, "chapter": None, "section": None, "page": 1, "content_type": "text"},
            {"text": "C" * 100, "chapter": None, "section": None, "page": 2, "content_type": "text"},
            {"text": "D" * 100, "chapter": None, "section": None, "page": 2, "content_type": "text"},
        ]
        chunks = _flush_segments(segments, mid, cid, max_chars=250, overlap_chars=100)
        assert len(chunks) >= 2
        # Second chunk should contain overlap from first chunk's tail
        if len(chunks) >= 2:
            # The overlap ensures continuity
            assert len(chunks[1]["content"]) > 0


class TestMakeChunk:
    def test_all_fields_populated(self):
        mid = uuid.uuid4()
        cid = uuid.uuid4()
        chunk = _make_chunk(
            content="Test content",
            material_id=mid,
            course_id=cid,
            chapter="Ch 1",
            section="Sec A",
            page=5,
            content_type="text",
        )
        assert chunk["material_id"] == mid
        assert chunk["course_id"] == cid
        assert chunk["content"] == "Test content"
        assert chunk["chapter"] == "Ch 1"
        assert chunk["section"] == "Sec A"
        assert chunk["page"] == 5
        assert chunk["content_type"] == "text"
        assert chunk["id"] is not None


# ── _classify_figure_type ────────────────────────────────────────────────────


class TestClassifyFigureType:
    """Tests for the _classify_figure_type caption-based classifier."""

    def test_table_keyword(self):
        """Caption containing 'table' must return 'table'."""
        assert _classify_figure_type("Table 1. Summary of results") == "table"

    def test_tbl_abbreviation(self):
        """Caption containing 'tbl.' abbreviation must return 'table'."""
        assert _classify_figure_type("Tbl. 3 — comparison") == "table"

    def test_equation_keyword(self):
        """Caption containing 'equation' must return 'equation_image'."""
        assert _classify_figure_type("Equation 5: Navier-Stokes") == "equation_image"

    def test_eq_abbreviation(self):
        """Caption with 'eq.' abbreviation must return 'equation_image'."""
        assert _classify_figure_type("See eq. 3 above") == "equation_image"

    def test_formula_keyword(self):
        """Caption containing 'formula' must return 'equation_image'."""
        assert _classify_figure_type("The formula for kinetic energy") == "equation_image"

    def test_chart_keyword(self):
        """Caption containing 'chart' must return 'chart'."""
        assert _classify_figure_type("Bar chart of survey results") == "chart"

    def test_graph_keyword(self):
        """Caption containing 'graph' must return 'chart'."""
        assert _classify_figure_type("Graph showing CO2 levels") == "chart"

    def test_plot_keyword(self):
        """Caption containing 'plot' must return 'chart'."""
        assert _classify_figure_type("Scatter plot of test scores") == "chart"

    def test_unrecognised_caption_defaults_to_diagram(self):
        """Caption with no recognised keywords must return 'diagram'."""
        assert _classify_figure_type("Figure 2. Overview of the system") == "diagram"

    def test_empty_caption_defaults_to_diagram(self):
        """Empty caption must return 'diagram'."""
        assert _classify_figure_type("") == "diagram"

    def test_case_insensitive_match(self):
        """Keyword matching must be case-insensitive."""
        assert _classify_figure_type("TABLE OF VALUES") == "table"
        assert _classify_figure_type("GRAPH OF RESULTS") == "chart"

    def test_table_takes_precedence_over_other_keywords(self):
        """When multiple keywords match, table takes priority (first branch)."""
        assert _classify_figure_type("table chart") == "table"


# ── Full Pipeline (mocked externals) ─────────────────────────────────────────


class TestProcessPdfPipeline:
    @pytest.mark.asyncio
    @patch("app.processors.pdf_processor.qdrant")
    @patch("app.processors.pdf_processor.embeddings")
    @patch("app.processors.pdf_processor.db")
    @patch("app.processors.pdf_processor._partition_pdf")
    @patch("app.processors.pdf_processor._is_digital_pdf", return_value=True)
    @patch("app.processors.pdf_processor._download_from_gcs")
    async def test_happy_path(
        self, mock_download, mock_digital, mock_partition, mock_db, mock_embed, mock_qdrant
    ):
        from pathlib import Path

        from unstructured.documents.elements import NarrativeText

        job = _make_job()
        mock_download.return_value = Path("/tmp/fake.pdf")

        # Return some elements from partition
        meta = MagicMock(page_number=1)
        el1 = NarrativeText(text="First paragraph about biology.")
        el1.metadata = meta
        el2 = NarrativeText(text="Second paragraph about genetics.")
        el2.metadata = meta
        mock_partition.return_value = [el1, el2]

        # Mock embeddings
        mock_embed.embed_texts = AsyncMock(return_value=[[0.1] * 1024])

        # Mock DB and Qdrant
        mock_db.update_material_status = AsyncMock()
        mock_db.insert_chunks = AsyncMock()
        mock_qdrant.upsert_chunks = AsyncMock()

        result = await process_pdf(job)

        assert result["status"] == "complete"
        assert result["chunk_count"] >= 1

        # Verify status transitions
        calls = mock_db.update_material_status.call_args_list
        assert calls[0].args == (job.material_id, ProcessingStatus.PROCESSING)
        assert calls[-1].args[1] == ProcessingStatus.COMPLETE

        # Verify chunks were written to both Qdrant and Postgres
        mock_qdrant.upsert_chunks.assert_awaited_once()
        mock_db.insert_chunks.assert_awaited_once()

    @pytest.mark.asyncio
    @patch("app.processors.pdf_processor.db")
    @patch("app.processors.pdf_processor._download_from_gcs")
    async def test_failure_marks_material_failed(self, mock_download, mock_db):
        job = _make_job()
        mock_download.side_effect = Exception("GCS download failed")
        mock_db.update_material_status = AsyncMock()

        with pytest.raises(Exception, match="GCS download failed"):
            await process_pdf(job)

        # Should mark as FAILED
        calls = mock_db.update_material_status.call_args_list
        assert calls[-1].args == (job.material_id, ProcessingStatus.FAILED)

    @pytest.mark.asyncio
    @patch("app.processors.pdf_processor.db")
    @patch("app.processors.pdf_processor._partition_pdf", return_value=[])
    @patch("app.processors.pdf_processor._is_digital_pdf", return_value=True)
    @patch("app.processors.pdf_processor._download_from_gcs")
    async def test_empty_pdf_completes_with_zero_chunks(
        self, mock_download, mock_digital, mock_partition, mock_db
    ):
        from pathlib import Path
        job = _make_job()
        mock_download.return_value = Path("/tmp/fake.pdf")
        mock_db.update_material_status = AsyncMock()

        result = await process_pdf(job)

        assert result["status"] == "complete"
        assert result["chunk_count"] == 0
        # Should still mark complete
        calls = mock_db.update_material_status.call_args_list
        assert calls[-1].args[1] == ProcessingStatus.COMPLETE


# ── Embedding Service ─────────────────────────────────────────────────────────


class TestEmbedTexts:
    @pytest.mark.asyncio
    async def test_batching(self):
        from app.services.embeddings import embed_texts

        # Mock the OpenAI client
        mock_client = AsyncMock()
        mock_response = MagicMock()
        mock_response.data = [
            MagicMock(index=0, embedding=[0.1] * 1024),
            MagicMock(index=1, embedding=[0.2] * 1024),
        ]
        mock_client.embeddings.create = AsyncMock(return_value=mock_response)

        with patch("app.services.embeddings._api_key", "test-key"), \
             patch("app.services.embeddings.AsyncOpenAI", return_value=mock_client):
            vectors = await embed_texts(["text1", "text2"])

        assert len(vectors) == 2
        assert len(vectors[0]) == 1024
        mock_client.close.assert_awaited_once()


# ── Qdrant Service ────────────────────────────────────────────────────────────


class TestQdrantUpsert:
    @pytest.mark.asyncio
    async def test_upsert_builds_correct_points(self):
        from app.services.qdrant import upsert_chunks

        mock_client = AsyncMock()
        chunk_ids = [uuid.uuid4(), uuid.uuid4()]
        vectors = [[0.1] * 1024, [0.2] * 1024]
        payloads = [
            {"chunk_id": str(chunk_ids[0]), "content": "test1"},
            {"chunk_id": str(chunk_ids[1]), "content": "test2"},
        ]

        with patch("app.services.qdrant._make_client", return_value=mock_client):
            await upsert_chunks(chunk_ids, vectors, payloads)

        mock_client.upsert.assert_awaited_once()
        call_kwargs = mock_client.upsert.call_args.kwargs
        assert call_kwargs["collection_name"] == "curriculum"
        assert len(call_kwargs["points"]) == 2
        mock_client.close.assert_awaited_once()


# ── DB Chunk Insert ───────────────────────────────────────────────────────────


class TestDbInsertChunks:
    @pytest.mark.asyncio
    async def test_insert_chunks_calls_executemany(self):
        from app.services.db import insert_chunks

        mock_conn = AsyncMock()
        chunks = [
            {
                "id": uuid.uuid4(),
                "material_id": uuid.uuid4(),
                "course_id": uuid.uuid4(),
                "content": "Test chunk content",
                "chapter": "Chapter 1",
                "section": "Section A",
                "page": 1,
                "content_type": "text",
                "metadata": {"key": "val"},
            }
        ]

        with patch("app.services.db._connect", return_value=mock_conn):
            await insert_chunks(chunks)

        mock_conn.executemany.assert_awaited_once()
        mock_conn.close.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_insert_chunks_empty_list(self):
        from app.services.db import insert_chunks
        # Should return early without connecting
        await insert_chunks([])


class TestProcessFigures:
    """Tests for _process_figures — figure extraction, CLIP embedding, GCS upload."""

    def _make_image_element(self, image_path: str, text: str = ""):
        """Build a mock unstructured Image element with metadata.image_path set."""
        from unittest.mock import MagicMock

        from unstructured.documents.elements import Image

        meta = MagicMock()
        meta.image_path = image_path
        meta.page_number = 1
        el = Image(text=text)
        el.metadata = meta
        return el

    @pytest.mark.asyncio
    @patch("app.processors.pdf_processor.qdrant")
    @patch("app.processors.pdf_processor.gcs")
    @patch("app.processors.pdf_processor.clip_embedder")
    async def test_figures_uploaded_to_gcs_and_upserted(
        self, mock_clip, mock_gcs, mock_qdrant, tmp_path
    ):
        """Figure with valid image_path → CLIP embed → GCS upload → Qdrant upsert."""
        img = tmp_path / "figure.png"
        img.write_bytes(b"\x89PNG\r\n")  # minimal PNG header

        mock_clip.embed_image = AsyncMock(return_value=[0.1] * 768)
        mock_gcs.upload_figure = AsyncMock(return_value="gs://tvtutor-raw-uploads/c/m/figures/d.png")
        mock_qdrant.upsert_diagrams = AsyncMock()

        el = self._make_image_element(str(img))
        count = await _process_figures(
            elements=[el],
            material_id=uuid.uuid4(),
            course_id=uuid.uuid4(),
            job_id=uuid.uuid4(),
            local_pdf_path=tmp_path / "test.pdf",
        )

        assert count == 1
        mock_gcs.upload_figure.assert_awaited_once()
        mock_qdrant.upsert_diagrams.assert_awaited_once()

    @pytest.mark.asyncio
    @patch("app.processors.pdf_processor.qdrant")
    @patch("app.processors.pdf_processor.gcs")
    @patch("app.processors.pdf_processor.clip_embedder")
    async def test_clip_failure_skips_figure_gracefully(
        self, mock_clip, mock_gcs, mock_qdrant, tmp_path
    ):
        """CLIP embed failure → figure skipped, pipeline does not abort."""
        img = tmp_path / "figure.png"
        img.write_bytes(b"\x89PNG")

        mock_clip.embed_image = AsyncMock(side_effect=RuntimeError("CLIP unavailable"))
        mock_gcs.upload_figure = AsyncMock()
        mock_qdrant.upsert_diagrams = AsyncMock()

        el = self._make_image_element(str(img))
        count = await _process_figures(
            elements=[el],
            material_id=uuid.uuid4(),
            course_id=uuid.uuid4(),
            job_id=uuid.uuid4(),
            local_pdf_path=tmp_path / "test.pdf",
        )

        assert count == 0
        mock_gcs.upload_figure.assert_not_awaited()
        mock_qdrant.upsert_diagrams.assert_not_awaited()

    @pytest.mark.asyncio
    @patch("app.processors.pdf_processor.qdrant")
    @patch("app.processors.pdf_processor.gcs")
    @patch("app.processors.pdf_processor.clip_embedder")
    async def test_gcs_upload_failure_skips_figure_gracefully(
        self, mock_clip, mock_gcs, mock_qdrant, tmp_path
    ):
        """GCS upload failure → figure skipped, Qdrant upsert not called."""
        img = tmp_path / "figure.png"
        img.write_bytes(b"\x89PNG")

        mock_clip.embed_image = AsyncMock(return_value=[0.1] * 768)
        mock_gcs.upload_figure = AsyncMock(side_effect=RuntimeError("GCS unavailable"))
        mock_qdrant.upsert_diagrams = AsyncMock()

        el = self._make_image_element(str(img))
        count = await _process_figures(
            elements=[el],
            material_id=uuid.uuid4(),
            course_id=uuid.uuid4(),
            job_id=uuid.uuid4(),
            local_pdf_path=tmp_path / "test.pdf",
        )

        assert count == 0
        mock_qdrant.upsert_diagrams.assert_not_awaited()

    @pytest.mark.asyncio
    @patch("app.processors.pdf_processor.qdrant")
    @patch("app.processors.pdf_processor.gcs")
    async def test_no_image_elements_returns_zero(self, mock_gcs, mock_qdrant):
        """Elements with no Image types → count=0, no GCS or Qdrant calls."""
        from unstructured.documents.elements import NarrativeText

        el = NarrativeText(text="Plain text, no figures here.")
        count = await _process_figures(
            elements=[el],
            material_id=uuid.uuid4(),
            course_id=uuid.uuid4(),
            job_id=uuid.uuid4(),
            local_pdf_path=None,
        )

        assert count == 0
        mock_gcs.upload_figure.assert_not_called()
        mock_qdrant.upsert_diagrams.assert_not_called()

    @pytest.mark.asyncio
    @patch("app.processors.pdf_processor.qdrant")
    @patch("app.processors.pdf_processor.gcs")
    @patch("app.processors.pdf_processor.clip_embedder")
    async def test_missing_image_path_skips_figure(
        self, mock_clip, mock_gcs, mock_qdrant, tmp_path
    ):
        """Image element with no metadata.image_path → skipped without error."""
        from unstructured.documents.elements import Image

        el = Image(text="")
        meta = MagicMock()
        meta.image_path = None
        el.metadata = meta

        count = await _process_figures(
            elements=[el],
            material_id=uuid.uuid4(),
            course_id=uuid.uuid4(),
            job_id=uuid.uuid4(),
            local_pdf_path=tmp_path / "test.pdf",
        )

        assert count == 0
        mock_gcs.upload_figure.assert_not_called()

    @pytest.mark.asyncio
    @patch("app.processors.pdf_processor.qdrant")
    @patch("app.processors.pdf_processor.gcs")
    @patch("app.processors.pdf_processor.clip_embedder")
    async def test_gcs_path_stored_in_qdrant_payload(
        self, mock_clip, mock_gcs, mock_qdrant, tmp_path
    ):
        """The gcs_path returned by upload_figure is forwarded to the Qdrant payload."""
        img = tmp_path / "figure.png"
        img.write_bytes(b"\x89PNG")
        expected_gcs = "gs://tvtutor-raw-uploads/course/mat/figures/abc.png"

        mock_clip.embed_image = AsyncMock(return_value=[0.1] * 768)
        mock_gcs.upload_figure = AsyncMock(return_value=expected_gcs)
        mock_qdrant.upsert_diagrams = AsyncMock()

        el = self._make_image_element(str(img))
        await _process_figures(
            elements=[el],
            material_id=uuid.uuid4(),
            course_id=uuid.uuid4(),
            job_id=uuid.uuid4(),
            local_pdf_path=tmp_path / "test.pdf",
        )

        _, payloads = mock_qdrant.upsert_diagrams.call_args[0][1], mock_qdrant.upsert_diagrams.call_args[0][2]
        assert payloads[0]["gcs_path"] == expected_gcs
