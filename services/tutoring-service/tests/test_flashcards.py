"""Unit tests for the flashcard module — pure functions only.

Covers:
  - extract_flashcards_from_text: definition patterns, Q&A patterns,
    deduplication, length capping, empty input
  - build_apkg: valid ZIP structure, correct files present, SQLite integrity
  - _note_checksum: deterministic, non-zero for non-empty input
"""
import io
import sqlite3
import zipfile

import pytest

from app.flashcards import (
    _MAX_AUTO_CARDS,
    _MAX_BACK_LEN,
    _MAX_FRONT_LEN,
    _note_checksum,
    build_apkg,
    extract_flashcards_from_text,
)


# ── extract_flashcards_from_text ──────────────────────────────────────────────

class TestExtractFlashcardsFromText:
    def test_empty_text_returns_empty(self):
        assert extract_flashcards_from_text("") == []

    def test_plain_prose_returns_empty(self):
        text = "This is just a paragraph without any structured content."
        assert extract_flashcards_from_text(text) == []

    def test_definition_colon_pattern(self):
        text = "**Photosynthesis**: The process by which plants convert sunlight into glucose."
        pairs = extract_flashcards_from_text(text)
        assert len(pairs) == 1
        assert "Photosynthesis" in pairs[0][0]
        assert "glucose" in pairs[0][1]

    def test_definition_dash_pattern(self):
        text = "- Osmosis — Movement of water across a semi-permeable membrane from low to high solute concentration."
        pairs = extract_flashcards_from_text(text)
        assert len(pairs) == 1
        assert "Osmosis" in pairs[0][0]

    def test_definition_em_dash_pattern(self):
        text = "Mitosis — Cell division producing two identical daughter cells."
        pairs = extract_flashcards_from_text(text)
        assert len(pairs) == 1
        assert "Mitosis" in pairs[0][0]

    def test_qa_pattern(self):
        text = "Q: What is Newton's second law?\nA: Force equals mass times acceleration (F = ma)."
        pairs = extract_flashcards_from_text(text)
        assert len(pairs) == 1
        assert "Newton" in pairs[0][0]
        assert "F = ma" in pairs[0][1]

    def test_qa_pattern_case_insensitive(self):
        text = "q: What is a cell?\na: The basic structural unit of life."
        pairs = extract_flashcards_from_text(text)
        assert len(pairs) == 1

    def test_multiple_definitions_extracted(self):
        text = (
            "**DNA**: Deoxyribonucleic acid — carries genetic information.\n"
            "**RNA**: Ribonucleic acid — transcribes and translates DNA.\n"
            "**ATP**: Adenosine triphosphate — the cell's energy currency.\n"
        )
        pairs = extract_flashcards_from_text(text)
        assert len(pairs) >= 2

    def test_deduplication_removes_duplicate_fronts(self):
        text = (
            "**Photosynthesis**: Converts sunlight to glucose.\n"
            "**Photosynthesis**: Converts light energy into chemical energy.\n"
        )
        pairs = extract_flashcards_from_text(text)
        fronts = [p[0] for p in pairs]
        # No duplicate fronts
        assert len(fronts) == len(set(fronts))

    def test_front_length_capped(self):
        long_term = "X" * 500
        text = f"**{long_term}**: Some definition here that is long enough to matter."
        pairs = extract_flashcards_from_text(text)
        if pairs:
            assert len(pairs[0][0]) <= _MAX_FRONT_LEN

    def test_back_length_capped(self):
        long_def = "Y" * 2000
        text = f"**Term**: {long_def}"
        pairs = extract_flashcards_from_text(text)
        if pairs:
            assert len(pairs[0][1]) <= _MAX_BACK_LEN

    def test_max_auto_cards_cap(self):
        # Generate more than _MAX_AUTO_CARDS definition lines
        lines = "\n".join(
            f"**Term{i}**: Definition number {i} with sufficient length."
            for i in range(_MAX_AUTO_CARDS + 10)
        )
        pairs = extract_flashcards_from_text(lines)
        assert len(pairs) <= _MAX_AUTO_CARDS

    def test_returns_list_of_tuples(self):
        text = "**Entropy**: A measure of disorder in a thermodynamic system."
        pairs = extract_flashcards_from_text(text)
        assert isinstance(pairs, list)
        if pairs:
            assert isinstance(pairs[0], tuple)
            assert len(pairs[0]) == 2


# ── _note_checksum ────────────────────────────────────────────────────────────

class TestNoteChecksum:
    def test_returns_integer(self):
        assert isinstance(_note_checksum("hello"), int)

    def test_deterministic(self):
        assert _note_checksum("same input") == _note_checksum("same input")

    def test_different_inputs_different_checksums(self):
        assert _note_checksum("apple") != _note_checksum("banana")

    def test_non_zero_for_nonempty(self):
        assert _note_checksum("anything") != 0

    def test_empty_string_does_not_crash(self):
        result = _note_checksum("")
        assert isinstance(result, int)


# ── build_apkg ────────────────────────────────────────────────────────────────

class TestBuildApkg:
    def _open_apkg(self, apkg_bytes: bytes) -> zipfile.ZipFile:
        return zipfile.ZipFile(io.BytesIO(apkg_bytes))

    def test_returns_bytes(self):
        result = build_apkg("Test Deck", [("front", "back")])
        assert isinstance(result, bytes)
        assert len(result) > 0

    def test_valid_zip_file(self):
        result = build_apkg("Test Deck", [("Q", "A")])
        assert zipfile.is_zipfile(io.BytesIO(result))

    def test_contains_collection_anki2(self):
        result = build_apkg("Test Deck", [("Q", "A")])
        with self._open_apkg(result) as zf:
            assert "collection.anki2" in zf.namelist()

    def test_contains_media_file(self):
        result = build_apkg("Test Deck", [("Q", "A")])
        with self._open_apkg(result) as zf:
            assert "media" in zf.namelist()

    def test_media_file_is_empty_json_object(self):
        result = build_apkg("Test Deck", [("Q", "A")])
        with self._open_apkg(result) as zf:
            media_content = zf.read("media").decode()
        assert media_content == "{}"

    def test_collection_is_valid_sqlite(self):
        result = build_apkg("Test Deck", [("Q", "A")])
        with self._open_apkg(result) as zf:
            db_bytes = zf.read("collection.anki2")
        conn = sqlite3.connect(":memory:")
        conn.deserialize(db_bytes)
        tables = conn.execute(
            "SELECT name FROM sqlite_master WHERE type='table'"
        ).fetchall()
        table_names = {t[0] for t in tables}
        assert {"col", "notes", "cards", "revlog", "graves"} <= table_names
        conn.close()

    def test_col_table_has_one_row(self):
        result = build_apkg("MyDeck", [("Q1", "A1")])
        with self._open_apkg(result) as zf:
            db_bytes = zf.read("collection.anki2")
        conn = sqlite3.connect(":memory:")
        conn.deserialize(db_bytes)
        count = conn.execute("SELECT COUNT(*) FROM col").fetchone()[0]
        assert count == 1
        conn.close()

    def test_notes_count_matches_cards_input(self):
        cards = [("Q1", "A1"), ("Q2", "A2"), ("Q3", "A3")]
        result = build_apkg("Test", cards)
        with self._open_apkg(result) as zf:
            db_bytes = zf.read("collection.anki2")
        conn = sqlite3.connect(":memory:")
        conn.deserialize(db_bytes)
        note_count = conn.execute("SELECT COUNT(*) FROM notes").fetchone()[0]
        card_count = conn.execute("SELECT COUNT(*) FROM cards").fetchone()[0]
        assert note_count == 3
        assert card_count == 3
        conn.close()

    def test_empty_cards_list_produces_valid_archive(self):
        result = build_apkg("Empty Deck", [])
        assert zipfile.is_zipfile(io.BytesIO(result))
        with self._open_apkg(result) as zf:
            db_bytes = zf.read("collection.anki2")
        conn = sqlite3.connect(":memory:")
        conn.deserialize(db_bytes)
        assert conn.execute("SELECT COUNT(*) FROM notes").fetchone()[0] == 0
        conn.close()

    def test_notes_contain_front_and_back(self):
        result = build_apkg("Deck", [("What is H2O?", "Water")])
        with self._open_apkg(result) as zf:
            db_bytes = zf.read("collection.anki2")
        conn = sqlite3.connect(":memory:")
        conn.deserialize(db_bytes)
        flds = conn.execute("SELECT flds FROM notes").fetchone()[0]
        assert "What is H2O?" in flds
        assert "Water" in flds
        conn.close()

    def test_field_separator_is_unit_separator(self):
        result = build_apkg("Deck", [("front text", "back text")])
        with self._open_apkg(result) as zf:
            db_bytes = zf.read("collection.anki2")
        conn = sqlite3.connect(":memory:")
        conn.deserialize(db_bytes)
        flds = conn.execute("SELECT flds FROM notes").fetchone()[0]
        # Anki uses ASCII 0x1f (unit separator) between fields
        assert "\x1f" in flds
        conn.close()

    def test_deck_name_in_decks_json(self):
        result = build_apkg("Chemistry Basics", [("Q", "A")])
        with self._open_apkg(result) as zf:
            db_bytes = zf.read("collection.anki2")
        conn = sqlite3.connect(":memory:")
        conn.deserialize(db_bytes)
        decks_json = conn.execute("SELECT decks FROM col").fetchone()[0]
        assert "Chemistry Basics" in decks_json
        conn.close()

    def test_different_deck_names_produce_different_files(self):
        apkg1 = build_apkg("Deck A", [("Q", "A")])
        apkg2 = build_apkg("Deck B", [("Q", "A")])
        assert apkg1 != apkg2

    def test_models_json_has_basic_note_type(self):
        result = build_apkg("Test", [("Q", "A")])
        with self._open_apkg(result) as zf:
            db_bytes = zf.read("collection.anki2")
        conn = sqlite3.connect(":memory:")
        conn.deserialize(db_bytes)
        models_json = conn.execute("SELECT models FROM col").fetchone()[0]
        assert "TeachersLounge Basic" in models_json
        conn.close()

    def test_cards_assigned_to_correct_deck(self):
        result = build_apkg("SpecificDeck", [("front", "back")])
        with self._open_apkg(result) as zf:
            db_bytes = zf.read("collection.anki2")
        conn = sqlite3.connect(":memory:")
        conn.deserialize(db_bytes)
        # cards.did must match the deck_id derived from the name hash
        card_did = conn.execute("SELECT did FROM cards").fetchone()[0]
        # Verify the deck_id is in the decks JSON
        decks_json = conn.execute("SELECT decks FROM col").fetchone()[0]
        assert str(card_did) in decks_json
        conn.close()
