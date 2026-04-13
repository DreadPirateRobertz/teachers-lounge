"""Tests for the eval-service database connection module.

Covers:
  - get_session: yields an AsyncSession via the session factory
  - close_engine: awaits engine.dispose()
"""
from unittest.mock import AsyncMock, MagicMock, patch

import pytest


class TestGetSession:
    """Unit tests for the get_session async context manager."""

    @pytest.mark.asyncio
    async def test_yields_session_from_factory(self):
        """get_session yields the session produced by the session factory."""
        from app.database import get_session

        mock_session = AsyncMock()
        mock_ctx = MagicMock()
        mock_ctx.__aenter__ = AsyncMock(return_value=mock_session)
        mock_ctx.__aexit__ = AsyncMock(return_value=None)

        with patch("app.database._session_factory", return_value=mock_ctx):
            async with get_session() as session:
                assert session is mock_session


class TestCloseEngine:
    """Unit tests for the close_engine shutdown helper."""

    @pytest.mark.asyncio
    async def test_calls_engine_dispose(self):
        """close_engine awaits _engine.dispose() to release pool connections."""
        from app.database import close_engine

        mock_engine = AsyncMock()

        with patch("app.database._engine", mock_engine):
            await close_engine()

        mock_engine.dispose.assert_awaited_once()
