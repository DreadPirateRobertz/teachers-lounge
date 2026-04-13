"""Unit tests for the HNSW benchmark harness pure logic.

These tests cover vector generation, recall computation, and percentile
math without requiring a live Qdrant instance.
"""
from __future__ import annotations

import numpy as np
import pytest

from scripts.bench.qdrant_hnsw import (
    BenchmarkResult,
    brute_force_top_k,
    generate_unit_vectors,
    percentile,
    recall_at_k,
)


class TestGenerateUnitVectors:
    """Cover seeded vector generation shape, norm, and determinism."""

    def test_shape_matches_request(self) -> None:
        """Generated matrix should have (n, dim) shape."""
        v = generate_unit_vectors(5, 1536, seed=0)
        assert v.shape == (5, 1536)

    def test_rows_are_unit_norm(self) -> None:
        """Every row should have L2 norm ≈ 1 for cosine-distance realism."""
        v = generate_unit_vectors(10, 128, seed=1)
        norms = np.linalg.norm(v, axis=1)
        assert np.allclose(norms, 1.0, atol=1e-6)

    def test_deterministic_under_same_seed(self) -> None:
        """Same seed must produce identical vectors for reproducible benches."""
        a = generate_unit_vectors(3, 32, seed=42)
        b = generate_unit_vectors(3, 32, seed=42)
        assert np.array_equal(a, b)

    def test_different_seeds_differ(self) -> None:
        """Different seeds should produce different vectors."""
        a = generate_unit_vectors(3, 32, seed=1)
        b = generate_unit_vectors(3, 32, seed=2)
        assert not np.array_equal(a, b)


class TestBruteForceTopK:
    """Cover the exact top-k brute-force ground-truth helper."""

    def test_returns_k_indices_per_query(self) -> None:
        """Output rows should each contain exactly k indices."""
        corpus = generate_unit_vectors(50, 16, seed=0)
        queries = generate_unit_vectors(3, 16, seed=1)
        idx = brute_force_top_k(corpus, queries, k=5)
        assert idx.shape == (3, 5)

    def test_nearest_to_self_is_self(self) -> None:
        """A query vector drawn from the corpus must retrieve itself first."""
        corpus = generate_unit_vectors(20, 8, seed=0)
        # Use first corpus row as query — nearest neighbour is itself.
        queries = corpus[:1]
        idx = brute_force_top_k(corpus, queries, k=1)
        assert idx[0, 0] == 0

    def test_k_greater_than_corpus_raises(self) -> None:
        """Requesting more neighbours than corpus size must fail loudly."""
        corpus = generate_unit_vectors(3, 4, seed=0)
        queries = generate_unit_vectors(1, 4, seed=1)
        with pytest.raises(ValueError):
            brute_force_top_k(corpus, queries, k=5)


class TestRecallAtK:
    """Cover the set-overlap recall@k metric."""

    def test_perfect_recall(self) -> None:
        """Identical predictions and ground truth → recall 1.0."""
        truth = np.array([[1, 2, 3], [4, 5, 6]])
        pred = np.array([[1, 2, 3], [4, 5, 6]])
        assert recall_at_k(truth, pred) == pytest.approx(1.0)

    def test_zero_recall(self) -> None:
        """Disjoint predictions → recall 0."""
        truth = np.array([[1, 2, 3]])
        pred = np.array([[7, 8, 9]])
        assert recall_at_k(truth, pred) == pytest.approx(0.0)

    def test_order_does_not_matter(self) -> None:
        """Recall is a set-overlap measure — order must not affect it."""
        truth = np.array([[1, 2, 3]])
        pred = np.array([[3, 2, 1]])
        assert recall_at_k(truth, pred) == pytest.approx(1.0)

    def test_partial_overlap(self) -> None:
        """Two of three present → 2/3."""
        truth = np.array([[1, 2, 3]])
        pred = np.array([[1, 2, 9]])
        assert recall_at_k(truth, pred) == pytest.approx(2 / 3)

    def test_shape_mismatch_raises(self) -> None:
        """Different k between truth and pred must fail loudly."""
        truth = np.array([[1, 2, 3]])
        pred = np.array([[1, 2]])
        with pytest.raises(ValueError):
            recall_at_k(truth, pred)


class TestPercentile:
    """Cover the linear-interpolation percentile helper."""

    def test_p50_of_uniform(self) -> None:
        """Median of 0..100 is 50."""
        vals = list(range(101))
        assert percentile(vals, 50) == pytest.approx(50.0)

    def test_p99_of_uniform(self) -> None:
        """99th percentile of 0..100 is 99."""
        vals = list(range(101))
        assert percentile(vals, 99) == pytest.approx(99.0)

    def test_empty_raises(self) -> None:
        """Empty input should fail — no well-defined percentile."""
        with pytest.raises(ValueError):
            percentile([], 50)


class TestBenchmarkResult:
    """Cover BenchmarkResult immutability and rendering."""

    def test_frozen_dataclass(self) -> None:
        """BenchmarkResult instances must be immutable so results can't be stomped."""
        r = BenchmarkResult(m=16, ef_construct=100, recall_at_10=0.9, p50_ms=1.0, p99_ms=5.0, n_queries=100)
        with pytest.raises(Exception):
            r.m = 8  # type: ignore[misc]

    def test_markdown_row(self) -> None:
        """markdown_row() must render a pipe-delimited line."""
        r = BenchmarkResult(m=16, ef_construct=100, recall_at_10=0.912, p50_ms=1.23, p99_ms=4.56, n_queries=100)
        row = r.markdown_row()
        assert row.startswith("| 16 | 100 |")
        assert "0.912" in row
        assert "4.56" in row
