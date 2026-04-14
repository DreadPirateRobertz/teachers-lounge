"""Pytest configuration for ingestion-service tests.

Stubs heavy native packages in ``sys.modules`` before any test module is
collected.  This lets ``app.tasks.pdf_ingest`` and ``app.table_extractor``
import at module level without requiring native extensions to be installed
in the test environment.  Individual tests replace these stubs with proper
``MagicMock`` objects via ``unittest.mock.patch``.

Stubbed packages:
- ``fitz`` (PyMuPDF) — native PDF renderer used by pdf_ingest
- ``pdfplumber`` — table extraction library used by table_extractor
"""

import sys
from unittest.mock import MagicMock

if "fitz" not in sys.modules:
    sys.modules["fitz"] = MagicMock()

if "pdfplumber" not in sys.modules:
    sys.modules["pdfplumber"] = MagicMock()
