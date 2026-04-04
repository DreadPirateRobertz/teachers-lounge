"""
Hybrid search combiner — merges dense and sparse results.

Phase 2 full implementation: Reciprocal Rank Fusion (RRF) over dense
(semantic) and sparse (BM25) result sets. For now, sparse is not wired —
this just returns the dense results as-is.
"""
from app.models import ChunkResult


def combine_dense_sparse(
    dense_results: list[ChunkResult],
    sparse_results: list[ChunkResult],
) -> list[ChunkResult]:
    """
    Stub: returns dense_results unchanged.

    Phase 2: implement RRF fusion:
        rrf_score(d) = Σ 1 / (k + rank_i(d))   for each result set i
    where k=60 is the standard RRF constant. Re-sort by fused score.
    BM25 sparse vectors are generated at ingestion time and stored in
    Qdrant's sparse vector field.
    """
    return dense_results
