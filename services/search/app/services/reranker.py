"""
Re-ranking interface stub — identity function, returns results unchanged.

Phase 2 full implementation: cross-encoder model (ms-marco-MiniLM-L-6-v2 or
similar) running on CPU in-cluster. Accepts query + top-k chunks, returns
re-scored and re-sorted results.
"""
from app.models import ChunkResult


def rerank(query: str, results: list[ChunkResult]) -> list[ChunkResult]:
    """Stub: return results unchanged. Phase 2: cross-encoder re-scoring."""
    return results
