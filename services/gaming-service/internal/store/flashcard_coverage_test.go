package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/teacherslounge/gaming-service/internal/model"
)

func flashcardScanRow() pgx.Row {
	now := time.Now()
	return &funcRow{fn: func(dest ...any) error {
		*(dest[0].(*string)) = "card-1"
		*(dest[1].(*string)) = "user-1"
		*(dest[2].(**string)) = nil
		*(dest[3].(**string)) = nil
		*(dest[4].(*string)) = "What is H2O?"
		*(dest[5].(*string)) = "Water"
		*(dest[6].(*string)) = "manual"
		*(dest[7].(**string)) = nil
		*(dest[8].(**string)) = nil
		*(dest[9].(*float64)) = 2.5
		*(dest[10].(*int)) = 1
		*(dest[11].(*int)) = 0
		*(dest[12].(*time.Time)) = now.Add(24 * time.Hour)
		*(dest[13].(**time.Time)) = nil
		*(dest[14].(*time.Time)) = now
		return nil
	}}
}

type flashcardRowsStub struct {
	count int
	pos   int
}

func (r *flashcardRowsStub) Next() bool { r.pos++; return r.pos-1 < r.count }
func (r *flashcardRowsStub) Err() error { return nil }
func (r *flashcardRowsStub) Close()     {}
func (r *flashcardRowsStub) CommandTag() pgconn.CommandTag               { return pgconn.CommandTag{} }
func (r *flashcardRowsStub) Conn() *pgx.Conn                             { return nil }
func (r *flashcardRowsStub) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *flashcardRowsStub) Values() ([]any, error)                      { return nil, nil }
func (r *flashcardRowsStub) RawValues() [][]byte                         { return nil }
func (r *flashcardRowsStub) Scan(dest ...any) error {
	now := time.Now()
	*(dest[0].(*string)) = "card-1"
	*(dest[1].(*string)) = "user-1"
	*(dest[2].(**string)) = nil
	*(dest[3].(**string)) = nil
	*(dest[4].(*string)) = "Q"
	*(dest[5].(*string)) = "A"
	*(dest[6].(*string)) = "quiz"
	*(dest[7].(**string)) = nil
	*(dest[8].(**string)) = nil
	*(dest[9].(*float64)) = 2.5
	*(dest[10].(*int)) = 1
	*(dest[11].(*int)) = 0
	*(dest[12].(*time.Time)) = now
	*(dest[13].(**time.Time)) = nil
	*(dest[14].(*time.Time)) = now
	return nil
}

type flashcardQueryDB struct {
	qrows    pgx.Rows
	queryErr error
	qrow     pgx.Row
}

func (d *flashcardQueryDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if d.qrow != nil {
		return d.qrow
	}
	return errFuncRow(pgx.ErrNoRows)
}
func (d *flashcardQueryDB) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return d.qrows, d.queryErr
}
func (d *flashcardQueryDB) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (d *flashcardQueryDB) Begin(_ context.Context) (pgx.Tx, error) { return nil, nil }

func TestCreateFlashcard_HappyPath(t *testing.T) {
	s := newMasteryStore(t, &rowQueueDB{rows: []pgx.Row{flashcardScanRow()}})
	got, err := s.CreateFlashcard(context.Background(), &model.Flashcard{UserID: "user-1", Front: "Q", Back: "A", Source: "manual"})
	if err != nil || got.Front != "What is H2O?" {
		t.Errorf("CreateFlashcard: err=%v front=%q", err, got.Front)
	}
}

func TestCreateFlashcard_DBError_Propagated(t *testing.T) {
	dbErr := errors.New("insert failed")
	s := newMasteryStore(t, &rowQueueDB{rows: []pgx.Row{errFuncRow(dbErr)}})
	if _, err := s.CreateFlashcard(context.Background(), &model.Flashcard{UserID: "u"}); !errors.Is(err, dbErr) {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

func TestGetFlashcard_HappyPath(t *testing.T) {
	s := newMasteryStore(t, &flashcardQueryDB{qrow: flashcardScanRow()})
	got, err := s.GetFlashcard(context.Background(), "card-1")
	if err != nil || got == nil || got.ID != "card-1" {
		t.Errorf("GetFlashcard: err=%v got=%v", err, got)
	}
}

func TestGetFlashcard_NotFound_ReturnsNil(t *testing.T) {
	s := newMasteryStore(t, &flashcardQueryDB{})
	got, err := s.GetFlashcard(context.Background(), "missing")
	if err != nil || got != nil {
		t.Errorf("expected nil,nil; got err=%v card=%v", err, got)
	}
}

func TestListFlashcards_HappyPath(t *testing.T) {
	s := newMasteryStore(t, &flashcardQueryDB{qrows: &flashcardRowsStub{count: 2}})
	cards, err := s.ListFlashcards(context.Background(), "user-1")
	if err != nil || len(cards) != 2 {
		t.Errorf("ListFlashcards: err=%v len=%d", err, len(cards))
	}
}

func TestListFlashcards_QueryError_Propagated(t *testing.T) {
	dbErr := errors.New("query failed")
	s := newMasteryStore(t, &flashcardQueryDB{queryErr: dbErr})
	if _, err := s.ListFlashcards(context.Background(), "u"); !errors.Is(err, dbErr) {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

func TestDueFlashcards_HappyPath(t *testing.T) {
	s := newMasteryStore(t, &flashcardQueryDB{qrows: &flashcardRowsStub{count: 1}})
	cards, err := s.DueFlashcards(context.Background(), "user-1")
	if err != nil || len(cards) != 1 {
		t.Errorf("DueFlashcards: err=%v len=%d", err, len(cards))
	}
}

func TestDueFlashcards_QueryError_Propagated(t *testing.T) {
	dbErr := errors.New("due failed")
	s := newMasteryStore(t, &flashcardQueryDB{queryErr: dbErr})
	if _, err := s.DueFlashcards(context.Background(), "u"); !errors.Is(err, dbErr) {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

func TestFlashcardsForSession_HappyPath(t *testing.T) {
	s := newMasteryStore(t, &flashcardQueryDB{qrows: &flashcardRowsStub{count: 3}})
	cards, err := s.FlashcardsForSession(context.Background(), "session-1")
	if err != nil || len(cards) != 3 {
		t.Errorf("FlashcardsForSession: err=%v len=%d", err, len(cards))
	}
}

func TestAllFlashcardsForExport_HappyPath(t *testing.T) {
	s := newMasteryStore(t, &flashcardQueryDB{qrows: &flashcardRowsStub{count: 5}})
	cards, err := s.AllFlashcardsForExport(context.Background(), "user-1")
	if err != nil || len(cards) != 5 {
		t.Errorf("AllFlashcardsForExport: err=%v len=%d", err, len(cards))
	}
}

func TestReviewFlashcard_BeginError_Propagated(t *testing.T) {
	beginErr := errors.New("pool exhausted")
	s := newMasteryStore(t, &txDB{txErr: beginErr})
	if _, err := s.ReviewFlashcard(context.Background(), "c", "u", 4); !errors.Is(err, beginErr) {
		t.Errorf("expected begin error, got %v", err)
	}
}

func TestReviewFlashcard_SelectError_Propagated(t *testing.T) {
	selectErr := errors.New("lock timeout")
	tx := &txWithQueryRow{mockTx: &mockTx{}, queryRow: errFuncRow(selectErr)}
	s := newMasteryStore(t, &txDB{tx: tx})
	if _, err := s.ReviewFlashcard(context.Background(), "c", "u", 4); err == nil {
		t.Fatal("expected error, got nil")
	}
}
