"""Token-aware text chunking utilities for the ingestion pipeline.

Splits extracted PDF text into overlapping chunks of a fixed token budget
using tiktoken. Overlap preserves cross-chunk context for retrieval.
"""

import tiktoken


def chunk_text(
    text: str,
    page: int,
    chunk_size: int = 512,
    overlap: int = 64,
    encoding_name: str = "cl100k_base",
) -> list[dict]:
    """Split a single page's text into overlapping token-sized chunks.

    Tokens are counted with tiktoken. The last chunk may be shorter than
    chunk_size. Overlap is implemented by rewinding the start pointer by
    ``overlap`` tokens after each chunk is emitted.

    Args:
        text: Raw page text to chunk (should be pre-stripped).
        page: 1-based page number recorded in each chunk's metadata.
        chunk_size: Maximum number of tokens per chunk.
        overlap: Number of tokens shared between consecutive chunks.
        encoding_name: tiktoken encoding name (default: cl100k_base).

    Returns:
        List of chunk dicts, each with keys: text, page, token_count.
        chunk_index is not set here; the caller assigns it globally.
    """
    enc = tiktoken.get_encoding(encoding_name)
    tokens = enc.encode(text)
    if not tokens:
        return []

    chunks: list[dict] = []
    start = 0

    while start < len(tokens):
        end = min(start + chunk_size, len(tokens))
        chunk_tokens = tokens[start:end]
        chunk_text_str = enc.decode(chunk_tokens)
        chunks.append(
            {
                "text": chunk_text_str,
                "page": page,
                "token_count": len(chunk_tokens),
            }
        )
        if end == len(tokens):
            break
        start = end - overlap

    return chunks


def chunk_pdf_pages(
    pages: list[str],
    chunk_size: int = 512,
    overlap: int = 64,
    encoding_name: str = "cl100k_base",
) -> list[dict]:
    """Chunk all pages of a PDF into overlapping token-sized segments.

    Iterates over pages in order, skipping blank pages. Each non-blank page
    is chunked via ``chunk_text`` and assigned a monotonically increasing
    global ``chunk_index`` across the entire document.

    Args:
        pages: List of raw page text strings extracted from the PDF.
            Index 0 is page 1, index 1 is page 2, etc.
        chunk_size: Maximum tokens per chunk (default 512).
        overlap: Token overlap between consecutive chunks (default 64).
        encoding_name: tiktoken encoding name.

    Returns:
        Flat list of chunk dicts across all pages. Each dict has:
        text, page (1-based), chunk_index (document-global), token_count.
    """
    all_chunks: list[dict] = []
    global_index = 0

    for page_num, page_text in enumerate(pages, start=1):
        stripped = page_text.strip()
        if not stripped:
            continue

        page_chunks = chunk_text(
            text=stripped,
            page=page_num,
            chunk_size=chunk_size,
            overlap=overlap,
            encoding_name=encoding_name,
        )
        for chunk in page_chunks:
            chunk["chunk_index"] = global_index
            global_index += 1

        all_chunks.extend(page_chunks)

    return all_chunks
