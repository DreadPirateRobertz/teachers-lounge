"""AI Gateway client — module-level singleton with connection pooling.

A new AsyncOpenAI instance per request (the previous pattern) creates a new
httpx.AsyncClient for every call, bypassing connection reuse and adding ~10ms
overhead per request. A module-level singleton reuses the underlying HTTP
connection pool across requests.

The singleton is lazily initialised on first call so that tests can patch
settings before the client is created.
"""
from openai import AsyncOpenAI

from .config import settings

_client: AsyncOpenAI | None = None


def get_gateway_client() -> AsyncOpenAI:
    """Return the module-level AI Gateway client, creating it on first call."""
    global _client
    if _client is None:
        _client = AsyncOpenAI(
            base_url=settings.ai_gateway_url + "/v1",
            api_key=settings.ai_gateway_key,
            timeout=60.0,
            # httpx connection pool: up to 100 keep-alive connections
            max_retries=0,   # retries handled at the LiteLLM layer
        )
    return _client


def reset_gateway_client() -> None:
    """Force re-initialisation on next call. Used in tests when settings change."""
    global _client
    _client = None
