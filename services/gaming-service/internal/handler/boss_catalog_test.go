package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/handler"
)

func setupBossCatalogRouter() http.Handler {
	h := handler.New(nil, nil, zap.NewNop())
	r := chi.NewRouter()
	r.Get("/gaming/boss/catalog", h.GetBossCatalog)
	r.Get("/gaming/boss/catalog/{bossId}", h.GetBossByID)
	return r
}

func TestGetBossCatalog_ReturnsAllBosses(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/gaming/boss/catalog", nil)
	w := httptest.NewRecorder()
	setupBossCatalogRouter().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var entries []handler.BossCatalogEntry
	if err := json.NewDecoder(w.Body).Decode(&entries); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(entries) != 6 {
		t.Errorf("expected 6 bosses, got %d", len(entries))
	}

	// Verify order: tier 1 first, tier 6 last.
	if entries[0].ID != "the_atom" {
		t.Errorf("first boss should be the_atom, got %s", entries[0].ID)
	}
	if entries[5].ID != "final_boss" {
		t.Errorf("last boss should be final_boss, got %s", entries[5].ID)
	}
}

func TestGetBossCatalog_IncludesVisualConfig(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/gaming/boss/catalog", nil)
	w := httptest.NewRecorder()
	setupBossCatalogRouter().ServeHTTP(w, r)

	var entries []handler.BossCatalogEntry
	json.NewDecoder(w.Body).Decode(&entries)

	for _, e := range entries {
		if e.Visual.PrimaryColor == "" {
			t.Errorf("boss %s missing visual.primary_color", e.ID)
		}
		if e.Visual.Geometry == "" {
			t.Errorf("boss %s missing visual.geometry", e.ID)
		}
		if len(e.Visual.TauntPool) == 0 {
			t.Errorf("boss %s missing visual.taunt_pool", e.ID)
		}
	}
}

func TestGetBossByID_Known(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/gaming/boss/catalog/the_atom", nil)
	w := httptest.NewRecorder()
	setupBossCatalogRouter().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var entry handler.BossCatalogEntry
	if err := json.NewDecoder(w.Body).Decode(&entry); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if entry.ID != "the_atom" {
		t.Errorf("expected id=the_atom, got %s", entry.ID)
	}
	if entry.Name != "THE ATOM" {
		t.Errorf("expected name=THE ATOM, got %s", entry.Name)
	}
	if entry.Visual.Geometry != "atom" {
		t.Errorf("expected geometry=atom, got %s", entry.Visual.Geometry)
	}
}

func TestGetBossByID_NotFound(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/gaming/boss/catalog/not_a_boss", nil)
	w := httptest.NewRecorder()
	setupBossCatalogRouter().ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetBossByID_FinalBoss(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/gaming/boss/catalog/final_boss", nil)
	w := httptest.NewRecorder()
	setupBossCatalogRouter().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var entry handler.BossCatalogEntry
	json.NewDecoder(w.Body).Decode(&entry)
	if entry.Tier != 6 {
		t.Errorf("final boss should be tier 6, got %d", entry.Tier)
	}
	if entry.MaxRounds != 10 {
		t.Errorf("final boss should have 10 rounds, got %d", entry.MaxRounds)
	}
}
