import hashlib
import logging
import re
from collections import Counter
from uuid import UUID

from qdrant_client import AsyncQdrantClient
from qdrant_client.models import PointStruct, SparseVector

from app.config import settings

logger = logging.getLogger(__name__)

# Connection params stored at init; client created per event loop since
# AsyncQdrantClient is bound to the loop it's created in. The Pub/Sub
# subscriber thread calls asyncio.run() which creates a new loop each time.
_client_kwargs: dict | None = None

_VOCAB_SIZE = 30_000


def init_client() -> None:
    """Store connection params. Actual client created lazily per-loop."""
    global _client_kwargs
    _client_kwargs = dict(host=settings.qdrant_host, port=settings.qdrant_port)
    if settings.qdrant_api_key is not None:
        _client_kwargs["api_key"] = settings.qdrant_api_key
    logger.info("qdrant config stored → %s:%d", settings.qdrant_host, settings.qdrant_port)


def _make_client() -> AsyncQdrantClient:
    """Create a new Qdrant client bound to the current event loop."""
    if _client_kwargs is None:
        raise RuntimeError("Qdrant not configured — call init_client() at startup")
    return AsyncQdrantClient(**_client_kwargs)


async def close_client() -> None:
    """No-op — clients are created and closed per upsert call."""
    pass


def _tokenize(text: str) -> dict[int, float]:
    """Produce a sparse BM25-style term-frequency vector from text.

    Maps each lowercase alphanumeric token to a deterministic integer index
    via sha1(token)[:8] % VOCAB_SIZE, with normalized TF as the weight.
    This matches the tokenization used by the search service so that at query
    time the two sparse vectors are compatible.

    Hash collisions are resolved by summing TF weights — benign for retrieval.

    Args:
        text: Raw chunk text to tokenize.

    Returns:
        Dict mapping token index → normalized TF weight, or {} for empty/punct input.
    """
    tokens = re.findall(r"[a-z0-9]+", text.lower())
    if not tokens:
        return {}
    counts = Counter(tokens)
    total = sum(counts.values())
    sparse: dict[int, float] = {}
    for token, count in counts.items():
        idx = int(hashlib.sha1(token.encode()).hexdigest()[:8], 16) % _VOCAB_SIZE
        sparse[idx] = sparse.get(idx, 0.0) + count / total
    return sparse


async def upsert_chunks(
    chunk_ids: list[UUID],
    vectors: list[list[float]],
    payloads: list[dict],
) -> None:
    """Upsert chunk vectors into the curriculum collection.

    Each chunk is stored with both a dense vector and a BM25-style sparse
    vector computed from the chunk's ``content`` payload field.  The sparse
    vector enables hybrid search at query time; chunks with empty content
    are stored with dense-only vectors.

    Each payload should contain: chunk_id, material_id, course_id,
    content, chapter, section, page, content_type.

    Args:
        chunk_ids: UUIDs for each chunk point.
        vectors: Corresponding dense embedding vectors.
        payloads: Metadata dicts for each chunk; must include ``content`` for
            sparse vector generation.
    """
    client = _make_client()
    points = []
    for chunk_id, dense_vector, payload in zip(chunk_ids, vectors, payloads):
        sparse_tf = _tokenize(payload.get("content", ""))
        point_vector: dict = {"dense": dense_vector}
        if sparse_tf:
            point_vector["sparse"] = SparseVector(
                indices=list(sparse_tf.keys()),
                values=list(sparse_tf.values()),
            )
        points.append(
            PointStruct(
                id=str(chunk_id),
                vector=point_vector,
                payload=payload,
            )
        )

    # Upsert in batches of 100 to avoid payload size limits
    batch_size = 100
    try:
        for i in range(0, len(points), batch_size):
            batch = points[i:i + batch_size]
            await client.upsert(
                collection_name=settings.curriculum_collection,
                points=batch,
            )
    finally:
        await client.close()

    logger.info("upserted %d points to collection=%s",
                len(points), settings.curriculum_collection)


async def upsert_diagrams(
    diagram_ids: list,
    vectors: list[list[float]],
    payloads: list[dict],
) -> None:
    """Upsert diagram CLIP vectors into the diagrams collection.

    Each payload should contain: diagram_id, course_id, gcs_path,
    caption, figure_type, chapter, page.

    Args:
        diagram_ids: List of diagram IDs (UUID or str).
        vectors: Corresponding 768-d CLIP image vectors.
        payloads: Metadata dicts for each diagram.
    """
    if not diagram_ids:
        return

    client = _make_client()
    points = [
        PointStruct(
            id=str(did),
            vector=vector,
            payload=payload,
        )
        for did, vector, payload in zip(diagram_ids, vectors, payloads)
    ]

    batch_size = 100
    try:
        for i in range(0, len(points), batch_size):
            batch = points[i:i + batch_size]
            await client.upsert(
                collection_name=settings.diagrams_collection,
                points=batch,
            )
    finally:
        await client.close()

    logger.info("upserted %d diagrams to collection=%s",
                len(points), settings.diagrams_collection)
