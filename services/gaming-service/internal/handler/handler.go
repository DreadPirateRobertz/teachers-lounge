package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/xp"
)

// Storer is the interface the handler needs from the store layer.
type Storer interface {
	GetXPAndLevel(ctx context.Context, userID string) (int64, int, error)
	UpsertXP(ctx context.Context, userID string, newXP int64, newLevel int) error
	GetProfile(ctx context.Context, userID string) (*model.Profile, error)
	StreakCheckin(ctx context.Context, userID string) (current, longest int, reset bool, err error)
	LeaderboardUpdate(ctx context.Context, userID string, xpVal int64) error
	LeaderboardUpdateCourse(ctx context.Context, userID, courseID string, xp int64) error
	LeaderboardTop10(ctx context.Context, userID string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error)
	LeaderboardGetPeriod(ctx context.Context, userID, period string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error)
	LeaderboardGetCourse(ctx context.Context, userID, courseID string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error)
	LeaderboardGetFriends(ctx context.Context, userID string, friendIDs []string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error)
	RandomQuote(ctx context.Context) (*model.Quote, error)

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
	SaveBattleSession(ctx context.Context, session *model.BattleSession) error
	GetBattleSession(ctx context.Context, sessionID string) (*model.BattleSession, error)
	DeleteBattleSession(ctx context.Context, sessionID string) error
	RecordBattleResult(ctx context.Context, result *model.BattleResult) error
	DeductGems(ctx context.Context, userID string, amount int) (int, error)

	// Loot / achievement methods
	GrantAchievement(ctx context.Context, userID, achievementType, badgeName string) (*model.Achievement, bool, error)
	GetAchievements(ctx context.Context, userID string) ([]model.Achievement, error)
	AddCosmeticItem(ctx context.Context, userID, key, value string) error

	// Learning style assessment
	CreateAssessmentSession(ctx context.Context, userID string) (*model.AssessmentSession, error)
	GetAssessmentSession(ctx context.Context, sessionID string) (*model.AssessmentSession, error)
	RecordAssessmentAnswer(ctx context.Context, sessionID, userID, questionID, chosenKey string) (*model.AssessmentSession, error)
}

// Handler holds the store and logger.
type Handler struct {
	store  Storer
	logger *zap.Logger
}

// New creates a Handler.
func New(store Storer, logger *zap.Logger) *Handler {
	return &Handler{store: store, logger: logger}
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

// LeaderboardUpdate handles POST /gaming/leaderboard/update.
// Optionally includes a course_id to also update the course-scoped board.
func (h *Handler) LeaderboardUpdate(w http.ResponseWriter, r *http.Request) {
	var req model.LeaderboardUpdateRequest
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

	if err := h.store.LeaderboardUpdate(r.Context(), req.UserID, req.XP); err != nil {
		h.logger.Error("leaderboard update", zap.String("user_id", req.UserID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if req.CourseID != "" {
		if err := h.store.LeaderboardUpdateCourse(r.Context(), req.UserID, req.CourseID, req.XP); err != nil {
			h.logger.Error("leaderboard course update", zap.String("user_id", req.UserID), zap.String("course_id", req.CourseID), zap.Error(err))
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetLeaderboard handles GET /gaming/leaderboard?period=all_time|weekly|monthly
func (h *Handler) GetLeaderboard(w http.ResponseWriter, r *http.Request) {
	callerID := middleware.UserIDFromContext(r.Context())
	period := r.URL.Query().Get("period")

	var (
		top10    []model.LeaderboardEntry
		userRank *model.LeaderboardEntry
		err      error
	)
	if period == "" || period == model.PeriodAllTime {
		top10, userRank, err = h.store.LeaderboardTop10(r.Context(), callerID)
	} else {
		top10, userRank, err = h.store.LeaderboardGetPeriod(r.Context(), callerID, period)
	}
	if err != nil {
		h.logger.Error("get leaderboard", zap.String("period", period), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, model.LeaderboardResponse{Top10: top10, UserRank: userRank})
}

// GetCourseLeaderboard handles GET /gaming/leaderboard/course/{courseId}
func (h *Handler) GetCourseLeaderboard(w http.ResponseWriter, r *http.Request) {
	courseID := chi.URLParam(r, "courseId")
	if courseID == "" {
		writeError(w, http.StatusBadRequest, "courseId required")
		return
	}
	callerID := middleware.UserIDFromContext(r.Context())

	top10, userRank, err := h.store.LeaderboardGetCourse(r.Context(), callerID, courseID)
	if err != nil {
		h.logger.Error("get course leaderboard", zap.String("course_id", courseID), zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, model.LeaderboardResponse{Top10: top10, UserRank: userRank})
}

// GetFriendLeaderboard handles GET /gaming/leaderboard/friends?friends=id1,id2,...
func (h *Handler) GetFriendLeaderboard(w http.ResponseWriter, r *http.Request) {
	callerID := middleware.UserIDFromContext(r.Context())
	if callerID == "" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	raw := r.URL.Query().Get("friends")
	var friendIDs []string
	for _, id := range strings.Split(raw, ",") {
		id = strings.TrimSpace(id)
		if id != "" && id != callerID {
			friendIDs = append(friendIDs, id)
		}
	}

	friends, userRank, err := h.store.LeaderboardGetFriends(r.Context(), callerID, friendIDs)
	if err != nil {
		h.logger.Error("get friend leaderboard", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, model.FriendLeaderboardResponse{Friends: friends, UserRank: userRank})
}

// RandomQuote handles GET /gaming/quotes/random
func (h *Handler) RandomQuote(w http.ResponseWriter, r *http.Request) {
	quote, err := h.store.RandomQuote(r.Context())
	if err != nil {
		h.logger.Error("random quote", zap.Error(err))
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
	w.Write([]byte(`{"status":"ok"}`))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
