"""Tests for the re-ranking service."""
import uuid
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from app.models import ChunkResult
from app.services.reranker import rerank


def _chunk(content: str = "text", score: float = 0.0) -> ChunkResult:
    return ChunkResult(
        chunk_id=uuid.uuid4(),
        material_id=uuid.uuid4(),
        course_id=uuid.uuid4(),
        content=content,
        score=score,
    )


class TestReranker:
    @pytest.mark.asyncio
    async def test_empty_input_returns_empty(self):
        assert await rerank("query", []) == []

    @pytest.mark.asyncio
    async def test_single_result_returns_unchanged(self):
        """Single result skips the rerank call entirely."""
        results = [_chunk("single")]
        out = await rerank("query", results)
        assert out == results

    @pytest.mark.asyncio
    async def test_disabled_when_model_empty(self):
        """Empty rerank_model config disables reranking."""
        results = [_chunk("a"), _chunk("b")]
        with patch("app.services.reranker.settings") as mock_settings:
            mock_settings.rerank_model = ""
            out = await rerank("query", results)
        assert out == results

    @pytest.mark.asyncio
    async def test_successful_rerank_reorders(self):
        """Gateway response re-orders results by relevance_score."""
        c1 = _chunk("low relevance", score=0.5)
        c2 = _chunk("high relevance", score=0.3)
        results = [c1, c2]

        mock_response = MagicMock()
        mock_response.raise_for_status = MagicMock()
        mock_response.json.return_value = {
            "results": [
                {"index": 1, "relevance_score": 0.95},
                {"index": 0, "relevance_score": 0.42},
            ]
        }

        mock_client = AsyncMock()
        mock_client.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client.__aexit__ = AsyncMock(return_value=False)
        mock_client.post = AsyncMock(return_value=mock_response)

        with patch("app.services.reranker.httpx.AsyncClient", return_value=mock_client):
            out = await rerank("my query", results)

        assert len(out) == 2
        # c2 (index 1) should be first with score 0.95
        assert out[0].content == "high relevance"
        assert out[0].score == 0.95
        assert out[1].content == "low relevance"
        assert out[1].score == 0.42

    @pytest.mark.asyncio
    async def test_gateway_sends_correct_payload(self):
        """Verify the HTTP request payload to the gateway."""
        results = [_chunk("doc A"), _chunk("doc B"), _chunk("doc C")]

        mock_response = MagicMock()
        mock_response.raise_for_status = MagicMock()
        mock_response.json.return_value = {
            "results": [
                {"index": 0, "relevance_score": 0.9},
                {"index": 1, "relevance_score": 0.8},
                {"index": 2, "relevance_score": 0.7},
            ]
        }

        mock_client = AsyncMock()
        mock_client.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client.__aexit__ = AsyncMock(return_value=False)
        mock_client.post = AsyncMock(return_value=mock_response)

        with patch("app.services.reranker.httpx.AsyncClient", return_value=mock_client):
            await rerank("entropy", results)

        call_args = mock_client.post.call_args
        payload = call_args.kwargs.get("json") or call_args[1].get("json")
        assert payload["query"] == "entropy"
        assert payload["documents"] == ["doc A", "doc B", "doc C"]
        assert payload["model"] == "rerank-english-v3.0"

    @pytest.mark.asyncio
    async def test_fallback_on_gateway_error(self):
        """Gateway failure falls back to original order, no exception raised."""
        results = [_chunk("a", score=0.5), _chunk("b", score=0.3)]

        mock_client = AsyncMock()
        mock_client.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client.__aexit__ = AsyncMock(return_value=False)
        mock_client.post = AsyncMock(side_effect=Exception("connection refused"))

        with patch("app.services.reranker.httpx.AsyncClient", return_value=mock_client):
            out = await rerank("query", results)

        assert out == results
        assert out[0].score == 0.5  # original scores preserved

    @pytest.mark.asyncio
    async def test_top_n_respects_config(self):
        """top_n in payload should be min(rerank_top_n, len(results))."""
        results = [_chunk(f"doc {i}") for i in range(20)]

        mock_response = MagicMock()
        mock_response.raise_for_status = MagicMock()
        mock_response.json.return_value = {
            "results": [{"index": i, "relevance_score": 1.0 - i * 0.05} for i in range(10)]
        }

        mock_client = AsyncMock()
        mock_client.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client.__aexit__ = AsyncMock(return_value=False)
        mock_client.post = AsyncMock(return_value=mock_response)

        with patch("app.services.reranker.httpx.AsyncClient", return_value=mock_client):
            out = await rerank("query", results)

        payload = mock_client.post.call_args.kwargs.get("json") or mock_client.post.call_args[1].get("json")
        assert payload["top_n"] == 10  # default rerank_top_n
        assert len(out) == 10
