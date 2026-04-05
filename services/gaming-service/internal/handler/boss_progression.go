package handler

import (
	"net/http"
	"sort"

	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/boss"
	"github.com/teacherslounge/gaming-service/internal/middleware"
)

// BossProgressionNode is the API shape for a single boss node on the progression map.
type BossProgressionNode struct {
	// BossID is the canonical boss identifier (e.g. "the_atom").
	BossID string `json:"boss_id"`
	// Name is the display name of the boss.
	Name string `json:"name"`
	// Topic is the chemistry topic this boss guards.
	Topic string `json:"topic"`
	// Tier is the difficulty tier (1–6). Bosses are ordered by ascending tier.
	Tier int `json:"tier"`
	// VictoryXP is the XP reward for defeating this boss.
	VictoryXP int `json:"victory_xp"`
	// PrimaryColor is the dominant neon color for this boss's node.
	PrimaryColor string `json:"primary_color"`
	// State describes whether this boss is defeated, currently available, or locked.
	// Values: "defeated" | "current" | "locked"
	State string `json:"state"`
}

// BossProgressionResponse is the full response for GET /gaming/boss/progression.
type BossProgressionResponse struct {
	// Nodes is the ordered list of boss progression nodes, sorted by ascending tier.
	Nodes []BossProgressionNode `json:"nodes"`
	// TotalDefeated is the number of bosses the authenticated user has beaten.
	TotalDefeated int `json:"total_defeated"`
}

// GetBossProgression handles GET /gaming/boss/progression.
//
// Returns the full boss trail for the authenticated user. Each node reports
// whether the boss is defeated, currently available to fight, or still locked
// behind undefeated prerequisites.
//
// Lock logic (boss tiers 1–6):
//   - Tier 1 is always available (current or defeated).
//   - Tier N is "current" when tier N-1 is defeated and tier N is not yet beaten.
//   - Tier N is "locked" when any required predecessor is not yet defeated.
//   - The final boss (tier 6) is unlocked only after all tier 1–5 bosses are defeated.
//   - If all bosses are defeated every node has state "defeated".
func (h *Handler) GetBossProgression(w http.ResponseWriter, r *http.Request) {
	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	defeatedIDs, err := h.store.GetDefeatedBossIDs(r.Context(), callerID)
	if err != nil {
		h.logger.Error("get boss progression: query defeated bosses",
			zap.String("user_id", callerID),
			zap.Error(err),
		)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	defeated := make(map[string]bool, len(defeatedIDs))
	for _, id := range defeatedIDs {
		defeated[id] = true
	}

	// Sort catalog by ascending tier for consistent trail ordering.
	sorted := make([]*boss.Def, 0, len(boss.Catalog))
	for _, def := range boss.Catalog {
		sorted = append(sorted, def)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Tier < sorted[j].Tier
	})

	nodes := make([]BossProgressionNode, 0, len(sorted))
	allPriorDefeated := true // tracks whether all bosses before current index are defeated

	for _, def := range sorted {
		state := bossNodeState(def, defeated, allPriorDefeated)
		if !defeated[def.ID] {
			// Once we hit an undefeated boss, everything after is locked.
			allPriorDefeated = false
		}

		nodes = append(nodes, BossProgressionNode{
			BossID:       def.ID,
			Name:         def.Name,
			Topic:        def.Topic,
			Tier:         def.Tier,
			VictoryXP:    def.VictoryXP,
			PrimaryColor: def.Visual.PrimaryColor,
			State:        state,
		})
	}

	writeJSON(w, http.StatusOK, BossProgressionResponse{
		Nodes:         nodes,
		TotalDefeated: len(defeatedIDs),
	})
}

// bossNodeState computes the progression state for a single boss node.
//
// A boss is "defeated" if the user has a victory in their history.
// A boss is "current" if all prior bosses are defeated but this one is not.
// A boss is "locked" if any required predecessor is not yet defeated.
func bossNodeState(def *boss.Def, defeated map[string]bool, allPriorDefeated bool) string {
	if defeated[def.ID] {
		return "defeated"
	}
	if allPriorDefeated {
		return "current"
	}
	return "locked"
}
