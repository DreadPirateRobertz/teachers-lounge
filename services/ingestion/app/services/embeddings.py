import logging

from openai import AsyncOpenAI

from app.config import settings

logger = logging.getLogger(__name__)

# Store config at init time; create client per-call since AsyncOpenAI
# is bound to the event loop it's created in and the Pub/Sub subscriber
# thread creates a new loop via asyncio.run().
_api_key: str | None = None


def init_client() -> None:
    """Store the OpenAI API key for use by embed_texts.

    Called once at application startup before the first embedding request.
    The key is stored at module level; the actual AsyncOpenAI client is
    created per-call to avoid event-loop binding issues with the Pub/Sub
    subscriber thread.
    """
    global _api_key
    _api_key = settings.openai_api_key
    logger.info("openai embedding config stored (model=%s, dim=%d)",
                settings.embedding_model, settings.embedding_dim)


async def embed_texts(texts: list[str]) -> list[list[float]]:
    """Embed a batch of texts. Returns list of 1024-dim vectors.

    Batches internally according to embedding_batch_size to stay within
    OpenAI rate limits. Input order is preserved.
    """
    if _api_key is None:
        raise RuntimeError("OpenAI not configured — call init_client() at startup")

    client = AsyncOpenAI(api_key=_api_key)
    all_embeddings: list[list[float]] = []
    batch_size = settings.embedding_batch_size

    try:
        for i in range(0, len(texts), batch_size):
            batch = texts[i:i + batch_size]
            response = await client.embeddings.create(
                model=settings.embedding_model,
                input=batch,
                dimensions=settings.embedding_dim,
                encoding_format="float",
            )
            # OpenAI returns embeddings sorted by index
            batch_embeddings = [
                item.embedding
                for item in sorted(response.data, key=lambda x: x.index)
            ]
            all_embeddings.extend(batch_embeddings)
    finally:
        await client.close()

    logger.info("embedded %d texts → %d vectors (dim=%d)",
                len(texts), len(all_embeddings), settings.embedding_dim)
    return all_embeddings
