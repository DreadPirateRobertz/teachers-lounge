"""Re-ranking service — calls AI Gateway (LiteLLM) /rerank endpoint.

Uses Cohere rerank-english-v3.0 (or configured model) via the gateway.
Falls back to passthrough (original RRF order) if reranking is disabled
or the gateway call fails, so search never breaks due to reranker issues.
"""
import logging

import httpx

from app.config import settings
from app.models import ChunkResult

logger = logging.getLogger(__name__)


async def rerank(query: str, results: list[ChunkResult]) -> list[ChunkResult]:
    """Re-rank *results* using a cross-encoder model via the AI Gateway.

    Returns results re-sorted by relevance score from the reranker.
    Falls back to original order on failure or if reranking is disabled.
    """
    if not settings.rerank_model or len(results) <= 1:
        return results

    documents = [r.content for r in results]
    top_n = min(settings.rerank_top_n, len(results))

    try:
        url = f"{settings.ai_gateway_url}/rerank"
        payload = {
            "model": settings.rerank_model,
            "query": query,
            "documents": documents,
            "top_n": top_n,
        }

        async with httpx.AsyncClient(timeout=10.0) as client:
            resp = await client.post(url, json=payload)
        resp.raise_for_status()

        reranked_indices = resp.json()["results"]
        # LiteLLM /rerank returns [{index: int, relevance_score: float}, ...]
        # sorted by relevance_score descending
        reranked = []
        for item in reranked_indices:
            idx = item["index"]
            score = item["relevance_score"]
            reranked.append(results[idx].model_copy(update={"score": score}))

        logger.info("rerank query=%r top_n=%d → %d results", query[:50], top_n, len(reranked))
        return reranked

    except Exception:
        logger.warning("rerank failed, falling back to RRF order", exc_info=True)
        return results
