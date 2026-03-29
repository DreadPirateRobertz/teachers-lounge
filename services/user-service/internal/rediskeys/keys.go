// Package rediskeys defines all Redis key patterns for TeachersLounge.
// Import this package in any service that reads/writes Redis to ensure
// consistent key naming across the stack.
package rediskeys

import "fmt"

// Session — active user session context (Hash, TTL: 24h)
//
//	Fields: user_id, email, subscription_status, account_type, course_id_active
func Session(userID string) string {
	return fmt.Sprintf("session:%s", userID)
}

// Streak — current streak count + last study date (Hash, no TTL)
//
//	Fields: count (int), last_study_date (YYYY-MM-DD)
func Streak(userID string) string {
	return fmt.Sprintf("streak:%s", userID)
}

// BossState — active boss battle state (Hash, TTL: 1h)
//
//	Fields: boss_name, topic, hp_current, hp_max, round, timer_end_unix
func BossState(userID, bossID string) string {
	return fmt.Sprintf("boss:%s:%s", userID, bossID)
}

// LeaderboardGlobal — global XP leaderboard (Sorted Set, no TTL)
//
//	Score: total XP. Member: user_id.
func LeaderboardGlobal() string {
	return "leaderboard:global"
}

// LeaderboardCourse — per-course XP leaderboard (Sorted Set, no TTL)
//
//	Score: XP earned in course. Member: user_id.
func LeaderboardCourse(courseID string) string {
	return fmt.Sprintf("leaderboard:course:%s", courseID)
}

// DailyQuests — daily quest progress (Hash, TTL: 24h)
//
//	Fields: quest_id → progress JSON {"target": 5, "current": 2, "xp_reward": 100}
func DailyQuests(userID string) string {
	return fmt.Sprintf("quests:daily:%s", userID)
}

// XPToday — XP earned today (String, TTL: 24h)
//
//	Value: integer XP count. Used for daily XP caps.
func XPToday(userID string) string {
	return fmt.Sprintf("xp:today:%s", userID)
}

// InsightCache — cached cross-student insight for a topic (String, TTL: 6h)
//
//	Value: JSON blob {"insight": "...", "confidence": 0.87, "generated_at": "..."}
func InsightCache(topic string) string {
	return fmt.Sprintf("cache:insight:%s", topic)
}

// NotifRateLimit — notification rate limit counter (String, TTL: 24h)
//
//	Value: integer count of push notifications sent today. Max: 3.
func NotifRateLimit(userID string) string {
	return fmt.Sprintf("ratelimit:%s:notif", userID)
}

// QuoteSeenToday — set of quote IDs seen today (Set, TTL: 24h)
//
//	Members: quote IDs (int). Prevents same quote repeating in a day.
func QuoteSeenToday(userID string) string {
	return fmt.Sprintf("quotes:seen:%s", userID)
}

// SessionRefreshLock — distributed lock for refresh token rotation (String, TTL: 5s)
//
//	Value: lock token (random UUID). Prevents concurrent refresh races.
func SessionRefreshLock(tokenHash string) string {
	return fmt.Sprintf("lock:refresh:%s", tokenHash)
}

// RateLimitLogin — login attempt rate limit (String, TTL: 15m)
//
//	Value: attempt count. Block after 10 attempts.
func RateLimitLogin(ip string) string {
	return fmt.Sprintf("ratelimit:login:%s", ip)
}

// TTLs in seconds — canonical values for all services
const (
	TTLSession         = 24 * 60 * 60  // 24h
	TTLBossState       = 60 * 60       // 1h
	TTLDailyQuests     = 24 * 60 * 60  // 24h
	TTLXPToday         = 24 * 60 * 60  // 24h
	TTLInsightCache    = 6 * 60 * 60   // 6h
	TTLNotifRateLimit  = 24 * 60 * 60  // 24h
	TTLQuoteSeen       = 24 * 60 * 60  // 24h
	TTLRefreshLock     = 5             // 5s
	TTLRateLimitLogin  = 15 * 60       // 15m
)

// MaxNotifPerDay is the maximum push notifications a user receives per day.
const MaxNotifPerDay = 3

// MaxLoginAttempts before rate-limit lockout.
const MaxLoginAttempts = 10
