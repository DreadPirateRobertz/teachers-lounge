package flashcard_test

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/teacherslounge/gaming-service/internal/flashcard"
	_ "modernc.org/sqlite"
)

// testCards returns a small slice of AnkiCards for use in tests.
func testCards() []flashcard.AnkiCard {
	return []flashcard.AnkiCard{
		{ID: "card-1", Front: "What is 2+2?", Back: "4", Topic: "math"},
		{ID: "card-2", Front: "Capital of France?", Back: "Paris", Topic: "geography"},
		{ID: "card-3", Front: "Go keyword for defer?", Back: "defer", Topic: "go"},
	}
}

// TestBuildAPKGReturnsBytes verifies that BuildAPKG returns non-nil, non-empty bytes.
func TestBuildAPKGReturnsBytes(t *testing.T) {
	data, err := flashcard.BuildAPKG("Test Deck", testCards())
	if err != nil {
		t.Fatalf("BuildAPKG returned error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("BuildAPKG returned empty bytes")
	}
}

// TestBuildAPKGIsValidZIP verifies that the output is a valid ZIP archive
// containing exactly the entries "collection.anki2" and "media".
func TestBuildAPKGIsValidZIP(t *testing.T) {
	data, err := flashcard.BuildAPKG("Test Deck", testCards())
	if err != nil {
		t.Fatalf("BuildAPKG: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a valid ZIP: %v", err)
	}

	names := make(map[string]bool)
	for _, f := range r.File {
		names[f.Name] = true
	}

	if !names["collection.anki2"] {
		t.Error("ZIP missing entry: collection.anki2")
	}
	if !names["media"] {
		t.Error("ZIP missing entry: media")
	}
}

// TestBuildAPKGMediaEntry verifies that the media ZIP entry contains exactly "{}".
func TestBuildAPKGMediaEntry(t *testing.T) {
	data, err := flashcard.BuildAPKG("Test Deck", testCards())
	if err != nil {
		t.Fatalf("BuildAPKG: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	for _, f := range r.File {
		if f.Name != "media" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open media entry: %v", err)
		}
		defer rc.Close()

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(rc)
		if buf.String() != "{}" {
			t.Errorf("media entry = %q, want {}", buf.String())
		}
		return
	}
	t.Fatal("media entry not found")
}

// TestBuildAPKGCollectionIsValidSQLite verifies that collection.anki2 is a
// valid SQLite database that can be opened and queried.
func TestBuildAPKGCollectionIsValidSQLite(t *testing.T) {
	data, err := flashcard.BuildAPKG("Test Deck", testCards())
	if err != nil {
		t.Fatalf("BuildAPKG: %v", err)
	}
	db := openAnki2FromAPKG(t, data)
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM col").Scan(&count); err != nil {
		t.Fatalf("query col: %v", err)
	}
	if count != 1 {
		t.Errorf("col row count = %d, want 1", count)
	}
}

// TestBuildAPKGNoteCount verifies that the notes table has exactly as many rows
// as there are input cards.
func TestBuildAPKGNoteCount(t *testing.T) {
	cards := testCards()
	data, err := flashcard.BuildAPKG("Test Deck", cards)
	if err != nil {
		t.Fatalf("BuildAPKG: %v", err)
	}
	db := openAnki2FromAPKG(t, data)
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM notes").Scan(&count); err != nil {
		t.Fatalf("query notes: %v", err)
	}
	if count != len(cards) {
		t.Errorf("notes count = %d, want %d", count, len(cards))
	}
}

// TestBuildAPKGCardCount verifies that the cards table has exactly as many rows
// as there are input cards.
func TestBuildAPKGCardCount(t *testing.T) {
	cards := testCards()
	data, err := flashcard.BuildAPKG("Test Deck", cards)
	if err != nil {
		t.Fatalf("BuildAPKG: %v", err)
	}
	db := openAnki2FromAPKG(t, data)
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM cards").Scan(&count); err != nil {
		t.Fatalf("query cards: %v", err)
	}
	if count != len(cards) {
		t.Errorf("cards count = %d, want %d", count, len(cards))
	}
}

// TestBuildAPKGFldsContainsFrontAndBack verifies that each note's flds column
// contains front and back text separated by the Anki field separator (\x1f).
// Notes are matched by GUID (card ID) rather than row order, since sha1-derived
// note IDs are not guaranteed to sort in insertion order.
func TestBuildAPKGFldsContainsFrontAndBack(t *testing.T) {
	cards := testCards()
	data, err := flashcard.BuildAPKG("Test Deck", cards)
	if err != nil {
		t.Fatalf("BuildAPKG: %v", err)
	}
	db := openAnki2FromAPKG(t, data)
	defer db.Close()

	// Build a map of guid → flds from the database.
	rows, err := db.Query("SELECT guid, flds FROM notes")
	if err != nil {
		t.Fatalf("query flds: %v", err)
	}
	defer rows.Close()

	guidToFlds := make(map[string]string)
	for rows.Next() {
		var guid, flds string
		if err := rows.Scan(&guid, &flds); err != nil {
			t.Fatalf("scan guid+flds: %v", err)
		}
		guidToFlds[guid] = flds
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows error: %v", err)
	}

	for _, card := range cards {
		flds, ok := guidToFlds[card.ID]
		if !ok {
			t.Errorf("note with guid %q not found in database", card.ID)
			continue
		}
		parts := strings.Split(flds, "\x1f")
		if len(parts) != 2 {
			t.Errorf("card %s: expected 2 fields, got %d", card.ID, len(parts))
		} else {
			if parts[0] != card.Front {
				t.Errorf("card %s front = %q, want %q", card.ID, parts[0], card.Front)
			}
			if parts[1] != card.Back {
				t.Errorf("card %s back = %q, want %q", card.ID, parts[1], card.Back)
			}
		}
	}
}

// TestBuildAPKGNoteIDsAreStable verifies that note and card IDs are deterministic:
// exporting the same cards twice produces identical IDs. This guards against the
// nowMs+i timestamp-based approach that breaks for large decks.
func TestBuildAPKGNoteIDsAreStable(t *testing.T) {
	cards := testCards()

	ids1 := exportNoteIDs(t, cards)
	ids2 := exportNoteIDs(t, cards)

	if len(ids1) != len(ids2) {
		t.Fatalf("id count mismatch: %d vs %d", len(ids1), len(ids2))
	}
	for k, v1 := range ids1 {
		v2, ok := ids2[k]
		if !ok {
			t.Errorf("guid %q missing from second export", k)
			continue
		}
		if v1 != v2 {
			t.Errorf("guid %q: noteID changed between exports (%d → %d)", k, v1, v2)
		}
	}
}

// exportNoteIDs runs BuildAPKG and returns a map of guid → noteID from the notes table.
func exportNoteIDs(t *testing.T, cards []flashcard.AnkiCard) map[string]int64 {
	t.Helper()
	data, err := flashcard.BuildAPKG("Stable IDs Test", cards)
	if err != nil {
		t.Fatalf("BuildAPKG: %v", err)
	}
	db := openAnki2FromAPKG(t, data)
	defer db.Close()

	rows, err := db.Query("SELECT guid, id FROM notes")
	if err != nil {
		t.Fatalf("query notes: %v", err)
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var guid string
		var id int64
		if err := rows.Scan(&guid, &id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		result[guid] = id
	}
	return result
}

// TestBuildAPKGEmptyCards verifies that BuildAPKG with no cards produces a
// valid .apkg with 0 notes and 0 cards.
func TestBuildAPKGEmptyCards(t *testing.T) {
	data, err := flashcard.BuildAPKG("Empty Deck", []flashcard.AnkiCard{})
	if err != nil {
		t.Fatalf("BuildAPKG empty: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty bytes for empty deck")
	}

	db := openAnki2FromAPKG(t, data)
	defer db.Close()

	var noteCount, cardCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM notes").Scan(&noteCount); err != nil {
		t.Fatalf("query notes: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM cards").Scan(&cardCount); err != nil {
		t.Fatalf("query cards: %v", err)
	}
	if noteCount != 0 {
		t.Errorf("expected 0 notes, got %d", noteCount)
	}
	if cardCount != 0 {
		t.Errorf("expected 0 cards, got %d", cardCount)
	}
}

// TestBuildAPKGDeckNameInDecksJSON verifies that the deck name provided to
// BuildAPKG appears in the decks JSON stored in the col table.
func TestBuildAPKGDeckNameInDecksJSON(t *testing.T) {
	deckName := "My Custom Deck"
	data, err := flashcard.BuildAPKG(deckName, testCards())
	if err != nil {
		t.Fatalf("BuildAPKG: %v", err)
	}
	db := openAnki2FromAPKG(t, data)
	defer db.Close()

	var decksJSON string
	if err := db.QueryRow("SELECT decks FROM col").Scan(&decksJSON); err != nil {
		t.Fatalf("query decks: %v", err)
	}

	var decks map[string]map[string]any
	if err := json.Unmarshal([]byte(decksJSON), &decks); err != nil {
		t.Fatalf("unmarshal decks json: %v", err)
	}

	found := false
	for _, deck := range decks {
		if name, ok := deck["name"].(string); ok && name == deckName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("deck name %q not found in decks JSON: %s", deckName, decksJSON)
	}
}

// openAnki2FromAPKG extracts and opens the collection.anki2 SQLite database
// from raw .apkg bytes, writing it to a temp file so modernc.org/sqlite can
// read an existing database image. The caller is responsible for closing the DB.
func openAnki2FromAPKG(t *testing.T, apkgData []byte) *sql.DB {
	t.Helper()

	r, err := zip.NewReader(bytes.NewReader(apkgData), int64(len(apkgData)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	for _, f := range r.File {
		if f.Name != "collection.anki2" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open collection.anki2: %v", err)
		}
		defer rc.Close()

		var buf bytes.Buffer
		if _, err := buf.ReadFrom(rc); err != nil {
			t.Fatalf("read collection.anki2: %v", err)
		}

		tmpPath := t.TempDir() + "/col.anki2"
		if err := os.WriteFile(tmpPath, buf.Bytes(), 0600); err != nil {
			t.Fatalf("write temp db: %v", err)
		}

		db, err := sql.Open("sqlite", tmpPath)
		if err != nil {
			t.Fatalf("open sqlite: %v", err)
		}
		return db
	}
	t.Fatal("collection.anki2 not found in zip")
	return nil
}
