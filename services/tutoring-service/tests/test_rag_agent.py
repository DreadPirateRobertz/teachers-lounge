"""Tests for the agentic RAG pipeline (rag_agent.py).

Covers chunk formatting, system-prompt construction, graceful fallback when
search returns nothing, and the build_rag_context orchestration contract.
"""
import uuid
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from app.rag_agent import _format_chunks, _build_system_prompt, build_rag_context
from app.search_client import SearchResult


def _make_chunk(**kwargs) -> SearchResult:
    """Factory for SearchResult test fixtures.

    Args:
        **kwargs: Override any default field values.

    Returns:
        A SearchResult instance with sensible defaults.
    """
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


class TestFormatChunks:
    """Unit tests for _format_chunks."""

    def test_empty_input_returns_empty_string(self):
        """Empty chunk list produces an empty string."""
        assert _format_chunks([]) == ""

    def test_single_chunk_includes_location(self):
        """Chapter, section, and page are all present in formatted output."""
        chunk = _make_chunk(chapter="Ch 5", section="5.3", page=87)
        result = _format_chunks([chunk])
        assert "Chapter: Ch 5" in result
        assert "Section: 5.3" in result
        assert "Page: 87" in result
        assert chunk.content in result

    def test_multiple_chunks_are_numbered(self):
        """Each chunk receives a 1-based index prefix."""
        chunks = [_make_chunk() for _ in range(3)]
        result = _format_chunks(chunks)
        assert "[1]" in result
        assert "[2]" in result
        assert "[3]" in result

    def test_missing_location_fields_show_fallback(self):
        """All-None location falls back to 'Location unknown'."""
        chunk = _make_chunk(chapter=None, section=None, page=None)
        assert "Location unknown" in _format_chunks([chunk])

    def test_partial_location_omits_missing_fields(self):
        """Only present location fields are rendered."""
        chunk = _make_chunk(chapter="Ch 3", section=None, page=None)
        result = _format_chunks([chunk])
        assert "Chapter: Ch 3" in result
        assert "Section" not in result


class TestBuildSystemPrompt:
    """Unit tests for _build_system_prompt."""

    def test_no_chunks_includes_no_material_notice(self):
        """Fallback prompt tells Nova no materials were retrieved."""
        prompt = _build_system_prompt([], student_turns=0)
        assert "No curriculum content was retrieved" in prompt
        assert "RETRIEVED CURRICULUM CONTENT" not in prompt

    def test_chunks_present_includes_context_block(self):
        """Retrieved content is embedded in the system prompt."""
        chunks = [_make_chunk()]
        prompt = _build_system_prompt(chunks, student_turns=10)
        assert "RETRIEVED CURRICULUM CONTENT" in prompt
        assert chunks[0].content in prompt

    def test_early_student_gets_foundational_note(self):
        """Students with fewer than 5 turns receive foundational framing."""
        prompt = _build_system_prompt([_make_chunk()], student_turns=2)
        assert "early interaction" in prompt.lower()

    def test_experienced_student_gets_continuation_note(self):
        """Students with 5+ turns receive a note about prior history."""
        prompt = _build_system_prompt([_make_chunk()], student_turns=10)
        assert "prior conversation history" in prompt.lower()

    def test_prompt_instructs_citation(self):
        """Nova is instructed to cite chapter/section/page."""
        prompt = _build_system_prompt([_make_chunk()], student_turns=5)
        assert "cite" in prompt.lower()


class TestBuildRagContext:
    """Integration-level tests for build_rag_context orchestration."""

    @pytest.mark.asyncio
    async def test_returns_prompt_and_chunks(self):
        """Happy path: chunks retrieved → enriched prompt and chunks returned."""
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
        """No chunks retrieved → fallback prompt and empty list, no exception."""
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
    async def test_search_failure_graceful(self):
        """fetch_curriculum_chunks swallows errors and returns []; context follows."""
        mock_db = AsyncMock()

        with (
            patch("app.rag_agent.get_history", AsyncMock(return_value=[])),
            patch("app.rag_agent.fetch_curriculum_chunks", AsyncMock(return_value=[])),
        ):
            _, returned = await build_rag_context(
                student_id=uuid.uuid4(),
                session_id=uuid.uuid4(),
                question="What is entropy?",
                course_id=uuid.uuid4(),
                db=mock_db,
            )

        assert returned == []

    @pytest.mark.asyncio
    async def test_early_student_turn_count_yields_foundational_note(self):
        """Students with < 5 turns get foundational framing in the prompt."""
        mock_db = AsyncMock()
        fake_history = (
            [MagicMock(role="student") for _ in range(3)]
            + [MagicMock(role="tutor") for _ in range(3)]
        )

        with (
            patch("app.rag_agent.get_history", AsyncMock(return_value=fake_history)),
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
