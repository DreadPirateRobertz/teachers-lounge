"""Tests for ingestion Qdrant upsert functions and BM25 tokenizer.

Covers:
- _tokenize: token extraction, TF normalization, determinism, edge cases
- upsert_chunks: sparse vectors generated from content, dense vectors preserved,
  empty-content chunks still upsert (dense-only), batching, error propagation
- upsert_diagrams: empty input no-ops, payloads forwarded correctly
"""
import uuid
from unittest.mock import AsyncMock, MagicMock, call, patch

import pytest

from app.services.qdrant import _tokenize, upsert_chunks, upsert_diagrams


# ── _tokenize ─────────────────────────────────────────────────────────────────


class TestTokenize:
    def test_empty_string_returns_empty(self):
        """Empty text produces no tokens and returns {}."""
        assert _tokenize("") == {}

    def test_punctuation_only_returns_empty(self):
        """Text with no alphanumeric characters returns {}."""
        assert _tokenize("!!! --- ???") == {}

    def test_single_token_tf_is_one(self):
        """A single unique token has TF weight 1.0."""
        result = _tokenize("entropy")
        assert len(result) == 1
        assert abs(list(result.values())[0] - 1.0) < 1e-9

    def test_tf_weights_sum_to_one(self):
        """TF weights for a multi-token string sum to 1.0 (ignoring collisions)."""
        result = _tokenize("the quick brown fox")
        assert abs(sum(result.values()) - 1.0) < 1e-6

    def test_case_insensitive(self):
        """Uppercase and lowercase inputs produce the same sparse vector."""
        assert _tokenize("Entropy") == _tokenize("entropy")
        assert _tokenize("MACHINE LEARNING") == _tokenize("machine learning")

    def test_repeated_token_higher_weight(self):
        """A token appearing multiple times has higher TF than a single-occurrence token."""
        import hashlib
        result = _tokenize("a a a b")
        a_idx = int(hashlib.sha1(b"a").hexdigest()[:8], 16) % 30_000
        b_idx = int(hashlib.sha1(b"b").hexdigest()[:8], 16) % 30_000
        if a_idx != b_idx:  # skip assertion if hash collision
            assert result[a_idx] > result[b_idx]

    def test_deterministic(self):
        """Same text always produces the identical sparse vector."""
        text = "thermodynamics entropy second law"
        assert _tokenize(text) == _tokenize(text)

    def test_indices_within_vocab_size(self):
        """All token indices fall within [0, 30000)."""
        result = _tokenize("the quick brown fox jumps over the lazy dog")
        for idx in result:
            assert 0 <= idx < 30_000


# ── upsert_chunks ─────────────────────────────────────────────────────────────


def _make_payload(content: str = "entropy is a measure of disorder") -> dict:
    """Return a minimal chunk payload dict."""
    return {
        "chunk_id": str(uuid.uuid4()),
        "material_id": str(uuid.uuid4()),
        "course_id": str(uuid.uuid4()),
        "content": content,
        "chapter": "Chapter 1",
        "section": "1.1",
        "page": 1,
        "content_type": "text",
    }


@pytest.fixture
def mock_qdrant_client():
    """Patch _make_client to return an async mock Qdrant client."""
    client = AsyncMock()
    client.upsert = AsyncMock()
    client.close = AsyncMock()
    with patch("app.services.qdrant._make_client", return_value=client):
        yield client


@pytest.mark.asyncio
async def test_upsert_chunks_stores_sparse_vector(mock_qdrant_client):
    """upsert_chunks generates and stores a sparse vector from chunk content."""
    cid = uuid.uuid4()
    vector = [0.1] * 1024
    payload = _make_payload("entropy measures disorder in a system")

    await upsert_chunks([cid], [vector], [payload])

    mock_qdrant_client.upsert.assert_awaited_once()
    call_kwargs = mock_qdrant_client.upsert.call_args
    points = call_kwargs.kwargs.get("points") or call_kwargs[1]["points"]
    assert len(points) == 1
    point = points[0]
    assert "dense" in point.vector
    assert "sparse" in point.vector
    sparse = point.vector["sparse"]
    assert len(sparse.indices) > 0
    assert len(sparse.values) == len(sparse.indices)


@pytest.mark.asyncio
async def test_upsert_chunks_preserves_dense_vector(mock_qdrant_client):
    """Dense vector is passed through unchanged alongside the sparse vector."""
    cid = uuid.uuid4()
    dense = [float(i) / 1024 for i in range(1024)]
    payload = _make_payload("test content")

    await upsert_chunks([cid], [dense], [payload])

    call_kwargs = mock_qdrant_client.upsert.call_args
    points = call_kwargs.kwargs.get("points") or call_kwargs[1]["points"]
    assert points[0].vector["dense"] == dense


@pytest.mark.asyncio
async def test_upsert_chunks_empty_content_omits_sparse(mock_qdrant_client):
    """Chunks with empty/punct-only content skip the sparse vector (dense-only upsert)."""
    cid = uuid.uuid4()
    payload = _make_payload(content="")

    await upsert_chunks([cid], [[0.0] * 1024], [payload])

    call_kwargs = mock_qdrant_client.upsert.call_args
    points = call_kwargs.kwargs.get("points") or call_kwargs[1]["points"]
    assert "dense" in points[0].vector
    assert "sparse" not in points[0].vector


@pytest.mark.asyncio
async def test_upsert_chunks_batches_correctly(mock_qdrant_client):
    """Points are batched into groups of ≤100 per upsert call."""
    n = 250
    chunk_ids = [uuid.uuid4() for _ in range(n)]
    vectors = [[0.1] * 1024 for _ in range(n)]
    payloads = [_make_payload(f"content {i}") for i in range(n)]

    await upsert_chunks(chunk_ids, vectors, payloads)

    assert mock_qdrant_client.upsert.await_count == 3  # ceil(250/100)


@pytest.mark.asyncio
async def test_upsert_chunks_client_closed_on_error(mock_qdrant_client):
    """Qdrant client is closed even when an upsert raises an exception."""
    mock_qdrant_client.upsert.side_effect = RuntimeError("connection lost")

    with pytest.raises(RuntimeError):
        await upsert_chunks(
            [uuid.uuid4()], [[0.1] * 1024], [_make_payload()]
        )

    mock_qdrant_client.close.assert_awaited_once()


@pytest.mark.asyncio
async def test_upsert_chunks_sparse_indices_within_vocab(mock_qdrant_client):
    """All sparse vector indices are within [0, 30000)."""
    payload = _make_payload("photosynthesis converts sunlight into glucose via chlorophyll")

    await upsert_chunks([uuid.uuid4()], [[0.0] * 1024], [payload])

    call_kwargs = mock_qdrant_client.upsert.call_args
    points = call_kwargs.kwargs.get("points") or call_kwargs[1]["points"]
    sparse = points[0].vector["sparse"]
    for idx in sparse.indices:
        assert 0 <= idx < 30_000


# ── upsert_diagrams ───────────────────────────────────────────────────────────


@pytest.mark.asyncio
async def test_upsert_diagrams_empty_input_no_op(mock_qdrant_client):
    """upsert_diagrams with empty lists makes no Qdrant call."""
    await upsert_diagrams([], [], [])
    mock_qdrant_client.upsert.assert_not_awaited()


@pytest.mark.asyncio
async def test_upsert_diagrams_forwards_payload(mock_qdrant_client):
    """upsert_diagrams passes diagram IDs and payloads to Qdrant correctly."""
    did = uuid.uuid4()
    vector = [0.0] * 768
    payload = {
        "diagram_id": str(did),
        "course_id": str(uuid.uuid4()),
        "gcs_path": "gs://bucket/fig.png",
        "caption": "Benzene ring",
        "figure_type": "diagram",
        "page": 3,
    }

    await upsert_diagrams([did], [vector], [payload])

    call_kwargs = mock_qdrant_client.upsert.call_args
    points = call_kwargs.kwargs.get("points") or call_kwargs[1]["points"]
    assert len(points) == 1
    assert points[0].payload == payload
    assert points[0].vector == vector
