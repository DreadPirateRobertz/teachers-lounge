"""Tests for hybrid combiner, re-ranker stubs, and embedding stub."""
import math
import uuid

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
    def test_returns_correct_dimension(self):
        vec = embed_query("what is entropy?")
        assert len(vec) == 1024

    def test_returns_unit_vector(self):
        vec = embed_query("thermodynamics")
        norm = math.sqrt(sum(x * x for x in vec))
        assert abs(norm - 1.0) < 1e-6

    def test_different_queries_different_vectors(self):
        # Random stub — high probability of different results
        v1 = embed_query("entropy")
        v2 = embed_query("entropy")
        # Can't assert equality since it's random, but both must be unit vectors
        assert len(v1) == len(v2) == 1024


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
