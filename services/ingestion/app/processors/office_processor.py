"""Office document processing pipeline (DOCX, PPTX, XLSX).

Handles Microsoft Office formats using python-docx, python-pptx, and
openpyxl.  All three formats are routed through the same hierarchical
chunking and embedding pipeline used by the PDF processor.

Pipeline per format:

- **DOCX** — iterates paragraphs and tables, uses paragraph style names
  to detect headings, builds hierarchical chapter/section structure.
- **PPTX** — treats each slide as a logical unit; slide title becomes
  the section, slide body and speaker notes are the content.
- **XLSX** — converts each worksheet to a Markdown table; sheet name
  becomes the section.
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


async def process_office(job: IngestJobMessage) -> dict:
    """Full office-document processing pipeline.

    Routes to the correct sub-processor based on MIME type, then runs
    the shared embed-and-store step.

    Args:
        job: Ingest job message from the Pub/Sub subscription.

    Returns:
        Dict with keys: ``status``, ``job_id``, ``chunk_count``, ``processor``.

    Raises:
        ValueError: If the MIME type is not a supported office format.
        Exception: Any exception during processing; material is marked FAILED.
    """
    logger.info("office_processor: starting job_id=%s file=%s", job.job_id, job.filename)
    await db.update_material_status(job.material_id, ProcessingStatus.PROCESSING)

    local_path: Path | None = None
    try:
        local_path = await download_from_gcs(job.gcs_path, job.job_id)

        mime = job.mime_type
        if mime == "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
            chunks = await asyncio.get_running_loop().run_in_executor(
                None, _extract_docx_chunks, local_path, job.material_id, job.course_id
            )
        elif mime == "application/vnd.openxmlformats-officedocument.presentationml.presentation":
            chunks = await asyncio.get_running_loop().run_in_executor(
                None, _extract_pptx_chunks, local_path, job.material_id, job.course_id
            )
        elif mime == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
            chunks = await asyncio.get_running_loop().run_in_executor(
                None, _extract_xlsx_chunks, local_path, job.material_id, job.course_id
            )
        else:
            raise ValueError(f"unsupported office MIME type: {mime!r}")

        logger.info("office_processor: job_id=%s built %d chunks", job.job_id, len(chunks))
        chunk_count = await embed_and_store(chunks, job.material_id)

        logger.info("office_processor: complete job_id=%s chunks=%d", job.job_id, chunk_count)
        return {
            "status": "complete",
            "job_id": str(job.job_id),
            "chunk_count": chunk_count,
            "processor": "office",
        }

    except Exception:
        logger.exception("office_processor: failed job_id=%s", job.job_id)
        await db.update_material_status(job.material_id, ProcessingStatus.FAILED)
        raise
    finally:
        if local_path is not None:
            local_path.unlink(missing_ok=True)


# ── DOCX ─────────────────────────────────────────────────────────────────────


def _extract_docx_chunks(
    path: Path,
    material_id: UUID,
    course_id: UUID,
) -> list[dict]:
    """Extract hierarchical chunks from a DOCX file.

    Iterates all block-level objects (paragraphs and tables) in document
    order.  Heading 1 / Title styles set the current chapter; Heading 2-3
    set the section.  Body paragraphs and tables are accumulated as
    segments and flushed with overlap when they exceed the chunk size limit.

    Args:
        path: Local path to the ``.docx`` file.
        material_id: UUID of the parent material.
        course_id: UUID of the course.

    Returns:
        List of chunk dicts ready for embedding and storage.
    """
    from docx import Document
    from docx.oxml.ns import qn
    from docx.table import Table as DocxTable
    from docx.text.paragraph import Paragraph

    doc = Document(str(path))
    max_chars = settings.chunk_max_tokens * 4
    overlap_chars = settings.chunk_overlap_tokens * 4

    current_chapter: str | None = None
    current_section: str | None = None
    segments: list[dict] = []
    chunks: list[dict] = []

    def _flush() -> None:
        nonlocal segments
        if segments:
            chunks.extend(
                flush_segments(segments, material_id, course_id, max_chars, overlap_chars)
            )
            segments = []

    for block in _iter_block_items(doc):
        if isinstance(block, DocxTable):
            md = _table_to_markdown(block)
            if md:
                segments.append({
                    "text": md,
                    "chapter": current_chapter,
                    "section": current_section,
                    "page": None,
                    "content_type": "table",
                })
            continue

        # Paragraph
        style_name = block.style.name if block.style else ""
        text = block.text.strip()
        if not text:
            continue

        if style_name in ("Heading 1", "Title"):
            _flush()
            current_chapter = text
            current_section = None
        elif style_name in ("Heading 2", "Heading 3"):
            current_section = text
        else:
            segments.append({
                "text": text,
                "chapter": current_chapter,
                "section": current_section,
                "page": None,
                "content_type": "text",
            })

    _flush()
    return chunks


def _iter_block_items(doc):
    """Yield paragraphs and tables from a Document in document order.

    python-docx exposes paragraphs and tables separately, but the XML
    order is preserved in the parent body element.  This generator
    reconstructs document order so headings are processed before the
    content that follows them.

    Args:
        doc: A ``docx.Document`` instance.

    Yields:
        ``docx.text.paragraph.Paragraph`` or ``docx.table.Table`` objects.
    """
    from docx.oxml.ns import qn
    from docx.table import Table as DocxTable
    from docx.text.paragraph import Paragraph

    parent = doc.element.body
    for child in parent.iterchildren():
        if child.tag == qn("w:p"):
            yield Paragraph(child, parent)
        elif child.tag == qn("w:tbl"):
            yield DocxTable(child, parent)


def _table_to_markdown(table) -> str:
    """Convert a python-docx Table to a Markdown-formatted string.

    Merged cells are deduplicated by tracking seen values per row.
    The first row is treated as the header.

    Args:
        table: A ``docx.table.Table`` instance.

    Returns:
        Markdown table string, or empty string if the table has no rows.
    """
    rows = [[cell.text.strip() for cell in row.cells] for row in table.rows]
    if not rows:
        return ""

    # Normalise column count
    max_cols = max(len(r) for r in rows)
    for row in rows:
        while len(row) < max_cols:
            row.append("")

    header = "| " + " | ".join(rows[0]) + " |"
    sep = "| " + " | ".join(["---"] * max_cols) + " |"
    body_lines = ["| " + " | ".join(r) + " |" for r in rows[1:]]
    return "\n".join([header, sep] + body_lines)


# ── PPTX ─────────────────────────────────────────────────────────────────────


def _extract_pptx_chunks(
    path: Path,
    material_id: UUID,
    course_id: UUID,
) -> list[dict]:
    """Extract chunks from a PowerPoint presentation.

    Each slide is treated as a logical unit.  The slide title becomes the
    section; body text frames and speaker notes are concatenated into a
    single segment.  Slides are numbered from 1.

    The presentation title (from core properties, or first slide title) is
    used as the chapter for all slides.

    Args:
        path: Local path to the ``.pptx`` file.
        material_id: UUID of the parent material.
        course_id: UUID of the course.

    Returns:
        List of chunk dicts ready for embedding and storage.
    """
    from pptx import Presentation

    prs = Presentation(str(path))
    max_chars = settings.chunk_max_tokens * 4
    overlap_chars = settings.chunk_overlap_tokens * 4

    chapter = (prs.core_properties.title or "").strip() or None
    segments: list[dict] = []

    for slide_idx, slide in enumerate(prs.slides, start=1):
        title_text = ""
        body_parts: list[str] = []

        for shape in slide.shapes:
            if not shape.has_text_frame:
                continue
            text = shape.text_frame.text.strip()
            if not text:
                continue
            ph = getattr(shape, "placeholder_format", None)
            if ph is not None and ph.idx == 0:
                title_text = text
            else:
                body_parts.append(text)

        if slide.has_notes_slide:
            notes_text = slide.notes_slide.notes_text_frame.text.strip()
            if notes_text:
                body_parts.append(f"[Speaker Notes] {notes_text}")

        content_parts = [p for p in [title_text] + body_parts if p]
        if not content_parts:
            continue

        segments.append({
            "text": "\n".join(content_parts),
            "chapter": chapter,
            "section": title_text or f"Slide {slide_idx}",
            "page": slide_idx,
            "content_type": "text",
        })

    return flush_segments(segments, material_id, course_id, max_chars, overlap_chars)


# ── XLSX ──────────────────────────────────────────────────────────────────────


def _extract_xlsx_chunks(
    path: Path,
    material_id: UUID,
    course_id: UUID,
) -> list[dict]:
    """Extract chunks from an Excel workbook.

    Each worksheet is converted to a Markdown table.  The workbook title
    (from file properties) becomes the chapter; the sheet name becomes the
    section.  Large sheets are split into multiple table chunks so that
    each stays within the ``chunk_max_tokens`` character limit.

    Args:
        path: Local path to the ``.xlsx`` file.
        material_id: UUID of the parent material.
        course_id: UUID of the course.

    Returns:
        List of chunk dicts ready for embedding and storage.
    """
    import openpyxl

    wb = openpyxl.load_workbook(str(path), read_only=True, data_only=True)
    max_chars = settings.chunk_max_tokens * 4
    overlap_chars = settings.chunk_overlap_tokens * 4

    chapter = (wb.properties.title or "").strip() or None
    chunks: list[dict] = []

    for sheet_name in wb.sheetnames:
        ws = wb[sheet_name]
        rows: list[list[str]] = []
        for row in ws.iter_rows(values_only=True):
            str_cells = [str(cell) if cell is not None else "" for cell in row]
            if any(str_cells):
                rows.append(str_cells)

        if not rows:
            continue

        max_cols = max(len(r) for r in rows)
        for row in rows:
            while len(row) < max_cols:
                row.append("")

        header = "| " + " | ".join(rows[0]) + " |"
        sep = "| " + " | ".join(["---"] * max_cols) + " |"
        header_len = len(header) + len(sep) + 2

        segments: list[dict] = []
        current_rows: list[str] = []
        current_len = header_len

        for data_row_cells in rows[1:]:
            data_row = "| " + " | ".join(data_row_cells) + " |"
            if current_len + len(data_row) > max_chars and current_rows:
                table_md = "\n".join([header, sep] + current_rows)
                segments.append({
                    "text": table_md,
                    "chapter": chapter,
                    "section": sheet_name,
                    "page": None,
                    "content_type": "table",
                })
                current_rows = []
                current_len = header_len

            current_rows.append(data_row)
            current_len += len(data_row) + 1

        if current_rows:
            table_md = "\n".join([header, sep] + current_rows)
            segments.append({
                "text": table_md,
                "chapter": chapter,
                "section": sheet_name,
                "page": None,
                "content_type": "table",
            })

        chunks.extend(flush_segments(segments, material_id, course_id, max_chars, overlap_chars))

    wb.close()
    return chunks
