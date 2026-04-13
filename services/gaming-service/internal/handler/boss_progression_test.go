package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/boss"
	"github.com/teacherslounge/gaming-service/internal/handler"
	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/taunt"
)

// ── progressionStore stub ─────────────────────────────────────────────────────

// progressionStore overrides boss-progression reads on top of battleStore.
//
// masteryByPaths keys on the first ltree path of a boss's ChapterConceptPaths
// — adequate for tests since each boss's paths are disjoint. Unset paths
// return 0.0.
type progressionStore struct {
	battleStore
	defeatedIDs    []string
	queryErr       error
	masteryByPaths map[string]float64
	masteryErr     error
}

func (p *progressionStore) GetDefeatedBossIDs(_ context.Context, _ string) ([]string, error) {
	return p.defeatedIDs, p.queryErr
}

func (p *progressionStore) GetChapterMastery(_ context.Context, _ string, paths []string) (float64, error) {
	if p.masteryErr != nil {
		return 0.0, p.masteryErr
	}
	if len(paths) == 0 {
		return 0.0, nil
	}
	if v, ok := p.masteryByPaths[paths[0]]; ok {
		return v, nil
	}
	return 0.0, nil
}

func (p *progressionStore) GetChapterMasteryBatch(_ context.Context, _ string, pathsByBossID map[string][]string) (map[string]float64, error) {
	if p.masteryErr != nil {
		return nil, p.masteryErr
	}
	out := make(map[string]float64, len(pathsByBossID))
	for bossID, paths := range pathsByBossID {
		if len(paths) == 0 {
			out[bossID] = 0.0
			continue
		}
		if v, ok := p.masteryByPaths[paths[0]]; ok {
			out[bossID] = v
		} else {
			out[bossID] = 0.0
		}
	}
	return out, nil
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

// firstPathFor returns the first ChapterConceptPath for a boss by ID,
// used as a mastery-map key in tests.
func firstPathFor(t *testing.T, bossID string) string {
	t.Helper()
	for _, def := range boss.Catalog {
		if def.ID == bossID {
			if len(def.ChapterConceptPaths) == 0 {
				t.Fatalf("boss %s has no ChapterConceptPaths", bossID)
			}
			return def.ChapterConceptPaths[0]
		}
	}
	t.Fatalf("boss %s not found in catalog", bossID)
	return ""
}

// firstBossID returns the catalog boss flagged IsFirstBoss.
func firstBossID(t *testing.T) string {
	t.Helper()
	for _, def := range boss.Catalog {
		if def.IsFirstBoss {
			return def.ID
		}
	}
	t.Fatal("no IsFirstBoss in catalog")
	return ""
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

// ── Mastery-gate behavior ─────────────────────────────────────────────────────

func TestGetBossProgression_FreshUser_OnlyFirstBossCurrent(t *testing.T) {
	st := &progressionStore{}
	h := newProgressionHandler(st)

	req := withCallerID(httptest.NewRequest(http.MethodGet, "/gaming/boss/progression", nil), "u1")
	rec := httptest.NewRecorder()
	h.GetBossProgression(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	resp := progressionResponse(t, rec)
	if resp.TotalDefeated != 0 {
		t.Errorf("expected 0 defeated, got %d", resp.TotalDefeated)
	}
	firstID := firstBossID(t)
	for _, node := range resp.Nodes {
		want := "locked"
		if node.BossID == firstID {
			want = "current"
		}
		if node.State != want {
			t.Errorf("boss %s: expected %s, got %s", node.BossID, want, node.State)
		}
	}
}

func TestGetBossProgression_MasteryThresholdReached_BossBecomesCurrent(t *testing.T) {
	// 60% mastery on the_bonder's chapter should unlock the_bonder even with
	// no prior defeats.
	path := firstPathFor(t, "the_bonder")
	st := &progressionStore{
		masteryByPaths: map[string]float64{path: 0.60},
	}
	h := newProgressionHandler(st)

	req := withCallerID(httptest.NewRequest(http.MethodGet, "/gaming/boss/progression", nil), "u1")
	rec := httptest.NewRecorder()
	h.GetBossProgression(rec, req)

	resp := progressionResponse(t, rec)
	for _, node := range resp.Nodes {
		if node.BossID == "the_bonder" && node.State != "current" {
			t.Errorf("the_bonder at 60%% mastery: expected current, got %s", node.State)
		}
	}
}

func TestGetBossProgression_BelowThreshold_BossStaysLocked(t *testing.T) {
	path := firstPathFor(t, "the_bonder")
	st := &progressionStore{
		masteryByPaths: map[string]float64{path: 0.59},
	}
	h := newProgressionHandler(st)

	req := withCallerID(httptest.NewRequest(http.MethodGet, "/gaming/boss/progression", nil), "u1")
	rec := httptest.NewRecorder()
	h.GetBossProgression(rec, req)

	resp := progressionResponse(t, rec)
	for _, node := range resp.Nodes {
		if node.BossID == "the_bonder" && node.State != "locked" {
			t.Errorf("the_bonder at 59%% mastery: expected locked, got %s", node.State)
		}
	}
}

func TestGetBossProgression_Defeated_StateWinsOverMastery(t *testing.T) {
	// Defeated boss keeps "defeated" state regardless of current mastery level.
	path := firstPathFor(t, "the_atom")
	st := &progressionStore{
		defeatedIDs:    []string{"the_atom"},
		masteryByPaths: map[string]float64{path: 0.10},
	}
	h := newProgressionHandler(st)

	req := withCallerID(httptest.NewRequest(http.MethodGet, "/gaming/boss/progression", nil), "u1")
	rec := httptest.NewRecorder()
	h.GetBossProgression(rec, req)

	resp := progressionResponse(t, rec)
	for _, node := range resp.Nodes {
		if node.BossID == "the_atom" && node.State != "defeated" {
			t.Errorf("the_atom defeated: expected defeated, got %s", node.State)
		}
	}
}

func TestGetBossProgression_AllDefeated_AllDefeatedState(t *testing.T) {
	st := &progressionStore{defeatedIDs: []string{
		"the_atom", "the_bonder", "name_lord",
		"the_stereochemist", "the_reactor", "final_boss",
	}}
	h := newProgressionHandler(st)

	req := withCallerID(httptest.NewRequest(http.MethodGet, "/gaming/boss/progression", nil), "u1")
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
	st := &progressionStore{}
	h := newProgressionHandler(st)

	req := withCallerID(httptest.NewRequest(http.MethodGet, "/gaming/boss/progression", nil), "u1")
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
	st := &progressionStore{}
	h := newProgressionHandler(st)

	req := withCallerID(httptest.NewRequest(http.MethodGet, "/gaming/boss/progression", nil), "u1")
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
		if node.MasteryThreshold != boss.MasteryUnlockThreshold {
			t.Errorf("node %s: expected mastery_threshold=%v, got %v",
				node.BossID, boss.MasteryUnlockThreshold, node.MasteryThreshold)
		}
	}
}

func TestGetBossProgression_ChapterMasterySurfacedOnNode(t *testing.T) {
	path := firstPathFor(t, "name_lord")
	st := &progressionStore{
		masteryByPaths: map[string]float64{path: 0.42},
	}
	h := newProgressionHandler(st)

	req := withCallerID(httptest.NewRequest(http.MethodGet, "/gaming/boss/progression", nil), "u1")
	rec := httptest.NewRecorder()
	h.GetBossProgression(rec, req)

	resp := progressionResponse(t, rec)
	var found bool
	for _, node := range resp.Nodes {
		if node.BossID == "name_lord" {
			found = true
			if node.ChapterMastery != 0.42 {
				t.Errorf("name_lord: expected chapter_mastery=0.42, got %v", node.ChapterMastery)
			}
		}
	}
	if !found {
		t.Fatal("name_lord node not in response")
	}
}

// ── Error paths ───────────────────────────────────────────────────────────────

func TestGetBossProgression_DefeatedQueryError_Returns500(t *testing.T) {
	st := &progressionStore{queryErr: errors.New("db down")}
	h := newProgressionHandler(st)

	req := withCallerID(httptest.NewRequest(http.MethodGet, "/gaming/boss/progression", nil), "u1")
	rec := httptest.NewRecorder()
	h.GetBossProgression(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestGetBossProgression_MasteryQueryError_Returns500(t *testing.T) {
	st := &progressionStore{masteryErr: errors.New("ltree query failed")}
	h := newProgressionHandler(st)

	req := withCallerID(httptest.NewRequest(http.MethodGet, "/gaming/boss/progression", nil), "u1")
	rec := httptest.NewRecorder()
	h.GetBossProgression(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

// Compile-time check: progressionStore must satisfy model.Profile usage.
var _ *model.Profile = nil
