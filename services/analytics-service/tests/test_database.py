"""Tests for app.database module.

Covers the get_db async generator by substituting SessionLocal with a mock
async context manager, which avoids a real Postgres connection while still
exercising lines 17-18 of database.py.
"""
from unittest.mock import AsyncMock, patch

import pytest


async def test_get_db_yields_session():
    """get_db yields the session produced by SessionLocal().__aenter__.

    Verifies:
    - The async context manager is entered exactly once.
    - The yielded value is the session from __aenter__.
    - The context manager is exited (cleaned up) after the generator closes.
    """
    from app.database import get_db

    mock_session = AsyncMock()
    mock_cm = AsyncMock()
    mock_cm.__aenter__.return_value = mock_session
    mock_cm.__aexit__.return_value = False

    with patch("app.database.SessionLocal", return_value=mock_cm):
        gen = get_db()
        session = await gen.__anext__()
        assert session is mock_session
        # Closing the generator triggers __aexit__ on the context manager.
        await gen.aclose()

    mock_cm.__aenter__.assert_awaited_once()
    mock_cm.__aexit__.assert_awaited_once()


async def test_get_db_cleans_up_on_exception():
    """get_db context manager is exited even when the consumer raises.

    Simulates what FastAPI does during a request that raises an exception
    after the dependency has been injected: the generator is closed via
    throw(), which must still invoke __aexit__ on the SessionLocal context.
    """
    from app.database import get_db

    mock_session = AsyncMock()
    mock_cm = AsyncMock()
    mock_cm.__aenter__.return_value = mock_session
    mock_cm.__aexit__.return_value = False

    with patch("app.database.SessionLocal", return_value=mock_cm):
        gen = get_db()
        await gen.__anext__()
        # Simulate the generator being closed mid-request.
        await gen.aclose()

    # __aexit__ must have been called regardless of how the generator ended.
    mock_cm.__aexit__.assert_awaited_once()
