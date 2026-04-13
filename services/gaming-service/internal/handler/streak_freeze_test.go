package handler_test

// Tests for StreakFreeze handler (tl-2n5).
//
// Uses a freezeStore stub that embeds noopStore and overrides only
// CreateStreakFreeze and IsStreakFrozen so each test case stays focused.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/handler"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/store"
	"github.com/teacherslounge/gaming-service/internal/taunt"
)

// ── freezeStore stub ──────────────────────────────────────────────────────────

type freezeStore struct {
	noopStore
	freezeFn func(ctx context.Context, userID string) (int, error)
	frozenFn func(ctx context.Context, userID string) (bool, error)
}

func (s *freezeStore) CreateStreakFreeze(ctx context.Context, userID string) (int, error) {
	if s.freezeFn != nil {
		return s.freezeFn(ctx, userID)
	}
	return 80, nil
}

func (s *freezeStore) IsStreakFrozen(ctx context.Context, userID string) (bool, error) {
	if s.frozenFn != nil {
		return s.frozenFn(ctx, userID)
	}
	return false, nil
}

func newFreezeHandler(s handler.Storer) *handler.Handler {
	return handler.New(s, taunt.StaticGenerator{}, zap.NewNop())
}

func freezeBody(userID string) *bytes.Buffer {
	b, _ := json.Marshal(model.StreakFreezeRequest{UserID: userID})
	return bytes.NewBuffer(b)
}

// ── tests ─────────────────────────────────────────────────────────────────────

// TestStreakFreeze_Success verifies a successful purchase returns 200 with
// active=true and the updated gem balance.
func TestStreakFreeze_Success(t *testing.T) {
	st := &freezeStore{
		freezeFn: func(_ context.Context, _ string) (int, error) { return 75, nil },
	}
	h := newFreezeHandler(st)

	req := httptest.NewRequest(http.MethodPost, "/gaming/streak/freeze", freezeBody("u1"))
	req = withUser(req, "u1")
	rec := httptest.NewRecorder()

	h.StreakFreeze(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp model.StreakFreezeResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Active {
		t.Error("expected active=true")
	}
	if resp.GemsLeft != 75 {
		t.Errorf("expected gems_left=75, got %d", resp.GemsLeft)
	}
}

// TestStreakFreeze_InsufficientCoins verifies 422 when the user cannot afford
// the freeze (store returns ErrInsufficientCoins).
func TestStreakFreeze_InsufficientCoins(t *testing.T) {
	st := &freezeStore{
		freezeFn: func(_ context.Context, _ string) (int, error) { return 0, store.ErrInsufficientCoins },
	}
	h := newFreezeHandler(st)

	req := httptest.NewRequest(http.MethodPost, "/gaming/streak/freeze", freezeBody("u1"))
	req = withUser(req, "u1")
	rec := httptest.NewRecorder()

	h.StreakFreeze(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rec.Code)
	}
}

// TestStreakFreeze_StoreError verifies 500 on an unexpected store error.
func TestStreakFreeze_StoreError(t *testing.T) {
	st := &freezeStore{
		freezeFn: func(_ context.Context, _ string) (int, error) { return 0, errors.New("db timeout") },
	}
	h := newFreezeHandler(st)

	req := httptest.NewRequest(http.MethodPost, "/gaming/streak/freeze", freezeBody("u1"))
	req = withUser(req, "u1")
	rec := httptest.NewRecorder()

	h.StreakFreeze(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

// TestStreakFreeze_Forbidden_WrongUser verifies 403 when the caller's JWT does
// not match the user_id in the request body.
func TestStreakFreeze_Forbidden_WrongUser(t *testing.T) {
	h := newFreezeHandler(&freezeStore{})

	req := httptest.NewRequest(http.MethodPost, "/gaming/streak/freeze", freezeBody("user-a"))
	req = withUser(req, "user-b") // mismatch
	rec := httptest.NewRecorder()

	h.StreakFreeze(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

// TestStreakFreeze_Forbidden_NoAuth verifies 403 when no JWT is present.
func TestStreakFreeze_Forbidden_NoAuth(t *testing.T) {
	h := newFreezeHandler(&freezeStore{})

	req := httptest.NewRequest(http.MethodPost, "/gaming/streak/freeze", freezeBody("u1"))
	// no withUser → empty context
	rec := httptest.NewRecorder()

	h.StreakFreeze(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

// TestStreakFreeze_InvalidBody verifies 400 for malformed JSON.
func TestStreakFreeze_InvalidBody(t *testing.T) {
	h := newFreezeHandler(&freezeStore{})

	req := httptest.NewRequest(http.MethodPost, "/gaming/streak/freeze", bytes.NewBufferString("{not-json"))
	req = withUser(req, "u1")
	rec := httptest.NewRecorder()

	h.StreakFreeze(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// TestStreakFreeze_MissingUserID verifies 400 when user_id is empty.
func TestStreakFreeze_MissingUserID(t *testing.T) {
	h := newFreezeHandler(&freezeStore{})

	req := httptest.NewRequest(http.MethodPost, "/gaming/streak/freeze", freezeBody(""))
	req = withUser(req, "u1")
	rec := httptest.NewRecorder()

	h.StreakFreeze(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}
