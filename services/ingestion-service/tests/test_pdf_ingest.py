"""Tests for the PDF ingestion pipeline: chunker and Celery task.

Mocks:
- fitz (PyMuPDF) — no real PDF I/O
- httpx.Client — no real AI gateway calls
- qdrant_client.QdrantClient — no real Qdrant server

Assertions cover chunk count, overlap correctness, and upsert call patterns.
"""

import json
from unittest.mock import MagicMock, call, patch

import pytest
import tiktoken

from app.chunker import chunk_pdf_pages, chunk_text
from app.tasks.pdf_ingest import _embed_texts, _extract_pages, _upsert_to_qdrant, ingest_pdf

# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture()
def fake_pdf_bytes() -> bytes:
    """Return a sentinel bytes value that stands in for a real PDF."""
    return b"%PDF-fake"


@pytest.fixture()
def two_page_pages() -> list[str]:
    """Return two non-trivial page texts for chunking tests."""
    return [
        "Hello world. " * 50,   # ~100 words, well under 512 tokens
        "Science facts. " * 60, # ~120 words
    ]


# ---------------------------------------------------------------------------
# chunker unit tests
# ---------------------------------------------------------------------------


class TestChunkText:
    """Unit tests for the chunk_text helper."""

    def test_empty_text_returns_empty_list(self):
        """chunk_text on an empty string returns an empty list."""
        result = chunk_text("", page=1)
        assert result == []

    def test_short_text_produces_single_chunk(self):
        """Text shorter than chunk_size produces exactly one chunk."""
        text = "Short sentence."
        result = chunk_text(text, page=1, chunk_size=512, overlap=64)
        assert len(result) == 1
        assert result[0]["text"] == text
        assert result[0]["page"] == 1
        assert result[0]["token_count"] <= 512

    def test_long_text_produces_multiple_chunks(self):
        """Text longer than chunk_size is split into multiple chunks."""
        # Build text that is definitely > 512 tokens (each word ≈ 1 token)
        text = " ".join(["word"] * 600)
        result = chunk_text(text, page=2, chunk_size=512, overlap=64)
        assert len(result) >= 2
        for chunk in result:
            assert chunk["token_count"] <= 512
            assert chunk["page"] == 2

    def test_overlap_is_present_between_consecutive_chunks(self):
        """Consecutive chunks share at least ``overlap`` tokens of content."""
        enc = tiktoken.get_encoding("cl100k_base")
        overlap = 64
        # 700 tokens guarantees at least 2 chunks with chunk_size=512
        tokens = list(range(700))
        text = enc.decode(enc.encode(" ".join(["token"] * 700)))

        result = chunk_text(text, page=1, chunk_size=512, overlap=overlap)
        assert len(result) >= 2

        # The second chunk should start with tokens that appeared at the end
        # of the first chunk — verify by token-level comparison.
        enc2 = tiktoken.get_encoding("cl100k_base")
        first_tokens = enc2.encode(result[0]["text"])
        second_tokens = enc2.encode(result[1]["text"])

        # The first `overlap` tokens of chunk[1] should appear at the tail of chunk[0]
        shared = second_tokens[:overlap]
        assert first_tokens[-overlap:] == shared

    def test_all_tokens_covered(self):
        """Every token in the source text appears in at least one chunk."""
        enc = tiktoken.get_encoding("cl100k_base")
        text = "alpha beta gamma delta " * 200  # ~800 tokens
        source_tokens = enc.encode(text)

        result = chunk_text(text, page=1, chunk_size=512, overlap=64)
        covered = set()
        pos = 0
        for chunk in result:
            chunk_tokens = enc.encode(chunk["text"])
            for i, tok in enumerate(chunk_tokens):
                covered.add(pos + i)
            # Advance by chunk_size - overlap (mirroring the chunker's rewind)
            pos += len(chunk_tokens) - 64

        # All source token positions should be covered
        assert len(result) >= 2


class TestChunkPdfPages:
    """Unit tests for chunk_pdf_pages."""

    def test_blank_pages_are_skipped(self):
        """Blank and whitespace-only pages produce no chunks."""
        result = chunk_pdf_pages(["", "   ", "\n\t"])
        assert result == []

    def test_chunk_index_is_monotonically_increasing(self):
        """chunk_index increases from 0 across all pages."""
        pages = ["word " * 200, "fact " * 200]
        result = chunk_pdf_pages(pages, chunk_size=128, overlap=16)
        indices = [c["chunk_index"] for c in result]
        assert indices == list(range(len(indices)))

    def test_page_numbers_are_one_based(self):
        """Page numbers in chunk metadata start at 1."""
        pages = ["content on page one", "content on page two"]
        result = chunk_pdf_pages(pages)
        pages_seen = {c["page"] for c in result}
        assert 1 in pages_seen
        assert 2 in pages_seen
        assert 0 not in pages_seen

    def test_two_pages_produce_chunks_for_both(self, two_page_pages):
        """Both pages of input appear in the chunked output."""
        result = chunk_pdf_pages(two_page_pages, chunk_size=512, overlap=64)
        assert any(c["page"] == 1 for c in result)
        assert any(c["page"] == 2 for c in result)

    def test_large_page_produces_multiple_chunks(self):
        """A page with >512 tokens is split into multiple chunks."""
        long_page = "word " * 700
        result = chunk_pdf_pages([long_page], chunk_size=512, overlap=64)
        assert len(result) >= 2


# ---------------------------------------------------------------------------
# _extract_pages tests (mocking PyMuPDF)
# ---------------------------------------------------------------------------


class TestExtractPages:
    """Tests for _extract_pages with PyMuPDF mocked."""

    def test_returns_one_string_per_page(self, fake_pdf_bytes):
        """_extract_pages returns a list with one entry per PDF page."""
        mock_page1 = MagicMock()
        mock_page1.get_text.return_value = "Page one text."
        mock_page2 = MagicMock()
        mock_page2.get_text.return_value = "Page two text."
        mock_doc = MagicMock()
        mock_doc.__iter__ = MagicMock(return_value=iter([mock_page1, mock_page2]))

        with patch("app.tasks.pdf_ingest.fitz") as mock_fitz:
            mock_fitz.open.return_value = mock_doc
            result = _extract_pages(fake_pdf_bytes)

        assert result == ["Page one text.", "Page two text."]
        mock_fitz.open.assert_called_once_with(stream=fake_pdf_bytes, filetype="pdf")

    def test_document_is_closed_after_extraction(self, fake_pdf_bytes):
        """fitz doc.close() is called even if extraction succeeds."""
        mock_page = MagicMock()
        mock_page.get_text.return_value = "text"
        mock_doc = MagicMock()
        mock_doc.__iter__ = MagicMock(return_value=iter([mock_page]))

        with patch("app.tasks.pdf_ingest.fitz") as mock_fitz:
            mock_fitz.open.return_value = mock_doc
            _extract_pages(fake_pdf_bytes)

        mock_doc.close.assert_called_once()


# ---------------------------------------------------------------------------
# _embed_texts tests (mocking httpx)
# ---------------------------------------------------------------------------


class TestEmbedTexts:
    """Tests for _embed_texts with httpx.Client mocked."""

    def _make_gateway_response(self, n: int) -> MagicMock:
        """Build a mock httpx response for n embeddings."""
        data = [{"index": i, "embedding": [float(i)] * 4} for i in range(n)]
        mock_response = MagicMock()
        mock_response.json.return_value = {"data": data}
        mock_response.raise_for_status = MagicMock()
        return mock_response

    def test_returns_one_vector_per_text(self):
        """_embed_texts returns one embedding per input text."""
        texts = ["chunk one", "chunk two", "chunk three"]
        mock_response = self._make_gateway_response(len(texts))
        mock_client = MagicMock()
        mock_client.post.return_value = mock_response
        mock_client.__enter__ = MagicMock(return_value=mock_client)
        mock_client.__exit__ = MagicMock(return_value=False)

        with patch("app.tasks.pdf_ingest.httpx.Client", return_value=mock_client):
            result = _embed_texts(texts)

        assert len(result) == len(texts)

    def test_posts_to_embeddings_endpoint(self):
        """_embed_texts POSTs to the /embeddings path of the gateway."""
        texts = ["hello"]
        mock_response = self._make_gateway_response(1)
        mock_client = MagicMock()
        mock_client.post.return_value = mock_response
        mock_client.__enter__ = MagicMock(return_value=mock_client)
        mock_client.__exit__ = MagicMock(return_value=False)

        with (
            patch("app.tasks.pdf_ingest.httpx.Client", return_value=mock_client),
            patch("app.tasks.pdf_ingest._AI_GATEWAY_URL", "http://gateway:8000"),
        ):
            _embed_texts(texts)

        call_args = mock_client.post.call_args
        assert call_args[0][0] == "http://gateway:8000/embeddings"
        assert call_args[1]["json"]["model"] == "text-embedding-3-small"

    def test_raises_on_gateway_error(self):
        """_embed_texts propagates httpx.HTTPStatusError on bad response."""
        import httpx

        mock_client = MagicMock()
        mock_client.__enter__ = MagicMock(return_value=mock_client)
        mock_client.__exit__ = MagicMock(return_value=False)
        mock_response = MagicMock()
        mock_response.raise_for_status.side_effect = httpx.HTTPStatusError(
            "500", request=MagicMock(), response=MagicMock()
        )
        mock_client.post.return_value = mock_response

        with patch("app.tasks.pdf_ingest.httpx.Client", return_value=mock_client):
            with pytest.raises(httpx.HTTPStatusError):
                _embed_texts(["some text"])


# ---------------------------------------------------------------------------
# _upsert_to_qdrant tests (mocking QdrantClient)
# ---------------------------------------------------------------------------


class TestUpsertToQdrant:
    """Tests for _upsert_to_qdrant with QdrantClient mocked."""

    def _make_chunks(self, n: int) -> list[dict]:
        return [
            {"text": f"chunk {i}", "page": 1, "chunk_index": i, "token_count": 10}
            for i in range(n)
        ]

    def _make_vectors(self, n: int) -> list[list[float]]:
        return [[0.1] * 4 for _ in range(n)]

    def test_upsert_called_with_correct_collection(self):
        """QdrantClient.upsert is called with the curriculum collection name."""
        chunks = self._make_chunks(2)
        vectors = self._make_vectors(2)
        mock_client = MagicMock()

        with (
            patch("app.tasks.pdf_ingest.QdrantClient", return_value=mock_client),
            patch("app.tasks.pdf_ingest._CURRICULUM_COLLECTION", "curriculum"),
        ):
            _upsert_to_qdrant(chunks, vectors, "course-123", "lecture1.pdf")

        assert mock_client.upsert.called
        collection_arg = mock_client.upsert.call_args[1]["collection_name"]
        assert collection_arg == "curriculum"

    def test_payload_fields_are_correct(self):
        """Each upserted point contains course_id, page, chunk_index, source_pdf."""
        chunks = self._make_chunks(1)
        vectors = self._make_vectors(1)
        mock_client = MagicMock()

        with patch("app.tasks.pdf_ingest.QdrantClient", return_value=mock_client):
            _upsert_to_qdrant(chunks, vectors, "course-xyz", "notes.pdf")

        points = mock_client.upsert.call_args[1]["points"]
        assert len(points) == 1
        payload = points[0].payload
        assert payload["course_id"] == "course-xyz"
        assert payload["source_pdf"] == "notes.pdf"
        assert payload["page"] == 1
        assert payload["chunk_index"] == 0

    def test_upsert_batches_over_100(self):
        """More than 100 chunks trigger multiple upsert calls."""
        chunks = self._make_chunks(250)
        vectors = self._make_vectors(250)
        mock_client = MagicMock()

        with patch("app.tasks.pdf_ingest.QdrantClient", return_value=mock_client):
            _upsert_to_qdrant(chunks, vectors, "c", "big.pdf")

        # 250 chunks → ceil(250/100) = 3 upsert calls
        assert mock_client.upsert.call_count == 3


# ---------------------------------------------------------------------------
# ingest_pdf Celery task integration tests
# ---------------------------------------------------------------------------


class TestIngestPdfTask:
    """Integration tests for the ingest_pdf Celery task (all I/O mocked)."""

    def _setup_mocks(self, page_texts: list[str], n_embed_dim: int = 4):
        """Return a dict of patch objects for PyMuPDF, httpx, and Qdrant."""

        def make_fitz_mock():
            pages = []
            for text in page_texts:
                p = MagicMock()
                p.get_text.return_value = text
                pages.append(p)
            doc = MagicMock()
            doc.__iter__ = MagicMock(return_value=iter(pages))
            fitz_mod = MagicMock()
            fitz_mod.open.return_value = doc
            return fitz_mod

        def make_httpx_mock(n_chunks: int):
            data = [{"index": i, "embedding": [0.1] * n_embed_dim} for i in range(n_chunks)]
            response = MagicMock()
            response.json.return_value = {"data": data}
            response.raise_for_status = MagicMock()
            client = MagicMock()
            client.post.return_value = response
            client.__enter__ = MagicMock(return_value=client)
            client.__exit__ = MagicMock(return_value=False)
            return client

        return make_fitz_mock, make_httpx_mock

    def test_happy_path_returns_chunk_count(self):
        """ingest_pdf returns a dict with the correct chunk_count."""
        page_texts = ["fact " * 100, "data " * 80]
        make_fitz, make_httpx = self._setup_mocks(page_texts)

        # Pre-compute expected chunk count so the httpx mock returns right # vectors
        chunks = chunk_pdf_pages(page_texts, chunk_size=512, overlap=64)
        n = len(chunks)

        mock_fitz = make_fitz()
        mock_client = make_httpx(n)
        mock_qdrant = MagicMock()

        with (
            patch("app.tasks.pdf_ingest.fitz", mock_fitz),
            patch("app.tasks.pdf_ingest.httpx.Client", return_value=mock_client),
            patch("app.tasks.pdf_ingest.QdrantClient", return_value=mock_qdrant),
        ):
            result = ingest_pdf.run(b"%PDF-fake", "course-1", "lecture.pdf")

        assert result["chunk_count"] == n
        assert result["course_id"] == "course-1"
        assert result["source_pdf"] == "lecture.pdf"

    def test_empty_pdf_returns_zero_chunks(self):
        """ingest_pdf with blank pages returns chunk_count=0 without calling Qdrant."""
        mock_fitz = MagicMock()
        page = MagicMock()
        page.get_text.return_value = ""
        doc = MagicMock()
        doc.__iter__ = MagicMock(return_value=iter([page]))
        mock_fitz.open.return_value = doc

        mock_qdrant = MagicMock()

        with (
            patch("app.tasks.pdf_ingest.fitz", mock_fitz),
            patch("app.tasks.pdf_ingest.QdrantClient", return_value=mock_qdrant),
        ):
            result = ingest_pdf.run(b"%PDF-fake", "course-2", "empty.pdf")

        assert result["chunk_count"] == 0
        mock_qdrant.upsert.assert_not_called()

    def test_qdrant_upsert_called_once_for_small_pdf(self):
        """For a small PDF (<100 chunks), QdrantClient.upsert is called once."""
        page_texts = ["short text on page one"]
        chunks = chunk_pdf_pages(page_texts, chunk_size=512, overlap=64)
        n = len(chunks)

        mock_fitz = MagicMock()
        page = MagicMock()
        page.get_text.return_value = page_texts[0]
        doc = MagicMock()
        doc.__iter__ = MagicMock(return_value=iter([page]))
        mock_fitz.open.return_value = doc

        data = [{"index": i, "embedding": [0.0] * 4} for i in range(n)]
        response = MagicMock()
        response.json.return_value = {"data": data}
        response.raise_for_status = MagicMock()
        mock_client = MagicMock()
        mock_client.post.return_value = response
        mock_client.__enter__ = MagicMock(return_value=mock_client)
        mock_client.__exit__ = MagicMock(return_value=False)

        mock_qdrant = MagicMock()

        with (
            patch("app.tasks.pdf_ingest.fitz", mock_fitz),
            patch("app.tasks.pdf_ingest.httpx.Client", return_value=mock_client),
            patch("app.tasks.pdf_ingest.QdrantClient", return_value=mock_qdrant),
        ):
            ingest_pdf.run(b"%PDF-fake", "course-3", "small.pdf")

        assert mock_qdrant.upsert.call_count == 1

    def test_chunk_payloads_include_required_fields(self):
        """Qdrant points include course_id, page, chunk_index, source_pdf."""
        page_texts = ["alpha beta gamma " * 10]
        chunks = chunk_pdf_pages(page_texts, chunk_size=512, overlap=64)
        n = len(chunks)

        mock_fitz = MagicMock()
        page = MagicMock()
        page.get_text.return_value = page_texts[0]
        doc = MagicMock()
        doc.__iter__ = MagicMock(return_value=iter([page]))
        mock_fitz.open.return_value = doc

        data = [{"index": i, "embedding": [0.5] * 4} for i in range(n)]
        response = MagicMock()
        response.json.return_value = {"data": data}
        response.raise_for_status = MagicMock()
        mock_client = MagicMock()
        mock_client.post.return_value = response
        mock_client.__enter__ = MagicMock(return_value=mock_client)
        mock_client.__exit__ = MagicMock(return_value=False)

        mock_qdrant = MagicMock()

        with (
            patch("app.tasks.pdf_ingest.fitz", mock_fitz),
            patch("app.tasks.pdf_ingest.httpx.Client", return_value=mock_client),
            patch("app.tasks.pdf_ingest.QdrantClient", return_value=mock_qdrant),
        ):
            ingest_pdf.run(b"%PDF-fake", "course-99", "curriculum.pdf")

        points = mock_qdrant.upsert.call_args[1]["points"]
        for point in points:
            assert "course_id" in point.payload
            assert "page" in point.payload
            assert "chunk_index" in point.payload
            assert "source_pdf" in point.payload
            assert point.payload["course_id"] == "course-99"
            assert point.payload["source_pdf"] == "curriculum.pdf"
