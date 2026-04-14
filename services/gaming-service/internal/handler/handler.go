package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/taunt"
	"github.com/teacherslounge/gaming-service/internal/xp"
)

// Storer is the interface the handler needs from the store layer.
type Storer interface {
	GetXPAndLevel(ctx context.Context, userID string) (int64, int, error)
	UpsertXP(ctx context.Context, userID string, newXP int64, newLevel int) error
	GetProfile(ctx context.Context, userID string) (*model.Profile, error)
	StreakCheckin(ctx context.Context, userID string) (current, longest int, reset bool, err error)
	CreateStreakFreeze(ctx context.Context, userID string, gemCost int) (gemsLeft int, expiresAt time.Time, err error)
	IsStreakFrozen(ctx context.Context, userID string) (bool, error)
	LeaderboardUpdate(ctx context.Context, userID string, xpVal int64) error
	LeaderboardUpdateCourse(ctx context.Context, userID, courseID string, xp int64) error
	LeaderboardTop10(ctx context.Context, userID string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error)
	LeaderboardGetPeriod(ctx context.Context, userID, period string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error)
	LeaderboardGetCourse(ctx context.Context, userID, courseID string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error)
	LeaderboardGetFriends(ctx context.Context, userID string, friendIDs []string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error)
	RandomQuote(ctx context.Context) (*model.Quote, error)
	RandomQuoteForUser(ctx context.Context, userID, quotectx string) (*model.Quote, error)

	// Quiz system
	GetRandomQuestions(ctx context.Context, topic string, n int) ([]*model.Question, error)
	GetQuestion(ctx context.Context, questionID string) (*model.Question, error)
	CreateQuizSession(ctx context.Context, userID string, topic, courseID *string, questionIDs []string) (*model.QuizSession, error)
	GetQuizSession(ctx context.Context, sessionID string) (*model.QuizSession, error)
	RecordAnswer(ctx context.Context, sessionID, userID, questionID, chosenKey string, isCorrect bool, hintsUsed, xpEarned int, timeMs *int) (*model.QuizSession, error)
	GetHintIndex(ctx context.Context, sessionID, questionID string) (int, error)
	IncrHintIndex(ctx context.Context, sessionID, questionID, userID string) (newIndex, gemsRemaining int, err error)
	GetDailyQuests(ctx context.Context, userID string) ([]model.QuestState, error)
	UpdateQuestProgress(ctx context.Context, userID string, action string) ([]model.QuestState, int, int, error)
	AwardQuestRewards(ctx context.Context, userID string, xpDelta, gemsDelta int) (newXP int64, newLevel int, leveledUp bool, newGems int, err error)

	// Boss battle methods
	GetDefeatedBossIDs(ctx context.Context, userID string) ([]string, error)
	GetChapterMastery(ctx context.Context, userID string, conceptPaths []string) (float64, error)
	GetChapterMasteryBatch(ctx context.Context, userID string, pathsByBossID map[string][]string) (map[string]float64, error)
	SaveBattleSession(ctx context.Context, session *model.BattleSession) error
	GetBattleSession(ctx context.Context, sessionID string) (*model.BattleSession, error)
	DeleteBattleSession(ctx context.Context, sessionID string) error
	RecordBattleResult(ctx context.Context, result *model.BattleResult) error
	DeductGems(ctx context.Context, userID string, amount int) (int, error)
	SaveTaunt(ctx context.Context, bossID string, round int, tauntText string) error
	GetRandomTaunt(ctx context.Context, bossID string, round int) (tauntText string, ok bool, err error)

	// Shop methods
	BuyPowerUp(ctx context.Context, userID string, pu model.PowerUpType, gemCost int) (gemsLeft, newCount int, err error)

	// Loot / achievement methods
	GrantAchievement(ctx context.Context, userID, achievementType, badgeName string) (*model.Achievement, bool, error)
	GetAchievements(ctx context.Context, userID string) ([]model.Achievement, error)
	AddCosmeticItem(ctx context.Context, userID, key, value string) error

	// Learning style assessment
	CreateAssessmentSession(ctx context.Context, userID string) (*model.AssessmentSession, error)
	GetAssessmentSession(ctx context.Context, sessionID string) (*model.AssessmentSession, error)
	RecordAssessmentAnswer(ctx context.Context, sessionID, userID, questionID, chosenKey string) (*model.AssessmentSession, error)

	// Flashcard system
	CreateFlashcard(ctx context.Context, card *model.Flashcard) (*model.Flashcard, error)
	GetFlashcard(ctx context.Context, id string) (*model.Flashcard, error)
	ListFlashcards(ctx context.Context, userID string) ([]*model.Flashcard, error)
	DueFlashcards(ctx context.Context, userID string) ([]*model.Flashcard, error)
	ReviewFlashcard(ctx context.Context, cardID, userID string, quality int) (*model.Flashcard, error)
	FlashcardsForSession(ctx context.Context, sessionID string) ([]*model.Flashcard, error)
	AllFlashcardsForExport(ctx context.Context, userID string) ([]*model.Flashcard, error)

	// WebSocket battle-state methods
	GetBattle(ctx context.Context, battleID string) (*model.BattleSession, error)
	UpdateBattleState(ctx context.Context, session *model.BattleSession) error
}

// Handler holds the store, taunt generator, logger, and WebSocket hub.
type Handler struct {
	store   Storer
	taunter taunt.Generator
	logger  *zap.Logger
	hub     *Hub
}

// New creates a Handler.
// taunter is used by Attack to produce contextual boss taunts on wrong answers;
// pass a taunt.StaticGenerator when the AI gateway is not configured.
func New(store Storer, taunter taunt.Generator, logger *zap.Logger) *Handler {
	return &Handler{store: store, taunter: taunter, logger: logger, hub: newHub()}
}

// GainXP handles POST /gaming/xp
func (h *Handler) GainXP(w http.ResponseWriter, r *http.Request) {
	var req model.GainXPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" || req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "user_id and positive amount required")
		return
	}

	// Only the authenticated user can gain XP for themselves.
	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" || callerID != req.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	currentXP, currentLevel, err := h.store.GetXPAndLevel(r.Context(), req.UserID)
	if err != nil {
		h.logger.Error("get xp/level", zap.String("user_id", req.UserID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	newXP, newLevel, leveledUp := xp.Apply(currentXP, currentLevel, req.Amount)

	if err := h.store.UpsertXP(r.Context(), req.UserID, newXP, newLevel); err != nil {
		h.logger.Error("upsert xp", zap.String("user_id", req.UserID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, model.GainXPResponse{
		NewXP:    newXP,
		LevelUp:  leveledUp,
		NewLevel: newLevel,
	})
}

// GetProfile handles GET /gaming/profile/{userId}
func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")

	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" || callerID != userID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	profile, err := h.store.GetProfile(r.Context(), userID)
	if err != nil {
		h.logger.Error("get profile", zap.String("user_id", userID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

// StreakCheckin handles POST /gaming/streak/checkin
func (h *Handler) StreakCheckin(w http.ResponseWriter, r *http.Request) {
	var req model.StreakCheckinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id required")
		return
	}

	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" || callerID != req.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	current, longest, reset, err := h.store.StreakCheckin(r.Context(), req.UserID)
	if err != nil {
		h.logger.Error("streak checkin", zap.String("user_id", req.UserID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, model.StreakCheckinResponse{
		CurrentStreak: current,
		LongestStreak: longest,
		Reset:         reset,
	})
}


// RandomQuote handles GET /gaming/quotes/random?context=<ctx>
//
// Optional query param "context" filters by quote context type
// (session_start, boss_fight, correct, wrong, victory, defeat, streak,
// achievement, comeback). Omitting it returns any context.
//
// When the caller is authenticated, seen-quote dedup is applied per user
// per day via Redis so the same quote is not repeated within a calendar day.
func (h *Handler) RandomQuote(w http.ResponseWriter, r *http.Request) {
	quotectx := r.URL.Query().Get("context")
	userID := middleware.UserIDFromContext(r.Context())

	var (
		quote *model.Quote
		err   error
	)
	if userID != "" {
		quote, err = h.store.RandomQuoteForUser(r.Context(), userID, quotectx)
	} else {
		quote, err = h.store.RandomQuote(r.Context())
	}
	if err != nil {
		h.logger.Error("random quote", zap.String("user_id", userID), zap.String("context", quotectx), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, quote)
}

// GetDailyQuests handles GET /gaming/quests/daily
func (h *Handler) GetDailyQuests(w http.ResponseWriter, r *http.Request) {
	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	quests, err := h.store.GetDailyQuests(r.Context(), callerID)
	if err != nil {
		h.logger.Error("get daily quests", zap.String("user_id", callerID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, model.DailyQuestsResponse{Quests: quests})
}

// UpdateQuestProgress handles POST /gaming/quests/progress
func (h *Handler) UpdateQuestProgress(w http.ResponseWriter, r *http.Request) {
	var req model.QuestProgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.UserID == "" || req.Action == "" {
		writeError(w, http.StatusBadRequest, "user_id and action required")
		return
	}

	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" || callerID != req.UserID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	quests, xpEarned, gemsEarned, err := h.store.UpdateQuestProgress(r.Context(), req.UserID, req.Action)
	if err != nil {
		h.logger.Error("update quest progress", zap.String("user_id", req.UserID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := model.QuestProgressResponse{
		Quests:      quests,
		XPAwarded:   xpEarned,
		GemsAwarded: gemsEarned,
	}

	if xpEarned > 0 || gemsEarned > 0 {
		newXP, newLevel, leveledUp, _, err := h.store.AwardQuestRewards(r.Context(), req.UserID, xpEarned, gemsEarned)
		if err != nil {
			h.logger.Error("award quest rewards", zap.String("user_id", req.UserID), zap.Error(err))
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		resp.NewXP = newXP
		resp.NewLevel = newLevel
		resp.LevelUp = leveledUp
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetAchievements handles GET /gaming/achievements/{userId}
func (h *Handler) GetAchievements(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" || callerID != userID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	achievements, err := h.store.GetAchievements(r.Context(), userID)
	if err != nil {
		h.logger.Error("get achievements", zap.String("user_id", userID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, model.AchievementsResponse{Achievements: achievements})
}

// Health handles GET /health
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
