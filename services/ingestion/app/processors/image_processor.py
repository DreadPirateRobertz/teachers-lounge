"""Image OCR processing pipeline using Google Document AI.

Handles JPEG and PNG uploads. Uses the Document AI OCR processor by default.
If the average word confidence is below the configured threshold AND a Form
Parser processor is configured, the document is re-processed with the Form
Parser for improved handwritten text recognition. Low-confidence regions are
flagged in chunk metadata for downstream quality signals.
"""
import asyncio
import logging
from pathlib import Path

from app.config import settings
from app.models import IngestJobMessage, ProcessingStatus
from app.services import db, embeddings, gcs, qdrant
from app.services.chunking import flush_segments as _flush_segments

logger = logging.getLogger(__name__)


async def process_image(job: IngestJobMessage) -> dict:
    """Full image OCR processing pipeline.

    Downloads the image, runs Google Document AI OCR (with optional Form
    Parser fallback for low-confidence / handwritten content), chunks the
    extracted text with confidence metadata, and writes to Qdrant + Postgres.

    Args:
        job: Pub/Sub message describing the upload.

    Returns:
        Dict with status, job_id, chunk_count, and processor key.
    """
    logger.info("image_processor: starting job_id=%s file=%s", job.job_id, job.filename)
    await db.update_material_status(job.material_id, ProcessingStatus.PROCESSING)

    local_path: Path | None = None

    try:
        local_path = await gcs.download_file(job.gcs_path, job.job_id)

        image_bytes = local_path.read_bytes()
        mime_type = job.mime_type  # image/jpeg or image/png

        loop = asyncio.get_running_loop()

        # First pass: OCR processor
        if not settings.document_ai_ocr_processor_id:
            raise RuntimeError(
                "document_ai_ocr_processor_id is not configured — "
                "set DOCUMENT_AI_OCR_PROCESSOR_ID environment variable"
            )

        segments = await loop.run_in_executor(
            None,
            _run_document_ai,
            image_bytes,
            mime_type,
            settings.document_ai_ocr_processor_id,
        )

        # Check average confidence; retry with Form Parser if content appears handwritten
        avg_confidence = _average_confidence(segments)
        if (
            avg_confidence < settings.document_ai_low_confidence_threshold
            and settings.document_ai_form_processor_id
        ):
            logger.info(
                "image_processor: job_id=%s avg_confidence=%.2f < %.2f — "
                "retrying with Form Parser",
                job.job_id, avg_confidence,
                settings.document_ai_low_confidence_threshold,
            )
            segments = await loop.run_in_executor(
                None,
                _run_document_ai,
                image_bytes,
                mime_type,
                settings.document_ai_form_processor_id,
            )

        logger.info(
            "image_processor: job_id=%s extracted %d segments avg_conf=%.2f",
            job.job_id, len(segments), _average_confidence(segments),
        )

        max_chars = settings.chunk_max_tokens * 4
        overlap_chars = settings.chunk_overlap_tokens * 4
        chunks = _flush_segments(segments, job.material_id, job.course_id, max_chars, overlap_chars)

        logger.info("image_processor: job_id=%s built %d chunks", job.job_id, len(chunks))

        if not chunks:
            await db.update_material_status(job.material_id, ProcessingStatus.COMPLETE, chunk_count=0)
            return {"status": "complete", "job_id": str(job.job_id), "chunk_count": 0, "processor": "image"}

        texts = [c["content"] for c in chunks]
        vectors = await embeddings.embed_texts(texts)

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
                "low_confidence": c.get("metadata", {}).get("low_confidence", False),
            }
            for c in chunks
        ]
        await qdrant.upsert_chunks(chunk_ids, vectors, payloads)
        await db.insert_chunks(chunks)
        await db.update_material_status(job.material_id, ProcessingStatus.COMPLETE, chunk_count=len(chunks))

        logger.info("image_processor: complete job_id=%s chunks=%d", job.job_id, len(chunks))
        return {
            "status": "complete",
            "job_id": str(job.job_id),
            "chunk_count": len(chunks),
            "processor": "image",
        }

    except Exception:
        logger.exception("image_processor: failed job_id=%s", job.job_id)
        await db.update_material_status(job.material_id, ProcessingStatus.FAILED)
        raise
    finally:
        if local_path:
            try:
                local_path.unlink(missing_ok=True)
            except Exception:
                pass


# ── Document AI ───────────────────────────────────────────────────────────────


def _run_document_ai(
    image_bytes: bytes,
    mime_type: str,
    processor_id: str,
) -> list[dict]:
    """Process an image with Google Document AI and extract text segments.

    Calls the Document AI synchronous process endpoint. Each paragraph-level
    block from the response becomes a segment. Word-level confidence scores
    are averaged per block; blocks below the configured threshold have
    ``low_confidence: True`` in their metadata.

    Args:
        image_bytes: Raw image content.
        mime_type: MIME type of the image (e.g., 'image/jpeg').
        processor_id: Document AI processor resource ID.

    Returns:
        List of segment dicts compatible with _flush_segments.
    """
    from google.cloud import documentai

    client = documentai.DocumentProcessorServiceClient()
    processor_name = client.processor_path(
        settings.gcp_project, settings.document_ai_location, processor_id
    )

    raw_doc = documentai.RawDocument(content=image_bytes, mime_type=mime_type)
    request = documentai.ProcessRequest(name=processor_name, raw_document=raw_doc)
    result = client.process_document(request=request)
    document = result.document

    return _extract_segments_from_document(
        document, threshold=settings.document_ai_low_confidence_threshold
    )


def _extract_segments_from_document(document, threshold: float = 0.7) -> list[dict]:
    """Convert a Document AI document response into text segments.

    Iterates over pages and their paragraph blocks. For each paragraph, the
    average word confidence is computed. Paragraphs with average confidence
    below the threshold are flagged as low_confidence in metadata.

    Args:
        document: A google.cloud.documentai.Document instance.
        threshold: Confidence below which a block is considered low confidence.

    Returns:
        List of segment dicts with optional 'low_confidence' metadata.
    """
    segments: list[dict] = []
    full_text = document.text

    for page_num, page in enumerate(document.pages, start=1):
        for block in page.blocks:
            text = _extract_layout_text(full_text, block.layout)
            text = text.strip()
            if not text:
                continue

            confidence = block.layout.confidence if block.layout else 1.0
            low_conf = confidence < threshold

            segments.append({
                "text": text,
                "chapter": None,
                "section": None,
                "page": page_num,
                "content_type": "text",
                "metadata": {"ocr_confidence": round(confidence, 3), "low_confidence": low_conf},
            })

    return segments


def _extract_layout_text(full_text: str, layout) -> str:
    """Extract the text for a layout element from the document full_text.

    Document AI stores all text in a single ``document.text`` string and uses
    ``TextAnchor.text_segments`` (start/end byte offsets) to reference regions.

    Args:
        full_text: The complete document text string.
        layout: A Document AI Layout object with a text_anchor field.

    Returns:
        Concatenated text for all text segments of this layout element.
    """
    if not layout or not layout.text_anchor:
        return ""
    parts: list[str] = []
    for seg in layout.text_anchor.text_segments:
        start = int(seg.start_index) if seg.start_index else 0
        end = int(seg.end_index) if seg.end_index else len(full_text)
        parts.append(full_text[start:end])
    return "".join(parts)


def _average_confidence(segments: list[dict]) -> float:
    """Compute the mean OCR confidence across all segments.

    Args:
        segments: List of segment dicts, each optionally containing
            ``metadata.ocr_confidence``.

    Returns:
        Mean confidence in [0.0, 1.0]; returns 1.0 if segments is empty.
    """
    if not segments:
        return 1.0
    confidences = [
        s.get("metadata", {}).get("ocr_confidence", 1.0)
        for s in segments
    ]
    return sum(confidences) / len(confidences)
