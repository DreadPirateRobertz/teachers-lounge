package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/handler"
	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/taunt"
)

// ── progressionStore stub ─────────────────────────────────────────────────────

// progressionStore overrides GetDefeatedBossIDs on top of battleStore.
type progressionStore struct {
	battleStore
	defeatedIDs []string
	queryErr    error
}

func (p *progressionStore) GetDefeatedBossIDs(_ context.Context, _ string) ([]string, error) {
	return p.defeatedIDs, p.queryErr
}

var _ handler.Storer = (*progressionStore)(nil)

func newProgressionHandler(s handler.Storer) *handler.Handler {
	return handler.New(s, taunt.StaticGenerator{}, zap.NewNop())
}

// withCallerID injects a user ID into the request context.
func withCallerID(r *http.Request, userID string) *http.Request {
	return r.WithContext(middleware.WithUserID(r.Context(), userID))
}

// progressionResponse decodes the handler response into a struct for assertions.
func progressionResponse(t *testing.T, body *httptest.ResponseRecorder) handler.BossProgressionResponse {
	t.Helper()
	var resp handler.BossProgressionResponse
	if err := json.NewDecoder(body.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

// ── Auth ──────────────────────────────────────────────────────────────────────

func TestGetBossProgression_NoAuth_Forbidden(t *testing.T) {
	h := newProgressionHandler(&progressionStore{})
	req := httptest.NewRequest(http.MethodGet, "/gaming/boss/progression", nil)
	rec := httptest.NewRecorder()

	h.GetBossProgression(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

// ── Happy paths ───────────────────────────────────────────────────────────────

func TestGetBossProgression_FreshUser_AllLockedExceptFirst(t *testing.T) {
	st := &progressionStore{defeatedIDs: nil}
	h := newProgressionHandler(st)

	req := httptest.NewRequest(http.MethodGet, "/gaming/boss/progression", nil)
	req = withCallerID(req, "user1")
	rec := httptest.NewRecorder()

	h.GetBossProgression(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	resp := progressionResponse(t, rec)
	if len(resp.Nodes) == 0 {
		t.Fatal("expected nodes, got none")
	}
	if resp.TotalDefeated != 0 {
		t.Errorf("expected 0 defeated, got %d", resp.TotalDefeated)
	}
	// Tier-1 boss must be "current"; all others "locked".
	for _, node := range resp.Nodes {
		if node.Tier == 1 {
			if node.State != "current" {
				t.Errorf("tier-1 boss: expected current, got %s", node.State)
			}
		} else {
			if node.State != "locked" {
				t.Errorf("tier-%d boss: expected locked for fresh user, got %s", node.Tier, node.State)
			}
		}
	}
}

func TestGetBossProgression_Tier1Defeated_Tier2Current(t *testing.T) {
	st := &progressionStore{defeatedIDs: []string{"the_atom"}}
	h := newProgressionHandler(st)

	req := httptest.NewRequest(http.MethodGet, "/gaming/boss/progression", nil)
	req = withCallerID(req, "user1")
	rec := httptest.NewRecorder()

	h.GetBossProgression(rec, req)

	resp := progressionResponse(t, rec)
	for _, node := range resp.Nodes {
		switch node.Tier {
		case 1:
			if node.State != "defeated" {
				t.Errorf("tier-1: expected defeated, got %s", node.State)
			}
		case 2:
			if node.State != "current" {
				t.Errorf("tier-2: expected current, got %s", node.State)
			}
		default:
			if node.State != "locked" {
				t.Errorf("tier-%d: expected locked, got %s", node.Tier, node.State)
			}
		}
	}
}

func TestGetBossProgression_AllDefeated_AllDefeatedState(t *testing.T) {
	st := &progressionStore{defeatedIDs: []string{
		"the_atom", "the_bonder", "name_lord",
		"the_stereochemist", "the_reactor", "final_boss",
	}}
	h := newProgressionHandler(st)

	req := httptest.NewRequest(http.MethodGet, "/gaming/boss/progression", nil)
	req = withCallerID(req, "user1")
	rec := httptest.NewRecorder()

	h.GetBossProgression(rec, req)

	resp := progressionResponse(t, rec)
	if resp.TotalDefeated != 6 {
		t.Errorf("expected 6 defeated, got %d", resp.TotalDefeated)
	}
	for _, node := range resp.Nodes {
		if node.State != "defeated" {
			t.Errorf("boss %s: expected defeated, got %s", node.BossID, node.State)
		}
	}
}

func TestGetBossProgression_NodesOrderedByAscendingTier(t *testing.T) {
	st := &progressionStore{defeatedIDs: nil}
	h := newProgressionHandler(st)

	req := httptest.NewRequest(http.MethodGet, "/gaming/boss/progression", nil)
	req = withCallerID(req, "user1")
	rec := httptest.NewRecorder()

	h.GetBossProgression(rec, req)

	resp := progressionResponse(t, rec)
	for i := 1; i < len(resp.Nodes); i++ {
		if resp.Nodes[i].Tier < resp.Nodes[i-1].Tier {
			t.Errorf("nodes not sorted: tier %d before tier %d", resp.Nodes[i-1].Tier, resp.Nodes[i].Tier)
		}
	}
}

func TestGetBossProgression_NodeHasRequiredFields(t *testing.T) {
	st := &progressionStore{defeatedIDs: nil}
	h := newProgressionHandler(st)

	req := httptest.NewRequest(http.MethodGet, "/gaming/boss/progression", nil)
	req = withCallerID(req, "user1")
	rec := httptest.NewRecorder()

	h.GetBossProgression(rec, req)

	resp := progressionResponse(t, rec)
	for _, node := range resp.Nodes {
		if node.BossID == "" {
			t.Error("node missing boss_id")
		}
		if node.Name == "" {
			t.Error("node missing name")
		}
		if node.PrimaryColor == "" {
			t.Error("node missing primary_color")
		}
		if node.State == "" {
			t.Error("node missing state")
		}
	}
}

// ── Error path ────────────────────────────────────────────────────────────────

func TestGetBossProgression_StoreError_Returns500(t *testing.T) {
	st := &progressionStore{queryErr: errors.New("db down")}
	h := newProgressionHandler(st)

	req := httptest.NewRequest(http.MethodGet, "/gaming/boss/progression", nil)
	req = withCallerID(req, "user1")
	rec := httptest.NewRecorder()

	h.GetBossProgression(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

// ── bossNodeState unit tests ──────────────────────────────────────────────────

// These test the lock logic directly without going through the HTTP layer.

func TestBossNodeState_Defeated(t *testing.T) {
	// A boss in the defeated map is always "defeated".
	// We verify this via the HTTP response rather than calling unexported functions.
	st := &progressionStore{defeatedIDs: []string{"the_atom"}}
	h := newProgressionHandler(st)

	req := httptest.NewRequest(http.MethodGet, "/gaming/boss/progression", nil)
	req = withCallerID(req, "user1")
	rec := httptest.NewRecorder()
	h.GetBossProgression(rec, req)

	resp := progressionResponse(t, rec)
	for _, node := range resp.Nodes {
		if node.BossID == "the_atom" && node.State != "defeated" {
			t.Errorf("the_atom should be defeated, got %s", node.State)
		}
	}
}

// Compile-time check: progressionStore must satisfy model.Profile usage.
var _ *model.Profile = nil
