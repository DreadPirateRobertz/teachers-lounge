package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ── funcRow ───────────────────────────────────────────────────────────────────

type funcRow struct {
	fn func(dest ...any) error
}

func (r *funcRow) Scan(dest ...any) error { return r.fn(dest...) }

func errFuncRow(err error) pgx.Row { return &funcRow{fn: func(_ ...any) error { return err }} }

// ── rowQueueDB ────────────────────────────────────────────────────────────────

type rowQueueDB struct {
	rows    []pgx.Row
	idx     int
	execErr error
}

func (d *rowQueueDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if d.idx >= len(d.rows) {
		return errFuncRow(errors.New("rowQueueDB: no more rows"))
	}
	r := d.rows[d.idx]
	d.idx++
	return r
}
func (d *rowQueueDB) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) { return nil, nil }
func (d *rowQueueDB) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, d.execErr
}
func (d *rowQueueDB) Begin(_ context.Context) (pgx.Tx, error) { return nil, nil }

// ── stringRows ────────────────────────────────────────────────────────────────

type stringRows struct {
	data []string
	pos  int
	err  error
}

func (r *stringRows) Next() bool                                  { r.pos++; return r.pos-1 < len(r.data) }
func (r *stringRows) Err() error                                  { return r.err }
func (r *stringRows) Close()                                      {}
func (r *stringRows) CommandTag() pgconn.CommandTag               { return pgconn.CommandTag{} }
func (r *stringRows) Conn() *pgx.Conn                             { return nil }
func (r *stringRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *stringRows) Values() ([]any, error)                      { return nil, nil }
func (r *stringRows) RawValues() [][]byte                         { return nil }
func (r *stringRows) Scan(dest ...any) error {
	if r.pos == 0 || r.pos-1 >= len(r.data) {
		return errors.New("stringRows: no current row")
	}
	*(dest[0].(*string)) = r.data[r.pos-1]
	return nil
}

// ── queryDB ───────────────────────────────────────────────────────────────────

type queryDB struct {
	qrows    pgx.Rows
	queryErr error
}

func (d *queryDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row     { return nil }
func (d *queryDB) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return d.qrows, d.queryErr
}
func (d *queryDB) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (d *queryDB) Begin(_ context.Context) (pgx.Tx, error) { return nil, nil }

// ── execDB ────────────────────────────────────────────────────────────────────

type execDB struct {
	execCalls int
	execErr   error
	row       pgx.Row
}

func (d *execDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row { return d.row }
func (d *execDB) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) { return nil, nil }
func (d *execDB) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	d.execCalls++
	return pgconn.CommandTag{}, d.execErr
}
func (d *execDB) Begin(_ context.Context) (pgx.Tx, error) { return nil, nil }

// ── achievementRows ───────────────────────────────────────────────────────────

type achievementRows struct {
	data [][]any
	pos  int
}

func (r *achievementRows) Next() bool { r.pos++; return r.pos-1 < len(r.data) }
func (r *achievementRows) Err() error { return nil }
func (r *achievementRows) Close()     {}
func (r *achievementRows) CommandTag() pgconn.CommandTag               { return pgconn.CommandTag{} }
func (r *achievementRows) Conn() *pgx.Conn                             { return nil }
func (r *achievementRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *achievementRows) Values() ([]any, error)                      { return nil, nil }
func (r *achievementRows) RawValues() [][]byte                         { return nil }
func (r *achievementRows) Scan(dest ...any) error {
	row := r.data[r.pos-1]
	*(dest[0].(*string)) = row[0].(string)
	*(dest[1].(*string)) = row[1].(string)
	*(dest[2].(*string)) = row[2].(string)
	*(dest[3].(*string)) = row[3].(string)
	*(dest[4].(*time.Time)) = row[4].(time.Time)
	return nil
}

type achievementQueryDB struct {
	rows     pgx.Rows
	queryErr error
}

func (d *achievementQueryDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return errFuncRow(pgx.ErrNoRows)
}
func (d *achievementQueryDB) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return d.rows, d.queryErr
}
func (d *achievementQueryDB) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (d *achievementQueryDB) Begin(_ context.Context) (pgx.Tx, error) { return nil, nil }

type grantAchievementDB struct {
	mainRow   pgx.Row
	fetchRow  pgx.Row
	callCount int
}

func (d *grantAchievementDB) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	d.callCount++
	if d.callCount == 1 {
		return d.mainRow
	}
	return d.fetchRow
}
func (d *grantAchievementDB) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}
func (d *grantAchievementDB) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (d *grantAchievementDB) Begin(_ context.Context) (pgx.Tx, error) { return nil, nil }

func achievementScanRow() pgx.Row {
	now := time.Now()
	return &funcRow{fn: func(dest ...any) error {
		*(dest[0].(*string)) = "ach-1"
		*(dest[1].(*string)) = "user-1"
		*(dest[2].(*string)) = "first_win"
		*(dest[3].(*string)) = "First Victory"
		*(dest[4].(*time.Time)) = now
		return nil
	}}
}

// ── GetProfile ────────────────────────────────────────────────────────────────

func TestGetProfile_HappyPath(t *testing.T) {
	db := &rowQueueDB{rows: []pgx.Row{&funcRow{fn: func(dest ...any) error {
		*(dest[0].(*string)) = "user-1"
		*(dest[1].(*int)) = 5
		*(dest[2].(*int64)) = 1500
		*(dest[3].(*int)) = 3
		*(dest[4].(*int)) = 7
		*(dest[5].(*int)) = 2
		*(dest[6].(*int)) = 50
		*(dest[7].(*[]byte)) = []byte(`{"shield":1}`)
		*(dest[8].(**time.Time)) = nil
		return nil
	}}}}
	s := newMasteryStore(t, db)
	p, err := s.GetProfile(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if p.Level != 5 || p.XP != 1500 {
		t.Errorf("Level=%d XP=%d; want 5 1500", p.Level, p.XP)
	}
}

func TestGetProfile_NilPowerUps_DefaultsToEmpty(t *testing.T) {
	db := &rowQueueDB{rows: []pgx.Row{&funcRow{fn: func(dest ...any) error {
		*(dest[0].(*string)) = "user-2"
		*(dest[1].(*int)) = 1
		*(dest[2].(*int64)) = 0
		*(dest[3].(*int)) = 0
		*(dest[4].(*int)) = 0
		*(dest[5].(*int)) = 0
		*(dest[6].(*int)) = 0
		*(dest[7].(*[]byte)) = nil
		*(dest[8].(**time.Time)) = nil
		return nil
	}}}}
	s := newMasteryStore(t, db)
	p, err := s.GetProfile(context.Background(), "user-2")
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if string(p.PowerUps) != "{}" {
		t.Errorf("PowerUps: got %q, want {}", string(p.PowerUps))
	}
}

func TestGetProfile_DBError_Propagated(t *testing.T) {
	dbErr := errors.New("connection refused")
	s := newMasteryStore(t, &rowQueueDB{rows: []pgx.Row{errFuncRow(dbErr)}})
	if _, err := s.GetProfile(context.Background(), "u"); !errors.Is(err, dbErr) {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

// ── UpsertXP ──────────────────────────────────────────────────────────────────

func TestUpsertXP_HappyPath(t *testing.T) {
	s := newMasteryStore(t, &rowQueueDB{})
	if err := s.UpsertXP(context.Background(), "user-1", 2000, 6); err != nil {
		t.Fatalf("UpsertXP: %v", err)
	}
}

func TestUpsertXP_DBError_Propagated(t *testing.T) {
	dbErr := errors.New("db down")
	s := newMasteryStore(t, &rowQueueDB{execErr: dbErr})
	if err := s.UpsertXP(context.Background(), "u", 100, 1); !errors.Is(err, dbErr) {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

// ── GetXPAndLevel ─────────────────────────────────────────────────────────────

func TestGetXPAndLevel_HappyPath(t *testing.T) {
	db := &rowQueueDB{rows: []pgx.Row{&funcRow{fn: func(dest ...any) error {
		*(dest[0].(*int64)) = 800
		*(dest[1].(*int)) = 4
		return nil
	}}}}
	s := newMasteryStore(t, db)
	xp, level, err := s.GetXPAndLevel(context.Background(), "u")
	if err != nil {
		t.Fatalf("GetXPAndLevel: %v", err)
	}
	if xp != 800 || level != 4 {
		t.Errorf("got xp=%d level=%d; want 800 4", xp, level)
	}
}

func TestGetXPAndLevel_NoRows_ReturnsDefaults(t *testing.T) {
	s := newMasteryStore(t, &rowQueueDB{rows: []pgx.Row{errFuncRow(errors.New("any"))}})
	xp, level, err := s.GetXPAndLevel(context.Background(), "fresh")
	if err != nil {
		t.Fatalf("GetXPAndLevel: %v", err)
	}
	if xp != 0 || level != 1 {
		t.Errorf("defaults: xp=%d level=%d; want 0 1", xp, level)
	}
}

// ── StreakCheckin ─────────────────────────────────────────────────────────────

func TestStreakCheckin_FirstCheckin_StartStreak(t *testing.T) {
	db := &rowQueueDB{rows: []pgx.Row{&funcRow{fn: func(dest ...any) error {
		*(dest[0].(*int)) = 1
		return nil
	}}}}
	s := newMasteryStore(t, db)
	current, longest, reset, err := s.StreakCheckin(context.Background(), "user-streak")
	if err != nil {
		t.Fatalf("StreakCheckin: %v", err)
	}
	if current != 1 || longest != 1 || reset {
		t.Errorf("got current=%d longest=%d reset=%v; want 1 1 false", current, longest, reset)
	}
}

func TestStreakCheckin_ConsecutiveCheckin_IncrementsStreak(t *testing.T) {
	db := &rowQueueDB{rows: []pgx.Row{
		&funcRow{fn: func(dest ...any) error { *(dest[0].(*int)) = 1; return nil }},
		&funcRow{fn: func(dest ...any) error { *(dest[0].(*int)) = 2; return nil }},
	}}
	s := newMasteryStore(t, db)
	ctx := context.Background()
	if _, _, _, err := s.StreakCheckin(ctx, "user-cons"); err != nil {
		t.Fatalf("first StreakCheckin: %v", err)
	}
	current, _, reset, err := s.StreakCheckin(ctx, "user-cons")
	if err != nil {
		t.Fatalf("second StreakCheckin: %v", err)
	}
	if current != 2 || reset {
		t.Errorf("got current=%d reset=%v; want 2 false", current, reset)
	}
}

func TestStreakCheckin_DBError_Propagated(t *testing.T) {
	dbErr := errors.New("upsert failed")
	s := newMasteryStore(t, &rowQueueDB{rows: []pgx.Row{errFuncRow(dbErr)}})
	if _, _, _, err := s.StreakCheckin(context.Background(), "u"); !errors.Is(err, dbErr) {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

// ── RandomQuote ───────────────────────────────────────────────────────────────

func TestRandomQuote_HappyPath(t *testing.T) {
	db := &rowQueueDB{rows: []pgx.Row{&funcRow{fn: func(dest ...any) error {
		*(dest[0].(*int)) = 42
		*(dest[1].(*string)) = "Live long and prosper."
		*(dest[2].(*string)) = "Spock"
		*(dest[3].(*string)) = "greeting"
		return nil
	}}}}
	s := newMasteryStore(t, db)
	q, err := s.RandomQuote(context.Background())
	if err != nil {
		t.Fatalf("RandomQuote: %v", err)
	}
	if q.Quote != "Live long and prosper." {
		t.Errorf("Quote: got %q", q.Quote)
	}
}

func TestRandomQuote_DBError_Propagated(t *testing.T) {
	dbErr := errors.New("quote db down")
	s := newMasteryStore(t, &rowQueueDB{rows: []pgx.Row{errFuncRow(dbErr)}})
	if _, err := s.RandomQuote(context.Background()); !errors.Is(err, dbErr) {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

// ── AwardQuestRewards ─────────────────────────────────────────────────────────

func TestAwardQuestRewards_HappyPath(t *testing.T) {
	db := &rowQueueDB{rows: []pgx.Row{
		&funcRow{fn: func(dest ...any) error {
			*(dest[0].(*int64)) = 500
			*(dest[1].(*int)) = 3
			return nil
		}},
		&funcRow{fn: func(dest ...any) error {
			*(dest[0].(*int)) = 65
			return nil
		}},
	}}
	s := newMasteryStore(t, db)
	newXP, newLevel, _, newGems, err := s.AwardQuestRewards(context.Background(), "u", 100, 15)
	if err != nil {
		t.Fatalf("AwardQuestRewards: %v", err)
	}
	if newXP < 600 || newLevel < 1 || newGems != 65 {
		t.Errorf("got xp=%d level=%d gems=%d", newXP, newLevel, newGems)
	}
}

func TestAwardQuestRewards_UpsertError_Propagated(t *testing.T) {
	dbErr := errors.New("upsert failed")
	db := &rowQueueDB{rows: []pgx.Row{errFuncRow(errors.New("no rows")), errFuncRow(dbErr)}}
	s := newMasteryStore(t, db)
	if _, _, _, _, err := s.AwardQuestRewards(context.Background(), "u", 50, 5); !errors.Is(err, dbErr) {
		t.Errorf("expected wrapped upsert error, got %v", err)
	}
}

// ── SaveTaunt ─────────────────────────────────────────────────────────────────

func TestSaveTaunt_HappyPath(t *testing.T) {
	s := newMasteryStore(t, &rowQueueDB{})
	if err := s.SaveTaunt(context.Background(), "the_atom", 1, "taunt"); err != nil {
		t.Fatalf("SaveTaunt: %v", err)
	}
}

func TestSaveTaunt_DBError_Propagated(t *testing.T) {
	dbErr := errors.New("exec failed")
	s := newMasteryStore(t, &rowQueueDB{execErr: dbErr})
	if err := s.SaveTaunt(context.Background(), "b", 1, "t"); !errors.Is(err, dbErr) {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

// ── GetRandomTaunt ────────────────────────────────────────────────────────────

func TestGetRandomTaunt_HappyPath(t *testing.T) {
	db := &rowQueueDB{rows: []pgx.Row{&funcRow{fn: func(dest ...any) error {
		*(dest[0].(*string)) = "I will destroy you."
		return nil
	}}}}
	s := newMasteryStore(t, db)
	taunt, ok, err := s.GetRandomTaunt(context.Background(), "the_atom", 2)
	if err != nil {
		t.Fatalf("GetRandomTaunt: %v", err)
	}
	if !ok || taunt != "I will destroy you." {
		t.Errorf("got ok=%v taunt=%q", ok, taunt)
	}
}

func TestGetRandomTaunt_NoRows_ReturnsOkFalse(t *testing.T) {
	s := newMasteryStore(t, &rowQueueDB{rows: []pgx.Row{errFuncRow(pgx.ErrNoRows)}})
	_, ok, err := s.GetRandomTaunt(context.Background(), "b", 99)
	if err != nil || ok {
		t.Errorf("expected nil err and ok=false, got err=%v ok=%v", err, ok)
	}
}

func TestGetRandomTaunt_DBError_Propagated(t *testing.T) {
	dbErr := errors.New("db error")
	s := newMasteryStore(t, &rowQueueDB{rows: []pgx.Row{errFuncRow(dbErr)}})
	if _, _, err := s.GetRandomTaunt(context.Background(), "b", 1); !errors.Is(err, dbErr) {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

// ── GetDefeatedBossIDs ────────────────────────────────────────────────────────

func TestGetDefeatedBossIDs_HappyPath(t *testing.T) {
	db := &queryDB{qrows: &stringRows{data: []string{"the_atom", "the_bonder"}}}
	s := newMasteryStore(t, db)
	ids, err := s.GetDefeatedBossIDs(context.Background(), "user-1")
	if err != nil || len(ids) != 2 {
		t.Errorf("GetDefeatedBossIDs: err=%v ids=%v", err, ids)
	}
}

func TestGetDefeatedBossIDs_QueryError_Propagated(t *testing.T) {
	dbErr := errors.New("query failed")
	db := &queryDB{queryErr: dbErr}
	s := newMasteryStore(t, db)
	if _, err := s.GetDefeatedBossIDs(context.Background(), "u"); !errors.Is(err, dbErr) {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

// ── GetAchievements ───────────────────────────────────────────────────────────

func TestGetAchievements_HappyPath(t *testing.T) {
	now := time.Now()
	db := &achievementQueryDB{rows: &achievementRows{data: [][]any{
		{"ach-1", "user-1", "first_win", "First Victory", now},
		{"ach-2", "user-1", "streak_7", "Week Warrior", now},
	}}}
	s := newMasteryStore(t, db)
	got, err := s.GetAchievements(context.Background(), "user-1")
	if err != nil || len(got) != 2 {
		t.Errorf("GetAchievements: err=%v len=%d", err, len(got))
	}
}

func TestGetAchievements_Empty_ReturnsEmptySlice(t *testing.T) {
	db := &achievementQueryDB{rows: &achievementRows{data: nil}}
	s := newMasteryStore(t, db)
	got, err := s.GetAchievements(context.Background(), "fresh")
	if err != nil {
		t.Fatalf("GetAchievements: %v", err)
	}
	if got == nil {
		t.Error("expected non-nil empty slice")
	}
}

func TestGetAchievements_QueryError_Propagated(t *testing.T) {
	dbErr := errors.New("query failed")
	db := &achievementQueryDB{queryErr: dbErr}
	s := newMasteryStore(t, db)
	if _, err := s.GetAchievements(context.Background(), "u"); !errors.Is(err, dbErr) {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

// ── GrantAchievement ──────────────────────────────────────────────────────────

func TestGrantAchievement_NewAchievement_ReturnsTrue(t *testing.T) {
	db := &grantAchievementDB{mainRow: achievementScanRow()}
	s := newMasteryStore(t, db)
	a, newly, err := s.GrantAchievement(context.Background(), "user-1", "first_win", "First Victory")
	if err != nil || !newly || a == nil {
		t.Errorf("GrantAchievement: err=%v newly=%v a=%v", err, newly, a)
	}
}

func TestGrantAchievement_Duplicate_FetchesExisting(t *testing.T) {
	db := &grantAchievementDB{
		mainRow:  errFuncRow(errors.New("no rows in result set")),
		fetchRow: achievementScanRow(),
	}
	s := newMasteryStore(t, db)
	a, newly, err := s.GrantAchievement(context.Background(), "user-1", "first_win", "First Victory")
	if err != nil || newly || a == nil {
		t.Errorf("GrantAchievement duplicate: err=%v newly=%v a=%v", err, newly, a)
	}
}

func TestGrantAchievement_DBError_Propagated(t *testing.T) {
	db := &grantAchievementDB{mainRow: errFuncRow(errors.New("constraint"))}
	s := newMasteryStore(t, db)
	if _, _, err := s.GrantAchievement(context.Background(), "u", "x", "y"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ── AddCosmeticItem ───────────────────────────────────────────────────────────

func TestAddCosmeticItem_HappyPath(t *testing.T) {
	db := &execDB{}
	s := newMasteryStore(t, db)
	if err := s.AddCosmeticItem(context.Background(), "u", "frame", "gold"); err != nil {
		t.Fatalf("AddCosmeticItem: %v", err)
	}
	if db.execCalls != 1 {
		t.Errorf("expected 1 Exec call, got %d", db.execCalls)
	}
}

func TestAddCosmeticItem_DBError_Propagated(t *testing.T) {
	dbErr := errors.New("exec failed")
	db := &execDB{execErr: dbErr}
	s := newMasteryStore(t, db)
	if err := s.AddCosmeticItem(context.Background(), "u", "k", "v"); !errors.Is(err, dbErr) {
		t.Errorf("expected wrapped error, got %v", err)
	}
}

// ── BuyPowerUp ────────────────────────────────────────────────────────────────

func TestBuyPowerUp_HappyPath(t *testing.T) {
	inv, _ := json.Marshal(map[string]int{"shield": 2})
	db := &rowQueueDB{rows: []pgx.Row{&funcRow{fn: func(dest ...any) error {
		*(dest[0].(*int)) = 80
		*(dest[1].(*[]byte)) = inv
		return nil
	}}}}
	s := newMasteryStore(t, db)
	gemsLeft, count, err := s.BuyPowerUp(context.Background(), "u", "shield", 20)
	if err != nil || gemsLeft != 80 || count != 2 {
		t.Errorf("BuyPowerUp: err=%v gems=%d count=%d", err, gemsLeft, count)
	}
}

func TestBuyPowerUp_ErrNoRows_ReturnsErrNoGems(t *testing.T) {
	s := newMasteryStore(t, &rowQueueDB{rows: []pgx.Row{errFuncRow(pgx.ErrNoRows)}})
	if _, _, err := s.BuyPowerUp(context.Background(), "u", "shield", 9999); err == nil {
		t.Fatal("expected ErrNoGems, got nil")
	}
}

func TestBuyPowerUp_DBError_Propagated(t *testing.T) {
	dbErr := errors.New("db error")
	s := newMasteryStore(t, &rowQueueDB{rows: []pgx.Row{errFuncRow(dbErr)}})
	if _, _, err := s.BuyPowerUp(context.Background(), "u", "shield", 10); !errors.Is(err, dbErr) {
		t.Errorf("expected wrapped error, got %v", err)
	}
}
