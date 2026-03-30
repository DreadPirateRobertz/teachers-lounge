import logging

from openai import AsyncOpenAI

from app.config import settings

logger = logging.getLogger(__name__)

_client: AsyncOpenAI | None = None


def init_client() -> None:
    global _client
    _client = AsyncOpenAI(api_key=settings.openai_api_key)
    logger.info("openai embedding client initialized (model=%s, dim=%d)",
                settings.embedding_model, settings.embedding_dim)


def get_client() -> AsyncOpenAI:
    if _client is None:
        raise RuntimeError("OpenAI client not initialized — call init_client() at startup")
    return _client


async def embed_texts(texts: list[str]) -> list[list[float]]:
    """Embed a batch of texts. Returns list of 1024-dim vectors.

    Batches internally according to embedding_batch_size to stay within
    OpenAI rate limits. Input order is preserved.
    """
    client = get_client()
    all_embeddings: list[list[float]] = []
    batch_size = settings.embedding_batch_size

    for i in range(0, len(texts), batch_size):
        batch = texts[i:i + batch_size]
        response = await client.embeddings.create(
            model=settings.embedding_model,
            input=batch,
            dimensions=settings.embedding_dim,
            encoding_format="float",
        )
        # OpenAI returns embeddings sorted by index
        batch_embeddings = [item.embedding for item in sorted(response.data, key=lambda x: x.index)]
        all_embeddings.extend(batch_embeddings)

    logger.info("embedded %d texts → %d vectors (dim=%d)",
                len(texts), len(all_embeddings), settings.embedding_dim)
    return all_embeddings
