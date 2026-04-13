"""Pytest configuration for ingestion-service tests.

Stubs the ``fitz`` (PyMuPDF) package in ``sys.modules`` before any test
module is collected.  This lets ``app.tasks.pdf_ingest`` import at module
level without requiring the native PyMuPDF extension to be installed in the
test environment.  Individual tests replace the stub with a proper
``MagicMock`` via ``unittest.mock.patch``.
"""

import sys
from unittest.mock import MagicMock

if "fitz" not in sys.modules:
    sys.modules["fitz"] = MagicMock()
