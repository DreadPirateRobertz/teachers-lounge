package handler_test

// Handler-layer tests for POST /gaming/streak/freeze (tl-2n5). Store-side
// coverage lives in internal/store/streak_freeze_test.go.
//
// Uses a small freezeStore fake that embeds noopStore and overrides only
// CreateStreakFreeze. Tests cover the three response branches:
//   - happy path        → 200 with new gem balance + expiry
//   - not enough gems   → 422 ErrNoGems
//   - already frozen    → 422 ErrAlreadyFrozen
//   - unauthenticated   → 403
//   - store hard error  → 500

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/handler"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/store"
	"github.com/teacherslounge/gaming-service/internal/taunt"
)

// freezeStore overrides CreateStreakFreeze to return canned results so we
// exercise each branch of the FreezeStreak handler.
type freezeStore struct {
	noopStore

	gemsLeft  int
	expiresAt time.Time
	err       error

	callCount   int
	lastUserID  string
	lastGemCost int
}

func (s *freezeStore) CreateStreakFreeze(_ context.Context, userID string, gemCost int) (int, time.Time, error) {
	s.callCount++
	s.lastUserID = userID
	s.lastGemCost = gemCost
	return s.gemsLeft, s.expiresAt, s.err
}

func newFreezeHandler(s *freezeStore) *handler.Handler {
	return handler.New(s, taunt.StaticGenerator{}, zap.NewNop())
}

func freezeRequest() *http.Request {
	return httptest.NewRequest(http.MethodPost, "/gaming/streak/freeze", nil)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestFreezeStreak_HappyPath_Returns200WithBalanceAndExpiry(t *testing.T) {
	expires := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	s := &freezeStore{gemsLeft: 150, expiresAt: expires}
	h := newFreezeHandler(s)

	rr := httptest.NewRecorder()
	h.FreezeStreak(rr, withUser(freezeRequest(), "u1"))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp model.StreakFreezeResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if resp.GemsLeft != 150 {
		t.Errorf("GemsLeft: want 150, got %d", resp.GemsLeft)
	}
	if !resp.ExpiresAt.Equal(expires) {
		t.Errorf("ExpiresAt: want %s, got %s", expires, resp.ExpiresAt)
	}
	if s.callCount != 1 {
		t.Errorf("CreateStreakFreeze calls: want 1, got %d", s.callCount)
	}
	if s.lastUserID != "u1" {
		t.Errorf("lastUserID: want u1, got %q", s.lastUserID)
	}
	if s.lastGemCost != model.StreakFreezeCost {
		t.Errorf("lastGemCost: want %d, got %d", model.StreakFreezeCost, s.lastGemCost)
	}
}

func TestFreezeStreak_NoCaller_Returns403(t *testing.T) {
	s := &freezeStore{}
	h := newFreezeHandler(s)

	rr := httptest.NewRecorder()
	// No WithUserID on the request → unauthenticated caller.
	h.FreezeStreak(rr, freezeRequest())

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
	if s.callCount != 0 {
		t.Errorf("store must not be called when caller is unauthenticated; got %d calls", s.callCount)
	}
}

func TestFreezeStreak_NotEnoughGems_Returns422(t *testing.T) {
	s := &freezeStore{err: store.ErrNoGems}
	h := newFreezeHandler(s)

	rr := httptest.NewRecorder()
	h.FreezeStreak(rr, withUser(freezeRequest(), "u1"))

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rr.Code)
	}
	var body map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&body)
	if body["error"] != "not enough gems" {
		t.Errorf("error message: want 'not enough gems', got %q", body["error"])
	}
}

func TestFreezeStreak_AlreadyFrozen_Returns422(t *testing.T) {
	s := &freezeStore{err: store.ErrAlreadyFrozen}
	h := newFreezeHandler(s)

	rr := httptest.NewRecorder()
	h.FreezeStreak(rr, withUser(freezeRequest(), "u1"))

	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rr.Code)
	}
	var body map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&body)
	if body["error"] != "streak already frozen" {
		t.Errorf("error message: want 'streak already frozen', got %q", body["error"])
	}
}

func TestFreezeStreak_StoreError_Returns500(t *testing.T) {
	s := &freezeStore{err: errors.New("db exploded")}
	h := newFreezeHandler(s)

	rr := httptest.NewRecorder()
	h.FreezeStreak(rr, withUser(freezeRequest(), "u1"))

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}
