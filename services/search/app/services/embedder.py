"""
Embedding stub — returns a random unit vector.

Phase 2 full implementation will call the OpenAI text-embedding-3-large API
(dimensions=1024) with Redis caching on the query text. See
docs/embedding-model-decision.md for rationale.
"""
import math
import random

from app.config import settings


async def embed_query(query: str) -> list[float]:
    """
    Return a random 1024-dim unit vector. Stub only.

    Async signature matches Phase 2 implementation (OpenAI API call with
    Redis cache). Callers must await this function.
    """
    raw = [random.gauss(0, 1) for _ in range(settings.embedding_dim)]
    norm = math.sqrt(sum(x * x for x in raw))
    return [x / norm for x in raw]
