"""Quiz answer endpoint — supports multiple-choice and SMILES molecule answers.

POST /v1/quiz/answer

Phase 6 addition: ``smiles_answer`` field accepts a SMILES string generated
from the Three.js molecule builder canvas.  The backend evaluates it against
the ``expected_smiles`` field using normalised string comparison (canonical
ordering, case-insensitive atom symbols).  When ``rdkit`` is available a
proper canonical SMILES comparison is performed; otherwise a stripped
case-insensitive string match is used as fallback.
"""

import logging
import re

from fastapi import APIRouter, HTTPException

from .models import QuizAnswerRequest, QuizAnswerResponse

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/quiz", tags=["quiz"])

_rdkit_available: bool | None = None


def _probe_rdkit() -> bool:
    """Return True if rdkit is importable."""
    global _rdkit_available
    if _rdkit_available is not None:
        return _rdkit_available
    try:
        from rdkit import Chem  # noqa: F401

        _rdkit_available = True
    except ImportError:
        logger.debug("rdkit not installed — using string-match SMILES comparison")
        _rdkit_available = False
    return _rdkit_available


def _normalize_smiles(smiles: str) -> str:
    """Return canonical SMILES via rdkit, or a stripped/lowercased string.

    Args:
        smiles: Raw SMILES string from the molecule builder or expected answer.

    Returns:
        Canonical SMILES string, or simplified normalised string when rdkit
        is unavailable.
    """
    smiles = smiles.strip()
    if _probe_rdkit():
        from rdkit import Chem

        mol = Chem.MolFromSmiles(smiles)
        if mol is None:
            return smiles.lower()
        return Chem.MolToSmiles(mol, canonical=True)
    # Fallback: strip whitespace, uppercase, remove stereo markers
    return re.sub(r"[\s@/\\]", "", smiles).upper()


def _evaluate_smiles(submitted: str, expected: str) -> tuple[bool, str]:
    """Compare submitted and expected SMILES strings.

    Args:
        submitted: SMILES from the student's molecule builder canvas.
        expected: Correct SMILES stored in the quiz prompt.

    Returns:
        Tuple of (is_correct, feedback_message).
    """
    norm_submitted = _normalize_smiles(submitted)
    norm_expected = _normalize_smiles(expected)
    correct = norm_submitted == norm_expected
    if correct:
        feedback = "Correct! Your molecular structure matches the expected answer."
    else:
        feedback = (
            "Not quite — your structure doesn't match the expected answer. "
            "Check your bond types and atom counts, then try again."
        )
    return correct, feedback


@router.post("/answer", response_model=QuizAnswerResponse)
async def submit_quiz_answer(body: QuizAnswerRequest) -> QuizAnswerResponse:
    """Evaluate a quiz answer submitted by the student.

    Accepts either a multiple-choice key (``chosen_key``) or a SMILES string
    from the molecule builder (``smiles_answer``).  Exactly one must be
    provided per request.

    For SMILES answers, ``expected_smiles`` must also be provided so the
    backend can evaluate correctness.  The comparison uses canonical SMILES
    via rdkit when available, or stripped string matching as fallback.

    Args:
        body: QuizAnswerRequest with either chosen_key or smiles_answer set.

    Returns:
        QuizAnswerResponse with correctness flag and feedback message.

    Raises:
        422: If neither chosen_key nor smiles_answer is provided.
        422: If smiles_answer is provided without expected_smiles.
    """
    if body.chosen_key is None and body.smiles_answer is None:
        raise HTTPException(
            status_code=422,
            detail="Provide either chosen_key or smiles_answer.",
        )

    if body.smiles_answer is not None:
        if not body.expected_smiles:
            raise HTTPException(
                status_code=422,
                detail="expected_smiles is required when submitting a smiles_answer.",
            )
        correct, feedback = _evaluate_smiles(body.smiles_answer, body.expected_smiles)
        return QuizAnswerResponse(
            correct=correct,
            feedback=feedback,
            answer_type="smiles",
            submitted=body.smiles_answer,
        )

    # Multiple-choice: server-side evaluation not implemented here (grading
    # logic lives in the session context); return a stub so the frontend can
    # display the answer.  Full MC grading is Phase 7.
    return QuizAnswerResponse(
        correct=False,
        feedback=(
            "Multiple-choice grading is processed by Professor Nova. "
            "See the tutor's next response for feedback."
        ),
        answer_type="multiple_choice",
        submitted=body.chosen_key or "",
    )
