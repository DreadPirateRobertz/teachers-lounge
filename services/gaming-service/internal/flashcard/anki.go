package flashcard

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite" // register the sqlite3 driver
)

// AnkiCard is the data for a single flashcard to export.
type AnkiCard struct {
	// ID is a unique identifier for the card (used as the Anki note GUID).
	ID string
	// Front is the question side of the card.
	Front string
	// Back is the answer side of the card.
	Back string
	// Topic is an optional topic label for the card.
	Topic string
}

// ankiColConf is the static configuration JSON stored in the col table.
const ankiColConf = `{"nextPos":1,"estTimes":true,"activeDecks":[1],"sortType":"noteFld","timeLim":0,"sortBackwards":false,"addToCur":true,"curDeck":1,"newBury":true,"newSpread":0,"dueCounts":true,"curModel":"1","collapseTime":1200}`

// ankiDconf is the static deck configuration JSON stored in the col table.
const ankiDconf = `{"1":{"id":1,"mod":0,"name":"Default","usn":0,"maxTaken":60,"autoplay":true,"timer":0,"replayq":true,"new":{"bury":true,"delays":[1,10],"initialFactor":2500,"ints":[1,4,7],"order":1,"perDay":20,"separate":true},"lapse":{"delays":[10],"leechAction":0,"leechFails":8,"minInt":1,"mult":0},"rev":{"bury":true,"ease4":1.3,"fuzz":0.05,"ivlFct":1,"maxIvl":36500,"minSpace":1,"perDay":100}}}`

// BuildAPKG creates a valid Anki 2.1 .apkg file (ZIP containing collection.anki2
// SQLite database + empty media file) from the provided cards.
// Returns the raw .apkg bytes.
//
// The .apkg uses the Basic note type with Front and Back fields. Each AnkiCard
// produces one note and one card in the Anki collection. The SM-2 scheduling
// state is left at Anki defaults; Anki will schedule cards from scratch.
func BuildAPKG(deckName string, cards []AnkiCard) ([]byte, error) {
	now := time.Now().Unix()
	nowMs := time.Now().UnixMilli()

	// Use a temp file path so we can VACUUM INTO it and read the raw bytes.
	tmpPath := fmt.Sprintf("/tmp/anki_%d.db", time.Now().UnixNano())
	defer os.Remove(tmpPath)

	// Open an in-memory SQLite database for building the collection.
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	defer db.Close()

	// Create all required Anki tables.
	if err := createAnkiTables(db); err != nil {
		return nil, fmt.Errorf("create anki tables: %w", err)
	}

	// Build models JSON: one Basic model with two fields and one card template.
	modelsJSON, err := buildModelsJSON(now)
	if err != nil {
		return nil, fmt.Errorf("build models json: %w", err)
	}

	// Build decks JSON with the provided deck name.
	decksJSON, err := buildDecksJSON(deckName, now)
	if err != nil {
		return nil, fmt.Errorf("build decks json: %w", err)
	}

	// Insert the single collection row.
	const insertCol = `INSERT INTO col
		(id, crt, mod, scm, ver, dty, usn, ls, conf, models, decks, dconf, tags)
		VALUES (1, ?, ?, ?, 11, 0, 0, 0, ?, ?, ?, ?, '{}')`
	if _, err := db.Exec(insertCol, now, now, now, ankiColConf, modelsJSON, decksJSON, ankiDconf); err != nil {
		return nil, fmt.Errorf("insert col: %w", err)
	}

	// Insert notes and cards for each AnkiCard.
	const insertNote = `INSERT INTO notes
		(id, guid, mid, mod, usn, tags, flds, sfld, csum, flags, data)
		VALUES (?, ?, 1, ?, 0, '', ?, ?, ?, 0, '')`
	const insertCard = `INSERT INTO cards
		(id, nid, did, ord, mod, usn, type, queue, due, ivl, factor, reps, lapses, left, odue, odid, flags, data)
		VALUES (?, ?, 1, 0, ?, 0, 0, 0, ?, 0, 2500, 0, 0, 0, 0, 0, 0, '')`

	for i, card := range cards {
		noteID := nowMs + int64(i)
		cardID := nowMs + int64(len(cards)) + int64(i)
		flds := card.Front + "\x1f" + card.Back
		csum := fieldChecksum(card.Front)

		if _, err := db.Exec(insertNote, noteID, card.ID, now, flds, card.Front, csum); err != nil {
			return nil, fmt.Errorf("insert note %d: %w", i, err)
		}
		if _, err := db.Exec(insertCard, cardID, noteID, now, i); err != nil {
			return nil, fmt.Errorf("insert card %d: %w", i, err)
		}
	}

	// Serialize the SQLite database to bytes via VACUUM INTO a temp file.
	if _, err := db.Exec("VACUUM INTO ?", tmpPath); err != nil {
		return nil, fmt.Errorf("vacuum into temp file: %w", err)
	}

	sqliteBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read vacuumed db: %w", err)
	}

	// Build the ZIP (.apkg) containing collection.anki2 and media.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	if err := writeZipEntry(zw, "collection.anki2", sqliteBytes); err != nil {
		return nil, fmt.Errorf("write collection.anki2: %w", err)
	}
	if err := writeZipEntry(zw, "media", []byte("{}")); err != nil {
		return nil, fmt.Errorf("write media: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}

	return buf.Bytes(), nil
}

// createAnkiTables creates all required Anki 2.1 collection tables in the given DB.
func createAnkiTables(db *sql.DB) error {
	const schema = `
CREATE TABLE col (id integer, crt integer, mod integer, scm integer, ver integer, dty integer, usn integer, ls integer, conf text, models text, decks text, dconf text, tags text);
CREATE TABLE notes (id integer, guid text, mid integer, mod integer, usn integer, tags text, flds text, sfld integer, csum integer, flags integer, data text);
CREATE TABLE cards (id integer, nid integer, did integer, ord integer, mod integer, usn integer, type integer, queue integer, due integer, ivl integer, factor integer, reps integer, lapses integer, left integer, odue integer, odid integer, flags integer, data text);
CREATE TABLE graves (usn integer, oid integer, type integer);
CREATE TABLE revlog (id integer, cid integer, usn integer, ease integer, ivl integer, lastIvl integer, factor integer, time integer, type integer);`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("create tables: %w", err)
	}
	return nil
}

// ankiFieldDef is one field in an Anki note type model.
type ankiFieldDef struct {
	Font   string `json:"font"`
	Media  []any  `json:"media"`
	Name   string `json:"name"`
	Ord    int    `json:"ord"`
	RTL    bool   `json:"rtl"`
	Size   int    `json:"size"`
	Sticky bool   `json:"sticky"`
}

// ankiTemplateDef is one card template in an Anki note type model.
type ankiTemplateDef struct {
	AFmt  string `json:"afmt"`
	BFont string `json:"bfont"`
	BSize int    `json:"bsize"`
	Did   any    `json:"did"`
	Name  string `json:"name"`
	Ord   int    `json:"ord"`
	QFmt  string `json:"qfmt"`
}

// ankiModelDef is an Anki note type (model).
type ankiModelDef struct {
	CSS       string            `json:"css"`
	Did       int               `json:"did"`
	Flds      []ankiFieldDef    `json:"flds"`
	ID        string            `json:"id"`
	LatexPost string            `json:"latexPost"`
	LatexPre  string            `json:"latexPre"`
	Mod       int64             `json:"mod"`
	Name      string            `json:"name"`
	Ord       int               `json:"ord"`
	SortF     int               `json:"sortf"`
	Tags      []any             `json:"tags"`
	Tmpls     []ankiTemplateDef `json:"tmpls"`
	Type      int               `json:"type"`
	USN       int               `json:"usn"`
	Vers      []any             `json:"vers"`
}

// buildModelsJSON returns the Anki models JSON for a single Basic note type.
func buildModelsJSON(now int64) (string, error) {
	model := ankiModelDef{
		CSS: ".card { font-family: arial; font-size: 20px; text-align: center; color: black; background-color: white; }",
		Did: 1,
		ID:  "1",
		LatexPost: `\end{document}`,
		LatexPre:  "\\documentclass[12pt]{article}\n\\special{papersize=3in,5in}\n\\usepackage[utf8]{inputenc}\n\\usepackage{amssymb,amsmath}\n\\pagestyle{empty}\n\\setlength{\\parindent}{0in}\n\\begin{document}\n",
		Mod:       now,
		Name:      "TeachersLounge Basic",
		Ord:       0,
		SortF:     0,
		Tags:      []any{},
		Type:      0,
		USN:       0,
		Vers:      []any{},
		Flds: []ankiFieldDef{
			{Font: "Arial", Media: []any{}, Name: "Front", Ord: 0, RTL: false, Size: 20, Sticky: false},
			{Font: "Arial", Media: []any{}, Name: "Back", Ord: 1, RTL: false, Size: 20, Sticky: false},
		},
		Tmpls: []ankiTemplateDef{
			{
				AFmt:  "{{FrontSide}}<hr id=answer>{{Back}}",
				BFont: "Arial",
				BSize: 12,
				Did:   nil,
				Name:  "Card 1",
				Ord:   0,
				QFmt:  "{{Front}}",
			},
		},
	}

	m := map[string]ankiModelDef{"1": model}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ankiDeckDef is a single Anki deck entry in the decks JSON.
type ankiDeckDef struct {
	Collapsed bool   `json:"collapsed"`
	Conf      int    `json:"conf"`
	Desc      string `json:"desc"`
	Dyn       int    `json:"dyn"`
	ExtendNew int    `json:"extendNew"`
	ExtendRev int    `json:"extendRev"`
	ID        int    `json:"id"`
	LrnToday  []int  `json:"lrnToday"`
	Mod       int64  `json:"mod"`
	Name      string `json:"name"`
	NewToday  []int  `json:"newToday"`
	RevToday  []int  `json:"revToday"`
	TimeToday []int  `json:"timeToday"`
	USN       int    `json:"usn"`
}

// buildDecksJSON returns the Anki decks JSON containing a single deck with the given name.
func buildDecksJSON(deckName string, now int64) (string, error) {
	deck := ankiDeckDef{
		Collapsed: false,
		Conf:      1,
		Desc:      "",
		Dyn:       0,
		ExtendNew: 10,
		ExtendRev: 50,
		ID:        1,
		LrnToday:  []int{0, 0},
		Mod:       now,
		Name:      deckName,
		NewToday:  []int{0, 0},
		RevToday:  []int{0, 0},
		TimeToday: []int{0, 0},
		USN:       0,
	}

	m := map[string]ankiDeckDef{"1": deck}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// fieldChecksum returns the first 8 decimal digits of the SHA-1 hash of the
// input string as an integer. This is the value Anki stores in the csum column
// of the notes table, used for duplicate detection.
func fieldChecksum(text string) int64 {
	h := sha1.Sum([]byte(text))
	// Interpret the first 4 bytes as a big-endian uint32.
	val := uint32(h[0])<<24 | uint32(h[1])<<16 | uint32(h[2])<<8 | uint32(h[3])
	// Truncate to 8 decimal digits.
	s := fmt.Sprintf("%d", val)
	if len(s) > 8 {
		s = s[:8]
	}
	var result int64
	fmt.Sscanf(s, "%d", &result)
	return result
}

// writeZipEntry writes a named entry with the given data into a zip.Writer.
func writeZipEntry(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
