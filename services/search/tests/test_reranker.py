"""
Tests for RRF fusion correctness.

These tests verify the mathematical properties of Reciprocal Rank Fusion
independent of any I/O or mock infrastructure.
"""
import uuid

import pytest

from app.models import ChunkResult
from app.services.hybrid import combine_dense_sparse


def _chunk(chunk_id: uuid.UUID | None = None) -> ChunkResult:
    return ChunkResult(
        chunk_id=chunk_id or uuid.uuid4(),
        material_id=uuid.uuid4(),
        course_id=uuid.uuid4(),
        content="text",
        score=0.0,
    )


class TestRRFFusion:
    def test_rank1_dense_score(self):
        """Rank-1 item in a single list: score = 1/(k+1) = 1/61."""
        c = _chunk()
        results = combine_dense_sparse([c], [])
        assert abs(results[0].score - 1 / 61) < 1e-12

    def test_rank2_dense_score(self):
        """Rank-2 item: score = 1/(60+2) = 1/62."""
        c1, c2 = _chunk(), _chunk()
        results = combine_dense_sparse([c1, c2], [])
        assert abs(results[1].score - 1 / 62) < 1e-12

    def test_double_ranked_score(self):
        """Rank-1 in both lists: score = 2/61."""
        shared_id = uuid.uuid4()
        shared = _chunk(chunk_id=shared_id)
        results = combine_dense_sparse([shared], [_chunk(chunk_id=shared_id)])
        assert abs(results[0].score - 2 / 61) < 1e-12

    def test_overlapping_beats_disjoint(self):
        """A doc at rank 1 in both lists must outscore any doc in only one list."""
        shared_id = uuid.uuid4()
        shared = _chunk(chunk_id=shared_id)
        other = _chunk()
        results = combine_dense_sparse([shared, other], [shared])
        assert results[0].chunk_id == shared_id

    def test_disjoint_rank1_tied(self):
        """Two docs each at rank 1 in disjoint lists have the same score."""
        d = _chunk()
        s = _chunk()
        results = combine_dense_sparse([d], [s])
        assert len(results) == 2
        assert abs(results[0].score - results[1].score) < 1e-12

    def test_output_length_equals_union(self):
        """Output contains exactly |dense ∪ sparse| unique documents."""
        shared_id = uuid.uuid4()
        shared = _chunk(chunk_id=shared_id)
        d_only = _chunk()
        s_only = _chunk()
        results = combine_dense_sparse([shared, d_only], [shared, s_only])
        assert len(results) == 3  # shared + d_only + s_only

    def test_empty_both(self):
        assert combine_dense_sparse([], []) == []

    def test_empty_sparse(self):
        chunks = [_chunk() for _ in range(3)]
        results = combine_dense_sparse(chunks, [])
        assert len(results) == 3
        assert results[0].score > results[1].score > results[2].score

    def test_empty_dense(self):
        chunks = [_chunk() for _ in range(3)]
        results = combine_dense_sparse([], chunks)
        assert len(results) == 3
        assert results[0].score > results[1].score > results[2].score

    def test_score_field_is_rrf_not_original(self):
        """Output score must be the RRF value, not the original Qdrant score."""
        c = _chunk()
        c.score = 0.99  # original score
        results = combine_dense_sparse([c], [])
        # RRF score for rank 1 = 1/61 ≈ 0.016, not 0.99
        assert abs(results[0].score - 1 / 61) < 1e-9
