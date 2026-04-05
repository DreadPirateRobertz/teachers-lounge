"""Tests for tutoring-service JSON structured logging configuration.

Verifies that configure_logging() produces JSON output with the required
fields: timestamp, level, service, trace_id, message.
"""
import json
import logging
import uuid
from io import StringIO

import pytest

from app.logging_config import configure_logging, get_trace_id, set_trace_id


class TestConfigureLogging:
    """Tests for the configure_logging() setup function."""

    def setup_method(self):
        """Reset root logger handlers before each test."""
        root = logging.getLogger()
        root.handlers.clear()

    def _capture_log(self, service_name: str, level: str = "DEBUG") -> tuple[logging.Logger, StringIO]:
        """Configure logging and return a logger + StringIO stream pair.

        Args:
            service_name: The service name to pass to configure_logging().
            level: Log level string (default "DEBUG" to capture all output).

        Returns:
            Tuple of (logger, stream) where stream receives JSON log lines.
        """
        stream = StringIO()
        configure_logging(service_name=service_name, log_level=level, stream=stream)
        return logging.getLogger("test"), stream

    def test_output_is_valid_json(self):
        """Each log line must be parseable as JSON."""
        logger, stream = self._capture_log("tutoring-service")
        logger.info("hello world")
        line = stream.getvalue().strip()
        assert line, "Expected at least one log line"
        record = json.loads(line)
        assert isinstance(record, dict)

    def test_required_fields_present(self):
        """Log record must contain timestamp, level, service, trace_id, message."""
        logger, stream = self._capture_log("tutoring-service")
        logger.info("test message")
        record = json.loads(stream.getvalue().strip())
        for field in ("timestamp", "level", "service", "trace_id", "message"):
            assert field in record, f"Missing required field: {field}"

    def test_service_field_matches_name(self):
        """service field must equal the name passed to configure_logging()."""
        logger, stream = self._capture_log("tutoring-service")
        logger.info("check service")
        record = json.loads(stream.getvalue().strip())
        assert record["service"] == "tutoring-service"

    def test_level_field_reflects_log_level(self):
        """level field must reflect the Python log level name."""
        logger, stream = self._capture_log("tutoring-service")
        logger.warning("a warning")
        record = json.loads(stream.getvalue().strip())
        assert record["level"] == "WARNING"

    def test_message_field_contains_log_message(self):
        """message field must contain the string passed to the logger."""
        logger, stream = self._capture_log("tutoring-service")
        logger.info("the quick brown fox")
        record = json.loads(stream.getvalue().strip())
        assert record["message"] == "the quick brown fox"

    def test_timestamp_is_iso8601_string(self):
        """timestamp field must be a non-empty ISO 8601 string."""
        logger, stream = self._capture_log("tutoring-service")
        logger.info("ts check")
        record = json.loads(stream.getvalue().strip())
        ts = record["timestamp"]
        assert isinstance(ts, str) and len(ts) > 0
        # Basic ISO 8601 shape: YYYY-MM-DD
        assert ts[:4].isdigit() and ts[4] == "-"

    def test_log_level_filter_respected(self):
        """Messages below the configured level must not appear in output."""
        logger, stream = self._capture_log("tutoring-service", level="WARNING")
        logger.debug("should be filtered")
        logger.info("also filtered")
        logger.warning("this should appear")
        lines = [ln for ln in stream.getvalue().strip().splitlines() if ln]
        assert len(lines) == 1
        record = json.loads(lines[0])
        assert record["level"] == "WARNING"

    def test_multiple_log_lines(self):
        """Multiple log calls must each produce a separate JSON line."""
        logger, stream = self._capture_log("tutoring-service")
        logger.info("first")
        logger.info("second")
        logger.info("third")
        lines = [ln for ln in stream.getvalue().strip().splitlines() if ln]
        assert len(lines) == 3
        messages = [json.loads(ln)["message"] for ln in lines]
        assert messages == ["first", "second", "third"]


class TestTraceID:
    """Tests for trace_id context variable management."""

    def test_default_trace_id_when_not_set(self):
        """get_trace_id() returns a non-empty default when no trace ID is set."""
        # Clear any previously set trace ID by setting to empty
        from app.logging_config import _trace_id_ctx
        token = _trace_id_ctx.set("")
        try:
            tid = get_trace_id()
            assert isinstance(tid, str) and len(tid) > 0
        finally:
            _trace_id_ctx.reset(token)

    def test_set_and_get_trace_id(self):
        """set_trace_id() stores a value retrievable by get_trace_id()."""
        tid = str(uuid.uuid4())
        token = set_trace_id(tid)
        try:
            assert get_trace_id() == tid
        finally:
            from app.logging_config import _trace_id_ctx
            _trace_id_ctx.reset(token)

    def test_trace_id_appears_in_log_output(self):
        """trace_id set via set_trace_id() must appear in the JSON log record."""
        stream = StringIO()
        configure_logging(service_name="tutoring-service", log_level="DEBUG", stream=stream)
        tid = str(uuid.uuid4())
        token = set_trace_id(tid)
        try:
            logging.getLogger("trace-test").info("trace check")
            record = json.loads(stream.getvalue().strip().splitlines()[-1])
            assert record["trace_id"] == tid
        finally:
            from app.logging_config import _trace_id_ctx
            _trace_id_ctx.reset(token)

    def test_default_trace_id_in_log_when_not_set(self):
        """When no trace_id is set, log record still has a non-empty trace_id."""
        from app.logging_config import _trace_id_ctx
        stream = StringIO()
        configure_logging(service_name="tutoring-service", log_level="DEBUG", stream=stream)
        token = _trace_id_ctx.set("")
        try:
            logging.getLogger("notrace-test").info("no trace")
            record = json.loads(stream.getvalue().strip().splitlines()[-1])
            assert isinstance(record["trace_id"], str) and len(record["trace_id"]) > 0
        finally:
            _trace_id_ctx.reset(token)
