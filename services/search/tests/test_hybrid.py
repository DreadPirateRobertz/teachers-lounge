"""Tests for hybrid RRF combiner, re-ranker stub, embedder, and BM25 tokenizer."""
import math
import uuid
from unittest.mock import AsyncMock, patch

import pytest

from app.models import ChunkResult
from app.services.hybrid import combine_dense_sparse
from app.services.qdrant import _tokenize


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
        """Stub embedding dimension matches settings.embedding_dim (default 1536)."""
        from app.services.embedder import embed_query
        from app.config import settings
        vec = await embed_query("what is entropy?")
        assert len(vec) == settings.embedding_dim

    @pytest.mark.asyncio
    async def test_returns_unit_vector(self):
        """Stub embedding is a unit vector (L2 norm ≈ 1.0)."""
        from app.services.embedder import embed_query
        vec = await embed_query("test query")
        norm = math.sqrt(sum(x * x for x in vec))
        assert abs(norm - 1.0) < 1e-6


# ---------------------------------------------------------------------------
# BM25 tokenizer
# ---------------------------------------------------------------------------

class TestTokenize:
    def test_empty_string_returns_empty(self):
        """Non-alphabetic-only strings that produce no tokens return {}."""
        assert _tokenize("") == {}

    def test_punctuation_only_returns_empty(self):
        """Strings with no alphanumeric tokens return {}."""
        assert _tokenize("!!! ???") == {}

    def test_single_token_has_tf_one(self):
        """A single unique token has normalized TF = 1.0."""
        result = _tokenize("entropy")
        assert len(result) == 1
        assert abs(list(result.values())[0] - 1.0) < 1e-9

    def test_tfs_sum_to_one(self):
        """All TF weights for a multi-token text sum to 1.0 (plus any collision sums)."""
        result = _tokenize("what is entropy in thermodynamics")
        assert abs(sum(result.values()) - 1.0) < 1e-6

    def test_case_insensitive(self):
        """Upper and lower case produce identical sparse vectors."""
        assert _tokenize("Entropy") == _tokenize("entropy")

    def test_repeated_token_increases_weight(self):
        """A token repeated more gets a higher TF weight than a single-occurrence token."""
        result = _tokenize("a a a b")
        a_idx = hash("a") % 30_000
        b_idx = hash("b") % 30_000
        if a_idx != b_idx:  # skip if hash collision
            assert result[a_idx] > result[b_idx]

    def test_deterministic(self):
        """Same input always produces the same sparse vector."""
        assert _tokenize("machine learning") == _tokenize("machine learning")

    def test_indices_within_vocab_size(self):
        """All token indices are in [0, 30000)."""
        result = _tokenize("the quick brown fox jumps over the lazy dog")
        for idx in result:
            assert 0 <= idx < 30_000


# ---------------------------------------------------------------------------
# RRF combiner
# ---------------------------------------------------------------------------

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

    def test_embedding_vector_is_passed_to_qdrant(self):
        """The vector returned by embed_query must be forwarded to dense_search unchanged."""
        fixed_vector = [0.1] * 1536
        course_id = uuid.uuid4()

        with (
            patch("app.routers.search.embed_query", new_callable=AsyncMock, return_value=fixed_vector),
            patch("app.routers.search.dense_search", new_callable=AsyncMock, return_value=[]) as mock_dense,
            patch("app.routers.search.sparse_search", new_callable=AsyncMock, return_value=[]),
            patch("app.routers.search.rerank", new_callable=AsyncMock, side_effect=lambda q, r: r),
        ):
            from fastapi.testclient import TestClient
            from app.main import app

            with TestClient(app) as client:
                client.get(f"/v1/search?q=entropy&course_id={course_id}")

            _, kwargs = mock_dense.call_args
            assert kwargs["query_vector"] == fixed_vector, (
                "The vector from embed_query must be passed verbatim to dense_search"
            )


class TestPipelineIntegration:
    def test_embedding_vector_forwarded_with_chapter(self):
        """Chapter param must be forwarded to both dense and sparse search."""
        fixed_vector = [0.1] * 1536
        course_id = uuid.uuid4()

        with (
            patch("app.routers.search.embed_query", new_callable=AsyncMock, return_value=fixed_vector),
            patch("app.routers.search.dense_search", new_callable=AsyncMock, return_value=[]) as mock_dense,
            patch("app.routers.search.sparse_search", new_callable=AsyncMock, return_value=[]) as mock_sparse,
            patch("app.routers.search.rerank", new_callable=AsyncMock, side_effect=lambda q, r: r),
        ):
            from fastapi.testclient import TestClient
            from app.main import app

            with TestClient(app) as client:
                client.get(f"/v1/search?q=entropy&course_id={course_id}&chapter=Chapter+5")

            _, dense_kwargs = mock_dense.call_args
            assert dense_kwargs["chapter"] == "Chapter 5"
            _, sparse_kwargs = mock_sparse.call_args
            assert sparse_kwargs["chapter"] == "Chapter 5"
