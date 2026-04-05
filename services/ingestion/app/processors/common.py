"""Shared utilities for all ingestion processors.

Provides the hierarchical chunking primitives and the common
embed-and-store pipeline step used by every file-type processor.

The PDF processor was the reference implementation; these functions
were extracted here so office, video, and image processors can reuse
them without duplication.
"""
import asyncio
import logging
import tempfile
from pathlib import Path
from uuid import UUID, uuid4

from google.cloud import storage

from app.config import settings
from app.models import IngestJobMessage, ProcessingStatus
from app.services import db, embeddings, qdrant

logger = logging.getLogger(__name__)


# ── GCS download (shared by all processors) ──────────────────────────────────


async def download_from_gcs(gcs_path: str, job_id: UUID) -> Path:
    """Download a GCS object to a local temporary file.

    Runs the blocking GCS SDK call in a thread executor so the async
    event loop is never blocked.

    Args:
        gcs_path: Full GCS URI, e.g. ``gs://bucket/path/to/file.pdf``.
        job_id: Job UUID used to name the temp file for traceability.

    Returns:
        Path to the downloaded temporary file.  Caller is responsible
        for deleting it after use (``path.unlink(missing_ok=True)``).
    """
    loop = asyncio.get_running_loop()
    return await loop.run_in_executor(None, _download_from_gcs_sync, gcs_path, job_id)


def _download_from_gcs_sync(gcs_path: str, job_id: UUID) -> Path:
    """Synchronous GCS download. Call via ``download_from_gcs`` instead.

    Args:
        gcs_path: Full GCS URI.
        job_id: Job UUID for temp-file naming.

    Returns:
        Path to the downloaded temporary file.
    """
    parts = gcs_path.replace("gs://", "").split("/", 1)
    bucket_name, blob_name = parts[0], parts[1]

    client = storage.Client(project=settings.gcp_project)
    blob = client.bucket(bucket_name).blob(blob_name)

    suffix = Path(blob_name).suffix or ".bin"
    tmp = tempfile.NamedTemporaryFile(
        delete=False, suffix=suffix, prefix=f"ingest-{job_id}-"
    )
    blob.download_to_filename(tmp.name)
    tmp.close()
    logger.info("downloaded %s → %s", gcs_path, tmp.name)
    return Path(tmp.name)


# ── Chunking primitives ───────────────────────────────────────────────────────


def make_chunk(
    content: str,
    material_id: UUID,
    course_id: UUID,
    chapter: str | None,
    section: str | None,
    page: int | None,
    content_type: str,
    metadata: dict | None = None,
) -> dict:
    """Create a single chunk dict ready for DB/Qdrant insertion.

    Args:
        content: The text content of the chunk.
        material_id: UUID of the parent material.
        course_id: UUID of the course this material belongs to.
        chapter: Top-level heading / chapter name, or None.
        section: Sub-heading / section name, or None.
        page: Source page number (or slide index, timestamp bucket), or None.
        content_type: One of ``"text"``, ``"table"``, ``"equation"``,
            ``"figure"``, ``"quiz"``.
        metadata: Optional extra key-value pairs stored in the JSONB
            ``metadata`` column (e.g. ``{"start_time": 42.3}``).

    Returns:
        Dict with keys: id, material_id, course_id, content, chapter,
        section, page, content_type, metadata.
    """
    return {
        "id": uuid4(),
        "material_id": material_id,
        "course_id": course_id,
        "content": content,
        "chapter": chapter,
        "section": section,
        "page": page,
        "content_type": content_type,
        "metadata": metadata or {},
    }


def flush_segments(
    segments: list[dict],
    material_id: UUID,
    course_id: UUID,
    max_chars: int,
    overlap_chars: int,
) -> list[dict]:
    """Merge text segments into chunks respecting ``max_chars``, with overlap.

    Segments are accumulated greedily until adding the next segment would
    exceed ``max_chars``, at which point the current buffer is emitted as
    a chunk.  The tail of the buffer (up to ``overlap_chars``) is kept as
    context for the next chunk.

    Args:
        segments: List of segment dicts.  Each must contain:
            ``text`` (str), ``chapter`` (str | None), ``section`` (str | None),
            ``page`` (int | None), ``content_type`` (str), and optionally
            ``metadata`` (dict).
        material_id: UUID of the parent material.
        course_id: UUID of the course.
        max_chars: Maximum character count per chunk.
        overlap_chars: Number of characters to carry over as overlap.

    Returns:
        List of chunk dicts produced by :func:`make_chunk`.
    """
    if not segments:
        return []

    chunks: list[dict] = []
    buf: list[str] = []
    buf_len = 0
    first_seg = segments[0]

    for seg in segments:
        seg_text = seg["text"]
        seg_len = len(seg_text)

        if buf_len + seg_len > max_chars and buf:
            content = "\n\n".join(buf)
            chunks.append(make_chunk(
                content=content,
                material_id=material_id,
                course_id=course_id,
                chapter=first_seg.get("chapter"),
                section=first_seg.get("section"),
                page=first_seg.get("page"),
                content_type=first_seg.get("content_type", "text"),
                metadata=first_seg.get("metadata"),
            ))

            # Carry tail of buffer as overlap context
            overlap_buf: list[str] = []
            overlap_len = 0
            for t in reversed(buf):
                if overlap_len + len(t) > overlap_chars:
                    break
                overlap_buf.insert(0, t)
                overlap_len += len(t)

            buf = overlap_buf
            buf_len = overlap_len
            first_seg = seg

        buf.append(seg_text)
        buf_len += seg_len

    if buf:
        chunks.append(make_chunk(
            content="\n\n".join(buf),
            material_id=material_id,
            course_id=course_id,
            chapter=first_seg.get("chapter"),
            section=first_seg.get("section"),
            page=first_seg.get("page"),
            content_type=first_seg.get("content_type", "text"),
            metadata=first_seg.get("metadata"),
        ))

    return chunks


# ── Shared embed-and-store pipeline ──────────────────────────────────────────


async def embed_and_store(
    chunks: list[dict],
    material_id: UUID,
) -> int:
    """Embed chunks, write to Qdrant and Postgres, update material status.

    This is the common final stage shared by every processor.  If chunks
    is empty the material is immediately marked complete with chunk_count=0.

    Args:
        chunks: List of chunk dicts as produced by :func:`make_chunk` or
            :func:`flush_segments`.  Each must have id, material_id,
            course_id, content, and the standard metadata fields.
        material_id: UUID of the parent material (used for status updates).

    Returns:
        Number of chunks stored.

    Raises:
        RuntimeError: If the embedding or Qdrant services are not configured.
    """
    if not chunks:
        await db.update_material_status(material_id, ProcessingStatus.COMPLETE, chunk_count=0)
        return 0

    # 1. Embed
    texts = [c["content"] for c in chunks]
    vectors = await embeddings.embed_texts(texts)

    # 2. Upsert to Qdrant
    chunk_ids = [c["id"] for c in chunks]
    payloads = [
        {
            "chunk_id": str(c["id"]),
            "material_id": str(c["material_id"]),
            "course_id": str(c["course_id"]),
            "content": c["content"],
            "chapter": c.get("chapter"),
            "section": c.get("section"),
            "page": c.get("page"),
            "content_type": c.get("content_type", "text"),
        }
        for c in chunks
    ]
    await qdrant.upsert_chunks(chunk_ids, vectors, payloads)

    # 3. Insert chunk metadata into Postgres
    await db.insert_chunks(chunks)

    # 4. Mark material complete
    await db.update_material_status(
        material_id, ProcessingStatus.COMPLETE, chunk_count=len(chunks)
    )

    return len(chunks)
