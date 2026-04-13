package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/model"
)

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

// GetLeaderboard handles GET /gaming/leaderboard?period=all_time|weekly|monthly.
// Returns top-10 entries ranked by XP for the requested period, plus the
// requesting user's rank on that board.
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

// GetCourseLeaderboard handles GET /gaming/leaderboard/course/{courseId}.
// Returns top-10 entries for the given course board plus the requesting
// user's rank on that course board.
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
// Returns entries for the caller plus listed friends ranked by global XP.
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
