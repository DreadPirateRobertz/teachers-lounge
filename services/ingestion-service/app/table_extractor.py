"""Table extraction from PDF pages using pdfplumber.

Detects tables on each PDF page, converts them to GitHub Flavored Markdown
table format, and returns structured chunks with ``type="table"`` metadata.
"""

import io

import pdfplumber


def _table_to_markdown(table: list[list[str | None]]) -> str:
    """Convert a pdfplumber table (list of rows) to a GFM markdown table.

    The first row is treated as the header row. A separator row of ``---``
    cells is inserted after the header. ``None`` cell values are normalized
    to empty strings.

    Args:
        table: List of rows returned by ``pdfplumber`` page.extract_tables().
               Each row is a list of cell values (str or None).

    Returns:
        Markdown-formatted table string, or empty string if ``table`` is empty.
    """
    if not table:
        return ""

    # Normalize None → ""
    rows = [[cell if cell is not None else "" for cell in row] for row in table]
    if not rows:
        return ""

    # Normalize row widths so all rows have the same number of columns
    max_cols = max(len(row) for row in rows)
    rows = [row + [""] * (max_cols - len(row)) for row in rows]

    def _format_row(cells: list[str]) -> str:
        # Escape pipe characters inside cells to avoid breaking the table
        escaped = [cell.replace("|", "\\|") for cell in cells]
        return "| " + " | ".join(escaped) + " |"

    header = rows[0]
    separator = ["---"] * max_cols
    body = rows[1:]

    lines = [_format_row(header), _format_row(separator)]
    lines.extend(_format_row(row) for row in body)
    return "\n".join(lines)


def extract_table_chunks(pdf_bytes: bytes) -> list[dict]:
    """Extract tables from all pages of a PDF using pdfplumber.

    Iterates over each page, detects tables via ``page.extract_tables()``,
    converts each table to Markdown, and yields a chunk dict per table.
    Pages with no tables are silently skipped. Empty or un-parseable tables
    are also skipped.

    The returned dicts do **not** include ``chunk_index``; the caller is
    responsible for assigning a document-global index (e.g. continuing the
    sequence after text chunks).

    Args:
        pdf_bytes: Raw PDF file content.

    Returns:
        List of chunk dicts, each with keys:
        ``text`` (Markdown table), ``page`` (1-based int),
        ``type`` (always ``"table"``), ``token_count`` (word count proxy).
    """
    chunks: list[dict] = []

    with pdfplumber.open(io.BytesIO(pdf_bytes)) as pdf:
        for page_num, page in enumerate(pdf.pages, start=1):
            tables = page.extract_tables()
            for table in tables:
                if not table:
                    continue
                markdown = _table_to_markdown(table)
                if not markdown:
                    continue
                chunks.append(
                    {
                        "text": markdown,
                        "page": page_num,
                        "type": "table",
                        # Word count is a cheap proxy; embedder will retokenise anyway
                        "token_count": len(markdown.split()),
                    }
                )

    return chunks
