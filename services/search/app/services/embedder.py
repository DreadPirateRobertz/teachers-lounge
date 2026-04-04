"""
Embedding service — calls AI Gateway (LiteLLM) to embed a query string.

Model: text-embedding-3-small via OpenAI provider on the gateway.
Dimension: 1536 (model native — no truncation needed for the small model).

Phase 4+ migration path: update ai_gateway_url to point at self-hosted
text-embeddings-inference; no code change required.

See docs/embedding-model-decision.md for model selection rationale.
"""
import logging

import httpx

from app.config import settings

logger = logging.getLogger(__name__)


async def embed_query(query: str) -> list[float]:
    """Return a 1536-dim float vector for *query* from the AI Gateway."""
    url = f"{settings.ai_gateway_url}/embeddings"
    payload = {"model": settings.embedding_model, "input": query}

    async with httpx.AsyncClient(timeout=10.0) as client:
        resp = await client.post(url, json=payload)

    try:
        resp.raise_for_status()
    except httpx.HTTPStatusError as exc:
        logger.error(
            "embedding request failed: %s — %s",
            exc.response.status_code,
            exc.response.text,
        )
        raise

    embedding: list[float] = resp.json()["data"][0]["embedding"]
    logger.debug("embedded query len=%d chars → %d dims", len(query), len(embedding))
    return embedding
