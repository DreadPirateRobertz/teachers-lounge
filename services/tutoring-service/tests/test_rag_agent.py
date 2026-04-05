"""
Tests for the agentic RAG pipeline — Phase 2 (tl-dkm) + Phase 5 (tl-vki).

Covers:
  - chunk formatting
  - system prompt construction (no gaps, with gaps, no chunks)
  - concept keyword matching (_find_concept_for_question)
  - build_rag_context orchestration: happy path, no-chunks fallback,
    gap detection + prompt injection, step-2 errors silently skipped
"""
import uuid
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from app.rag_agent import (
    _build_system_prompt,
    _find_concept_for_question,
    _format_chunks,
    build_rag_context,
)
from app.search_client import SearchResult


def _make_chunk(**kwargs) -> SearchResult:
    """Build a SearchResult with sensible defaults."""
    defaults = dict(
        chunk_id=str(uuid.uuid4()),
        material_id=str(uuid.uuid4()),
        course_id=str(uuid.uuid4()),
        content="A chiral center is a carbon atom bonded to four different substituents.",
        score=0.92,
        chapter="Chapter 5",
        section="5.3",
        page=87,
        content_type="text",
    )
    defaults.update(kwargs)
    return SearchResult(**defaults)


def _make_concept(name: str, concept_id: uuid.UUID | None = None) -> MagicMock:
    """Build a minimal Concept stub with id and name."""
    c = MagicMock()
    c.id = concept_id or uuid.uuid4()
    c.name = name
    c.prerequisites = []
    return c


def _make_gap(name: str, mastery: float = 0.0) -> dict:
    """Build a minimal gap dict as returned by detect_gaps()."""
    return {
        "concept_id": uuid.uuid4(),
        "concept_name": name,
        "mastery_score": mastery,
        "required_by": [],
    }


# ---------------------------------------------------------------------------
# _format_chunks
# ---------------------------------------------------------------------------

class TestFormatChunks:
    def test_empty_input_returns_empty_string(self):
        assert _format_chunks([]) == ""

    def test_single_chunk_includes_location(self):
        chunk = _make_chunk(chapter="Ch 5", section="5.3", page=87)
        result = _format_chunks([chunk])
        assert "Chapter: Ch 5" in result
        assert "Section: 5.3" in result
        assert "Page: 87" in result
        assert chunk.content in result

    def test_multiple_chunks_are_numbered(self):
        chunks = [_make_chunk() for _ in range(3)]
        result = _format_chunks(chunks)
        assert "[1]" in result
        assert "[2]" in result
        assert "[3]" in result

    def test_missing_location_fields_show_fallback(self):
        chunk = _make_chunk(chapter=None, section=None, page=None)
        result = _format_chunks([chunk])
        assert "Location unknown" in result

    def test_partial_location_omits_missing_fields(self):
        chunk = _make_chunk(chapter="Ch 3", section=None, page=None)
        result = _format_chunks([chunk])
        assert "Chapter: Ch 3" in result
        assert "Section" not in result


# ---------------------------------------------------------------------------
# _find_concept_for_question
# ---------------------------------------------------------------------------

class TestFindConceptForQuestion:
    def test_exact_name_match(self):
        cid = uuid.uuid4()
        concepts = [_make_concept("Photosynthesis", cid), _make_concept("Mitosis")]
        result = _find_concept_for_question("How does photosynthesis work?", concepts)
        assert result is not None
        assert result.id == cid

    def test_partial_word_match_prefers_most_words(self):
        """Concept with more name words in the question wins."""
        c1 = _make_concept("Newton laws")
        c2 = _make_concept("Newton")
        concepts = [c2, c1]
        result = _find_concept_for_question("Explain Newton laws of motion", concepts)
        assert result is c1

    def test_no_match_returns_none(self):
        concepts = [_make_concept("Algebra"), _make_concept("Calculus")]
        result = _find_concept_for_question("What is the speed of light?", concepts)
        assert result is None

    def test_empty_concepts_returns_none(self):
        result = _find_concept_for_question("What is entropy?", [])
        assert result is None

    def test_case_insensitive(self):
        cid = uuid.uuid4()
        concepts = [_make_concept("Entropy", cid)]
        result = _find_concept_for_question("WHAT IS ENTROPY?", concepts)
        assert result is not None
        assert result.id == cid


# ---------------------------------------------------------------------------
# _build_system_prompt
# ---------------------------------------------------------------------------

class TestBuildSystemPrompt:
    def test_no_chunks_returns_no_material_notice(self):
        prompt = _build_system_prompt([], student_turns=0)
        assert "No curriculum content was retrieved" in prompt
        assert "RETRIEVED CURRICULUM CONTENT" not in prompt

    def test_chunks_present_includes_context_block(self):
        chunks = [_make_chunk()]
        prompt = _build_system_prompt(chunks, student_turns=10)
        assert "RETRIEVED CURRICULUM CONTENT" in prompt
        assert chunks[0].content in prompt

    def test_early_student_gets_foundational_note(self):
        prompt = _build_system_prompt([_make_chunk()], student_turns=2)
        assert "early interaction" in prompt.lower()

    def test_experienced_student_gets_continuation_note(self):
        prompt = _build_system_prompt([_make_chunk()], student_turns=10)
        assert "prior conversation history" in prompt.lower()

    def test_prompt_includes_citation_instruction(self):
        chunks = [_make_chunk()]
        prompt = _build_system_prompt(chunks, student_turns=5)
        assert "cite" in prompt.lower()

    def test_no_gaps_omits_gap_block(self):
        prompt = _build_system_prompt([_make_chunk()], student_turns=5, gaps=[])
        assert "PREREQUISITE GAPS" not in prompt

    def test_gaps_injects_gap_block(self):
        gaps = [_make_gap("Variables", mastery=0.3), _make_gap("Functions", mastery=0.5)]
        prompt = _build_system_prompt([_make_chunk()], student_turns=5, gaps=gaps)
        assert "PREREQUISITE GAPS DETECTED" in prompt
        assert "Variables" in prompt
        assert "Functions" in prompt
        assert "30%" in prompt  # mastery formatted as percent
        assert "50%" in prompt

    def test_gaps_with_no_chunks_still_injects_gap_block(self):
        """Gap redirect is appended even when there's no curriculum content."""
        gaps = [_make_gap("Algebra", mastery=0.0)]
        prompt = _build_system_prompt([], student_turns=0, gaps=gaps)
        assert "PREREQUISITE GAPS DETECTED" in prompt
        assert "Algebra" in prompt
        assert "No curriculum content was retrieved" in prompt

    def test_gaps_none_treated_as_no_gaps(self):
        prompt = _build_system_prompt([_make_chunk()], student_turns=5, gaps=None)
        assert "PREREQUISITE GAPS" not in prompt


# ---------------------------------------------------------------------------
# build_rag_context
# ---------------------------------------------------------------------------

class TestBuildRagContext:
    @pytest.mark.asyncio
    async def test_returns_prompt_and_chunks(self):
        mock_db = AsyncMock()
        chunks = [_make_chunk()]

        with (
            patch("app.rag_agent.get_history", AsyncMock(return_value=[])),
            patch("app.rag_agent.get_course_concepts", AsyncMock(return_value=[])),
            patch("app.rag_agent.fetch_curriculum_chunks", AsyncMock(return_value=chunks)),
        ):
            prompt, returned = await build_rag_context(
                student_id=uuid.uuid4(),
                session_id=uuid.uuid4(),
                question="What is a chiral center?",
                course_id=uuid.uuid4(),
                db=mock_db,
            )

        assert "RETRIEVED CURRICULUM CONTENT" in prompt
        assert returned == chunks
        assert chunks[0].content in prompt

    @pytest.mark.asyncio
    async def test_no_chunks_returns_fallback_prompt_and_empty_list(self):
        mock_db = AsyncMock()

        with (
            patch("app.rag_agent.get_history", AsyncMock(return_value=[])),
            patch("app.rag_agent.get_course_concepts", AsyncMock(return_value=[])),
            patch("app.rag_agent.fetch_curriculum_chunks", AsyncMock(return_value=[])),
        ):
            prompt, returned = await build_rag_context(
                student_id=uuid.uuid4(),
                session_id=uuid.uuid4(),
                question="What is osmosis?",
                course_id=uuid.uuid4(),
                db=mock_db,
            )

        assert returned == []
        assert "No curriculum content was retrieved" in prompt

    @pytest.mark.asyncio
    async def test_search_failure_returns_empty_chunks(self):
        """fetch_curriculum_chunks catches its own exceptions; we get [] and fallback prompt."""
        mock_db = AsyncMock()

        with (
            patch("app.rag_agent.get_history", AsyncMock(return_value=[])),
            patch("app.rag_agent.get_course_concepts", AsyncMock(return_value=[])),
            patch("app.rag_agent.fetch_curriculum_chunks", AsyncMock(return_value=[])),
        ):
            prompt, returned = await build_rag_context(
                student_id=uuid.uuid4(),
                session_id=uuid.uuid4(),
                question="What is entropy?",
                course_id=uuid.uuid4(),
                db=mock_db,
            )

        assert returned == []

    @pytest.mark.asyncio
    async def test_student_turn_count_affects_prompt(self):
        """Early-stage student (<5 turns) gets foundational framing."""
        mock_db = AsyncMock()

        fake_interactions = [MagicMock(role="student") for _ in range(3)] + \
                            [MagicMock(role="tutor") for _ in range(3)]

        with (
            patch("app.rag_agent.get_history", AsyncMock(return_value=fake_interactions)),
            patch("app.rag_agent.get_course_concepts", AsyncMock(return_value=[])),
            patch("app.rag_agent.fetch_curriculum_chunks", AsyncMock(return_value=[_make_chunk()])),
        ):
            prompt, _ = await build_rag_context(
                student_id=uuid.uuid4(),
                session_id=uuid.uuid4(),
                question="What is covalent bonding?",
                course_id=uuid.uuid4(),
                db=mock_db,
            )

        assert "early interaction" in prompt.lower()

    @pytest.mark.asyncio
    async def test_gaps_detected_appear_in_prompt(self):
        """When concept has prerequisite gaps, system prompt includes gap redirect."""
        mock_db = AsyncMock()
        chunks = [_make_chunk()]
        concept_id = uuid.uuid4()
        target_concept = _make_concept("Recursion", concept_id)
        gap = _make_gap("Functions", mastery=0.2)

        with (
            patch("app.rag_agent.get_history", AsyncMock(return_value=[])),
            patch("app.rag_agent.get_course_concepts", AsyncMock(return_value=[target_concept])),
            patch("app.rag_agent.get_student_mastery", AsyncMock(return_value={})),
            patch("app.rag_agent.detect_gaps", return_value=[gap]),
            patch("app.rag_agent._find_concept_for_question", return_value=target_concept),
            patch("app.rag_agent.fetch_curriculum_chunks", AsyncMock(return_value=chunks)),
        ):
            prompt, returned = await build_rag_context(
                student_id=uuid.uuid4(),
                session_id=uuid.uuid4(),
                question="What is recursion?",
                course_id=uuid.uuid4(),
                db=mock_db,
            )

        assert "PREREQUISITE GAPS DETECTED" in prompt
        assert "Functions" in prompt
        assert returned == chunks

    @pytest.mark.asyncio
    async def test_no_gaps_prompt_is_clean(self):
        """When no gaps are detected, gap block is absent from prompt."""
        mock_db = AsyncMock()
        chunks = [_make_chunk()]
        target_concept = _make_concept("Calculus")

        with (
            patch("app.rag_agent.get_history", AsyncMock(return_value=[])),
            patch("app.rag_agent.get_course_concepts", AsyncMock(return_value=[target_concept])),
            patch("app.rag_agent.get_student_mastery", AsyncMock(return_value={})),
            patch("app.rag_agent.detect_gaps", return_value=[]),
            patch("app.rag_agent._find_concept_for_question", return_value=target_concept),
            patch("app.rag_agent.fetch_curriculum_chunks", AsyncMock(return_value=chunks)),
        ):
            prompt, _ = await build_rag_context(
                student_id=uuid.uuid4(),
                session_id=uuid.uuid4(),
                question="What is calculus?",
                course_id=uuid.uuid4(),
                db=mock_db,
            )

        assert "PREREQUISITE GAPS" not in prompt

    @pytest.mark.asyncio
    async def test_concept_graph_error_is_silently_skipped(self):
        """DB error in step 2 does not raise — pipeline continues without gaps."""
        mock_db = AsyncMock()
        chunks = [_make_chunk()]

        with (
            patch("app.rag_agent.get_history", AsyncMock(return_value=[])),
            patch("app.rag_agent.get_course_concepts", AsyncMock(side_effect=RuntimeError("db down"))),
            patch("app.rag_agent.fetch_curriculum_chunks", AsyncMock(return_value=chunks)),
        ):
            prompt, returned = await build_rag_context(
                student_id=uuid.uuid4(),
                session_id=uuid.uuid4(),
                question="What is entropy?",
                course_id=uuid.uuid4(),
                db=mock_db,
            )

        # Pipeline completes without raising; no gap block in prompt
        assert returned == chunks
        assert "PREREQUISITE GAPS" not in prompt

    @pytest.mark.asyncio
    async def test_question_with_no_concept_match_skips_gap_check(self):
        """If no concept matches the question, detect_gaps is never called."""
        mock_db = AsyncMock()
        chunks = [_make_chunk()]
        concepts = [_make_concept("Algebra"), _make_concept("Geometry")]

        with (
            patch("app.rag_agent.get_history", AsyncMock(return_value=[])),
            patch("app.rag_agent.get_course_concepts", AsyncMock(return_value=concepts)),
            patch("app.rag_agent.get_student_mastery", AsyncMock(return_value={})),
            patch("app.rag_agent.detect_gaps") as mock_detect,
            patch("app.rag_agent.fetch_curriculum_chunks", AsyncMock(return_value=chunks)),
        ):
            await build_rag_context(
                student_id=uuid.uuid4(),
                session_id=uuid.uuid4(),
                question="What is the speed of light?",  # no concept keyword match
                course_id=uuid.uuid4(),
                db=mock_db,
            )

        mock_detect.assert_not_called()


# ---------------------------------------------------------------------------
# Phase 7 OTel span — build_rag_context
# ---------------------------------------------------------------------------

class TestBuildRagContextOtelSpan:
    """Verify that build_rag_context emits a correctly-attributed OTel span."""

    @pytest.mark.asyncio
    async def test_span_created_with_correct_attributes(self):
        """build_rag_context starts a span named 'rag_agent.build_context'
        and sets chunk_count, gap_count, course_id attributes."""
        from opentelemetry.sdk.trace import TracerProvider
        from opentelemetry.sdk.trace.export.in_memory_span_exporter import InMemorySpanExporter
        from opentelemetry.sdk.trace.export import SimpleSpanProcessor
        from opentelemetry import trace

        exporter = InMemorySpanExporter()
        provider = TracerProvider()
        provider.add_span_processor(SimpleSpanProcessor(exporter))
        trace.set_tracer_provider(provider)

        # Re-import to pick up the new tracer provider
        import importlib
        import app.rag_agent as ra_mod
        importlib.reload(ra_mod)
        from app.rag_agent import build_rag_context as _build

        mock_db = AsyncMock()
        course_id = uuid.uuid4()
        chunks = [_make_chunk()]

        with (
            patch("app.rag_agent.get_history", AsyncMock(return_value=[])),
            patch("app.rag_agent.get_course_concepts", AsyncMock(return_value=[])),
            patch("app.rag_agent.fetch_curriculum_chunks", AsyncMock(return_value=chunks)),
        ):
            await _build(
                student_id=uuid.uuid4(),
                session_id=uuid.uuid4(),
                question="What is a chiral center?",
                course_id=course_id,
                db=mock_db,
            )

        spans = exporter.get_finished_spans()
        rag_spans = [s for s in spans if s.name == "rag_agent.build_context"]
        assert len(rag_spans) == 1

        attrs = rag_spans[0].attributes
        assert attrs.get("chunk_count") == 1
        assert attrs.get("gap_count") == 0
        assert attrs.get("course_id") == str(course_id)

        # Restore default no-op provider so other tests aren't affected
        trace.set_tracer_provider(trace.NoOpTracerProvider())
        importlib.reload(ra_mod)
