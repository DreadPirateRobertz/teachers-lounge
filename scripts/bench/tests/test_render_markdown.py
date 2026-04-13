"""Tests for the markdown rendering helper.

Kept separate from the main test module so they can evolve independently
of the pure-math tests.
"""
from __future__ import annotations

from scripts.bench.qdrant_hnsw import BenchmarkResult, render_markdown


def _meta() -> dict:
    return {
        "corpus_size": 100,
        "query_size": 10,
        "dim": 1536,
        "seed": 0,
        "qdrant_url": "http://localhost:6333",
        "qdrant_version": "1.9.7",
    }


def test_render_includes_every_result_row() -> None:
    """Every BenchmarkResult should appear as its own row in the table."""
    results = [
        BenchmarkResult(m=8, ef_construct=64, recall_at_10=0.9, p50_ms=1.0, p99_ms=2.0, n_queries=10),
        BenchmarkResult(m=16, ef_construct=100, recall_at_10=0.95, p50_ms=1.5, p99_ms=3.0, n_queries=10),
    ]
    md = render_markdown(results, _meta())
    assert "| 8 | 64 |" in md
    assert "| 16 | 100 |" in md
    assert "Methodology" in md
    assert "Results" in md


def test_render_flags_flat_recall() -> None:
    """When recall is uniform across the grid the report must call it out."""
    results = [
        BenchmarkResult(m=m, ef_construct=ef, recall_at_10=1.0, p50_ms=1.0, p99_ms=2.0, n_queries=10)
        for m in (8, 16, 32)
        for ef in (64, 100, 200)
    ]
    md = render_markdown(results, _meta())
    assert "Recall caveat" in md
    assert "No config change proposed" in md


def test_render_omits_caveat_when_recall_varies() -> None:
    """If recall actually discriminates the grid, the caveat must be absent."""
    results = [
        BenchmarkResult(m=8, ef_construct=64, recall_at_10=0.80, p50_ms=1.0, p99_ms=2.0, n_queries=10),
        BenchmarkResult(m=16, ef_construct=100, recall_at_10=0.95, p50_ms=1.5, p99_ms=3.0, n_queries=10),
    ]
    md = render_markdown(results, _meta())
    assert "Recall caveat" not in md
    assert "Recommendation" in md
