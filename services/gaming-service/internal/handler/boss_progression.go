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
	// ChapterMastery is the user's average mastery score [0.0, 1.0] across this
	// boss's chapter concepts. Drives the "X% to unlock" progress bar on the
	// trail UI. 0.0 for users with no mastery recorded for the chapter.
	ChapterMastery float64 `json:"chapter_mastery"`
	// MasteryThreshold is the score at which the boss unlocks. Copied from the
	// package constant so the client doesn't have to hard-code it.
	MasteryThreshold float64 `json:"mastery_threshold"`
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
// behind a mastery threshold.
//
// Unlock logic (per boss):
//   - "defeated" if the user has a victory recorded for this boss.
//   - "current" if the user has not defeated this boss AND either the boss is
//     flagged IsFirstBoss (onboarding) OR the user's average mastery across
//     the chapter's concepts ≥ boss.MasteryUnlockThreshold (60%).
//   - "locked" otherwise.
//
// Each node also exposes the current chapter mastery score so the frontend
// can render a "X% to unlock" progress bar on locked nodes.
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
	sorted = append(sorted, boss.Catalog...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Tier < sorted[j].Tier
	})

	nodes := make([]BossProgressionNode, 0, len(sorted))
	for _, def := range sorted {
		mastery, err := h.store.GetChapterMastery(r.Context(), callerID, def.ChapterConceptPaths)
		if err != nil {
			h.logger.Error("get boss progression: query chapter mastery",
				zap.String("user_id", callerID),
				zap.String("boss_id", def.ID),
				zap.Error(err),
			)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		nodes = append(nodes, BossProgressionNode{
			BossID:           def.ID,
			Name:             def.Name,
			Topic:            def.Topic,
			Tier:             def.Tier,
			VictoryXP:        def.VictoryXP,
			PrimaryColor:     def.Visual.PrimaryColor,
			State:            bossNodeState(def, defeated, mastery),
			ChapterMastery:   mastery,
			MasteryThreshold: boss.MasteryUnlockThreshold,
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
// A boss is "current" if the user has not defeated it AND either the boss is
// the onboarding boss (IsFirstBoss) OR the user's chapter mastery has reached
// the unlock threshold (boss.MasteryUnlockThreshold).
// Otherwise it is "locked".
func bossNodeState(def *boss.Def, defeated map[string]bool, chapterMastery float64) string {
	if defeated[def.ID] {
		return "defeated"
	}
	if def.IsFirstBoss {
		return "current"
	}
	if chapterMastery >= boss.MasteryUnlockThreshold {
		return "current"
	}
	return "locked"
}
