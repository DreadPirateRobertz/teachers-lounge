"""Tests for the ingest_pdf Celery task entry point.

Covers four paths through :func:`app.tasks.ingest_pdf.ingest_pdf`:

- Happy path: GCS download succeeds, PDF parses, chunks upserted to Qdrant.
- File-not-found: GCS download raises :exc:`FileNotFoundError`; propagates.
- Qdrant upsert failure: Qdrant raises; propagates to caller.
- Malformed PDF: unstructured partition raises; propagates to caller.

All external I/O (GCS, Qdrant, embeddings, DB) is mocked with
:mod:`unittest.mock` so no real network calls are made.
"""

from __future__ import annotations

import uuid
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from app.tasks.ingest_pdf import ingest_pdf

# ── helpers ──────────────────────────────────────────────────────────────────


def _make_args(**overrides) -> dict:
    """Return a default ingest_pdf kwargs dict, optionally overridden.

    Args:
        **overrides: Fields to replace in the default dict.

    Returns:
        Dict suitable for ``ingest_pdf(**kwargs)``.
    """
    defaults = {
        "job_id": uuid.uuid4(),
        "user_id": uuid.uuid4(),
        "course_id": uuid.uuid4(),
        "material_id": uuid.uuid4(),
        "gcs_path": "gs://tvtutor-raw-uploads/u/c/j/lecture.pdf",
        "mime_type": "application/pdf",
        "filename": "lecture.pdf",
    }
    defaults.update(overrides)
    return defaults


def _narrative_element(text: str, page: int = 1):
    """Build a minimal unstructured NarrativeText element for mocking.

    Args:
        text: Text content of the element.
        page: Page number to attach via metadata.

    Returns:
        A NarrativeText instance with ``metadata.page_number`` set.
    """
    from unstructured.documents.elements import NarrativeText

    meta = MagicMock(page_number=page)
    el = NarrativeText(text=text)
    el.metadata = meta
    return el


# ── test cases ────────────────────────────────────────────────────────────────


class TestIngestPdfTask:
    """Unit tests for app.tasks.ingest_pdf.ingest_pdf."""

    # ------------------------------------------------------------------
    # Happy path
    # ------------------------------------------------------------------

    @patch("app.processors.pdf_processor.qdrant")
    @patch("app.processors.pdf_processor.embeddings")
    @patch("app.processors.pdf_processor.db")
    @patch("app.processors.pdf_processor._partition_pdf")
    @patch("app.processors.pdf_processor._is_digital_pdf", return_value=True)
    @patch("app.processors.pdf_processor._download_from_gcs")
    def test_happy_path_returns_complete_result(
        self,
        mock_download,
        mock_digital,
        mock_partition,
        mock_db,
        mock_embed,
        mock_qdrant,
    ):
        """PDF uploaded → chunks extracted → upserted to Qdrant → status complete.

        Verifies the full pipeline executes end-to-end with all external
        dependencies mocked, and that the returned dict has the expected shape.
        """
        mock_download.return_value = Path("/tmp/fake_lecture.pdf")
        mock_partition.return_value = [
            _narrative_element("Thermodynamics is the branch of physics.", page=1),
            _narrative_element("The first law of thermodynamics states.", page=1),
        ]
        mock_embed.embed_texts = AsyncMock(return_value=[[0.1] * 1024])
        mock_db.update_material_status = AsyncMock()
        mock_db.insert_chunks = AsyncMock()
        mock_qdrant.upsert_chunks = AsyncMock()
        mock_qdrant.upsert_diagrams = AsyncMock()

        kwargs = _make_args()
        result = ingest_pdf(**kwargs)

        assert result["status"] == "complete"
        assert result["chunk_count"] >= 1
        assert result["processor"] == "pdf"
        assert str(kwargs["job_id"]) == result["job_id"]

        # Chunks must be written to both Qdrant and Postgres
        mock_qdrant.upsert_chunks.assert_awaited_once()
        mock_db.insert_chunks.assert_awaited_once()

    # ------------------------------------------------------------------
    # File-not-found error path
    # ------------------------------------------------------------------

    @patch("app.processors.pdf_processor.db")
    @patch("app.processors.pdf_processor._download_from_gcs")
    def test_gcs_file_not_found_propagates(self, mock_download, mock_db):
        """FileNotFoundError from GCS download propagates to the caller.

        The material should be marked FAILED before the exception re-raises.
        """
        from app.models import ProcessingStatus

        mock_download.side_effect = FileNotFoundError(
            "gs://tvtutor-raw-uploads/u/c/j/missing.pdf not found"
        )
        mock_db.update_material_status = AsyncMock()

        kwargs = _make_args(gcs_path="gs://tvtutor-raw-uploads/u/c/j/missing.pdf")

        with pytest.raises(FileNotFoundError, match="missing.pdf not found"):
            ingest_pdf(**kwargs)

        # DB must record FAILED status
        calls = mock_db.update_material_status.call_args_list
        statuses = [c.args[1] for c in calls]
        assert ProcessingStatus.FAILED in statuses

    # ------------------------------------------------------------------
    # Qdrant upsert failure path
    # ------------------------------------------------------------------

    @patch("app.processors.pdf_processor.qdrant")
    @patch("app.processors.pdf_processor.embeddings")
    @patch("app.processors.pdf_processor.db")
    @patch("app.processors.pdf_processor._partition_pdf")
    @patch("app.processors.pdf_processor._is_digital_pdf", return_value=True)
    @patch("app.processors.pdf_processor._download_from_gcs")
    def test_qdrant_upsert_failure_propagates(
        self,
        mock_download,
        mock_digital,
        mock_partition,
        mock_db,
        mock_embed,
        mock_qdrant,
    ):
        """Qdrant upsert error propagates to the caller.

        Ensures pipeline does not silently swallow Qdrant failures and that
        the material is marked FAILED in the database.
        """
        from app.models import ProcessingStatus

        mock_download.return_value = Path("/tmp/fake.pdf")
        mock_partition.return_value = [
            _narrative_element("Vector database stores high-dimensional embeddings.", page=1),
        ]
        mock_embed.embed_texts = AsyncMock(return_value=[[0.2] * 1024])
        mock_db.update_material_status = AsyncMock()
        mock_db.insert_chunks = AsyncMock()
        mock_qdrant.upsert_chunks = AsyncMock(
            side_effect=RuntimeError("Qdrant collection not found")
        )

        with pytest.raises(RuntimeError, match="Qdrant collection not found"):
            ingest_pdf(**_make_args())

        # DB must record FAILED status
        calls = mock_db.update_material_status.call_args_list
        statuses = [c.args[1] for c in calls]
        assert ProcessingStatus.FAILED in statuses

    # ------------------------------------------------------------------
    # Malformed PDF path
    # ------------------------------------------------------------------

    @patch("app.processors.pdf_processor.db")
    @patch("app.processors.pdf_processor._partition_pdf")
    @patch("app.processors.pdf_processor._is_digital_pdf", return_value=False)
    @patch("app.processors.pdf_processor._download_from_gcs")
    def test_malformed_pdf_propagates(
        self,
        mock_download,
        mock_digital,
        mock_partition,
        mock_db,
    ):
        """Exception from PDF parser propagates to the caller.

        Simulates unstructured.io raising when it encounters a corrupt or
        truncated PDF, ensuring the pipeline fails loudly rather than silently
        producing empty output.
        """
        from app.models import ProcessingStatus

        mock_download.return_value = Path("/tmp/corrupt.pdf")
        mock_partition.side_effect = Exception("PDFSyntaxError: unexpected EOF")
        mock_db.update_material_status = AsyncMock()

        with pytest.raises(Exception, match="PDFSyntaxError"):
            ingest_pdf(**_make_args(filename="corrupt.pdf"))

        # DB must record FAILED status
        calls = mock_db.update_material_status.call_args_list
        statuses = [c.args[1] for c in calls]
        assert ProcessingStatus.FAILED in statuses
