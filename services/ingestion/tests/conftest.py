"""Shared pytest fixtures for the ingestion service test suite."""
from __future__ import annotations

from unittest.mock import patch

import pytest


@pytest.fixture()
def patch_settings():
    """Override Settings fields for the duration of a test.

    Usage::

        def test_something(patch_settings):
            patch_settings(clip_model="stub", figure_min_width=50)
            ...

    Args:
        None — the fixture itself returns a callable.

    Yields:
        A callable ``apply(**overrides)`` that patches ``app.config.settings``
        attributes in-place and restores originals on teardown.
    """
    restore: dict[str, object] = {}

    def apply(**overrides: object) -> None:
        """Apply attribute overrides to the live settings singleton.

        Args:
            **overrides: Keyword arguments mapping settings field names to
                temporary values.
        """
        from app.config import settings

        for key, value in overrides.items():
            restore[key] = getattr(settings, key)
            object.__setattr__(settings, key, value)

    yield apply

    # Teardown — restore originals
    from app.config import settings

    for key, original in restore.items():
        object.__setattr__(settings, key, original)
