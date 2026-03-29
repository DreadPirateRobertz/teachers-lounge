"""Tests for upload endpoint: file type validation and size enforcement."""
import uuid
from io import BytesIO
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
from fastapi.testclient import TestClient

from app.main import app
from app.models import ACCEPTED_MIME_TYPES

# Use a fixed user UUID as the stub Bearer token
USER_ID = uuid.uuid4()
COURSE_ID = uuid.uuid4()
AUTH_HEADERS = {"Authorization": f"Bearer {USER_ID}"}


@pytest.fixture
def client():
    with TestClient(app) as c:
        yield c


def _upload(client, content_type: str, data: bytes = b"fake-content", filename: str = "test.pdf"):
    return client.post(
        f"/v1/ingest/upload?course_id={COURSE_ID}",
        headers=AUTH_HEADERS,
        files={"file": (filename, BytesIO(data), content_type)},
    )


class TestFileTypeValidation:
    @patch("app.routers.ingest.gcs.upload_raw_file", return_value="gs://bucket/path")
    @patch("app.routers.ingest.db.create_material", new_callable=AsyncMock)
    @patch("app.routers.ingest.pubsub.publish_ingest_job")
    def test_accepted_pdf(self, mock_pub, mock_db, mock_gcs, client):
        resp = _upload(client, "application/pdf", filename="notes.pdf")
        assert resp.status_code == 202
        body = resp.json()
        assert body["status"] == "pending"
        assert "job_id" in body
        assert "material_id" in body
        mock_gcs.assert_called_once()
        mock_pub.assert_called_once()

    @patch("app.routers.ingest.gcs.upload_raw_file", return_value="gs://bucket/path")
    @patch("app.routers.ingest.db.create_material", new_callable=AsyncMock)
    @patch("app.routers.ingest.pubsub.publish_ingest_job")
    def test_accepted_docx(self, mock_pub, mock_db, mock_gcs, client):
        mime = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
        resp = _upload(client, mime, filename="essay.docx")
        assert resp.status_code == 202

    @patch("app.routers.ingest.gcs.upload_raw_file", return_value="gs://bucket/path")
    @patch("app.routers.ingest.db.create_material", new_callable=AsyncMock)
    @patch("app.routers.ingest.pubsub.publish_ingest_job")
    def test_accepted_mp4(self, mock_pub, mock_db, mock_gcs, client):
        resp = _upload(client, "video/mp4", filename="lecture.mp4")
        assert resp.status_code == 202

    @patch("app.routers.ingest.gcs.upload_raw_file", return_value="gs://bucket/path")
    @patch("app.routers.ingest.db.create_material", new_callable=AsyncMock)
    @patch("app.routers.ingest.pubsub.publish_ingest_job")
    def test_accepted_jpeg(self, mock_pub, mock_db, mock_gcs, client):
        resp = _upload(client, "image/jpeg", filename="scan.jpg")
        assert resp.status_code == 202

    def test_rejected_txt(self, client):
        resp = _upload(client, "text/plain", filename="notes.txt")
        assert resp.status_code == 415

    def test_rejected_exe(self, client):
        resp = _upload(client, "application/octet-stream", filename="malware.exe")
        assert resp.status_code == 415

    def test_rejected_html(self, client):
        resp = _upload(client, "text/html", filename="page.html")
        assert resp.status_code == 415

    def test_all_accepted_types_pass_validation(self, client):
        """Smoke test: every ACCEPTED_MIME_TYPE must return 202, not 415."""
        for mime in ACCEPTED_MIME_TYPES:
            with (
                patch("app.routers.ingest.gcs.upload_raw_file", return_value="gs://b/p"),
                patch("app.routers.ingest.db.create_material", new_callable=AsyncMock),
                patch("app.routers.ingest.pubsub.publish_ingest_job"),
            ):
                resp = _upload(client, mime)
                assert resp.status_code == 202, f"expected 202 for {mime}, got {resp.status_code}"


class TestSizeEnforcement:
    def test_over_limit_rejected(self, client):
        big_data = b"x" * (501 * 1024 * 1024)  # 501 MB
        with patch("app.config.settings.max_upload_bytes", 500 * 1024 * 1024):
            resp = _upload(client, "application/pdf", data=big_data)
        assert resp.status_code == 413

    @patch("app.routers.ingest.gcs.upload_raw_file", return_value="gs://bucket/path")
    @patch("app.routers.ingest.db.create_material", new_callable=AsyncMock)
    @patch("app.routers.ingest.pubsub.publish_ingest_job")
    def test_at_limit_accepted(self, mock_pub, mock_db, mock_gcs, client):
        exact_data = b"x" * (500 * 1024 * 1024)
        with patch("app.config.settings.max_upload_bytes", 500 * 1024 * 1024):
            resp = _upload(client, "application/pdf", data=exact_data)
        assert resp.status_code == 202


class TestAuth:
    def test_missing_token_rejected(self, client):
        resp = client.post(
            f"/v1/ingest/upload?course_id={COURSE_ID}",
            files={"file": ("test.pdf", BytesIO(b"data"), "application/pdf")},
        )
        assert resp.status_code == 403

    def test_non_uuid_token_rejected(self, client):
        resp = client.post(
            f"/v1/ingest/upload?course_id={COURSE_ID}",
            headers={"Authorization": "Bearer not-a-uuid"},
            files={"file": ("test.pdf", BytesIO(b"data"), "application/pdf")},
        )
        assert resp.status_code == 401
