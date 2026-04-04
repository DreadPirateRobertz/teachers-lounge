"""
Tests for the agentic RAG pipeline.

Covers: chunk formatting, system prompt construction, graceful fallback when
search returns nothing, and the build_rag_context orchestration contract.
"""
import uuid
from unittest.mock import AsyncMock, patch

import pytest

from app.rag_agent import _format_chunks, _build_system_prompt, build_rag_context
from app.search_client import SearchResult


def _make_chunk(**kwargs) -> SearchResult:
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
        """fetch_curriculum_chunks catches its own exceptions; we get [] and a fallback prompt."""
        mock_db = AsyncMock()

        with (
            patch("app.rag_agent.get_history", AsyncMock(return_value=[])),
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
        from unittest.mock import MagicMock
        mock_db = AsyncMock()

        # Simulate 3 student turns in history
        fake_interactions = [MagicMock(role="student") for _ in range(3)] + \
                            [MagicMock(role="tutor") for _ in range(3)]

        with (
            patch("app.rag_agent.get_history", AsyncMock(return_value=fake_interactions)),
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
