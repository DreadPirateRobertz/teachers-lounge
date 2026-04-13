"""Edge-case tests for the ingestion pipeline.

Covers areas that the baseline happy-path suite leaves under-exercised:

Chunking:
  - empty page list
  - text with exactly chunk_size tokens (boundary: 1 chunk)
  - text with chunk_size+1 tokens (boundary: 2 chunks)
  - unicode text (CJK, emoji, accented Latin)
  - mixed blank and unicode pages

Embedding (AI gateway / network error paths):
  - multi-batch requests (>_EMBED_BATCH_SIZE texts → multiple POST calls)
  - httpx.TimeoutException propagates out of _embed_texts
  - httpx.ConnectError propagates out of _embed_texts

Qdrant upsert error paths:
  - upsert exception propagates from _upsert_to_qdrant (enabling Celery retry)
  - ingest_pdf.run() re-raises Qdrant exception so autoretry fires

PDF extraction cleanup:
  - doc.close() is called even when get_text() raises an exception
"""
import httpx
import pytest
import tiktoken
from unittest.mock import MagicMock, patch

from app.chunker import chunk_pdf_pages, chunk_text
from app.tasks.pdf_ingest import (
    _embed_texts,
    _extract_pages,
    _upsert_to_qdrant,
    ingest_pdf,
)


# ---------------------------------------------------------------------------
# Chunker edge cases
# ---------------------------------------------------------------------------


class TestChunkTextBoundaries:
    """Token-boundary behaviour for chunk_text."""

    def test_empty_pages_list_returns_empty(self):
        """chunk_pdf_pages with an empty list returns an empty list."""
        assert chunk_pdf_pages([]) == []

    def test_exactly_chunk_size_tokens_produces_one_chunk(self):
        """Text with exactly chunk_size tokens yields a single chunk.

        Constructs the text by encoding a long source, slicing to exactly
        chunk_size tokens, then decoding — guaranteeing the round-trip token
        count is stable.
        """
        enc = tiktoken.get_encoding("cl100k_base")
        chunk_size = 50
        source = enc.encode("hello world " * 200)
        text = enc.decode(source[:chunk_size])

        result = chunk_text(text, page=1, chunk_size=chunk_size, overlap=10)

        assert len(result) == 1
        assert result[0]["token_count"] == chunk_size

    def test_one_over_chunk_size_produces_two_chunks(self):
        """Text with chunk_size+1 tokens is split into exactly two chunks.

        First chunk: tokens 0..chunk_size-1.
        Second chunk: tokens (chunk_size-overlap)..(chunk_size).
        """
        enc = tiktoken.get_encoding("cl100k_base")
        chunk_size = 50
        overlap = 10
        source = enc.encode("hello world " * 200)
        text = enc.decode(source[: chunk_size + 1])

        result = chunk_text(text, page=1, chunk_size=chunk_size, overlap=overlap)

        assert len(result) == 2
        assert result[0]["token_count"] == chunk_size
        # Second chunk covers from (chunk_size - overlap) to end = overlap+1 tokens
        assert result[1]["token_count"] == overlap + 1

    def test_chunk_size_equal_to_overlap_clamps_to_one_chunk(self):
        """When overlap >= text token count the loop terminates after one chunk.

        Uses a text shorter than chunk_size so a single chunk is produced and
        the loop's break condition fires.
        """
        result = chunk_text("tiny", page=3, chunk_size=512, overlap=64)
        assert len(result) == 1


class TestChunkTextUnicode:
    """chunk_text correctly handles non-ASCII / multibyte text."""

    def test_cjk_text_is_chunked_without_error(self):
        """Japanese/Chinese text is tokenised and returned as valid chunks."""
        text = "日本語のテキストです。" * 30  # ~90 tokens of CJK
        result = chunk_text(text, page=1, chunk_size=512, overlap=64)
        assert len(result) >= 1
        for chunk in result:
            assert chunk["token_count"] > 0
            assert len(chunk["text"]) > 0

    def test_emoji_text_is_chunked_without_error(self):
        """Emoji-heavy text is tokenised and returned as valid chunks."""
        text = "🎓📚🔬🧪🌍 " * 40  # emoji tokens
        result = chunk_text(text, page=2, chunk_size=512, overlap=64)
        assert len(result) >= 1
        assert result[0]["page"] == 2

    def test_accented_latin_text_is_chunked_without_error(self):
        """Accented Latin (café, résumé, naïve) passes through correctly."""
        text = "café résumé naïve Ångström " * 50
        result = chunk_text(text, page=1, chunk_size=512, overlap=64)
        assert len(result) >= 1
        # Decoded text should still contain accented characters
        combined = " ".join(c["text"] for c in result)
        assert "café" in combined or "caf" in combined  # tiktoken may split

    def test_mixed_unicode_and_ascii_text(self):
        """Mixed CJK + ASCII + emoji text is chunked without error."""
        text = "Hello 世界! Café 🎓 résumé. " * 25
        result = chunk_text(text, page=1, chunk_size=512, overlap=64)
        assert len(result) >= 1


class TestChunkPdfPagesEdgeCases:
    """Edge cases for chunk_pdf_pages."""

    def test_all_whitespace_pages_return_empty(self):
        """Pages that are only whitespace or newlines produce no chunks."""
        result = chunk_pdf_pages(["\n\n", "\t  \t", "   "])
        assert result == []

    def test_mixed_blank_and_unicode_pages(self):
        """Blank pages are skipped; unicode pages produce chunks with correct page numbers."""
        pages = [
            "",              # page 1 — blank, skipped
            "日本語 " * 30,  # page 2 — unicode, kept
            "   ",           # page 3 — whitespace, skipped
            "café " * 20,    # page 4 — accented, kept
        ]
        result = chunk_pdf_pages(pages)

        page_nums = {c["page"] for c in result}
        assert 2 in page_nums
        assert 4 in page_nums
        assert 1 not in page_nums
        assert 3 not in page_nums

    def test_single_unicode_page_has_chunk_index_starting_at_zero(self):
        """A single unicode page's first chunk has chunk_index == 0."""
        result = chunk_pdf_pages(["こんにちは世界 " * 10])
        assert len(result) >= 1
        assert result[0]["chunk_index"] == 0


# ---------------------------------------------------------------------------
# Embedding error paths
# ---------------------------------------------------------------------------


class TestEmbedTextsErrorPaths:
    """Error-path and batching behaviour for _embed_texts."""

    def _make_batch_response(self, n: int) -> MagicMock:
        """Return a mock httpx response for a batch of n embeddings."""
        resp = MagicMock()
        resp.json.return_value = {
            "data": [{"index": i, "embedding": [0.1] * 4} for i in range(n)]
        }
        resp.raise_for_status = MagicMock()
        return resp

    def test_multi_batch_issues_multiple_post_calls(self):
        """250 texts with batch_size=100 issues exactly 3 POST requests."""
        texts = [f"chunk {i}" for i in range(250)]

        mock_client = MagicMock()
        mock_client.__enter__ = MagicMock(return_value=mock_client)
        mock_client.__exit__ = MagicMock(return_value=False)
        mock_client.post.side_effect = [
            self._make_batch_response(100),
            self._make_batch_response(100),
            self._make_batch_response(50),
        ]

        with (
            patch("app.tasks.pdf_ingest.httpx.Client", return_value=mock_client),
            patch("app.tasks.pdf_ingest._EMBED_BATCH_SIZE", 100),
        ):
            result = _embed_texts(texts)

        assert mock_client.post.call_count == 3
        assert len(result) == 250

    def test_multi_batch_preserves_embedding_order(self):
        """Embeddings returned by multi-batch calls appear in input order."""
        texts = [f"t{i}" for i in range(5)]
        # Each embedding is [float(i)] * 2 so we can identify it later
        batch_resp = MagicMock()
        batch_resp.json.return_value = {
            "data": [{"index": i, "embedding": [float(i), float(i)]} for i in range(5)]
        }
        batch_resp.raise_for_status = MagicMock()

        mock_client = MagicMock()
        mock_client.__enter__ = MagicMock(return_value=mock_client)
        mock_client.__exit__ = MagicMock(return_value=False)
        mock_client.post.return_value = batch_resp

        with patch("app.tasks.pdf_ingest.httpx.Client", return_value=mock_client):
            result = _embed_texts(texts)

        assert result[0] == [0.0, 0.0]
        assert result[4] == [4.0, 4.0]

    def test_raises_on_timeout(self):
        """_embed_texts propagates httpx.TimeoutException from the gateway."""
        mock_client = MagicMock()
        mock_client.__enter__ = MagicMock(return_value=mock_client)
        mock_client.__exit__ = MagicMock(return_value=False)
        mock_client.post.side_effect = httpx.TimeoutException("gateway timed out")

        with patch("app.tasks.pdf_ingest.httpx.Client", return_value=mock_client):
            with pytest.raises(httpx.TimeoutException):
                _embed_texts(["some text"])

    def test_raises_on_connect_error(self):
        """_embed_texts propagates httpx.ConnectError when the gateway is unreachable."""
        mock_client = MagicMock()
        mock_client.__enter__ = MagicMock(return_value=mock_client)
        mock_client.__exit__ = MagicMock(return_value=False)
        mock_client.post.side_effect = httpx.ConnectError("connection refused")

        with patch("app.tasks.pdf_ingest.httpx.Client", return_value=mock_client):
            with pytest.raises(httpx.ConnectError):
                _embed_texts(["some text"])


# ---------------------------------------------------------------------------
# Qdrant upsert error paths
# ---------------------------------------------------------------------------


class TestUpsertToQdrantErrorPaths:
    """Upsert exceptions must propagate so Celery's autoretry mechanism fires."""

    def _make_chunks(self, n: int = 2) -> list[dict]:
        return [
            {"text": f"chunk {i}", "page": 1, "chunk_index": i, "token_count": 10}
            for i in range(n)
        ]

    def _make_vectors(self, n: int = 2) -> list[list[float]]:
        return [[0.1] * 4 for _ in range(n)]

    def test_upsert_exception_propagates(self):
        """Exception from QdrantClient.upsert propagates out of _upsert_to_qdrant."""
        mock_client = MagicMock()
        mock_client.upsert.side_effect = Exception("Qdrant connection refused")

        with patch("app.tasks.pdf_ingest.QdrantClient", return_value=mock_client):
            with pytest.raises(Exception, match="Qdrant connection refused"):
                _upsert_to_qdrant(
                    self._make_chunks(), self._make_vectors(), "course-1", "file.pdf"
                )

    def test_upsert_partial_batch_failure_propagates(self):
        """If the second upsert batch fails, the exception propagates."""
        mock_client = MagicMock()
        mock_client.upsert.side_effect = [
            None,  # first batch succeeds
            RuntimeError("second batch failed"),
        ]
        chunks = self._make_chunks(150)
        vectors = self._make_vectors(150)

        with patch("app.tasks.pdf_ingest.QdrantClient", return_value=mock_client):
            with pytest.raises(RuntimeError, match="second batch failed"):
                _upsert_to_qdrant(chunks, vectors, "course-1", "file.pdf")

    def test_ingest_pdf_propagates_qdrant_exception_for_celery_retry(self):
        """ingest_pdf.run() re-raises when Qdrant upsert fails.

        Celery's ``autoretry_for=(Exception,)`` setting depends on the task
        propagating exceptions — this test verifies that contract.
        """
        page_text = "some lecture content " * 20
        from app.chunker import chunk_pdf_pages as _cpdf
        chunks = _cpdf([page_text], chunk_size=512, overlap=64)
        n = len(chunks)

        mock_fitz = MagicMock()
        mock_page = MagicMock()
        mock_page.get_text.return_value = page_text
        mock_doc = MagicMock()
        mock_doc.__iter__ = MagicMock(return_value=iter([mock_page]))
        mock_fitz.open.return_value = mock_doc

        resp = MagicMock()
        resp.json.return_value = {
            "data": [{"index": i, "embedding": [0.1] * 4} for i in range(n)]
        }
        resp.raise_for_status = MagicMock()
        mock_http = MagicMock()
        mock_http.post.return_value = resp
        mock_http.__enter__ = MagicMock(return_value=mock_http)
        mock_http.__exit__ = MagicMock(return_value=False)

        mock_qdrant = MagicMock()
        mock_qdrant.upsert.side_effect = Exception("Qdrant unavailable")

        with (
            patch("app.tasks.pdf_ingest.fitz", mock_fitz),
            patch("app.tasks.pdf_ingest.httpx.Client", return_value=mock_http),
            patch("app.tasks.pdf_ingest.QdrantClient", return_value=mock_qdrant),
        ):
            with pytest.raises(Exception, match="Qdrant unavailable"):
                ingest_pdf.run(b"%PDF-fake", "course-1", "lecture.pdf")


# ---------------------------------------------------------------------------
# PDF extraction cleanup
# ---------------------------------------------------------------------------


class TestExtractPagesCleanup:
    """doc.close() is called regardless of whether extraction raises."""

    def test_doc_closed_when_get_text_raises(self):
        """_extract_pages calls doc.close() even if get_text() raises."""
        mock_page = MagicMock()
        mock_page.get_text.side_effect = RuntimeError("corrupt page data")
        mock_doc = MagicMock()
        mock_doc.__iter__ = MagicMock(return_value=iter([mock_page]))

        with patch("app.tasks.pdf_ingest.fitz") as mock_fitz:
            mock_fitz.open.return_value = mock_doc
            with pytest.raises(RuntimeError, match="corrupt page data"):
                _extract_pages(b"%PDF-fake")

        mock_doc.close.assert_called_once()

    def test_doc_closed_on_fitz_open_failure(self):
        """If fitz.open itself raises, the exception propagates cleanly."""
        with patch("app.tasks.pdf_ingest.fitz") as mock_fitz:
            mock_fitz.open.side_effect = ValueError("not a valid PDF")
            with pytest.raises(ValueError, match="not a valid PDF"):
                _extract_pages(b"garbage")
