"""Tests for hybrid RRF combiner, re-ranker, and full search pipeline."""
import uuid
from unittest.mock import AsyncMock, patch

import pytest

from app.models import ChunkResult
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


class TestRRFCombiner:
    def test_empty_inputs_return_empty(self):
        assert combine_dense_sparse([], []) == []

    def test_dense_only_returns_correct_rrf_scores(self):
        """With sparse empty, each result gets 1/(60+rank) as its RRF score."""
        d1, d2 = _chunk(0.9), _chunk(0.8)
        results = combine_dense_sparse([d1, d2], [])
        assert len(results) == 2
        assert abs(results[0].score - 1 / 61) < 1e-9
        assert abs(results[1].score - 1 / 62) < 1e-9
        # Order preserved (rank 1 > rank 2)
        assert results[0].chunk_id == d1.chunk_id

    def test_disjoint_lists_same_score(self):
        """Documents at rank 1 in disjoint lists get the same RRF score."""
        d = _chunk(0.9)
        s = _chunk(0.5)
        results = combine_dense_sparse([d], [s])
        assert len(results) == 2
        for r in results:
            assert abs(r.score - 1 / 61) < 1e-9

    def test_overlapping_doc_gets_higher_score(self):
        """A doc ranked #1 in both dense and sparse should outscore all others."""
        shared_id = uuid.uuid4()
        shared = _chunk(0.9, chunk_id=shared_id)
        dense_only = _chunk(0.95)
        sparse_only = _chunk(0.95)

        results = combine_dense_sparse([shared, dense_only], [shared, sparse_only])
        assert results[0].chunk_id == shared_id
        assert abs(results[0].score - 2 / 61) < 1e-9

    def test_custom_k_zero(self):
        d = _chunk(0.9)
        results = combine_dense_sparse([d], [], k=0)
        assert abs(results[0].score - 1.0) < 1e-9  # 1/(0+1)

    def test_output_sorted_descending(self):
        chunks = [_chunk(float(i)) for i in range(5)]
        results = combine_dense_sparse(chunks, [])
        scores = [r.score for r in results]
        assert scores == sorted(scores, reverse=True)

    def test_embedding_vector_is_passed_to_qdrant(self):
        """The vector returned by embed_query must be forwarded to dense_search unchanged."""
        fixed_vector = [0.1] * 1536
        course_id = uuid.uuid4()

        with (
            patch("app.services.embedder.embed_query", new_callable=AsyncMock, return_value=fixed_vector),
            patch("app.routers.search.dense_search", new_callable=AsyncMock, return_value=[]) as mock_dense,
            patch("app.routers.search.sparse_search", new_callable=AsyncMock, return_value=[]),
        ):
            from fastapi.testclient import TestClient
            from app.main import app

            with TestClient(app) as client:
                client.get(f"/v1/search?q=entropy&course_id={course_id}")

            _, kwargs = mock_dense.call_args
            assert kwargs["query_vector"] == fixed_vector, (
                "The vector from embed_query must be passed verbatim to dense_search"
            )


class TestReranker:
    def test_identity_returns_same_results(self):
        results = [_chunk(0.9), _chunk(0.8), _chunk(0.7)]
        assert rerank("entropy", results) == results

    def test_empty_input(self):
        assert rerank("query", []) == []
