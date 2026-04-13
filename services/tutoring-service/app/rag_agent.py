"""Agentic RAG pipeline — Phase 2 + Phase 5 (prerequisite-aware) + Phase 7 tracing.

Phase 2 scope (tl-dkm):
  Step 1 — Student context: recent interaction history
  Step 2 — Concept graph prerequisite check (IMPLEMENTED in tl-vki)
  Step 3 — Hybrid curriculum retrieval via Search Service (IMPLEMENTED)
  Step 4 — Cross-student insights from BigQuery cache (IMPLEMENTED)
  Step 5 — Enriched system prompt with chapter/section/page citations (IMPLEMENTED)
  Step 6 — Interaction log: record concept engagement in SKM (IMPLEMENTED)
            Spaced repetition scheduling deferred to Phase 5.

Phase 5 additions (tl-vki):
  Step 2 — Load course concepts + student mastery; keyword-match the question
  to a target concept; detect prerequisite gaps below MASTERY_THRESHOLD;
  inject a gap-redirect block into the system prompt when gaps are found.

Phase 7 additions (tl-dkg):
  OpenTelemetry custom span: rag_agent.build_context wraps the full pipeline.
  Records chunk_count, gap_count, and course_id as span attributes for
  latency-per-step analysis in Grafana Cloud Trace.

Exit criteria (tl-dkm):
  Step 4: when cross-student insights are cached in Redis for the target concept,
  Professor Nova's system prompt includes aggregate struggle/style observations.
  Step 6: after concept detection, the student's SKM is updated (last_reviewed_at)
  so the concept is marked as recently engaged even before quiz confirmation.
"""
import logging
from uuid import UUID

from opentelemetry import trace
from sqlalchemy.ext.asyncio import AsyncSession

from .cache import get_cross_student_insights
from .graph import (
    detect_gaps,
    get_course_concepts,
    get_student_mastery,
)
from .history import get_history
from .knowledge_model import log_concept_interaction
from .orm import Concept
from .search_client import SearchResult, fetch_curriculum_chunks

logger = logging.getLogger(__name__)
_tracer = trace.get_tracer("tutoring-service.rag_agent")


def _log_safe(val: object) -> str:
    """Sanitize a value for log output to prevent log injection.

    Args:
        val: Any value to sanitize.

    Returns:
        String with newline characters escaped.
    """
    return str(val).replace("\n", "\\n").replace("\r", "\\r")

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
    that directs Professor Nova to answer from general knowledge. If the concept
    graph is unavailable or the question cannot be mapped to a concept, step 2
    is silently skipped and the chat continues without gap detection.

    Args:
        student_id: UUID of the authenticated student.
        session_id: UUID of the current chat session.
        question: The student's raw question text.
        course_id: UUID of the student's enrolled course (search scope).
        db: Async SQLAlchemy session for history retrieval.

    Returns:
        Tuple of (system_prompt, source_chunks).
    """
    with _tracer.start_as_current_span("rag_agent.build_context") as span:
        span.set_attribute("student_id", str(student_id))
        span.set_attribute("session_id", str(session_id))
        span.set_attribute("course_id", str(course_id))

        # Step 1: Student context — interaction count as simple engagement signal.
        recent_history = await get_history(db, session_id, limit=20)
        student_turns = sum(1 for i in recent_history if i.role == "student")

        # Step 2: Concept graph prerequisite check (tl-vki).
        # Keyword-match the question to a course concept, then detect mastery gaps
        # in its transitive prerequisites. Non-fatal: skip on any DB error.
        gaps: list[dict] = []
        target: Concept | None = None
        try:
            concepts = await get_course_concepts(db, course_id)
            if concepts:
                mastery = await get_student_mastery(db, student_id, course_id)
                target = _find_concept_for_question(question, concepts)
                if target is not None:
                    gaps = detect_gaps(target.id, concepts, mastery)
                    if gaps:
                        logger.info(
                            "prereq_gaps student_id=%s target_concept=%s gaps=%d",
                            _log_safe(student_id), _log_safe(target.name), len(gaps),
                        )
        except Exception:
            # Non-fatal: concept graph is best-effort; skip rather than break chat.
            logger.exception("step2 concept graph check failed — skipping")

        # Step 3: Retrieve curriculum chunks via hybrid vector search.
        chunks = await fetch_curriculum_chunks(question, course_id, limit=8)

        # Step 4: Cross-student insights from Redis cache (populated by BigQuery job).
        # If the target concept has cached aggregate insights, inject them into the
        # prompt so Professor Nova can mention common struggle points and effective
        # explanation styles. Non-fatal: gracefully skip on cache miss or Redis error.
        insights: list[str] = []
        if target is not None:
            try:
                cached = await get_cross_student_insights(target.name)
                if cached:
                    insights = cached
                    logger.info(
                        "cross_student_insights student_id=%s concept=%s count=%d",
                        _log_safe(student_id), _log_safe(target.name), len(insights),
                    )
            except Exception:
                # Non-fatal: insights are best-effort enrichment.
                logger.exception("step4 cross-student insight lookup failed — skipping")

        # Step 5: Build enriched system prompt including retrieved context, gaps, and insights.
        system_prompt = _build_system_prompt(chunks, student_turns, gaps=gaps, insights=insights)

        # Step 6: Log concept interaction in the Student Knowledge Model (tl-dkm).
        # Creates or touches the StudentConceptMastery row to record engagement.
        # Mastery score is NOT modified here — that is managed by the SRS system (Phase 5).
        # Non-fatal: a DB error must not prevent the tutoring response from streaming.
        if target is not None:
            try:
                await log_concept_interaction(db, student_id, target.id)
            except Exception:
                logger.exception(
                    "step6 SKM interaction log failed for concept %s — skipping",
                    _log_safe(target.name),
                )

        span.set_attribute("chunk_count", len(chunks))
        span.set_attribute("gap_count", len(gaps))
        span.set_attribute("insight_count", len(insights))
        logger.info(
            "rag_context student_id=%s course_id=%s chunks=%d gaps=%d insights=%d",
            _log_safe(student_id), _log_safe(course_id), len(chunks), len(gaps), len(insights),
        )

        return system_prompt, chunks


def _find_concept_for_question(question: str, concepts: list[Concept]) -> Concept | None:
    """Find the most relevant concept for a question by keyword matching.

    Scores each concept by counting how many of its name words appear in the
    question text (case-insensitive). Returns the highest-scoring concept, or
    None if no concept name words appear in the question at all.

    Args:
        question: The student's raw question text.
        concepts: All concepts in the student's enrolled course.

    Returns:
        The best-matching Concept, or None if no keyword match found.
    """
    q_lower = question.lower()
    best: Concept | None = None
    best_score = 0
    for concept in concepts:
        words = concept.name.lower().split()
        score = sum(1 for w in words if w in q_lower)
        if score > best_score:
            best_score = score
            best = concept
    return best if best_score > 0 else None


def _build_system_prompt(
    chunks: list[SearchResult],
    student_turns: int,
    gaps: list[dict] | None = None,
    insights: list[str] | None = None,
) -> str:
    """Build the enriched system prompt from retrieved chunks, student context, gaps, and insights.

    When prerequisite gaps are detected, appends a redirect block instructing
    Professor Nova to gently steer toward foundational concepts before answering
    the student's original question.

    When cross-student insights are available, appends an observation block so
    Professor Nova can proactively address common struggle points and prefer
    explanation styles with higher success rates for this topic.

    Args:
        chunks: Curriculum chunks returned by the Search Service.
        student_turns: Number of student messages in the current session.
        gaps: List of gap dicts from detect_gaps() — each has concept_name,
            mastery_score, required_by. None or empty means no gaps detected.
        insights: List of insight strings from the cross-student cache (e.g.
            "72% of students struggle with step 3 of this derivation"). None or
            empty means no insights available.

    Returns:
        Full system prompt string for the AI Gateway.
    """
    if not chunks:
        base = (
            PROFESSOR_NOVA_SYSTEM_PROMPT
            + "\n\n[No curriculum content was retrieved for this query — the course "
            "materials may not yet be indexed. Draw on your broad knowledge and be "
            "transparent that you are not referencing their specific textbook right now.]"
        )
    else:
        context_block = _format_chunks(chunks)
        experience_note = (
            "This is an early interaction — be patient, foundational, and check for prerequisite gaps."
            if student_turns < 5
            else "This student has prior conversation history — you may build on earlier exchanges."
        )
        base = f"""{PROFESSOR_NOVA_SYSTEM_PROMPT}

--- RETRIEVED CURRICULUM CONTENT ---
The following excerpts are from the student's enrolled course materials.
Ground your response in this content and cite sources (chapter / section / page) \
when you reference them.

{context_block}
--- END CURRICULUM CONTENT ---

Student context: {experience_note}"""

    if insights:
        base = base + _build_insight_block(insights)

    if not gaps:
        return base

    gap_lines = "\n".join(
        f"  - {g['concept_name']} (mastery: {g['mastery_score']:.0%})"
        for g in gaps
    )
    gap_block = f"""

--- PREREQUISITE GAPS DETECTED ---
Before the student can fully grasp the topic they asked about, they have unmastered \
prerequisites:
{gap_lines}

Recommended approach:
1. Warmly acknowledge their question — don't make them feel bad for asking.
2. Explain that covering a foundational concept first will make the answer much clearer.
3. Begin with the first unmastered prerequisite listed above.
4. Keep it brief — return to their original question once the gap is addressed.
--- END PREREQUISITE GAPS ---"""

    return base + gap_block


def _build_insight_block(insights: list[str]) -> str:
    """Format cross-student insights as a system prompt block.

    Args:
        insights: Non-empty list of aggregate insight strings produced by the
            offline BigQuery analytics job (e.g. difficulty rates, effective
            explanation styles, common misconceptions).

    Returns:
        Formatted prompt block string to append to the base system prompt.
    """
    insight_lines = "\n".join(f"  - {insight}" for insight in insights)
    return f"""

--- CROSS-STUDENT INSIGHTS ---
Aggregate observations from students who have studied this concept before:
{insight_lines}

Use these insights to proactively address common struggles, prefer explanation
styles with higher success rates, and warn about frequent misconceptions.
--- END CROSS-STUDENT INSIGHTS ---"""


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
