package handlers

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/hub"
)

// NotifyHandler handles internal notification endpoints called by peer services
// such as tutoring-service and gaming-service.
type NotifyHandler struct {
	hub    *hub.Hub
	logger *zap.Logger
}

// NewNotifyHandler returns a configured NotifyHandler.
//
// hub is the broadcast registry used to push events to connected WebSocket
// clients; logger records delivery activity.
func NewNotifyHandler(h *hub.Hub, logger *zap.Logger) *NotifyHandler {
	return &NotifyHandler{hub: h, logger: logger}
}

// bossUnlockRequest is the payload for POST /internal/notify/boss-unlock.
type bossUnlockRequest struct {
	BossID    string `json:"boss_id"`
	ChapterID string `json:"chapter_id"`
	MemberID  string `json:"member_id"`
}

// BossUnlock handles POST /internal/notify/boss-unlock.
//
// Called by tutoring-service or gaming-service when a boss is unlocked for a
// member. Broadcasts a boss_unlocked WebSocket event to all active sessions
// registered for that member. Returns 204 No Content — callers must not block
// on delivery confirmation, as WebSocket delivery is best-effort.
func (h *NotifyHandler) BossUnlock(w http.ResponseWriter, r *http.Request) {
	var req bossUnlockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.MemberID == "" || req.BossID == "" || req.ChapterID == "" {
		http.Error(w, "member_id, boss_id, and chapter_id are required", http.StatusBadRequest)
		return
	}

	payload := hub.BossUnlockPayload{
		Event:     "boss_unlocked",
		BossID:    req.BossID,
		ChapterID: req.ChapterID,
		MemberID:  req.MemberID,
	}

	h.hub.Broadcast(req.MemberID, payload)
	h.logger.Info("boss_unlocked broadcast",
		zap.String("member_id", req.MemberID),
		zap.String("boss_id", req.BossID),
		zap.String("chapter_id", req.ChapterID),
	)

	w.WriteHeader(http.StatusNoContent)
}
