package model

import (
	"encoding/json"
	"time"
)

// Profile is the gaming state for a single user.
type Profile struct {
	UserID          string          `json:"user_id"`
	Level           int             `json:"level"`
	XP              int64           `json:"xp"`
	CurrentStreak   int             `json:"current_streak"`
	LongestStreak   int             `json:"longest_streak"`
	BossesDefeated  int             `json:"bosses_defeated"`
	Gems            int             `json:"gems"`
	PowerUps        json.RawMessage `json:"power_ups"`
	LastStudyDate   *time.Time      `json:"last_study_date,omitempty"`
}

// GainXPRequest is the request body for POST /gaming/xp.
type GainXPRequest struct {
	UserID string `json:"user_id"`
	Action string `json:"action"`
	Amount int64  `json:"amount"`
}

// GainXPResponse is the response body for POST /gaming/xp.
type GainXPResponse struct {
	NewXP    int64 `json:"new_xp"`
	LevelUp  bool  `json:"level_up"`
	NewLevel int   `json:"new_level"`
}

// StreakCheckinRequest is the request body for POST /gaming/streak/checkin.
type StreakCheckinRequest struct {
	UserID string `json:"user_id"`
}

// StreakCheckinResponse is the response body for POST /gaming/streak/checkin.
type StreakCheckinResponse struct {
	CurrentStreak int  `json:"current_streak"`
	LongestStreak int  `json:"longest_streak"`
	Reset         bool `json:"reset"`
}

// Leaderboard period constants for GET /gaming/leaderboard?period=
const (
	PeriodAllTime = "all_time"
	PeriodWeekly  = "weekly"
	PeriodMonthly = "monthly"
)

// LeaderboardUpdateRequest is the request body for POST /gaming/leaderboard/update.
// CourseID is optional; when set the score is also recorded on the course-scoped board.
type LeaderboardUpdateRequest struct {
	UserID   string `json:"user_id"`
	XP       int64  `json:"xp"`
	CourseID string `json:"course_id,omitempty"`
}

// LeaderboardEntry is one row in the leaderboard response.
// IsRival is true when the entry represents a simulated competitor rather than
// a real user; the frontend uses this flag to render the "rival" badge.
type LeaderboardEntry struct {
	UserID  string  `json:"user_id"`
	XP      float64 `json:"xp"`
	Rank    int64   `json:"rank"`
	IsRival bool    `json:"is_rival,omitempty"`
}

// LeaderboardResponse is the response body for GET /gaming/leaderboard (all variants).
type LeaderboardResponse struct {
	Top10    []LeaderboardEntry `json:"top_10"`
	UserRank *LeaderboardEntry  `json:"user_rank,omitempty"`
}

// FriendLeaderboardResponse is the response body for GET /gaming/leaderboard/friends.
type FriendLeaderboardResponse struct {
	Friends  []LeaderboardEntry `json:"friends"`
	UserRank *LeaderboardEntry  `json:"user_rank,omitempty"`
}

// Quote is a row from scifi_quotes.
type Quote struct {
	ID          int    `json:"id"`
	Quote       string `json:"quote"`
	Attribution string `json:"attribution"`
	Context     string `json:"context"`
}

// QuestState is the live state of a single daily quest for a user.
type QuestState struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Progress    int    `json:"progress"`
	Target      int    `json:"target"`
	Completed   bool   `json:"completed"`
	XPReward    int    `json:"xp_reward"`
	GemsReward  int    `json:"gems_reward"`
}

// DailyQuestsResponse is the response body for GET /gaming/quests/daily.
type DailyQuestsResponse struct {
	Quests []QuestState `json:"quests"`
}

// QuestProgressRequest is the request body for POST /gaming/quests/progress.
type QuestProgressRequest struct {
	UserID string `json:"user_id"`
	Action string `json:"action"`
}

// QuestProgressResponse is the response body for POST /gaming/quests/progress.
type QuestProgressResponse struct {
	Quests      []QuestState `json:"quests"`
	XPAwarded   int          `json:"xp_awarded"`
	GemsAwarded int          `json:"gems_awarded"`
	NewXP       int64        `json:"new_xp,omitempty"`
	NewLevel    int          `json:"new_level,omitempty"`
	LevelUp     bool         `json:"level_up,omitempty"`
}
