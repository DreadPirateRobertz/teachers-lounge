"""
Hybrid search combiner — Reciprocal Rank Fusion (RRF) over dense and sparse results.

RRF formula (Cormack et al., 2009):
    rrf_score(d) = Σ  1 / (k + rank_i(d))   for each result list i
where k=60 is the standard constant that dampens the impact of very high ranks.

Documents that appear in both lists receive contributions from both, naturally
boosting results strong on both semantic and lexical axes.  Documents only in
one list still receive a score from that single list.

BM25 sparse vectors are generated at ingestion time and stored in Qdrant's
sparse vector field.  At query time the sparse field is searched in parallel
with the dense field, and the two ranked lists are fused here before re-ranking.
"""
from uuid import UUID

from app.models import ChunkResult

_RRF_K = 60


def combine_dense_sparse(
    dense_results: list[ChunkResult],
    sparse_results: list[ChunkResult],
    k: int = 60,
) -> list[ChunkResult]:
    """
    Fuse *dense_results* and *sparse_results* using Reciprocal Rank Fusion.

    Returns a new list sorted by descending RRF score.  The ``score`` field
    on each returned ChunkResult is the RRF score (not the original Qdrant
    cosine/dot score).  When *sparse_results* is empty the output preserves
    dense result ordering (each item gets score 1/(k + rank)).
    """
    rrf_scores: dict[UUID, float] = {}
    chunks_by_id: dict[UUID, ChunkResult] = {}

    for rank, chunk in enumerate(dense_results, start=1):
        cid = chunk.chunk_id
        rrf_scores[cid] = rrf_scores.get(cid, 0.0) + 1.0 / (k + rank)
        chunks_by_id[cid] = chunk

    for rank, chunk in enumerate(sparse_results, start=1):
        cid = chunk.chunk_id
        rrf_scores[cid] = rrf_scores.get(cid, 0.0) + 1.0 / (k + rank)
        chunks_by_id[cid] = chunk

    sorted_ids = sorted(rrf_scores, key=lambda cid: rrf_scores[cid], reverse=True)
    return [
        chunks_by_id[cid].model_copy(update={"score": rrf_scores[cid]})
        for cid in sorted_ids
    ]
