"""Unit tests for app/services/gcs.py — all GCS operations mocked.

Covers:
  - _upload_raw_file_sync: happy path, blob metadata, exception propagation
  - upload_raw_file: async wrapper, path-traversal stripping
  - _upload_figure_sync: happy path, exception propagation
  - upload_figure: async wrapper
  - _download_file_sync: happy path, exception propagation
"""
from __future__ import annotations

import uuid
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

_USER_ID = uuid.UUID("aaaaaaaa-0000-0000-0000-000000000001")
_COURSE_ID = uuid.UUID("bbbbbbbb-0000-0000-0000-000000000002")
_JOB_ID = uuid.UUID("cccccccc-0000-0000-0000-000000000003")

_GCS_CLIENT = "google.cloud.storage.Client"


def _make_blob_chain(mock_client_class: MagicMock) -> MagicMock:
    """Wire up client → bucket → blob and return the blob mock.

    Args:
        mock_client_class: The patched Client class mock.

    Returns:
        The MagicMock representing the GCS blob object.
    """
    mock_blob = MagicMock()
    mock_bucket = MagicMock()
    mock_bucket.blob.return_value = mock_blob
    mock_client_class.return_value.bucket.return_value = mock_bucket
    return mock_blob


# ── _upload_raw_file_sync ─────────────────────────────────────────────────────


class TestUploadRawFileSync:
    """Tests for _upload_raw_file_sync."""

    def test_returns_gcs_uri(self, patch_settings):
        """Happy path: must return a gs:// URI containing bucket and blob name.

        Args:
            patch_settings: Fixture that overrides settings for the test.
        """
        patch_settings(gcp_project="test-proj", gcs_raw_bucket="raw-bucket")
        from app.services.gcs import _upload_raw_file_sync

        with patch(_GCS_CLIENT) as mock_client_class:
            _make_blob_chain(mock_client_class)
            result = _upload_raw_file_sync(
                b"pdfbytes", _USER_ID, _COURSE_ID, _JOB_ID, "lecture.pdf", "application/pdf"
            )

        assert result.startswith("gs://raw-bucket/")
        assert str(_USER_ID) in result
        assert str(_COURSE_ID) in result
        assert str(_JOB_ID) in result
        assert "lecture.pdf" in result

    def test_upload_from_string_called_with_correct_args(self, patch_settings):
        """upload_from_string must receive the raw bytes and content_type.

        Args:
            patch_settings: Fixture that overrides settings for the test.
        """
        patch_settings(gcp_project="test-proj", gcs_raw_bucket="raw-bucket")
        from app.services.gcs import _upload_raw_file_sync

        data = b"hello pdf content"
        with patch(_GCS_CLIENT) as mock_client_class:
            mock_blob = _make_blob_chain(mock_client_class)
            _upload_raw_file_sync(
                data, _USER_ID, _COURSE_ID, _JOB_ID, "notes.pdf", "application/pdf"
            )

        mock_blob.upload_from_string.assert_called_once_with(data, content_type="application/pdf")

    def test_blob_metadata_set(self, patch_settings):
        """Blob metadata must include user_id, course_id, and job_id strings.

        Args:
            patch_settings: Fixture that overrides settings for the test.
        """
        patch_settings(gcp_project="test-proj", gcs_raw_bucket="raw-bucket")
        from app.services.gcs import _upload_raw_file_sync

        with patch(_GCS_CLIENT) as mock_client_class:
            mock_blob = _make_blob_chain(mock_client_class)
            _upload_raw_file_sync(b"x", _USER_ID, _COURSE_ID, _JOB_ID, "f.pdf", "application/pdf")

        assert mock_blob.metadata["user_id"] == str(_USER_ID)
        assert mock_blob.metadata["course_id"] == str(_COURSE_ID)
        assert mock_blob.metadata["job_id"] == str(_JOB_ID)

    def test_propagates_storage_exception(self, patch_settings):
        """A GCS error during upload must propagate to the caller.

        Args:
            patch_settings: Fixture that overrides settings for the test.
        """
        patch_settings(gcp_project="test-proj", gcs_raw_bucket="raw-bucket")
        from app.services.gcs import _upload_raw_file_sync

        with patch(_GCS_CLIENT) as mock_client_class:
            mock_blob = _make_blob_chain(mock_client_class)
            mock_blob.upload_from_string.side_effect = Exception("GCS unavailable")

            with pytest.raises(Exception, match="GCS unavailable"):
                _upload_raw_file_sync(
                    b"x", _USER_ID, _COURSE_ID, _JOB_ID, "f.pdf", "application/pdf"
                )


# ── upload_raw_file (async) ───────────────────────────────────────────────────


class TestUploadRawFile:
    """Tests for the async upload_raw_file wrapper."""

    @pytest.mark.asyncio
    async def test_returns_gcs_uri(self, patch_settings):
        """Async wrapper must return the gs:// URI from the sync helper.

        Args:
            patch_settings: Fixture that overrides settings for the test.
        """
        patch_settings(gcp_project="test-proj", gcs_raw_bucket="raw-bucket")
        from app.services.gcs import upload_raw_file

        with patch(_GCS_CLIENT) as mock_client_class:
            _make_blob_chain(mock_client_class)
            result = await upload_raw_file(
                b"bytes", _USER_ID, _COURSE_ID, _JOB_ID, "slides.pdf", "application/pdf"
            )

        assert result.startswith("gs://raw-bucket/")

    @pytest.mark.asyncio
    async def test_strips_directory_traversal_from_filename(self, patch_settings):
        """Path components in the filename must be stripped before the blob name.

        Args:
            patch_settings: Fixture that overrides settings for the test.
        """
        patch_settings(gcp_project="test-proj", gcs_raw_bucket="raw-bucket")
        from app.services.gcs import upload_raw_file

        with patch(_GCS_CLIENT) as mock_client_class:
            _make_blob_chain(mock_client_class)
            result = await upload_raw_file(
                b"bytes", _USER_ID, _COURSE_ID, _JOB_ID, "../../etc/passwd", "text/plain"
            )

        assert "passwd" in result
        assert ".." not in result


# ── _upload_figure_sync ───────────────────────────────────────────────────────


class TestUploadFigureSync:
    """Tests for _upload_figure_sync."""

    def test_returns_gcs_uri(self, patch_settings, tmp_path):
        """Happy path: must return gs:// URI for the figure bucket.

        Args:
            patch_settings: Fixture that overrides settings for the test.
            tmp_path: pytest built-in temp directory.
        """
        patch_settings(gcp_project="test-proj", gcs_figures_bucket="figures-bucket")
        from app.services.gcs import _upload_figure_sync

        img = tmp_path / "fig_001.png"
        img.write_bytes(b"\x89PNG\r\n")

        with patch(_GCS_CLIENT) as mock_client_class:
            _make_blob_chain(mock_client_class)
            result = _upload_figure_sync(img, "course/job/fig_001.png")

        assert result == "gs://figures-bucket/course/job/fig_001.png"

    def test_upload_from_filename_called_with_correct_args(self, patch_settings, tmp_path):
        """upload_from_filename must be called with the image path and image/png.

        Args:
            patch_settings: Fixture that overrides settings for the test.
            tmp_path: pytest built-in temp directory.
        """
        patch_settings(gcp_project="test-proj", gcs_figures_bucket="figures-bucket")
        from app.services.gcs import _upload_figure_sync

        img = tmp_path / "fig.png"
        img.write_bytes(b"\x89PNG\r\n")

        with patch(_GCS_CLIENT) as mock_client_class:
            mock_blob = _make_blob_chain(mock_client_class)
            _upload_figure_sync(img, "some/blob/name.png")

        mock_blob.upload_from_filename.assert_called_once_with(
            str(img), content_type="image/png"
        )

    def test_propagates_exception(self, patch_settings, tmp_path):
        """A GCS error during figure upload must propagate to the caller.

        Args:
            patch_settings: Fixture that overrides settings for the test.
            tmp_path: pytest built-in temp directory.
        """
        patch_settings(gcp_project="test-proj", gcs_figures_bucket="figures-bucket")
        from app.services.gcs import _upload_figure_sync

        img = tmp_path / "fig.png"
        img.write_bytes(b"\x89PNG\r\n")

        with patch(_GCS_CLIENT) as mock_client_class:
            mock_blob = _make_blob_chain(mock_client_class)
            mock_blob.upload_from_filename.side_effect = OSError("network failure")

            with pytest.raises(OSError, match="network failure"):
                _upload_figure_sync(img, "some/blob/name.png")


# ── upload_figure (async) ─────────────────────────────────────────────────────


class TestUploadFigure:
    """Tests for the async upload_figure wrapper."""

    @pytest.mark.asyncio
    async def test_returns_gcs_uri(self, patch_settings, tmp_path):
        """Async wrapper must return the gs:// URI from the sync helper.

        Args:
            patch_settings: Fixture that overrides settings for the test.
            tmp_path: pytest built-in temp directory.
        """
        patch_settings(gcp_project="test-proj", gcs_figures_bucket="figures-bucket")
        from app.services.gcs import upload_figure

        img = tmp_path / "fig.png"
        img.write_bytes(b"\x89PNG\r\n")

        with patch(_GCS_CLIENT) as mock_client_class:
            _make_blob_chain(mock_client_class)
            result = await upload_figure(img, "mat/job/fig.png")

        assert result == "gs://figures-bucket/mat/job/fig.png"


# ── _download_file_sync ───────────────────────────────────────────────────────


class TestDownloadFileSync:
    """Tests for _download_file_sync."""

    def test_returns_path_to_temp_file(self, patch_settings):
        """Happy path: must return a Path after download_to_filename is called.

        Args:
            patch_settings: Fixture that overrides settings for the test.
        """
        patch_settings(gcp_project="test-proj")
        from app.services.gcs import _download_file_sync

        with patch(_GCS_CLIENT) as mock_client_class:
            mock_blob = _make_blob_chain(mock_client_class)

            def fake_download(path: str) -> None:
                """Write dummy bytes to the destination path."""
                Path(path).write_bytes(b"PDF content")

            mock_blob.download_to_filename.side_effect = fake_download

            result = _download_file_sync(
                "gs://raw-bucket/user/course/job/file.pdf", _JOB_ID
            )

        assert isinstance(result, Path)
        assert result.exists()
        result.unlink(missing_ok=True)

    def test_propagates_download_exception(self, patch_settings):
        """A GCS error during download must propagate to the caller.

        Args:
            patch_settings: Fixture that overrides settings for the test.
        """
        patch_settings(gcp_project="test-proj")
        from app.services.gcs import _download_file_sync

        with patch(_GCS_CLIENT) as mock_client_class:
            mock_blob = _make_blob_chain(mock_client_class)
            mock_blob.download_to_filename.side_effect = Exception("download failed")

            with pytest.raises(Exception, match="download failed"):
                _download_file_sync(
                    "gs://raw-bucket/user/course/job/file.pdf", _JOB_ID
                )
