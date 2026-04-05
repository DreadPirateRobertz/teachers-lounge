"""JSON structured logging configuration for ingestion.

Configures the Python root logger to emit one JSON object per line with the
required fields: timestamp, level, service, trace_id, message.

A per-request trace ID is stored in a contextvars.ContextVar so that every
log record emitted within a request automatically carries the correct ID
without requiring callers to pass it explicitly.

Usage::

    from app.logging_config import configure_logging, set_trace_id

    # At application startup:
    configure_logging(service_name="tutoring-service", log_level="INFO")

    # In request middleware:
    token = set_trace_id(request.headers.get("X-Trace-ID", str(uuid.uuid4())))
    try:
        response = await call_next(request)
    finally:
        _trace_id_ctx.reset(token)
"""
import logging
import sys
from contextvars import ContextVar, Token
from typing import IO

from pythonjsonlogger.json import JsonFormatter

# Module-level context variable holding the active trace ID for the current
# async task / thread.  Empty string means "not set"; get_trace_id() fills it.
_trace_id_ctx: ContextVar[str] = ContextVar("trace_id", default="")

_NO_TRACE = "no-trace"


def get_trace_id() -> str:
    """Return the trace ID for the current request context.

    Returns the value set by set_trace_id(), or the sentinel string
    ``"no-trace"`` when no trace ID has been established.

    Returns:
        The active trace ID string, never empty.
    """
    return _trace_id_ctx.get() or _NO_TRACE


def set_trace_id(trace_id: str) -> Token[str]:
    """Store a trace ID in the current async context.

    The returned token must be passed to ``_trace_id_ctx.reset(token)`` to
    restore the previous value (typically in a ``finally`` block).

    Args:
        trace_id: The trace ID string to associate with the current context.

    Returns:
        A :class:`contextvars.Token` for resetting the context variable.
    """
    return _trace_id_ctx.set(trace_id)


class _ServiceFilter(logging.Filter):
    """Inject ``service`` and ``trace_id`` into every LogRecord.

    Args:
        service_name: The value written to the ``service`` field of each record.
    """

    def __init__(self, service_name: str) -> None:
        """Initialise the filter with the service name.

        Args:
            service_name: Identifier written to the ``service`` JSON field.
        """
        super().__init__()
        self._service_name = service_name

    def filter(self, record: logging.LogRecord) -> bool:
        """Add service and trace_id attributes to the log record.

        Args:
            record: The LogRecord being processed.

        Returns:
            Always ``True`` — this filter never drops records.
        """
        record.service = self._service_name  # type: ignore[attr-defined]
        record.trace_id = get_trace_id()  # type: ignore[attr-defined]
        return True


def configure_logging(
    service_name: str,
    log_level: str = "INFO",
    stream: IO[str] | None = None,
) -> None:
    """Configure the root logger to emit JSON-structured log lines.

    Replaces any existing root-logger handlers with a single StreamHandler
    that formats records as JSON objects containing:

    * ``timestamp`` — ISO 8601 datetime (renamed from ``asctime``)
    * ``level``     — Python level name (renamed from ``levelname``)
    * ``service``   — the value of *service_name*
    * ``trace_id``  — per-request ID from the context variable
    * ``message``   — the formatted log message

    Custom LogRecord attributes (``service``, ``trace_id``) are injected by
    :class:`_ServiceFilter` and automatically included in the JSON output by
    ``python-json-logger`` alongside the standard fields.

    Args:
        service_name: Identifies this service in every log record.
        log_level: Minimum severity to emit (case-insensitive). Defaults to
            ``"INFO"``.
        stream: Output stream. Defaults to ``sys.stdout``. Pass a
            :class:`io.StringIO` in tests to capture output.
    """
    formatter = JsonFormatter(
        fmt="%(asctime)s %(levelname)s %(message)s",
        rename_fields={
            "asctime": "timestamp",
            "levelname": "level",
        },
        datefmt="%Y-%m-%dT%H:%M:%S",
    )

    handler = logging.StreamHandler(stream or sys.stdout)
    handler.setFormatter(formatter)
    handler.addFilter(_ServiceFilter(service_name))

    root = logging.getLogger()
    root.handlers.clear()
    root.addHandler(handler)
    root.setLevel(log_level.upper())
