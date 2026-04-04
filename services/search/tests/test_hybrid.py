"""Tests for hybrid combiner (RRF), re-ranker stub, and embedder."""
import math
import uuid
from unittest.mock import AsyncMock, patch

import pytest

from app.models import ChunkResult
from app.services.embedder import embed_query
from app.services.hybrid import combine_dense_sparse
from app.services.reranker import rerank


def _chunk(score: float, chunk_id: uuid.UUID | None = None) -> ChunkResult:
    return ChunkResult(
        chunk_id=chunk_id or uuid.uuid4(),
        material_id=uuid.uuid4(),
        course_id=uuid.uuid4(),
        content="text",
        score=score,
    )


# ---------------------------------------------------------------------------
# Embedder — stub path (no API key configured)
# ---------------------------------------------------------------------------

class TestEmbedderStub:
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
            patch("app.routers.search.sparse_search", new_callable=AsyncMock, return_value=[]),
        ):
            from fastapi.testclient import TestClient
            from app.main import app

            with TestClient(app) as client:
                client.get(f"/v1/search?q=entropy&course_id={course_id}")

            _, kwargs = mock_search.call_args
            assert kwargs["query_vector"] == fixed_vector, (
                "The vector from embed_query must be passed verbatim to dense_search"
            )


# ---------------------------------------------------------------------------
# Hybrid combiner — fast path (sparse empty)
# ---------------------------------------------------------------------------

class TestHybridCombinerFastPath:
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

    def test_single_dense_chunk_passthrough(self):
        dense = [_chunk(0.95)]
        result = combine_dense_sparse(dense, [])
        assert len(result) == 1
        assert result[0].score == 0.95


# ---------------------------------------------------------------------------
# Hybrid combiner — RRF fusion (sparse non-empty)
# ---------------------------------------------------------------------------

class TestRRFFusion:
    def test_chunk_in_both_lists_ranks_above_dense_only(self):
        """
        common appears at rank 2 in dense and rank 1 in sparse.
        dense_only appears at rank 1 in dense only.

        RRF scores:
          dense_only: 1/(60+1) ≈ 0.0164
          common:     1/(60+2) + 1/(60+1) ≈ 0.0161 + 0.0164 = 0.0325

        common should win.
        """
        common_id = uuid.uuid4()
        common = _chunk(0.8, chunk_id=common_id)
        dense_only = _chunk(0.9)

        result = combine_dense_sparse(
            dense_results=[dense_only, common],   # rank 1, rank 2
            sparse_results=[common],              # rank 1
        )

        assert result[0].chunk_id == common_id

    def test_sparse_only_chunk_included_in_results(self):
        """Chunks appearing only in sparse are still returned."""
        sparse_chunk = _chunk(0.7)
        dense = [_chunk(0.9)]
        result = combine_dense_sparse(dense, [sparse_chunk])
        ids = {str(r.chunk_id) for r in result}
        assert str(sparse_chunk.chunk_id) in ids

    def test_result_length_is_union_of_both_lists(self):
        dense = [_chunk(0.9), _chunk(0.8)]
        sparse = [_chunk(0.7), _chunk(0.6)]
        result = combine_dense_sparse(dense, sparse)
        assert len(result) == 4

    def test_overlapping_lists_deduplicated(self):
        """Same chunk in both lists counts once in output."""
        shared_id = uuid.uuid4()
        shared = _chunk(0.9, chunk_id=shared_id)
        result = combine_dense_sparse([shared], [shared])
        matching = [r for r in result if r.chunk_id == shared_id]
        assert len(matching) == 1

    def test_original_scores_preserved_after_rrf(self):
        """RRF reorders but must not overwrite the original retrieval score."""
        chunk = _chunk(0.75)
        result = combine_dense_sparse([chunk], [chunk])
        assert result[0].score == 0.75


# ---------------------------------------------------------------------------
# Re-ranker stub
# ---------------------------------------------------------------------------

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
