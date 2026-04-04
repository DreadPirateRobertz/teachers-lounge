"""Agentic RAG pipeline — Phase 2 implementation of the 6-step loop.

Phase 2 scope:
  Step 1 — Student context: recent interaction history (full SKM in Phase 5)
  Step 2 — Concept graph prerequisite check: deferred to Phase 5
  Step 3 — Hybrid curriculum retrieval via Search Service (IMPLEMENTED)
  Step 4 — Cross-student insights from BigQuery: deferred to Phase 7
  Step 5 — Enriched system prompt with chapter/section/page citations (IMPLEMENTED)
  Step 6 — Interaction log + spaced rep: log only (spaced rep in Phase 5)

Exit criteria (tl-dkm): student uploads PDF, asks a question, receives
a grounded answer with chapter/section/page citations from their material.
"""
import logging
from uuid import UUID

from sqlalchemy.ext.asyncio import AsyncSession

from .history import get_history
from .search_client import SearchResult, fetch_curriculum_chunks

logger = logging.getLogger(__name__)

PROFESSOR_NOVA_SYSTEM_PROMPT = """\
You are Professor Nova, the AI tutor for TeachersLounge — a gamified learning \
platform. You are brilliant, patient, encouraging, and a little bit nerdy. You \
use vivid analogies, celebrate curiosity, and make hard concepts feel approachable.

Guidelines:
- Ask a clarifying question before a long explanation if the question is vague.
- Use concrete examples. Always pair an abstraction with something tangible.
- When a student is wrong, be gentle but clear — explain why, don't just give the answer.
- Use LaTeX notation for math/formulas: $E = mc^2$ inline, $$...$$ for display.
- Keep responses focused. If a topic is huge, offer to go deeper on a specific part.\
"""


async def build_rag_context(
    student_id: UUID,
    session_id: UUID,
    question: str,
    course_id: UUID,
    db: AsyncSession,
) -> tuple[str, list[SearchResult]]:
    """Run the agentic RAG loop and return an enriched system prompt + source chunks.

    The system prompt is injected into the messages list sent to the AI Gateway.
    source_chunks are returned to the chat layer for source attribution in the
    SSE sources event.

    Degrades gracefully: if the Search Service is unavailable, returns a prompt
    that directs Professor Nova to answer from general knowledge.

    Args:
        student_id: UUID of the authenticated student.
        session_id: UUID of the current chat session.
        question: The student's raw question text.
        course_id: UUID of the student's enrolled course (search scope).
        db: Async SQLAlchemy session for history retrieval.

    Returns:
        Tuple of (system_prompt, source_chunks).
    """
    # Step 1: Student context — interaction count as simple engagement signal.
    # Full SKM (learning style, mastery graph, misconception log) is Phase 5.
    recent_history = await get_history(db, session_id, limit=20)
    student_turns = sum(1 for i in recent_history if i.role == "student")

    # Step 2: Concept graph prerequisite check — Phase 5 (ltree / Neo4j)

    # Step 3: Retrieve curriculum chunks via hybrid vector search
    chunks = await fetch_curriculum_chunks(question, course_id, limit=8)

    # Step 4: Cross-student insights (72% struggle here, visual works better) — Phase 7

    # Step 5: Build enriched system prompt including retrieved context
    system_prompt = _build_system_prompt(chunks, student_turns)

    # Step 6: Interaction embedding + spaced-repetition scheduling — Phase 5
    logger.info(
        "rag_context student_id=%s course_id=%s chunks=%d",
        student_id, course_id, len(chunks),
    )

    return system_prompt, chunks


def _build_system_prompt(chunks: list[SearchResult], student_turns: int) -> str:
    """Build the enriched system prompt from retrieved chunks and student context.

    Args:
        chunks: Curriculum chunks returned by the Search Service.
        student_turns: Number of student messages in the current session.

    Returns:
        Full system prompt string for the AI Gateway.
    """
    if not chunks:
        return (
            PROFESSOR_NOVA_SYSTEM_PROMPT
            + "\n\n[No curriculum content was retrieved for this query — the course "
            "materials may not yet be indexed. Draw on your broad knowledge and be "
            "transparent that you are not referencing their specific textbook right now.]"
        )

    context_block = _format_chunks(chunks)
    experience_note = (
        "This is an early interaction — be patient, foundational, and check for prerequisite gaps."
        if student_turns < 5
        else "This student has prior conversation history — you may build on earlier exchanges."
    )

    return f"""{PROFESSOR_NOVA_SYSTEM_PROMPT}

--- RETRIEVED CURRICULUM CONTENT ---
The following excerpts are from the student's enrolled course materials.
Ground your response in this content and cite sources (chapter / section / page) \
when you reference them.

{context_block}
--- END CURRICULUM CONTENT ---

Student context: {experience_note}"""


def _format_chunks(chunks: list[SearchResult]) -> str:
    """Format retrieved chunks as a numbered, location-annotated context block.

    Args:
        chunks: Ordered list of curriculum chunks from the Search Service.

    Returns:
        Multi-line string with each chunk numbered and its location annotated.
    """
    if not chunks:
        return ""
    parts = []
    for i, chunk in enumerate(chunks, 1):
        loc_parts = []
        if chunk.chapter:
            loc_parts.append(f"Chapter: {chunk.chapter}")
        if chunk.section:
            loc_parts.append(f"Section: {chunk.section}")
        if chunk.page is not None:
            loc_parts.append(f"Page: {chunk.page}")
        loc = " | ".join(loc_parts) if loc_parts else "Location unknown"
        parts.append(f"[{i}] ({loc})\n{chunk.content}")
    return "\n\n".join(parts)
