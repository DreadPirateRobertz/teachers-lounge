"""PyMuPDF-based figure/diagram extraction for PDF ingestion (Phase 6).

Extracts embedded images from PDF pages as PNG crops and detects nearby
figure captions by scanning text blocks adjacent to each image bounding box.
Used by the PDF processor to feed CLIP embedding and Qdrant diagram upsert.

Falls back to an empty list if ``fitz`` (PyMuPDF) is not installed so that
the rest of the ingestion pipeline can run in lightweight environments.
"""
from __future__ import annotations

import logging
import tempfile
from dataclasses import dataclass, field
from pathlib import Path

logger = logging.getLogger(__name__)

# Minimum pixel dimensions for an extracted image to be considered a figure.
# Tiny images (icons, decorative glyphs) are filtered out.
_DEFAULT_MIN_WIDTH = 100
_DEFAULT_MIN_HEIGHT = 100

# How many points below (or above) an image bbox we look for caption text.
_CAPTION_SEARCH_MARGIN_PT = 40


@dataclass
class FigureInfo:
    """Extracted figure metadata from a single PDF image.

    Attributes:
        page: 1-indexed page number where the figure appears.
        image_path: Path to the extracted PNG temp file.
        caption: Detected caption text (empty string if none found).
        figure_type: Rough semantic category: ``diagram``, ``chart``,
            ``table``, or ``equation_image``.
        bbox: Image bounding box ``(x0, y0, x1, y1)`` in PDF user-space
            points, relative to the page origin (top-left).
    """

    page: int
    image_path: Path
    caption: str
    figure_type: str
    bbox: tuple[float, float, float, float] = field(default_factory=lambda: (0.0, 0.0, 0.0, 0.0))


def _classify_figure_type(caption: str) -> str:
    """Classify a figure by type based on caption keywords.

    Args:
        caption: The figure caption text (may be empty).

    Returns:
        One of ``"table"``, ``"equation_image"``, ``"chart"``, or ``"diagram"``.
    """
    lower = caption.lower()
    if any(w in lower for w in ("table", "tbl.")):
        return "table"
    if any(w in lower for w in ("equation", "eq.", "formula")):
        return "equation_image"
    if any(w in lower for w in ("chart", "graph", "plot")):
        return "chart"
    return "diagram"


def _find_caption(page, img_bbox, margin_pt: float = _CAPTION_SEARCH_MARGIN_PT) -> str:
    """Search for a figure caption near an image bounding box.

    Looks for text blocks immediately below (or above, as fallback) the image
    boundary within *margin_pt* PDF points.

    Args:
        page: A ``fitz.Page`` object.
        img_bbox: ``fitz.Rect`` of the image on the page.
        margin_pt: Search radius in PDF points.

    Returns:
        The closest text block content, or an empty string if none found.
    """
    import fitz  # noqa: PLC0415 — lazy to avoid import error at module load

    # Search area below the image
    below_rect = fitz.Rect(
        img_bbox.x0,
        img_bbox.y1,
        img_bbox.x1,
        img_bbox.y1 + margin_pt,
    )
    # Search area above the image
    above_rect = fitz.Rect(
        img_bbox.x0,
        img_bbox.y0 - margin_pt,
        img_bbox.x1,
        img_bbox.y0,
    )

    blocks = page.get_text("blocks")  # (x0, y0, x1, y1, text, block_no, block_type)
    best_text = ""
    best_dist = float("inf")

    for block in blocks:
        bx0, by0, bx1, by1, text, *_ = block
        text = text.strip()
        if not text:
            continue
        block_rect = fitz.Rect(bx0, by0, bx1, by1)

        # Prefer caption text directly below the image
        if block_rect.intersects(below_rect):
            dist = abs(by0 - img_bbox.y1)
            if dist < best_dist:
                best_dist = dist
                best_text = text
        elif block_rect.intersects(above_rect) and not best_text:
            best_text = text

    return best_text


def _render_png(doc, xref: int) -> bytes | None:
    """Render a PDF image reference to PNG bytes.

    Args:
        doc: Open ``fitz.Document`` instance.
        xref: Cross-reference number of the image within the document.

    Returns:
        PNG bytes, or ``None`` if rendering fails.
    """
    import fitz  # noqa: PLC0415

    try:
        base_image = doc.extract_image(xref)
    except Exception as exc:
        logger.debug("figure_extractor: extract_image xref=%d failed: %s", xref, exc)
        return None

    img_ext = base_image.get("ext", "png")
    if img_ext.lower() == "png":
        return base_image["image"]

    try:
        pix = fitz.Pixmap(doc, xref)
        if pix.n > 4:  # CMYK → RGB
            pix = fitz.Pixmap(fitz.csRGB, pix)
        return pix.tobytes("png")
    except Exception as exc:
        logger.debug("figure_extractor: PNG re-encode xref=%d failed: %s", xref, exc)
        return None


def _write_temp_png(png_bytes: bytes) -> Path | None:
    """Write PNG bytes to a temporary file and return its Path.

    Args:
        png_bytes: Raw PNG image bytes to persist.

    Returns:
        Path to the created temp file, or ``None`` if writing fails.
    """
    try:
        tmp = tempfile.NamedTemporaryFile(delete=False, suffix=".png", prefix="fig-")
        tmp.write(png_bytes)
        tmp.close()
        return Path(tmp.name)
    except Exception as exc:
        logger.debug("figure_extractor: temp file write failed: %s", exc)
        return None


def _extract_page_figures(
    doc,
    page,
    page_number: int,
    min_width: int,
    min_height: int,
) -> list[FigureInfo]:
    """Extract all qualifying figures from a single PDF page.

    Args:
        doc: Open ``fitz.Document`` instance.
        page: ``fitz.Page`` to process.
        page_number: 1-indexed page number (stored in returned :class:`FigureInfo`).
        min_width: Minimum image width in pixels to include.
        min_height: Minimum image height in pixels to include.

    Returns:
        List of :class:`FigureInfo` objects for qualifying images on this page.
    """
    figures: list[FigureInfo] = []
    for img_ref in page.get_images(full=True):
        xref = img_ref[0]
        try:
            base_image = doc.extract_image(xref)
        except Exception as exc:
            logger.debug(
                "figure_extractor: extract_image xref=%d page=%d failed: %s",
                xref, page_number, exc,
            )
            continue

        width = base_image.get("width", 0)
        height = base_image.get("height", 0)
        if width < min_width or height < min_height:
            logger.debug(
                "figure_extractor: skipping small image xref=%d (%dx%d)", xref, width, height
            )
            continue

        png_bytes = _render_png(doc, xref)
        if png_bytes is None:
            continue

        image_path = _write_temp_png(png_bytes)
        if image_path is None:
            continue

        try:
            img_bbox = page.get_image_bbox(img_ref)
        except Exception:
            img_bbox = None

        caption = ""
        if img_bbox is not None:
            try:
                caption = _find_caption(page, img_bbox)
            except Exception:
                pass

        bbox_tuple: tuple[float, float, float, float] = (
            (img_bbox.x0, img_bbox.y0, img_bbox.x1, img_bbox.y1)
            if img_bbox is not None
            else (0.0, 0.0, 0.0, 0.0)
        )

        figures.append(FigureInfo(
            page=page_number,
            image_path=image_path,
            caption=caption,
            figure_type=_classify_figure_type(caption),
            bbox=bbox_tuple,
        ))
        logger.debug(
            "figure_extractor: extracted page=%d xref=%d size=%dx%d caption=%r",
            page_number, xref, width, height, caption[:40] if caption else "",
        )

    return figures


def extract_figures(
    pdf_path: Path,
    min_width: int = _DEFAULT_MIN_WIDTH,
    min_height: int = _DEFAULT_MIN_HEIGHT,
) -> list[FigureInfo]:
    """Extract embedded figures from a PDF file using PyMuPDF.

    Iterates over every page, collects embedded raster images that meet the
    minimum dimension threshold, renders each image to a temporary PNG file,
    and attempts caption detection by scanning adjacent text blocks.

    Silently degrades and returns an empty list when PyMuPDF (``fitz``) is
    not installed so lightweight environments (CI without torch/GPU) still work.

    Args:
        pdf_path: Path to the local PDF file to process.
        min_width: Minimum image width in pixels to include (filters icons).
        min_height: Minimum image height in pixels to include (filters icons).

    Returns:
        List of :class:`FigureInfo` objects, one per qualifying figure.
        Temporary PNG files are created in the system temp directory; callers
        are responsible for cleanup (``info.image_path.unlink(missing_ok=True)``).
    """
    try:
        import fitz  # noqa: PLC0415
    except ImportError:
        logger.warning("PyMuPDF (fitz) not installed — figure extraction disabled")
        return []

    try:
        doc = fitz.open(str(pdf_path))
    except Exception as exc:
        logger.warning("figure_extractor: failed to open %s: %s", pdf_path, exc)
        return []

    figures: list[FigureInfo] = []
    try:
        for page_index in range(len(doc)):
            page = doc[page_index]
            figures.extend(
                _extract_page_figures(doc, page, page_index + 1, min_width, min_height)
            )
    finally:
        doc.close()

    logger.info("figure_extractor: extracted %d figures from %s", len(figures), pdf_path.name)
    return figures
