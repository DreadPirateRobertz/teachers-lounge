"""Tests for CLIP image embedder, Qdrant diagram writer, and PyMuPDF figure extractor (Phase 6).

Coverage targets:
  - app/processors/figure_extractor.py  (extract_figures, _find_caption, _classify_figure_type)
  - app/services/clip_embedder.py  (embed_image, embed_image_sync stub mode)
  - app/services/qdrant_writer.py  (upsert_diagram, _upsert_diagram_sync)
"""
from __future__ import annotations

import math
import tempfile
import uuid
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

# ── Helpers ───────────────────────────────────────────────────────────────────


def _write_png_file(width: int = 64, height: int = 64) -> Path:
    """Write a minimal RGB PNG to a temp file and return its Path.

    Args:
        width: Image width in pixels.
        height: Image height in pixels.

    Returns:
        Path to the temporary PNG file.
    """
    from PIL import Image

    img = Image.new("RGB", (width, height), color=(128, 200, 64))
    tmp = tempfile.NamedTemporaryFile(suffix=".png", delete=False)
    img.save(tmp.name)
    return Path(tmp.name)


# ── TestCLIPImageEmbedder ─────────────────────────────────────────────────────


class TestCLIPImageEmbedder:
    """Tests for app/services/clip_embedder.py embed_image() in stub mode."""

    @pytest.mark.asyncio
    async def test_stub_returns_768d_vector(self, patch_settings):
        """Stub mode must return a list of exactly 768 floats.

        Args:
            patch_settings: Fixture that overrides settings for the test.
        """
        patch_settings(clip_model="stub-triggers-import-error", clip_embedding_dim=768)
        # Force re-probe by resetting module cache
        import app.services.clip_embedder as mod
        mod._clip_available = None
        mod._model = None
        mod._processor = None

        image_path = _write_png_file()
        try:
            with patch("app.services.clip_embedder._probe_clip", return_value=False):
                from app.services.clip_embedder import embed_image_sync
                vector = embed_image_sync(image_path)
        finally:
            image_path.unlink(missing_ok=True)

        assert isinstance(vector, list)
        assert len(vector) == 768
        assert all(isinstance(v, float) for v in vector)

    @pytest.mark.asyncio
    async def test_stub_vector_is_unit_length(self, patch_settings):
        """Stub vector must have Euclidean norm of approximately 1.0.

        Args:
            patch_settings: Fixture that overrides settings for the test.
        """
        patch_settings(clip_embedding_dim=768)
        image_path = _write_png_file()
        try:
            with patch("app.services.clip_embedder._probe_clip", return_value=False):
                from app.services.clip_embedder import embed_image_sync
                vector = embed_image_sync(image_path)
        finally:
            image_path.unlink(missing_ok=True)

        norm = math.sqrt(sum(v * v for v in vector))
        assert abs(norm - 1.0) < 1e-6

    def test_stub_is_deterministic_for_same_path(self, patch_settings):
        """Same file path must produce the same stub vector (md5-seeded).

        Args:
            patch_settings: Fixture that overrides settings for the test.
        """
        patch_settings(clip_embedding_dim=768)
        image_path = _write_png_file()
        try:
            with patch("app.services.clip_embedder._probe_clip", return_value=False):
                from app.services.clip_embedder import embed_image_sync
                v1 = embed_image_sync(image_path)
                v2 = embed_image_sync(image_path)
        finally:
            image_path.unlink(missing_ok=True)

        assert v1 == v2

    def test_different_paths_produce_different_vectors(self, patch_settings):
        """Different file paths must produce different stub vectors.

        Args:
            patch_settings: Fixture that overrides settings for the test.
        """
        patch_settings(clip_embedding_dim=768)
        p1 = _write_png_file()
        p2 = _write_png_file()
        try:
            with patch("app.services.clip_embedder._probe_clip", return_value=False):
                from app.services.clip_embedder import embed_image_sync
                v1 = embed_image_sync(p1)
                v2 = embed_image_sync(p2)
        finally:
            p1.unlink(missing_ok=True)
            p2.unlink(missing_ok=True)

        assert v1 != v2


# ── TestQdrantWriter ──────────────────────────────────────────────────────────


class TestQdrantWriter:
    """Tests for app/services/qdrant_writer.py."""

    @pytest.mark.asyncio
    async def test_upsert_nonfatal_on_qdrant_error(self, patch_settings):
        """A Qdrant error must be caught and logged, not re-raised.

        Args:
            patch_settings: Fixture that overrides settings for the test.
        """
        patch_settings(
            qdrant_host="localhost",
            qdrant_port=6333,
            diagrams_collection="diagrams",
            clip_embedding_dim=768,
        )
        from app.services.qdrant_writer import upsert_diagram

        with patch(
            "app.services.qdrant_writer._upsert_diagram_sync",
            side_effect=RuntimeError("Qdrant down"),
        ):
            # Must not raise
            await upsert_diagram(
                diagram_id=uuid.uuid4(),
                material_id=uuid.uuid4(),
                course_id=uuid.uuid4(),
                page=1,
                caption="Figure 1. Test",
                image_b64_thumb="aGVsbG8=",
                clip_vector=[0.0] * 768,
            )

    @pytest.mark.asyncio
    async def test_upsert_calls_sync_helper(self, patch_settings):
        """A successful upsert_diagram call must invoke the sync helper once.

        Args:
            patch_settings: Fixture that overrides settings for the test.
        """
        patch_settings(
            qdrant_host="localhost",
            qdrant_port=6333,
            diagrams_collection="diagrams",
            clip_embedding_dim=768,
        )
        from app.services.qdrant_writer import upsert_diagram

        with patch("app.services.qdrant_writer._upsert_diagram_sync") as mock_sync:
            await upsert_diagram(
                diagram_id=uuid.uuid4(),
                material_id=uuid.uuid4(),
                course_id=uuid.uuid4(),
                page=2,
                caption="",
                image_b64_thumb="aGVsbG8=",
                clip_vector=[0.1] * 768,
            )

        mock_sync.assert_called_once()

    def test_sync_creates_collection_if_missing(self, patch_settings):
        """_upsert_diagram_sync must create the collection when absent.

        Args:
            patch_settings: Fixture that overrides settings for the test.
        """
        patch_settings(
            qdrant_host="localhost",
            qdrant_port=6333,
            diagrams_collection="diagrams",
            clip_embedding_dim=768,
        )
        from app.services.qdrant_writer import _upsert_diagram_sync

        mock_client = MagicMock()
        mock_resp = MagicMock()
        mock_resp.collections = []
        mock_client.get_collections.return_value = mock_resp

        with patch("app.services.qdrant_writer.QdrantClient", return_value=mock_client):
            _upsert_diagram_sync(
                diagram_id=uuid.uuid4(),
                material_id=uuid.uuid4(),
                course_id=uuid.uuid4(),
                page=1,
                caption="Figure 1.",
                image_b64_thumb="aGVsbG8=",
                clip_vector=[0.0] * 768,
            )

        mock_client.create_collection.assert_called_once()
        mock_client.upsert.assert_called_once()

    def test_sync_skips_create_when_collection_exists(self, patch_settings):
        """_upsert_diagram_sync must not call create_collection if present.

        Args:
            patch_settings: Fixture that overrides settings for the test.
        """
        patch_settings(
            qdrant_host="localhost",
            qdrant_port=6333,
            diagrams_collection="diagrams",
            clip_embedding_dim=768,
        )
        from app.services.qdrant_writer import _upsert_diagram_sync

        mock_client = MagicMock()
        existing = MagicMock()
        existing.name = "diagrams"
        mock_resp = MagicMock()
        mock_resp.collections = [existing]
        mock_client.get_collections.return_value = mock_resp

        with patch("app.services.qdrant_writer.QdrantClient", return_value=mock_client):
            _upsert_diagram_sync(
                diagram_id=uuid.uuid4(),
                material_id=uuid.uuid4(),
                course_id=uuid.uuid4(),
                page=3,
                caption="",
                image_b64_thumb="aGVsbG8=",
                clip_vector=[0.0] * 768,
            )

        mock_client.create_collection.assert_not_called()
        mock_client.upsert.assert_called_once()

    def test_upsert_payload_contains_expected_keys(self, patch_settings):
        """Upserted point payload must include all required metadata keys.

        Args:
            patch_settings: Fixture that overrides settings for the test.
        """
        patch_settings(
            qdrant_host="localhost",
            qdrant_port=6333,
            diagrams_collection="diagrams",
            clip_embedding_dim=768,
        )
        from app.services.qdrant_writer import _upsert_diagram_sync

        mock_client = MagicMock()
        mock_resp = MagicMock()
        mock_resp.collections = []
        mock_client.get_collections.return_value = mock_resp

        d_id = uuid.uuid4()
        m_id = uuid.uuid4()
        c_id = uuid.uuid4()

        with patch("app.services.qdrant_writer.QdrantClient", return_value=mock_client):
            _upsert_diagram_sync(
                diagram_id=d_id,
                material_id=m_id,
                course_id=c_id,
                page=5,
                caption="A test caption",
                image_b64_thumb="dGVzdA==",
                clip_vector=[0.5] * 768,
            )

        call_args = mock_client.upsert.call_args
        points = call_args.kwargs.get("points") or call_args.args[1]
        assert len(points) == 1
        payload = points[0].payload
        assert payload["material_id"] == str(m_id)
        assert payload["course_id"] == str(c_id)
        assert payload["page"] == 5
        assert payload["caption"] == "A test caption"
        assert payload["image_b64_thumb"] == "dGVzdA=="


# ── Helpers: synthetic PDF creation ─────────────────────────────────────────


def _make_pdf_with_image(image_path: Path) -> Path:
    """Create a minimal PDF that embeds *image_path* on page 1.

    Args:
        image_path: Path to a PNG/JPEG file to embed in the PDF.

    Returns:
        Path to the created temporary PDF file.
    """
    import fitz  # PyMuPDF

    doc = fitz.open()
    page = doc.new_page(width=595, height=842)  # A4 points
    rect = fitz.Rect(100, 100, 400, 350)
    page.insert_image(rect, filename=str(image_path))

    tmp = tempfile.NamedTemporaryFile(delete=False, suffix=".pdf")
    doc.save(tmp.name)
    doc.close()
    return Path(tmp.name)


def _make_pdf_with_caption(image_path: Path, caption: str) -> Path:
    """Create a PDF that embeds *image_path* with *caption* text below it.

    Args:
        image_path: Path to a PNG/JPEG file to embed.
        caption: Caption text written directly below the image.

    Returns:
        Path to the created temporary PDF file.
    """
    import fitz

    doc = fitz.open()
    page = doc.new_page(width=595, height=842)
    img_rect = fitz.Rect(100, 100, 400, 350)
    page.insert_image(img_rect, filename=str(image_path))
    # Write caption text just below the image
    text_point = fitz.Point(100, 370)
    page.insert_text(text_point, caption, fontsize=10)

    tmp = tempfile.NamedTemporaryFile(delete=False, suffix=".pdf")
    doc.save(tmp.name)
    doc.close()
    return Path(tmp.name)


def _make_empty_pdf() -> Path:
    """Create a minimal PDF with no embedded images.

    Returns:
        Path to the created temporary PDF file.
    """
    import fitz

    doc = fitz.open()
    page = doc.new_page(width=595, height=842)
    page.insert_text(fitz.Point(50, 100), "Text only page, no images.", fontsize=12)

    tmp = tempfile.NamedTemporaryFile(delete=False, suffix=".pdf")
    doc.save(tmp.name)
    doc.close()
    return Path(tmp.name)


# ── TestFigureExtractor ───────────────────────────────────────────────────────


class TestFigureExtractor:
    """Tests for app/processors/figure_extractor.extract_figures()."""

    def test_extract_figures_returns_list_from_pdf_with_image(self):
        """extract_figures must return at least one FigureInfo for a PDF containing an image.

        Args:
            None
        """
        from app.processors.figure_extractor import extract_figures

        img_path = _write_png_file(200, 200)
        pdf_path = _make_pdf_with_image(img_path)
        try:
            figures = extract_figures(pdf_path)
        finally:
            img_path.unlink(missing_ok=True)
            pdf_path.unlink(missing_ok=True)
            for fig in figures:
                fig.image_path.unlink(missing_ok=True)

        assert len(figures) >= 1

    def test_extract_figures_empty_for_text_only_pdf(self):
        """extract_figures must return an empty list when the PDF has no embedded images.

        Args:
            None
        """
        from app.processors.figure_extractor import extract_figures

        pdf_path = _make_empty_pdf()
        try:
            figures = extract_figures(pdf_path)
        finally:
            pdf_path.unlink(missing_ok=True)

        assert figures == []

    def test_extracted_figure_png_file_is_readable(self):
        """Each extracted figure's image_path must point to a valid PNG file.

        Args:
            None
        """
        from PIL import Image

        from app.processors.figure_extractor import extract_figures

        img_path = _write_png_file(200, 200)
        pdf_path = _make_pdf_with_image(img_path)
        figures = []
        try:
            figures = extract_figures(pdf_path)
            assert len(figures) >= 1
            img = Image.open(figures[0].image_path)
            assert img.width > 0 and img.height > 0
        finally:
            img_path.unlink(missing_ok=True)
            pdf_path.unlink(missing_ok=True)
            for fig in figures:
                fig.image_path.unlink(missing_ok=True)

    def test_figure_page_number_is_correct(self):
        """Extracted FigureInfo.page must match the actual PDF page number (1-indexed).

        Args:
            None
        """
        from app.processors.figure_extractor import extract_figures

        img_path = _write_png_file(200, 200)
        pdf_path = _make_pdf_with_image(img_path)
        figures = []
        try:
            figures = extract_figures(pdf_path)
            assert len(figures) >= 1
            assert figures[0].page == 1
        finally:
            img_path.unlink(missing_ok=True)
            pdf_path.unlink(missing_ok=True)
            for fig in figures:
                fig.image_path.unlink(missing_ok=True)

    def test_small_images_filtered_by_min_dimensions(self):
        """Images smaller than min_width/min_height must be filtered out.

        Args:
            None
        """
        from app.processors.figure_extractor import extract_figures

        # Create a small icon-sized image
        img_path = _write_png_file(50, 50)
        pdf_path = _make_pdf_with_image(img_path)
        try:
            figures = extract_figures(pdf_path, min_width=100, min_height=100)
        finally:
            img_path.unlink(missing_ok=True)
            pdf_path.unlink(missing_ok=True)
            for fig in figures:
                fig.image_path.unlink(missing_ok=True)

        assert figures == []

    def test_figure_metadata_tags_type_as_diagram_by_default(self):
        """FigureInfo.figure_type must default to ``diagram`` when no caption keywords match.

        Args:
            None
        """
        from app.processors.figure_extractor import extract_figures

        img_path = _write_png_file(200, 200)
        pdf_path = _make_pdf_with_image(img_path)
        figures = []
        try:
            figures = extract_figures(pdf_path)
            assert len(figures) >= 1
            assert figures[0].figure_type == "diagram"
        finally:
            img_path.unlink(missing_ok=True)
            pdf_path.unlink(missing_ok=True)
            for fig in figures:
                fig.image_path.unlink(missing_ok=True)

    def test_no_fitz_returns_empty_list(self):
        """extract_figures must return [] gracefully when fitz is not importable.

        Simulates a missing PyMuPDF installation by patching the ``fitz``
        module reference in sys.modules to None, which causes the lazy import
        inside extract_figures to raise ImportError.

        Args:
            None
        """
        import importlib
        import sys

        import app.processors.figure_extractor as fe_mod

        pdf_path = _make_empty_pdf()
        try:
            saved = sys.modules.get("fitz")
            sys.modules["fitz"] = None  # type: ignore[assignment]
            try:
                importlib.reload(fe_mod)
                result = fe_mod.extract_figures(pdf_path)
            finally:
                if saved is None:
                    del sys.modules["fitz"]
                else:
                    sys.modules["fitz"] = saved
                importlib.reload(fe_mod)

            assert result == []
        finally:
            pdf_path.unlink(missing_ok=True)


# ── TestClassifyFigureType ────────────────────────────────────────────────────


class TestClassifyFigureType:
    """Tests for app/processors/figure_extractor._classify_figure_type()."""

    def test_chart_caption_yields_chart(self):
        """Caption containing 'chart' must produce figure_type='chart'.

        Args:
            None
        """
        from app.processors.figure_extractor import _classify_figure_type

        assert _classify_figure_type("Figure 1. Student performance chart") == "chart"

    def test_graph_caption_yields_chart(self):
        """Caption containing 'graph' must produce figure_type='chart'.

        Args:
            None
        """
        from app.processors.figure_extractor import _classify_figure_type

        assert _classify_figure_type("Figure 2. Error rate graph over time") == "chart"

    def test_table_caption_yields_table(self):
        """Caption containing 'table' must produce figure_type='table'.

        Args:
            None
        """
        from app.processors.figure_extractor import _classify_figure_type

        assert _classify_figure_type("Table 3. Summary of results") == "table"

    def test_equation_caption_yields_equation_image(self):
        """Caption containing 'equation' must produce figure_type='equation_image'.

        Args:
            None
        """
        from app.processors.figure_extractor import _classify_figure_type

        assert _classify_figure_type("Equation 5. Bayes theorem") == "equation_image"

    def test_empty_caption_yields_diagram(self):
        """Empty caption must produce the default figure_type='diagram'.

        Args:
            None
        """
        from app.processors.figure_extractor import _classify_figure_type

        assert _classify_figure_type("") == "diagram"

    def test_unrecognized_caption_yields_diagram(self):
        """Caption without keyword matches must produce figure_type='diagram'.

        Args:
            None
        """
        from app.processors.figure_extractor import _classify_figure_type

        assert _classify_figure_type("Figure 4. System architecture overview") == "diagram"
