"""Tests for CLIP image embedder and Qdrant diagram writer (Phase 6).

Coverage targets:
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
