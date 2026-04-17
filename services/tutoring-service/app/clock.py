"""Clock dependency — injectable UTC time source for deterministic tests.

Routes that compute due dates or compare timestamps depend on :func:`get_clock`
rather than calling ``datetime.now`` directly. Tests override ``get_clock`` via
``app.dependency_overrides`` to freeze time.
"""

from __future__ import annotations

from datetime import datetime, timezone
from typing import Callable

Clock = Callable[[], datetime]


def _utc_now() -> datetime:
    """Return the current UTC time; the default production clock."""
    return datetime.now(timezone.utc)


def get_clock() -> Clock:
    """FastAPI dependency returning the active clock callable.

    Production callers receive :func:`_utc_now`. Tests may override this
    dependency to inject a deterministic clock::

        app.dependency_overrides[get_clock] = lambda: lambda: frozen_dt
    """
    return _utc_now
