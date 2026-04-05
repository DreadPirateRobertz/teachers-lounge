"""Shared text chunking helpers used by all processor pipelines.

Provides flush_segments and make_chunk which are used by the PDF, office,
video, and image processors to convert text segments into embeddale chunks.
"""
from uuid import UUID, uuid4


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
    """Construct a chunk dict for storage in Qdrant and Postgres.

    Args:
        content: Text content of the chunk.
        material_id: UUID of the parent material record.
        course_id: UUID of the course this material belongs to.
        chapter: Chapter heading active at this chunk, if any.
        section: Section heading active at this chunk, if any.
        page: Page (or slide) number, if known.
        content_type: Semantic type label (text, table, equation, transcript, etc.).
        metadata: Optional extra metadata stored as JSON in the chunks table.

    Returns:
        Dict with all fields needed for db.insert_chunks and qdrant.upsert_chunks.
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
    """Merge text segments into chunks respecting max_chars with overlap.

    Segments are consumed in order. When adding a segment would push the
    accumulated buffer past max_chars, the buffer is emitted as a chunk and
    replaced with an overlap tail from the previous buffer to preserve
    cross-chunk context.

    Each segment dict must contain:
        text, chapter, section, page, content_type, metadata.

    The first segment in each buffer determines the chunk's structural
    metadata (chapter, section, page, content_type, metadata).

    Args:
        segments: Ordered list of text segment dicts.
        material_id: UUID propagated into each chunk.
        course_id: UUID propagated into each chunk.
        max_chars: Maximum character length of a chunk (rough token proxy: 4 chars ≈ 1 token).
        overlap_chars: How many characters of the previous chunk to repeat at the
            start of the next chunk for context continuity.

    Returns:
        List of chunk dicts ready for embedding and storage.
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
            # Emit current buffer as a chunk
            content = "\n\n".join(buf)
            chunks.append(make_chunk(
                content=content,
                material_id=material_id,
                course_id=course_id,
                chapter=first_seg.get("chapter"),
                section=first_seg.get("section"),
                page=first_seg.get("page"),
                content_type=first_seg.get("content_type", "text"),
                metadata=first_seg.get("metadata") or {},
            ))

            # Overlap: keep the tail of the buffer
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

    # Emit the final buffer
    if buf:
        content = "\n\n".join(buf)
        chunks.append(make_chunk(
            content=content,
            material_id=material_id,
            course_id=course_id,
            chapter=first_seg.get("chapter"),
            section=first_seg.get("section"),
            page=first_seg.get("page"),
            content_type=first_seg.get("content_type", "text"),
            metadata=first_seg.get("metadata") or {},
        ))

    return chunks
