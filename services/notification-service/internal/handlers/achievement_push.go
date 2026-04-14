package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/model"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/push"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/store"
)

// Push-response reasons returned in PushDispatchResponse.Reason when Skipped
// is true. These are contract-stable strings so callers can branch on them.
const (
	skipReasonDuplicate = "duplicate"
	skipReasonNoTokens  = "no_tokens"
)

// fanoutTimeout bounds the background FCM fan-out goroutine. FCM's own
// per-request timeout (10s in FCMPusher) applies per token; this outer
// ceiling ensures the goroutine cannot outlive request processing by more
// than ~30s even for users with many devices.
const fanoutTimeout = 30 * time.Second

// AchievementPushStore is the subset of store.Store the achievement-push
// handlers depend on. Defined here so tests can substitute an in-memory
// fake without a real Postgres pool.
type AchievementPushStore interface {
	GetPushTokens(ctx context.Context, userID string) ([]string, error)
	DeletePushToken(ctx context.Context, userID, token string) error
	MarkLevelUpNotified(ctx context.Context, userID string, dedupWindow time.Duration) (bool, error)
	MarkQuestCompleteNotified(ctx context.Context, userID string, dedupWindow time.Duration) (bool, error)
}

// AchievementPushHandler serves POST /internal/push/level-up and
// POST /internal/push/quest-complete — gaming-service triggers fired when
// a user crosses a level boundary or completes a daily quest.
//
// FCM fan-out is performed asynchronously in a goroutine so the HTTP caller
// (gaming-service) is not blocked on upstream FCM latency. The response
// reports only whether the dispatch was accepted or deduplicated.
//
// For deterministic unit testing, the handler exposes WaitForFanout which
// tests can call to block until all in-flight goroutines have finished.
type AchievementPushHandler struct {
	store  AchievementPushStore
	pusher push.Pusher
	logger *zap.Logger
	wg     sync.WaitGroup
}

// NewAchievementPushHandler returns a configured AchievementPushHandler.
// pusher delivers FCM notifications; pass push.LogPusher when FCM is not
// configured so the code path still exercises.
func NewAchievementPushHandler(s AchievementPushStore, pusher push.Pusher, logger *zap.Logger) *AchievementPushHandler {
	return &AchievementPushHandler{store: s, pusher: pusher, logger: logger}
}

// WaitForFanout blocks until every background FCM fan-out goroutine
// started by this handler has finished. Intended for tests and graceful
// shutdown; production callers do not need to invoke it.
func (h *AchievementPushHandler) WaitForFanout() {
	h.wg.Wait()
}

// LevelUp handles POST /internal/push/level-up.
//
// Request body: LevelUpRequest{user_id, new_level}. Responds 202 Accepted
// with PushDispatchResponse. Fan-out runs in a background goroutine.
func (h *AchievementPushHandler) LevelUp(w http.ResponseWriter, r *http.Request) {
	var req model.LevelUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.UserID == "" || req.NewLevel <= 0 {
		http.Error(w, "user_id and positive new_level are required", http.StatusBadRequest)
		return
	}

	title := "Level up!"
	body := fmt.Sprintf("You reached level %d.", req.NewLevel)
	h.dispatch(w, r, dispatchParams{
		userID:      req.UserID,
		title:       title,
		body:        body,
		dedupWindow: store.DedupTTLLevelUp,
		markFn:      h.store.MarkLevelUpNotified,
		event:       "level_up",
	})
}

// QuestComplete handles POST /internal/push/quest-complete.
//
// Request body: QuestCompleteRequest{user_id, quest_title, xp_reward}.
// Responds 202 Accepted with PushDispatchResponse.
func (h *AchievementPushHandler) QuestComplete(w http.ResponseWriter, r *http.Request) {
	var req model.QuestCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.UserID == "" || req.QuestTitle == "" {
		http.Error(w, "user_id and quest_title are required", http.StatusBadRequest)
		return
	}

	title := "Quest complete!"
	body := fmt.Sprintf("%s — +%d XP", req.QuestTitle, req.XPReward)
	h.dispatch(w, r, dispatchParams{
		userID:      req.UserID,
		title:       title,
		body:        body,
		dedupWindow: store.DedupTTLQuestComplete,
		markFn:      h.store.MarkQuestCompleteNotified,
		event:       "quest_complete",
	})
}

// dispatchParams carries per-event inputs to the shared dispatch helper.
type dispatchParams struct {
	userID, title, body, event string
	dedupWindow                time.Duration
	markFn                     func(ctx context.Context, userID string, dedupWindow time.Duration) (bool, error)
}

// dispatch is the shared handler path used by LevelUp and QuestComplete.
// It runs guards in the request goroutine (sync, response-affecting) and
// then hands off FCM fan-out to a background goroutine (async).
//
// Guard order — critical to avoid race conditions and wasted stamps:
//  1. GetPushTokens — early exit when user has no devices, preserves the
//     ability to notify on the next event once a token is registered.
//  2. Mark*Notified — atomic stamp-if-eligible. Stamping BEFORE fan-out
//     prevents duplicate pushes during FCM outages (every retry would stamp
//     a fresh window otherwise).
//  3. Fan-out goroutine — one push per registered token, with stale-token
//     purge on InvalidRegistration / NotRegistered.
func (h *AchievementPushHandler) dispatch(w http.ResponseWriter, r *http.Request, p dispatchParams) {
	ctx := r.Context()

	tokens, err := h.store.GetPushTokens(ctx, p.userID)
	if err != nil {
		h.logger.Error("achievement-push: get tokens",
			zap.String("event", p.event), zap.String("user_id", p.userID), zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if len(tokens) == 0 {
		writeDispatch(w, h.logger, p.event, model.PushDispatchResponse{
			Skipped: true, Reason: skipReasonNoTokens,
		})
		return
	}

	stamped, err := p.markFn(ctx, p.userID, p.dedupWindow)
	if err != nil {
		h.logger.Error("achievement-push: dedup stamp",
			zap.String("event", p.event), zap.String("user_id", p.userID), zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !stamped {
		writeDispatch(w, h.logger, p.event, model.PushDispatchResponse{
			Skipped: true, Reason: skipReasonDuplicate,
		})
		return
	}

	// Hand off FCM fan-out to a background goroutine so gaming-service does
	// not block on FCM latency. Use a fresh context detached from the
	// request so the goroutine is not cancelled when the response returns.
	h.wg.Add(1)
	go h.fanout(tokens, p)

	writeDispatch(w, h.logger, p.event, model.PushDispatchResponse{Skipped: false})
}

// fanout delivers one FCM push per token and purges tokens that FCM
// reports as permanently invalid. Runs in a goroutine; errors are logged
// but never propagated.
func (h *AchievementPushHandler) fanout(tokens []string, p dispatchParams) {
	defer h.wg.Done()
	ctx, cancel := context.WithTimeout(context.Background(), fanoutTimeout)
	defer cancel()

	for _, token := range tokens {
		if err := h.pusher.Send(ctx, token, p.title, p.body, nil); err != nil {
			h.logger.Warn("achievement-push: FCM send failed",
				zap.String("event", p.event),
				zap.String("user_id", p.userID),
				zap.Error(err))
			if isStaleTokenError(err) {
				if delErr := h.store.DeletePushToken(ctx, p.userID, token); delErr != nil {
					h.logger.Warn("achievement-push: delete stale token",
						zap.String("user_id", p.userID), zap.Error(delErr))
				}
			}
		}
	}
}

// writeDispatch buffers the JSON body before writing headers so an encode
// failure does not leave the client with a 202 status and a broken body.
func writeDispatch(w http.ResponseWriter, logger *zap.Logger, event string, resp model.PushDispatchResponse) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(resp); err != nil {
		logger.Error("achievement-push: encode response", zap.String("event", event), zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write(buf.Bytes())
}
