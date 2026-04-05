"""Tests for upload endpoint: file type validation and size enforcement."""
import time
import uuid
from io import BytesIO
from unittest.mock import AsyncMock, patch

import pytest
from fastapi.testclient import TestClient
from jose import jwt

from app.config import settings
from app.main import app
from app.models import ACCEPTED_MIME_TYPES

SECRET = "test-jwt-secret"
ALGORITHM = "HS256"
AUDIENCE = "teacherslounge-services"

USER_ID = uuid.uuid4()
COURSE_ID = uuid.uuid4()


def _make_token(
    uid: str | None = None,
    audience: str = AUDIENCE,
    exp_offset: int = 3600,
) -> str:
    """Build a signed HS256 test token matching User Service format.

    Args:
        uid: User UUID string to embed in the ``uid`` claim.  Defaults to
            the module-level ``USER_ID``.
        audience: Audience claim value.  Defaults to the expected audience.
        exp_offset: Seconds from now until expiry.  Defaults to 3600.

    Returns:
        Signed JWT string.
    """
    payload = {
        "aud": audience,
        "uid": uid or str(USER_ID),
        "exp": int(time.time()) + exp_offset,
    }
    return jwt.encode(payload, SECRET, algorithm=ALGORITHM)


@pytest.fixture(autouse=True)
def _patch_jwt_settings(monkeypatch):
    """Override JWT settings so tests use the local test secret."""
    monkeypatch.setattr(settings, "jwt_secret", SECRET)
    monkeypatch.setattr(settings, "jwt_algorithm", ALGORITHM)
    monkeypatch.setattr(settings, "jwt_audience", AUDIENCE)


AUTH_HEADERS = {"Authorization": f"Bearer {_make_token()}"}


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
    @patch("app.routers.ingest.gcs.upload_raw_file", new_callable=AsyncMock, return_value="gs://bucket/path")
    @patch("app.routers.ingest.db.create_material", new_callable=AsyncMock)
    @patch("app.routers.ingest.pubsub.publish_ingest_job", new_callable=AsyncMock)
    def test_accepted_pdf(self, mock_pub, mock_db, mock_gcs, client):
        resp = _upload(client, "application/pdf", filename="notes.pdf")
        assert resp.status_code == 202
        body = resp.json()
        assert body["status"] == "pending"
        assert "job_id" in body
        assert "material_id" in body
        mock_gcs.assert_called_once()
        mock_pub.assert_called_once()

    @patch("app.routers.ingest.gcs.upload_raw_file", new_callable=AsyncMock, return_value="gs://bucket/path")
    @patch("app.routers.ingest.db.create_material", new_callable=AsyncMock)
    @patch("app.routers.ingest.pubsub.publish_ingest_job", new_callable=AsyncMock)
    def test_accepted_docx(self, mock_pub, mock_db, mock_gcs, client):
        mime = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
        resp = _upload(client, mime, filename="essay.docx")
        assert resp.status_code == 202

    @patch("app.routers.ingest.gcs.upload_raw_file", new_callable=AsyncMock, return_value="gs://bucket/path")
    @patch("app.routers.ingest.db.create_material", new_callable=AsyncMock)
    @patch("app.routers.ingest.pubsub.publish_ingest_job", new_callable=AsyncMock)
    def test_accepted_mp4(self, mock_pub, mock_db, mock_gcs, client):
        resp = _upload(client, "video/mp4", filename="lecture.mp4")
        assert resp.status_code == 202

    @patch("app.routers.ingest.gcs.upload_raw_file", new_callable=AsyncMock, return_value="gs://bucket/path")
    @patch("app.routers.ingest.db.create_material", new_callable=AsyncMock)
    @patch("app.routers.ingest.pubsub.publish_ingest_job", new_callable=AsyncMock)
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
                patch("app.routers.ingest.gcs.upload_raw_file", new_callable=AsyncMock, return_value="gs://b/p"),
                patch("app.routers.ingest.db.create_material", new_callable=AsyncMock),
                patch("app.routers.ingest.pubsub.publish_ingest_job", new_callable=AsyncMock),
            ):
                resp = _upload(client, mime)
                assert resp.status_code == 202, f"expected 202 for {mime}, got {resp.status_code}"


class TestSizeEnforcement:
    def test_over_limit_rejected(self, client):
        big_data = b"x" * (501 * 1024 * 1024)  # 501 MB
        with patch("app.config.settings.max_upload_bytes", 500 * 1024 * 1024):
            resp = _upload(client, "application/pdf", data=big_data)
        assert resp.status_code == 413

    @patch("app.routers.ingest.gcs.upload_raw_file", new_callable=AsyncMock, return_value="gs://bucket/path")
    @patch("app.routers.ingest.db.create_material", new_callable=AsyncMock)
    @patch("app.routers.ingest.pubsub.publish_ingest_job", new_callable=AsyncMock)
    def test_at_limit_accepted(self, mock_pub, mock_db, mock_gcs, client):
        exact_data = b"x" * (500 * 1024 * 1024)
        with patch("app.config.settings.max_upload_bytes", 500 * 1024 * 1024):
            resp = _upload(client, "application/pdf", data=exact_data)
        assert resp.status_code == 202


class TestSecurity:
    @patch("app.routers.ingest.gcs.upload_raw_file", new_callable=AsyncMock, return_value="gs://bucket/path")
    @patch("app.routers.ingest.db.create_material", new_callable=AsyncMock)
    @patch("app.routers.ingest.pubsub.publish_ingest_job", new_callable=AsyncMock)
    def test_path_traversal_stripped(self, mock_pub, mock_db, mock_gcs, client):
        """Filenames with directory components must be sanitised before reaching GCS."""
        resp = client.post(
            f"/v1/ingest/upload?course_id={COURSE_ID}",
            headers=AUTH_HEADERS,
            files={"file": ("../../etc/passwd", BytesIO(b"data"), "application/pdf")},
        )
        assert resp.status_code == 202
        # The filename passed to GCS must not contain path separators
        _, kwargs = mock_gcs.call_args
        assert "/" not in kwargs["filename"]
        assert ".." not in kwargs["filename"]

    @patch("app.routers.ingest.gcs.upload_raw_file", new_callable=AsyncMock, return_value="gs://bucket/path")
    @patch("app.routers.ingest.db.create_material", new_callable=AsyncMock)
    @patch("app.routers.ingest.pubsub.publish_ingest_job", new_callable=AsyncMock)
    def test_user_id_stored_in_db(self, mock_pub, mock_db, mock_gcs, client):
        """user_id must be passed to create_material — regression for silent data loss bug."""
        resp = client.post(
            f"/v1/ingest/upload?course_id={COURSE_ID}",
            headers=AUTH_HEADERS,
            files={"file": ("notes.pdf", BytesIO(b"data"), "application/pdf")},
        )
        assert resp.status_code == 202
        _, kwargs = mock_db.call_args
        assert kwargs["user_id"] == USER_ID

    @patch("app.routers.ingest.gcs.upload_raw_file", new_callable=AsyncMock, return_value="gs://bucket/path")
    @patch("app.routers.ingest.db.create_material", new_callable=AsyncMock)
    @patch("app.routers.ingest.pubsub.publish_ingest_job", new_callable=AsyncMock)
    def test_gcs_and_pubsub_awaited(self, mock_pub, mock_db, mock_gcs, client):
        """GCS upload and Pub/Sub publish must be awaited (async), not called synchronously."""
        resp = client.post(
            f"/v1/ingest/upload?course_id={COURSE_ID}",
            headers=AUTH_HEADERS,
            files={"file": ("notes.pdf", BytesIO(b"data"), "application/pdf")},
        )
        assert resp.status_code == 202
        mock_gcs.assert_awaited_once()
        mock_pub.assert_awaited_once()


class TestAuth:
    def test_missing_token_rejected(self, client):
        """No Authorization header → 403 (HTTPBearer auto_error=True)."""
        resp = client.post(
            f"/v1/ingest/upload?course_id={COURSE_ID}",
            files={"file": ("test.pdf", BytesIO(b"data"), "application/pdf")},
        )
        assert resp.status_code == 403

    def test_malformed_token_rejected(self, client):
        """Gibberish Bearer value → 401 (not a valid JWT)."""
        resp = client.post(
            f"/v1/ingest/upload?course_id={COURSE_ID}",
            headers={"Authorization": "Bearer not.a.jwt"},
            files={"file": ("test.pdf", BytesIO(b"data"), "application/pdf")},
        )
        assert resp.status_code == 401

    def test_wrong_audience_rejected(self, client):
        """Token signed for a different audience → 401."""
        token = _make_token(audience="wrong-service")
        resp = client.post(
            f"/v1/ingest/upload?course_id={COURSE_ID}",
            headers={"Authorization": f"Bearer {token}"},
            files={"file": ("test.pdf", BytesIO(b"data"), "application/pdf")},
        )
        assert resp.status_code == 401

    def test_expired_token_rejected(self, client):
        """Expired token → 401."""
        token = _make_token(exp_offset=-1)
        resp = client.post(
            f"/v1/ingest/upload?course_id={COURSE_ID}",
            headers={"Authorization": f"Bearer {token}"},
            files={"file": ("test.pdf", BytesIO(b"data"), "application/pdf")},
        )
        assert resp.status_code == 401

    def test_wrong_secret_rejected(self, client):
        """Token signed with a different secret → 401."""
        bad_token = jwt.encode(
            {"aud": AUDIENCE, "uid": str(USER_ID), "exp": int(time.time()) + 3600},
            "wrong-secret",
            algorithm=ALGORITHM,
        )
        resp = client.post(
            f"/v1/ingest/upload?course_id={COURSE_ID}",
            headers={"Authorization": f"Bearer {bad_token}"},
            files={"file": ("test.pdf", BytesIO(b"data"), "application/pdf")},
        )
        assert resp.status_code == 401

    def test_missing_audience_rejected(self, client):
        """Token with no aud claim → 401."""
        no_aud_token = jwt.encode(
            {"uid": str(USER_ID), "exp": int(time.time()) + 3600},
            SECRET,
            algorithm=ALGORITHM,
        )
        resp = client.post(
            f"/v1/ingest/upload?course_id={COURSE_ID}",
            headers={"Authorization": f"Bearer {no_aud_token}"},
            files={"file": ("test.pdf", BytesIO(b"data"), "application/pdf")},
        )
        assert resp.status_code == 401

    def test_valid_token_accepted(self, client):
        """Well-formed JWT with correct audience → upload proceeds (202)."""
        with (
            patch("app.routers.ingest.gcs.upload_raw_file", new_callable=AsyncMock, return_value="gs://b/p"),
            patch("app.routers.ingest.db.create_material", new_callable=AsyncMock),
            patch("app.routers.ingest.pubsub.publish_ingest_job", new_callable=AsyncMock),
        ):
            resp = client.post(
                f"/v1/ingest/upload?course_id={COURSE_ID}",
                headers={"Authorization": f"Bearer {_make_token()}"},
                files={"file": ("notes.pdf", BytesIO(b"data"), "application/pdf")},
            )
        assert resp.status_code == 202


    def test_missing_uid_and_sub_rejected(self, client):
        """Token with valid aud but no uid or sub claim → 401."""
        no_uid_token = jwt.encode(
            {"aud": AUDIENCE, "exp": int(time.time()) + 3600},
            SECRET,
            algorithm=ALGORITHM,
        )
        resp = client.get(
            f"/v1/ingest/{uuid.uuid4()}/status",
            headers={"Authorization": f"Bearer {no_uid_token}"},
        )
        assert resp.status_code == 401

    def test_non_uuid_uid_rejected(self, client):
        """Token with uid that is not a valid UUID string → 401."""
        bad_uid_token = jwt.encode(
            {"aud": AUDIENCE, "uid": "not-a-valid-uuid", "exp": int(time.time()) + 3600},
            SECRET,
            algorithm=ALGORITHM,
        )
        resp = client.get(
            f"/v1/ingest/{uuid.uuid4()}/status",
            headers={"Authorization": f"Bearer {bad_uid_token}"},
        )
        assert resp.status_code == 401

    def test_sub_claim_accepted_as_fallback(self, client):
        """Token with valid sub but no uid → auth succeeds via sub fallback."""
        sub_token = jwt.encode(
            {"aud": AUDIENCE, "sub": str(USER_ID), "exp": int(time.time()) + 3600},
            SECRET,
            algorithm=ALGORITHM,
        )
        with patch(
            "app.routers.ingest.db.get_material_status",
            new_callable=AsyncMock,
            return_value={"processing_status": "pending", "chunk_count": 0},
        ):
            resp = client.get(
                f"/v1/ingest/{uuid.uuid4()}/status",
                headers={"Authorization": f"Bearer {sub_token}"},
            )
        assert resp.status_code == 200



class TestMaterialStatus:
    """Tests for GET /v1/ingest/{material_id}/status."""

    def test_returns_status_for_known_material(self, client):
        """Known material_id returns 200 with status and chunk_count."""
        material_id = uuid.uuid4()
        with patch(
            "app.routers.ingest.db.get_material_status",
            new_callable=AsyncMock,
            return_value={"processing_status": "complete", "chunk_count": 42},
        ):
            resp = client.get(
                f"/v1/ingest/{material_id}/status",
                headers=AUTH_HEADERS,
            )
        assert resp.status_code == 200
        body = resp.json()
        assert body["material_id"] == str(material_id)
        assert body["processing_status"] == "complete"
        assert body["chunk_count"] == 42

    def test_returns_404_for_unknown_material(self, client):
        """Unknown material_id returns 404."""
        with patch(
            "app.routers.ingest.db.get_material_status",
            new_callable=AsyncMock,
            return_value=None,
        ):
            resp = client.get(
                f"/v1/ingest/{uuid.uuid4()}/status",
                headers=AUTH_HEADERS,
            )
        assert resp.status_code == 404
        assert resp.json()["detail"] == "material not found"

    def test_pending_status_returned(self, client):
        """Freshly-uploaded material shows processing_status=pending."""
        material_id = uuid.uuid4()
        with patch(
            "app.routers.ingest.db.get_material_status",
            new_callable=AsyncMock,
            return_value={"processing_status": "pending", "chunk_count": 0},
        ):
            resp = client.get(f"/v1/ingest/{material_id}/status", headers=AUTH_HEADERS)
        assert resp.status_code == 200
        assert resp.json()["processing_status"] == "pending"

    def test_failed_status_returned(self, client):
        """Failed processing returns processing_status=failed with chunk_count=0."""
        material_id = uuid.uuid4()
        with patch(
            "app.routers.ingest.db.get_material_status",
            new_callable=AsyncMock,
            return_value={"processing_status": "failed", "chunk_count": 0},
        ):
            resp = client.get(f"/v1/ingest/{material_id}/status", headers=AUTH_HEADERS)
        assert resp.status_code == 200
        assert resp.json()["processing_status"] == "failed"

    def test_requires_auth(self, client):
        """Status endpoint requires a valid Bearer token."""
        resp = client.get(f"/v1/ingest/{uuid.uuid4()}/status")
        assert resp.status_code == 403
