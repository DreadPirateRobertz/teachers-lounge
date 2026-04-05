"""Shared pytest fixtures for the analytics service test suite.

Uses FastAPI's TestClient with dependency overrides to avoid real database or
JWT infrastructure.  All database calls are replaced by an AsyncMock whose
return values are configured per-test.
"""
import pytest
from fastapi.testclient import TestClient
from jose import jwt
from unittest.mock import AsyncMock, MagicMock, patch

from app.main import app
from app.config import settings
from app.database import get_db
from app.auth import require_auth

# ── Helpers ──────────────────────────────────────────────────────────────────

TEST_USER_ID = "00000000-0000-0000-0000-000000000001"
OTHER_USER_ID = "00000000-0000-0000-0000-000000000002"


def make_token(user_id: str = TEST_USER_ID) -> str:
    """Create a signed JWT for use in test Authorization headers.

    Args:
        user_id: The ``sub`` claim to embed in the token.

    Returns:
        A JWT string signed with the test JWT_SECRET.
    """
    return jwt.encode(
        {"sub": user_id, "aud": settings.jwt_audience},
        settings.jwt_secret,
        algorithm=settings.jwt_algorithm,
    )


def auth_headers(user_id: str = TEST_USER_ID) -> dict[str, str]:
    """Return an Authorization header dict for the given user.

    Args:
        user_id: UUID to embed in the JWT.

    Returns:
        Dict suitable for passing as ``headers=`` to TestClient requests.
    """
    return {"Authorization": f"Bearer {make_token(user_id)}"}


# ── DB mock factory ──────────────────────────────────────────────────────────

def make_db_override(execute_side_effects: list):
    """Build a get_db dependency override with controlled execute results.

    Each item in ``execute_side_effects`` is the return value for one
    successive call to ``db.execute()``.  Items can be MagicMocks with a
    ``.mappings().first()`` or ``.mappings().all()`` chain already configured.

    Args:
        execute_side_effects: Ordered list of mock result objects to return
            from ``db.execute``.

    Returns:
        An async generator function compatible with FastAPI's ``Depends``.
    """
    async def _override():
        mock_db = AsyncMock()
        mock_db.execute.side_effect = execute_side_effects
        yield mock_db

    return _override


# ── Fixtures ─────────────────────────────────────────────────────────────────

@pytest.fixture()
def client():
    """Yield a TestClient with no dependency overrides applied.

    Tests that need DB or auth overrides should apply them via
    ``app.dependency_overrides`` directly.

    Yields:
        A ``TestClient`` wrapping the analytics FastAPI app.
    """
    with TestClient(app) as c:
        yield c
