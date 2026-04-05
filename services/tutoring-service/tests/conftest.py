"""
Shared test fixtures for the tutoring service.
"""
from unittest.mock import MagicMock

import pytest

from app.config import Settings, settings
from app.database import get_db
from app.gateway import reset_gateway_client
from app.main import app


@pytest.fixture(autouse=True)
def disable_lifespan_events(monkeypatch):
    """Prevent app startup/shutdown from attempting real DB connections in CI.

    The startup handler calls engine.begin() to run Alembic metadata.create_all,
    which requires a live PostgreSQL connection.  Tests that use AsyncClient with
    ASGITransport trigger lifespan events, so we clear them here.
    """
    monkeypatch.setattr(app.router, "on_startup", [])
    monkeypatch.setattr(app.router, "on_shutdown", [])


@pytest.fixture()
def patch_settings(monkeypatch):
    """
    Override settings values for a single test without mutating the module-level
    singleton.  Returns the patched settings object.

    Usage:
        def test_something(patch_settings):
            patch_settings(jwt_secret="test-secret")
    """
    def _patch(**kwargs):
        for key, value in kwargs.items():
            monkeypatch.setattr(settings, key, value)
        return settings

    return _patch


@pytest.fixture(autouse=True)
def override_get_db():
    """Override get_db with a no-op mock so tests never open a real DB connection.

    Tests that need specific DB behavior (e.g. test_reviews.py) can write to
    app.dependency_overrides[get_db] directly — their override takes precedence.
    This fixture handles teardown (clears the override) for all tests.
    """
    async def _null_db():
        yield MagicMock()

    app.dependency_overrides[get_db] = _null_db
    yield
    app.dependency_overrides.pop(get_db, None)


@pytest.fixture(autouse=True)
def reset_singleton():
    """
    Reset the gateway client singleton before every test so that tests which
    patch settings don't share a stale client with a different base_url/key.
    """
    reset_gateway_client()
    yield
    reset_gateway_client()
