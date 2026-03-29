"""Tests for hybrid combiner, re-ranker stubs, and embedding stub."""
import math
import uuid
from unittest.mock import AsyncMock, patch

import pytest

from app.models import ChunkResult
from app.services.embedder import embed_query
from app.services.hybrid import combine_dense_sparse
from app.services.reranker import rerank


def _chunk(score: float) -> ChunkResult:
    return ChunkResult(
        chunk_id=uuid.uuid4(),
        material_id=uuid.uuid4(),
        course_id=uuid.uuid4(),
        content="text",
        score=score,
    )


class TestEmbedder:
    @pytest.mark.asyncio
    async def test_returns_correct_dimension(self):
        vec = await embed_query("what is entropy?")
        assert len(vec) == 1024

    @pytest.mark.asyncio
    async def test_returns_unit_vector(self):
        vec = await embed_query("thermodynamics")
        norm = math.sqrt(sum(x * x for x in vec))
        assert abs(norm - 1.0) < 1e-6

    @pytest.mark.asyncio
    async def test_embedding_vector_is_passed_to_qdrant(self):
        """The vector returned by embed_query must be forwarded to dense_search unchanged."""
        fixed_vector = [0.1] * 1024
        course_id = uuid.uuid4()

        with (
            patch("app.services.embedder.embed_query", new_callable=AsyncMock, return_value=fixed_vector),
            patch("app.routers.search.dense_search", new_callable=AsyncMock, return_value=[]) as mock_search,
        ):
            from fastapi.testclient import TestClient
            from app.main import app

            with TestClient(app) as client:
                client.get(f"/v1/search?q=entropy&course_id={course_id}")

            _, kwargs = mock_search.call_args
            assert kwargs["query_vector"] == fixed_vector, (
                "The vector from embed_query must be passed verbatim to dense_search"
            )


class TestHybridCombiner:
    def test_returns_dense_when_sparse_empty(self):
        dense = [_chunk(0.9), _chunk(0.8)]
        result = combine_dense_sparse(dense, sparse_results=[])
        assert result == dense

    def test_returns_dense_unchanged_order(self):
        dense = [_chunk(0.9), _chunk(0.7), _chunk(0.5)]
        result = combine_dense_sparse(dense, sparse_results=[])
        assert [r.score for r in result] == [0.9, 0.7, 0.5]

    def test_empty_inputs_return_empty(self):
        result = combine_dense_sparse([], [])
        assert result == []

    def test_dense_only_passthrough(self):
        dense = [_chunk(0.95)]
        result = combine_dense_sparse(dense, [])
        assert len(result) == 1
        assert result[0].score == 0.95


class TestReranker:
    def test_identity_returns_same_results(self):
        results = [_chunk(0.9), _chunk(0.8), _chunk(0.7)]
        reranked = rerank("entropy", results)
        assert reranked == results

    def test_preserves_order(self):
        results = [_chunk(s) for s in [0.9, 0.6, 0.8]]
        reranked = rerank("query", results)
        assert [r.score for r in reranked] == [0.9, 0.6, 0.8]

    def test_empty_input(self):
        assert rerank("query", []) == []
