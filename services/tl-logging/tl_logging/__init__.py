"""Shared JSON structured logging for TeachersLounge Python services.

Re-exports the public API from :mod:`tl_logging.logging_config` so callers
can import directly from the package root::

    from tl_logging import configure_logging, get_trace_id, set_trace_id
"""
from tl_logging.logging_config import (  # noqa: F401
    _trace_id_ctx,
    configure_logging,
    get_trace_id,
    set_trace_id,
)

__all__ = ["configure_logging", "get_trace_id", "set_trace_id", "_trace_id_ctx"]
