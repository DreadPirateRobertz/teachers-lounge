"""Tests for POST /v1/quiz/answer (Phase 6 molecule builder + multiple-choice)."""

import pytest
from httpx import AsyncClient
from httpx._transports.asgi import ASGITransport

from app.main import app


async def _post_answer(payload: dict) -> dict:
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
        resp = await client.post("/v1/quiz/answer", json=payload)
    return resp


class TestQuizAnswerSmiles:
    async def test_correct_smiles_returns_correct(self):
        """Matching SMILES strings (case/whitespace normalised) return correct=True."""
        resp = await _post_answer(
            {
                "smiles_answer": "c1ccccc1",
                "expected_smiles": "c1ccccc1",
            }
        )
        assert resp.status_code == 200
        body = resp.json()
        assert body["correct"] is True
        assert body["answer_type"] == "smiles"
        assert "Correct" in body["feedback"]

    async def test_wrong_smiles_returns_incorrect(self):
        """Non-matching SMILES returns correct=False."""
        resp = await _post_answer(
            {
                "smiles_answer": "CCO",
                "expected_smiles": "c1ccccc1",
            }
        )
        assert resp.status_code == 200
        body = resp.json()
        assert body["correct"] is False
        assert body["answer_type"] == "smiles"
        assert body["submitted"] == "CCO"

    async def test_smiles_without_expected_returns_422(self):
        """smiles_answer without expected_smiles is rejected."""
        resp = await _post_answer({"smiles_answer": "c1ccccc1"})
        assert resp.status_code == 422

    async def test_smiles_normalisation_strips_whitespace(self):
        """Leading/trailing whitespace in SMILES is normalised before comparison."""
        resp = await _post_answer(
            {
                "smiles_answer": "  c1ccccc1  ",
                "expected_smiles": "c1ccccc1",
            }
        )
        assert resp.status_code == 200
        assert resp.json()["correct"] is True


class TestQuizAnswerMultipleChoice:
    async def test_chosen_key_returns_mc_type(self):
        """chosen_key submissions return answer_type=multiple_choice."""
        resp = await _post_answer({"chosen_key": "B"})
        assert resp.status_code == 200
        body = resp.json()
        assert body["answer_type"] == "multiple_choice"
        assert body["submitted"] == "B"

    async def test_chosen_key_feedback_message(self):
        """Multiple-choice response includes a useful feedback message."""
        resp = await _post_answer({"chosen_key": "A"})
        assert resp.status_code == 200
        assert len(resp.json()["feedback"]) > 0


class TestQuizAnswerValidation:
    async def test_empty_body_returns_422(self):
        """Request with neither chosen_key nor smiles_answer is rejected."""
        resp = await _post_answer({})
        assert resp.status_code == 422

    async def test_both_fields_smiles_takes_precedence(self):
        """When both fields are set, SMILES evaluation takes precedence."""
        resp = await _post_answer(
            {
                "chosen_key": "A",
                "smiles_answer": "c1ccccc1",
                "expected_smiles": "c1ccccc1",
            }
        )
        assert resp.status_code == 200
        assert resp.json()["answer_type"] == "smiles"
