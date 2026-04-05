"""Image OCR processing pipeline via Google Document AI.

Handles JPEG and PNG files containing scanned documents, handwritten
notes, or diagrams with embedded text.  Google Document AI performs
OCR (and optionally handwriting recognition), extracting text blocks
that are then chunked and embedded.

Pipeline:

1. Download from GCS to local temp file.
2. Send to Document AI using the configured processor (OCR or form parser).
3. Reassemble text from document blocks in reading order.
4. Build text chunks using the shared ``flush_segments`` primitive.
5. Embed and store via the shared ``embed_and_store`` step.

Note:
    CLIP multi-modal embeddings for diagram retrieval are deferred to
    Phase 7 as specified in the design doc.
"""
import asyncio
import logging
from pathlib import Path
from uuid import UUID

from app.config import settings
from app.models import IngestJobMessage, ProcessingStatus
from app.processors.common import (
    download_from_gcs,
    embed_and_store,
    flush_segments,
)
from app.services import db

logger = logging.getLogger(__name__)

# Mapping from our MIME type strings to Document AI MIME type strings
_MIME_TO_DAI: dict[str, str] = {
    "image/jpeg": "image/jpeg",
    "image/png": "image/png",
}


async def process_image(job: IngestJobMessage) -> dict:
    """Full image OCR processing pipeline.

    Calls Google Document AI to extract text, builds chunks, and runs
    the shared embed-and-store step.

    Args:
        job: Ingest job message from the Pub/Sub subscription.

    Returns:
        Dict with keys: ``status``, ``job_id``, ``chunk_count``, ``processor``.

    Raises:
        ValueError: If ``document_ai_processor_name`` is not configured.
        Exception: Any exception marks the material FAILED.
    """
    if not settings.document_ai_processor_name:
        raise ValueError(
            "document_ai_processor_name must be set to use the image processor. "
            "Format: projects/{project}/locations/{location}/processors/{id}"
        )

    logger.info("image_processor: starting job_id=%s file=%s", job.job_id, job.filename)
    await db.update_material_status(job.material_id, ProcessingStatus.PROCESSING)

    local_path: Path | None = None
    try:
        local_path = await download_from_gcs(job.gcs_path, job.job_id)
        mime_type = _MIME_TO_DAI.get(job.mime_type, "image/jpeg")

        # Run blocking Document AI call in executor
        text_blocks = await asyncio.get_running_loop().run_in_executor(
            None, _ocr_with_document_ai, local_path, mime_type
        )
        logger.info("image_processor: job_id=%s extracted %d text blocks", job.job_id, len(text_blocks))

        chunks = _build_image_chunks(text_blocks, job.material_id, job.course_id)
        logger.info("image_processor: job_id=%s built %d chunks", job.job_id, len(chunks))

        chunk_count = await embed_and_store(chunks, job.material_id)
        logger.info("image_processor: complete job_id=%s chunks=%d", job.job_id, chunk_count)
        return {
            "status": "complete",
            "job_id": str(job.job_id),
            "chunk_count": chunk_count,
            "processor": "image",
        }

    except Exception:
        logger.exception("image_processor: failed job_id=%s", job.job_id)
        await db.update_material_status(job.material_id, ProcessingStatus.FAILED)
        raise
    finally:
        if local_path is not None:
            local_path.unlink(missing_ok=True)


# ── Document AI OCR ───────────────────────────────────────────────────────────


def _ocr_with_document_ai(image_path: Path, mime_type: str) -> list[str]:
    """Call Google Document AI and return a list of text block strings.

    Reads the full document text and splits it into paragraph-level
    blocks using the page layout information.  Blocks are returned in
    reading order (top-to-bottom, left-to-right).

    Args:
        image_path: Local path to the image file.
        mime_type: MIME type string to pass to Document AI
            (e.g. ``"image/jpeg"``).

    Returns:
        List of text strings, one per paragraph-level block.  Empty
        blocks are omitted.

    Raises:
        google.api_core.exceptions.GoogleAPICallError: If the Document AI
            request fails.
    """
    from google.cloud import documentai

    client = documentai.DocumentProcessorServiceClient()
    image_bytes = image_path.read_bytes()

    request = documentai.ProcessRequest(
        name=settings.document_ai_processor_name,
        raw_document=documentai.RawDocument(
            content=image_bytes,
            mime_type=mime_type,
        ),
    )
    response = client.process_document(request=request)
    document = response.document

    if not document.text:
        return []

    # Extract paragraph-level text blocks in reading order
    text_blocks: list[str] = []
    for page in document.pages:
        for paragraph in page.paragraphs:
            block_text = _extract_layout_text(document.text, paragraph.layout)
            stripped = block_text.strip()
            if stripped:
                text_blocks.append(stripped)

    # Fallback: if no paragraph layout, use the full document text
    if not text_blocks and document.text.strip():
        text_blocks = [document.text.strip()]

    return text_blocks


def _extract_layout_text(full_text: str, layout) -> str:
    """Extract the text for a Document AI layout element.

    Document AI stores all text in a single ``document.text`` string and
    references it via ``TextAnchor`` segments (start/end byte offsets).

    Args:
        full_text: The complete ``document.text`` string.
        layout: A ``documentai.Document.Page.Layout`` instance.

    Returns:
        The extracted text for this layout element.
    """
    if not layout.text_anchor or not layout.text_anchor.text_segments:
        return ""
    parts: list[str] = []
    for segment in layout.text_anchor.text_segments:
        start = int(segment.start_index)
        end = int(segment.end_index)
        parts.append(full_text[start:end])
    return "".join(parts)


# ── Chunk building ────────────────────────────────────────────────────────────


def _build_image_chunks(
    text_blocks: list[str],
    material_id: UUID,
    course_id: UUID,
) -> list[dict]:
    """Build chunks from Document AI text blocks.

    Since images have no inherent hierarchy (no headings), all blocks
    are treated as plain text segments.  Consecutive blocks are merged
    up to the ``chunk_max_tokens`` character limit.

    Args:
        text_blocks: List of text strings extracted by Document AI.
        material_id: UUID of the parent material.
        course_id: UUID of the course.

    Returns:
        List of chunk dicts ready for embedding and storage.
    """
    max_chars = settings.chunk_max_tokens * 4
    overlap_chars = settings.chunk_overlap_tokens * 4

    segments = [
        {
            "text": block,
            "chapter": None,
            "section": None,
            "page": None,
            "content_type": "text",
        }
        for block in text_blocks
        if block.strip()
    ]

    return flush_segments(segments, material_id, course_id, max_chars, overlap_chars)
