"""Unit tests for app.services.db — asyncpg interactions tested via mocks.

All tests mock ``app.services.db._connect`` to return an AsyncMock connection
so no real Postgres connection is required.
"""
from __future__ import annotations

import uuid
from unittest.mock import AsyncMock, MagicMock, call, patch

import pytest

from app.models import ProcessingStatus
from app.services.db import (
    create_material,
    get_material_status,
    insert_chunks,
    update_material_status,
)


@pytest.fixture()
def mock_conn():
    """Return a mock asyncpg connection with all async methods stubbed.

    Returns:
        MagicMock with ``execute``, ``executemany``, ``fetchrow``, and
        ``close`` as ``AsyncMock`` instances.
    """
    conn = MagicMock()
    conn.execute = AsyncMock()
    conn.executemany = AsyncMock()
    conn.fetchrow = AsyncMock()
    conn.close = AsyncMock()
    return conn


class TestGetMaterialStatus:
    """Unit tests for get_material_status(material_id)."""

    async def test_returns_dict_when_row_found(self, mock_conn):
        """Existing material_id returns processing_status and chunk_count dict."""
        material_id = uuid.uuid4()
        mock_conn.fetchrow.return_value = {
            "processing_status": "complete",
            "chunk_count": 87,
        }
        with patch("app.services.db._connect", AsyncMock(return_value=mock_conn)):
            result = await get_material_status(material_id)

        assert result == {"processing_status": "complete", "chunk_count": 87}
        mock_conn.fetchrow.assert_awaited_once()
        mock_conn.close.assert_awaited_once()

    async def test_returns_none_when_row_missing(self, mock_conn):
        """Unknown material_id returns None (no row in materials table)."""
        mock_conn.fetchrow.return_value = None
        with patch("app.services.db._connect", AsyncMock(return_value=mock_conn)):
            result = await get_material_status(uuid.uuid4())

        assert result is None
        mock_conn.close.assert_awaited_once()

    async def test_connection_closed_on_fetchrow_error(self, mock_conn):
        """Connection is closed even when fetchrow raises an exception."""
        mock_conn.fetchrow.side_effect = RuntimeError("db error")
        with patch("app.services.db._connect", AsyncMock(return_value=mock_conn)):
            with pytest.raises(RuntimeError, match="db error"):
                await get_material_status(uuid.uuid4())

        mock_conn.close.assert_awaited_once()

    async def test_correct_query_parameters_passed(self, mock_conn):
        """get_material_status passes the material_id UUID to fetchrow."""
        material_id = uuid.uuid4()
        mock_conn.fetchrow.return_value = {
            "processing_status": "pending",
            "chunk_count": 0,
        }
        with patch("app.services.db._connect", AsyncMock(return_value=mock_conn)):
            await get_material_status(material_id)

        call_args = mock_conn.fetchrow.call_args
        # Second positional arg is the material_id parameter
        assert call_args[0][1] == material_id


class TestCreateMaterial:
    """Unit tests for create_material()."""

    async def test_inserts_row_and_closes_connection(self, mock_conn):
        """create_material executes an INSERT and closes the connection."""
        material_id = uuid.uuid4()
        course_id = uuid.uuid4()
        user_id = uuid.uuid4()

        with patch("app.services.db._connect", AsyncMock(return_value=mock_conn)):
            await create_material(
                material_id=material_id,
                course_id=course_id,
                user_id=user_id,
                filename="notes.pdf",
                gcs_path="gs://bucket/path/notes.pdf",
                file_type="pdf",
            )

        mock_conn.execute.assert_awaited_once()
        mock_conn.close.assert_awaited_once()

    async def test_execute_receives_correct_args(self, mock_conn):
        """All kwargs are forwarded to the INSERT statement in correct order."""
        material_id = uuid.uuid4()
        course_id = uuid.uuid4()
        user_id = uuid.uuid4()

        with patch("app.services.db._connect", AsyncMock(return_value=mock_conn)):
            await create_material(
                material_id=material_id,
                course_id=course_id,
                user_id=user_id,
                filename="lecture.mp4",
                gcs_path="gs://bucket/lecture.mp4",
                file_type="video",
            )

        positional = mock_conn.execute.call_args[0]
        # SQL is first arg; subsequent args are the bind parameters
        assert positional[1] == material_id
        assert positional[2] == course_id
        assert positional[3] == user_id
        assert positional[4] == "lecture.mp4"
        assert positional[5] == "gs://bucket/lecture.mp4"
        assert positional[6] == "video"
        # Status is always PENDING on creation
        assert positional[7] == ProcessingStatus.PENDING

    async def test_connection_closed_on_execute_error(self, mock_conn):
        """Connection is closed even when execute raises."""
        mock_conn.execute.side_effect = RuntimeError("db unavailable")

        with patch("app.services.db._connect", AsyncMock(return_value=mock_conn)):
            with pytest.raises(RuntimeError, match="db unavailable"):
                await create_material(
                    material_id=uuid.uuid4(),
                    course_id=uuid.uuid4(),
                    user_id=uuid.uuid4(),
                    filename="f.pdf",
                    gcs_path="gs://b/f.pdf",
                    file_type="pdf",
                )

        mock_conn.close.assert_awaited_once()


class TestUpdateMaterialStatus:
    """Unit tests for update_material_status()."""

    async def test_updates_status_without_chunk_count(self, mock_conn):
        """Omitting chunk_count issues the 2-parameter UPDATE."""
        material_id = uuid.uuid4()

        with patch("app.services.db._connect", AsyncMock(return_value=mock_conn)):
            await update_material_status(material_id, ProcessingStatus.PROCESSING)

        mock_conn.execute.assert_awaited_once()
        positional = mock_conn.execute.call_args[0]
        assert positional[1] == ProcessingStatus.PROCESSING
        assert positional[2] == material_id
        mock_conn.close.assert_awaited_once()

    async def test_updates_status_with_chunk_count(self, mock_conn):
        """Providing chunk_count issues the 3-parameter UPDATE."""
        material_id = uuid.uuid4()

        with patch("app.services.db._connect", AsyncMock(return_value=mock_conn)):
            await update_material_status(material_id, ProcessingStatus.COMPLETE, chunk_count=42)

        positional = mock_conn.execute.call_args[0]
        assert positional[1] == ProcessingStatus.COMPLETE
        assert positional[2] == 42
        assert positional[3] == material_id
        mock_conn.close.assert_awaited_once()

    async def test_connection_closed_on_execute_error(self, mock_conn):
        """Connection is closed even when execute raises."""
        mock_conn.execute.side_effect = RuntimeError("timeout")

        with patch("app.services.db._connect", AsyncMock(return_value=mock_conn)):
            with pytest.raises(RuntimeError):
                await update_material_status(uuid.uuid4(), ProcessingStatus.FAILED)

        mock_conn.close.assert_awaited_once()


class TestInsertChunks:
    """Unit tests for insert_chunks()."""

    async def test_empty_list_is_noop(self, mock_conn):
        """Calling with an empty list does not open a DB connection."""
        with patch("app.services.db._connect", AsyncMock(return_value=mock_conn)) as mock_connect:
            await insert_chunks([])

        mock_connect.assert_not_awaited()
        mock_conn.executemany.assert_not_awaited()

    async def test_inserts_all_chunks(self, mock_conn):
        """executemany is called once with all chunk rows."""
        material_id = uuid.uuid4()
        course_id = uuid.uuid4()
        chunks = [
            {
                "id": uuid.uuid4(),
                "material_id": material_id,
                "course_id": course_id,
                "content": "Organic chemistry intro.",
                "chapter": "Chapter 1",
                "section": "1.1",
                "page": 1,
                "content_type": "text",
                "figure_gcs_path": None,
                "metadata": {},
            },
            {
                "id": uuid.uuid4(),
                "material_id": material_id,
                "course_id": course_id,
                "content": "Alkanes and alkenes.",
                "chapter": "Chapter 1",
                "section": "1.2",
                "page": 3,
                "content_type": "text",
                "figure_gcs_path": None,
                "metadata": {"difficulty": 0.3},
            },
        ]

        with patch("app.services.db._connect", AsyncMock(return_value=mock_conn)):
            await insert_chunks(chunks)

        mock_conn.executemany.assert_awaited_once()
        _, rows = mock_conn.executemany.call_args[0]
        assert len(rows) == 2
        mock_conn.close.assert_awaited_once()

    async def test_connection_closed_on_executemany_error(self, mock_conn):
        """Connection is closed even when executemany raises."""
        mock_conn.executemany.side_effect = RuntimeError("constraint violation")
        chunk = {
            "id": uuid.uuid4(),
            "material_id": uuid.uuid4(),
            "course_id": uuid.uuid4(),
            "content": "test",
            "chapter": None,
            "section": None,
            "page": 1,
            "content_type": "text",
            "figure_gcs_path": None,
            "metadata": {},
        }

        with patch("app.services.db._connect", AsyncMock(return_value=mock_conn)):
            with pytest.raises(RuntimeError):
                await insert_chunks([chunk])

        mock_conn.close.assert_awaited_once()
