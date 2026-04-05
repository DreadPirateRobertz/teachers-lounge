package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/teacherslounge/gaming-service/internal/boss"
)

// BossCatalogEntry is the API shape for a single boss definition.
type BossCatalogEntry struct {
	ID                string           `json:"id"`
	Name              string           `json:"name"`
	Topic             string           `json:"topic"`
	Tier              int              `json:"tier"`
	MaxRounds         int              `json:"max_rounds"`
	VictoryXP         int              `json:"victory_xp"`
	Taunt             string           `json:"taunt"`
	Visual            BossVisualConfig `json:"visual"`
}

// BossVisualConfig is the visual/animation metadata served to the frontend.
type BossVisualConfig struct {
	PrimaryColor      string   `json:"primary_color"`
	SecondaryColor    string   `json:"secondary_color"`
	Geometry          string   `json:"geometry"`
	TauntPool         []string `json:"taunt_pool"`
	AttackDescription string   `json:"attack_description"`
	IdleDescription   string   `json:"idle_description"`
}

// GetBossCatalog handles GET /gaming/boss/catalog.
// Returns the ordered list of all boss definitions with visual metadata.
// This endpoint does not require authentication — the catalog is public.
func (h *Handler) GetBossCatalog(w http.ResponseWriter, r *http.Request) {
	entries := make([]BossCatalogEntry, 0, len(boss.Catalog))
	for _, def := range boss.Catalog {
		entries = append(entries, bossDefToEntry(def))
	}
	writeJSON(w, http.StatusOK, entries)
}

// GetBossByID handles GET /gaming/boss/catalog/{bossId}.
// Returns a single boss definition by ID with full visual metadata.
func (h *Handler) GetBossByID(w http.ResponseWriter, r *http.Request) {
	bossID := chi.URLParam(r, "bossId")
	def := boss.ByID(bossID)
	if def == nil {
		writeError(w, http.StatusNotFound, "boss not found")
		return
	}
	writeJSON(w, http.StatusOK, bossDefToEntry(def))
}

func bossDefToEntry(def *boss.Def) BossCatalogEntry {
	return BossCatalogEntry{
		ID:        def.ID,
		Name:      def.Name,
		Topic:     def.Topic,
		Tier:      def.Tier,
		MaxRounds: def.MaxRounds,
		VictoryXP: def.VictoryXP,
		Taunt:     def.Taunt,
		Visual: BossVisualConfig{
			PrimaryColor:      def.Visual.PrimaryColor,
			SecondaryColor:    def.Visual.SecondaryColor,
			Geometry:          def.Visual.Geometry,
			TauntPool:         def.Visual.TauntPool,
			AttackDescription: def.Visual.AttackDescription,
			IdleDescription:   def.Visual.IdleDescription,
		},
	}
}
