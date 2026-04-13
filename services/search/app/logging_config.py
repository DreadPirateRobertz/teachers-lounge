"""JSON structured logging configuration for search — delegates to tl_logging."""
from tl_logging.logging_config import (  # noqa: F401
    _trace_id_ctx,
    configure_logging,
    get_trace_id,
    set_trace_id,
)

__all__ = ["configure_logging", "get_trace_id", "set_trace_id", "_trace_id_ctx"]
