"""
Hybrid search combiner — Reciprocal Rank Fusion (RRF) over dense and sparse results.

RRF formula: rrf_score(d) = Σ 1 / (k + rank_i(d))   for each result set i
where k=60 is the standard constant (Cormack et al., 2009).

Chunks appearing in both dense and sparse result sets receive additive RRF
scores and float to the top, rewarding evidence from both semantic similarity
and keyword overlap. Original chunk scores are preserved — only ordering changes.

Fast path: when sparse_results is empty (sparse search not wired yet at
ingestion time), dense_results are returned unchanged in O(1).
"""
from app.models import ChunkResult

_RRF_K = 60


def combine_dense_sparse(
    dense_results: list[ChunkResult],
    sparse_results: list[ChunkResult],
) -> list[ChunkResult]:
    """
    Merge dense and sparse results using Reciprocal Rank Fusion.

    Chunks present in both lists are promoted (additive scores). Chunks
    present in only one list are included but ranked lower than overlap.
    """
    if not sparse_results:
        return dense_results  # fast path: no fusion needed

    rrf_scores: dict[str, float] = {}
    chunk_by_id: dict[str, ChunkResult] = {}

    for rank, chunk in enumerate(dense_results, start=1):
        key = str(chunk.chunk_id)
        rrf_scores[key] = rrf_scores.get(key, 0.0) + 1.0 / (_RRF_K + rank)
        chunk_by_id[key] = chunk

    for rank, chunk in enumerate(sparse_results, start=1):
        key = str(chunk.chunk_id)
        rrf_scores[key] = rrf_scores.get(key, 0.0) + 1.0 / (_RRF_K + rank)
        if key not in chunk_by_id:
            chunk_by_id[key] = chunk

    sorted_ids = sorted(rrf_scores, key=lambda k: rrf_scores[k], reverse=True)
    return [chunk_by_id[cid] for cid in sorted_ids]
