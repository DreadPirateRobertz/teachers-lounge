"""
Embedder — wraps OpenAI text-embedding-3-large API (Phase 2).

Phase 2: OpenAI API with in-process LRU cache for repeated queries.
Phase 4+: migrate to self-hosted e5-large-v2. See docs/embedding-model-decision.md.

When OPENAI_API_KEY is not set (dev/test without API access), falls back to a
random unit vector of dimension settings.embedding_dim so the service still
boots and existing tests pass.
"""
import hashlib
import logging
import math
import random

from app.config import settings

logger = logging.getLogger(__name__)

_openai_client = None  # lazily initialised to avoid import-time cost
_cache: dict[str, list[float]] = {}
_CACHE_MAX = 2048


def _get_openai_client():
    global _openai_client
    if _openai_client is None:
        from openai import AsyncOpenAI  # import here so tests can run without the package
        _openai_client = AsyncOpenAI(api_key=settings.openai_api_key)
    return _openai_client


def _random_unit_vector(dim: int) -> list[float]:
    raw = [random.gauss(0, 1) for _ in range(dim)]
    norm = math.sqrt(sum(x * x for x in raw))
    return [x / norm for x in raw]


async def embed_query(query: str) -> list[float]:
    """Embed query text into a dense unit vector of dimension settings.embedding_dim.

    Uses OpenAI text-embedding-3-large when OPENAI_API_KEY is configured,
    requesting exactly settings.embedding_dim dimensions (default 1536).
    Falls back to a random unit vector of the same dimension for local dev /
    CI without API access.
    Results are cached in-process (up to _CACHE_MAX entries) to avoid
    redundant API calls for repeated questions.
    """
    if settings.openai_api_key is None:
        logger.debug("No OPENAI_API_KEY — using random stub embedding for query")
        return _random_unit_vector(settings.embedding_dim)

    cache_key = hashlib.md5(query.encode()).hexdigest()
    if cache_key in _cache:
        return _cache[cache_key]

    client = _get_openai_client()
    response = await client.embeddings.create(
        model=settings.openai_embedding_model,
        input=[query],
        dimensions=settings.embedding_dim,
    )
    vec = response.data[0].embedding

    if len(_cache) >= _CACHE_MAX:
        # Evict the oldest entry (insertion-ordered dict, Python 3.7+)
        del _cache[next(iter(_cache))]
    _cache[cache_key] = vec
    return vec
