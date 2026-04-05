import asyncio
import logging
import tempfile
from pathlib import Path
from uuid import UUID, uuid4

# google-cloud-storage, pdfminer, and unstructured are imported lazily inside
# functions so that importing this module does not trigger those heavy imports.
# This keeps test collection fast for modules that only need routing logic.
from app.config import settings
from app.models import IngestJobMessage, ProcessingStatus
from app.services import clip_embedder, db, embeddings, gcs, qdrant

logger = logging.getLogger(__name__)


async def process_pdf(job: IngestJobMessage) -> dict:
    """Full PDF processing pipeline:

    1. Download from GCS to temp file
    2. Detect digital vs scanned via pdfminer heuristic
    3. Parse with unstructured.io (layout-aware)
    4. Build hierarchical chunks with metadata
    5. Generate embeddings via OpenAI text-embedding-3-large
    6. Write vectors to Qdrant curriculum collection
    7. Write chunk metadata to Postgres chunks table
    8. Update material status to complete
    """
    logger.info("pdf_processor: starting job_id=%s file=%s", job.job_id, job.filename)

    # Mark as processing
    await db.update_material_status(job.material_id, ProcessingStatus.PROCESSING)

    try:
        # 1. Download from GCS
        local_path = await _download_from_gcs(job.gcs_path, job.job_id)

        # 2. Detect digital vs scanned
        is_digital = await asyncio.get_running_loop().run_in_executor(
            None, _is_digital_pdf, local_path
        )
        logger.info("job_id=%s digital=%s", job.job_id, is_digital)

        # 3. Parse with unstructured.io
        elements = await asyncio.get_running_loop().run_in_executor(
            None, _partition_pdf, local_path, is_digital
        )
        logger.info("job_id=%s extracted %d elements", job.job_id, len(elements))

        # 4. Build hierarchical chunks
        chunks = _build_hierarchical_chunks(
            elements=elements,
            material_id=job.material_id,
            course_id=job.course_id,
        )
        logger.info("job_id=%s built %d chunks", job.job_id, len(chunks))

        if not chunks:
            logger.warning("job_id=%s no chunks produced — marking complete with 0", job.job_id)
            await db.update_material_status(
                job.material_id, ProcessingStatus.COMPLETE, chunk_count=0
            )
            return {"status": "complete", "job_id": str(job.job_id), "chunk_count": 0}

        # 5. Generate embeddings
        texts = [c["content"] for c in chunks]
        vectors = await embeddings.embed_texts(texts)

        # 6. Write to Qdrant
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

        # 7. Write chunk metadata to Postgres
        await db.insert_chunks(chunks)

        # 8. Extract figures and generate CLIP embeddings for diagram search
        diagram_count = await _process_figures(
            elements=elements,
            material_id=job.material_id,
            course_id=job.course_id,
            job_id=job.job_id,
            local_pdf_path=local_path,
        )
        logger.info("job_id=%s diagram_count=%d", job.job_id, diagram_count)

        # 9. Update material status
        await db.update_material_status(
            job.material_id, ProcessingStatus.COMPLETE, chunk_count=len(chunks)
        )

        logger.info("pdf_processor: complete job_id=%s chunks=%d diagrams=%d",
                    job.job_id, len(chunks), diagram_count)
        return {
            "status": "complete",
            "job_id": str(job.job_id),
            "chunk_count": len(chunks),
            "diagram_count": diagram_count,
            "processor": "pdf",
        }

    except Exception:
        logger.exception("pdf_processor: failed job_id=%s", job.job_id)
        await db.update_material_status(job.material_id, ProcessingStatus.FAILED)
        raise
    finally:
        # Clean up temp file
        try:
            local_path.unlink(missing_ok=True)
        except Exception:
            pass


async def _download_from_gcs(gcs_path: str, job_id: UUID) -> Path:
    """Download a GCS object to a local temp file. Returns the Path."""
    loop = asyncio.get_running_loop()
    return await loop.run_in_executor(None, _download_from_gcs_sync, gcs_path, job_id)


def _download_from_gcs_sync(gcs_path: str, job_id: UUID) -> Path:
    # gcs_path is like gs://bucket/path/to/file.pdf
    from google.cloud import storage  # lazy import — avoids heavy dep at module load

    parts = gcs_path.replace("gs://", "").split("/", 1)
    bucket_name, blob_name = parts[0], parts[1]

    client = storage.Client(project=settings.gcp_project)
    bucket = client.bucket(bucket_name)
    blob = bucket.blob(blob_name)

    suffix = Path(blob_name).suffix or ".pdf"
    tmp = tempfile.NamedTemporaryFile(delete=False, suffix=suffix, prefix=f"ingest-{job_id}-")
    blob.download_to_filename(tmp.name)
    tmp.close()
    logger.info("downloaded %s → %s", gcs_path, tmp.name)
    return Path(tmp.name)


def _is_digital_pdf(path: Path) -> bool:
    """Heuristic: if pdfminer can extract >50 chars of text, it's a digital PDF."""
    from pdfminer.high_level import extract_text as pdfminer_extract_text  # lazy

    try:
        text = pdfminer_extract_text(str(path), maxpages=3)
        return len(text.strip()) > 50
    except Exception:
        return False


def _partition_pdf(path: Path, is_digital: bool) -> list:
    """Use unstructured.io to parse PDF with layout awareness."""
    from unstructured.partition.pdf import partition_pdf  # lazy

    strategy = "hi_res" if not is_digital else "fast"
    return partition_pdf(
        filename=str(path),
        strategy=strategy,
        include_page_breaks=True,
        infer_table_structure=True,
    )


def _build_hierarchical_chunks(
    elements: list,
    material_id: UUID,
    course_id: UUID,
) -> list[dict]:
    """Build hierarchical chunks from unstructured elements.

    Tracks current chapter/section headings and groups content
    into chunks of ~chunk_max_tokens, splitting at paragraph
    boundaries when possible.
    """
    # Lazy imports — unstructured is only needed when actually processing a PDF
    from unstructured.documents.elements import (  # noqa: PLC0415
        Formula,
        Table,
        Title,
    )

    max_chars = settings.chunk_max_tokens * 4  # rough token-to-char ratio
    overlap_chars = settings.chunk_overlap_tokens * 4

    current_chapter: str | None = None
    current_section: str | None = None
    current_page: int | None = None
    chunks: list[dict] = []

    # Accumulate text segments with their metadata
    segments: list[dict] = []

    for element in elements:
        page = _get_page_number(element)
        if page is not None:
            current_page = page

        if isinstance(element, Title):
            text = element.text.strip()
            if not text:
                continue
            # Heuristic: short titles = chapter, longer titles = section
            if len(text) < 60 and text[0].isupper():
                # Flush accumulated segments before chapter change
                if segments:
                    chunks.extend(_flush_segments(
                        segments, material_id, course_id, max_chars, overlap_chars
                    ))
                    segments = []
                current_chapter = text
                current_section = None
            else:
                current_section = text
            continue

        content_type = _classify_element(element)
        text = element.text.strip() if hasattr(element, "text") else ""
        if not text:
            continue

        # For tables, wrap in markdown code block for preservation
        if isinstance(element, Table) and hasattr(element, "metadata"):
            html = getattr(element.metadata, "text_as_html", None)
            if html:
                text = f"[TABLE]\n{html}\n[/TABLE]"
                content_type = "table"

        if isinstance(element, Formula):
            text = f"[EQUATION] {text} [/EQUATION]"
            content_type = "equation"

        segments.append({
            "text": text,
            "chapter": current_chapter,
            "section": current_section,
            "page": current_page,
            "content_type": content_type,
        })

    # Flush remaining segments
    if segments:
        chunks.extend(_flush_segments(
            segments, material_id, course_id, max_chars, overlap_chars
        ))

    return chunks


def _flush_segments(
    segments: list[dict],
    material_id: UUID,
    course_id: UUID,
    max_chars: int,
    overlap_chars: int,
) -> list[dict]:
    """Merge segments into chunks respecting max_chars, with overlap."""
    chunks: list[dict] = []
    buf: list[str] = []
    buf_len = 0
    first_seg = segments[0] if segments else {}

    for seg in segments:
        seg_text = seg["text"]
        seg_len = len(seg_text)

        if buf_len + seg_len > max_chars and buf:
            # Emit current buffer as a chunk
            content = "\n\n".join(buf)
            chunks.append(_make_chunk(
                content=content,
                material_id=material_id,
                course_id=course_id,
                chapter=first_seg.get("chapter"),
                section=first_seg.get("section"),
                page=first_seg.get("page"),
                content_type=first_seg.get("content_type", "text"),
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

    # Final chunk
    if buf:
        content = "\n\n".join(buf)
        chunks.append(_make_chunk(
            content=content,
            material_id=material_id,
            course_id=course_id,
            chapter=first_seg.get("chapter"),
            section=first_seg.get("section"),
            page=first_seg.get("page"),
            content_type=first_seg.get("content_type", "text"),
        ))

    return chunks


def _make_chunk(
    content: str,
    material_id: UUID,
    course_id: UUID,
    chapter: str | None,
    section: str | None,
    page: int | None,
    content_type: str,
) -> dict:
    return {
        "id": uuid4(),
        "material_id": material_id,
        "course_id": course_id,
        "content": content,
        "chapter": chapter,
        "section": section,
        "page": page,
        "content_type": content_type,
        "metadata": {},
    }


def _get_page_number(element) -> int | None:
    """Extract page number from unstructured element metadata."""
    meta = getattr(element, "metadata", None)
    if meta is None:
        return None
    return getattr(meta, "page_number", None)


def _classify_element(element) -> str:
    """Map unstructured element type to our content_type enum."""
    from unstructured.documents.elements import (  # noqa: PLC0415
        FigureCaption,
        Formula,
        Image,
        Table,
    )

    if isinstance(element, Table):
        return "table"
    if isinstance(element, Formula):
        return "equation"
    if isinstance(element, (FigureCaption, Image)):
        return "figure"
    return "text"


async def _process_figures(
    elements: list,
    material_id: UUID,
    course_id: UUID,
    job_id: UUID,
    local_pdf_path: Path,
) -> int:
    """Extract figures from parsed PDF elements, generate CLIP embeddings, and upsert.

    Iterates over elements looking for Image/FigureCaption pairs.  For each
    figure that has a saved image path in its metadata (unstructured hi_res
    mode writes these to a temp dir), embeds the image with CLIP and upserts
    the vector into the diagrams Qdrant collection.

    Degrades gracefully — any per-figure error is logged and skipped so that
    the main ingestion pipeline is not interrupted.

    Args:
        elements: Parsed unstructured elements from the PDF.
        material_id: UUID of the material record.
        course_id: UUID of the course this material belongs to.
        job_id: UUID of the ingest job (used for logging).
        local_pdf_path: Path to the local PDF (used for context only).

    Returns:
        Number of diagrams successfully upserted.
    """
    diagram_ids: list[UUID] = []
    vectors: list[list[float]] = []
    payloads: list[dict] = []

    current_chapter: str | None = None
    current_page: int | None = None
    pending_caption: str | None = None

    for element in elements:
        page = _get_page_number(element)
        if page is not None:
            current_page = page

        from unstructured.documents.elements import (  # noqa: PLC0415
            FigureCaption,
            Image,
            Title as _Title,
        )
        if isinstance(element, _Title):
            text = element.text.strip()
            if text and len(text) < 60 and text[0].isupper():
                current_chapter = text
            continue

        if isinstance(element, FigureCaption):
            pending_caption = element.text.strip()
            continue

        if isinstance(element, Image):
            meta = getattr(element, "metadata", None)
            image_path_str = getattr(meta, "image_path", None) if meta else None

            if not image_path_str:
                pending_caption = None
                continue

            image_path = Path(image_path_str)
            if not image_path.exists():
                logger.debug("job_id=%s figure image not found: %s", job_id, image_path)
                pending_caption = None
                continue

            caption = pending_caption or element.text.strip() or ""
            pending_caption = None

            # Determine figure type from caption heuristics
            caption_lower = caption.lower()
            if any(w in caption_lower for w in ("table", "tbl.")):
                figure_type = "table"
            elif any(w in caption_lower for w in ("equation", "eq.", "formula")):
                figure_type = "equation_image"
            elif any(w in caption_lower for w in ("chart", "graph", "plot")):
                figure_type = "chart"
            else:
                figure_type = "diagram"

            try:
                vector = await clip_embedder.embed_image(image_path)
            except Exception as exc:
                logger.warning("job_id=%s CLIP embed failed for %s: %s", job_id, image_path, exc)
                continue

            diagram_id = uuid4()
            gcs_blob_name = f"{course_id}/{material_id}/figures/{diagram_id}.png"

            # Upload extracted figure to GCS so the frontend can serve it.
            # Degrade gracefully — a failed upload is logged and the figure skipped
            # rather than aborting the entire ingestion job.
            try:
                gcs_path = await gcs.upload_figure(image_path, gcs_blob_name)
            except Exception as exc:
                logger.warning(
                    "job_id=%s GCS figure upload failed for %s: %s", job_id, diagram_id, exc
                )
                continue

            diagram_ids.append(diagram_id)
            vectors.append(vector)
            payloads.append({
                "diagram_id": str(diagram_id),
                "material_id": str(material_id),
                "course_id": str(course_id),
                "gcs_path": gcs_path,
                "caption": caption,
                "figure_type": figure_type,
                "chapter": current_chapter,
                "page": current_page,
            })

    if diagram_ids:
        await qdrant.upsert_diagrams(diagram_ids, vectors, payloads)

    return len(diagram_ids)
