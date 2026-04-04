package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/teacherslounge/gaming-service/internal/model"
)

const (
	streakKeyPrefix   = "streak:"
	leaderboardKey    = "leaderboard:global"
	streakResetWindow = 24 * time.Hour

	weeklyTTL  = 14 * 24 * time.Hour
	monthlyTTL = 62 * 24 * time.Hour
)

// Store holds Postgres and Redis clients.
type Store struct {
	db  *pgxpool.Pool
	rdb redis.Cmdable
}

// New creates a Store with the given Postgres pool and Redis client.
func New(db *pgxpool.Pool, rdb redis.Cmdable) *Store {
	return &Store{db: db, rdb: rdb}
}

// GetProfile fetches the gaming profile for a user. Returns nil, nil if not found.
func (s *Store) GetProfile(ctx context.Context, userID string) (*model.Profile, error) {
	const q = `
		SELECT user_id, level, xp, current_streak, longest_streak,
		       bosses_defeated, gems, power_ups, last_study_date
		FROM gaming_profiles
		WHERE user_id = $1`

	p := &model.Profile{}
	var powerUpsRaw []byte
	err := s.db.QueryRow(ctx, q, userID).Scan(
		&p.UserID, &p.Level, &p.XP,
		&p.CurrentStreak, &p.LongestStreak,
		&p.BossesDefeated, &p.Gems,
		&powerUpsRaw, &p.LastStudyDate,
	)
	if err != nil {
		return nil, fmt.Errorf("get profile %s: %w", userID, err)
	}
	if powerUpsRaw != nil {
		p.PowerUps = json.RawMessage(powerUpsRaw)
	} else {
		p.PowerUps = json.RawMessage("{}")
	}
	return p, nil
}

// UpsertXP creates or updates a gaming profile's XP and level.
// Returns the updated xp and level.
func (s *Store) UpsertXP(ctx context.Context, userID string, newXP int64, newLevel int) error {
	const q = `
		INSERT INTO gaming_profiles (user_id, xp, level)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE
		SET xp = EXCLUDED.xp, level = EXCLUDED.level`

	_, err := s.db.Exec(ctx, q, userID, newXP, newLevel)
	if err != nil {
		return fmt.Errorf("upsert xp %s: %w", userID, err)
	}
	return nil
}

// GetXPAndLevel fetches current xp and level for a user. Returns 0, 1 if not found.
func (s *Store) GetXPAndLevel(ctx context.Context, userID string) (xp int64, level int, err error) {
	const q = `SELECT xp, level FROM gaming_profiles WHERE user_id = $1`
	err = s.db.QueryRow(ctx, q, userID).Scan(&xp, &level)
	if err != nil {
		// treat not found as fresh user
		return 0, 1, nil
	}
	return xp, level, nil
}

// StreakCheckin increments the streak counter in Redis, resets if > 24h gap,
// then updates longest_streak in Postgres if exceeded.
// Returns current streak, longest streak, and whether the streak was reset.
func (s *Store) StreakCheckin(ctx context.Context, userID string) (current, longest int, reset bool, err error) {
	key := streakKeyPrefix + userID

	// Fetch last check-in time from Redis hash
	vals, err := s.rdb.HMGet(ctx, key, "count", "last_ts").Result()
	if err != nil {
		return 0, 0, false, fmt.Errorf("streak hmget %s: %w", userID, err)
	}

	now := time.Now().UTC()
	nowUnix := now.Unix()

	var currentStreak int
	reset = false

	if vals[0] != nil && vals[1] != nil {
		// Parse existing streak
		var count int64
		var lastTS int64
		fmt.Sscan(vals[0].(string), &count)
		fmt.Sscan(vals[1].(string), &lastTS)

		lastTime := time.Unix(lastTS, 0)
		if now.Sub(lastTime) > streakResetWindow {
			// Gap > 24h: reset
			currentStreak = 1
			reset = true
		} else {
			currentStreak = int(count) + 1
		}
	} else {
		currentStreak = 1
	}

	// Store updated streak in Redis (no TTL — streak persists)
	if err := s.rdb.HMSet(ctx, key, map[string]any{
		"count":   currentStreak,
		"last_ts": nowUnix,
	}).Err(); err != nil {
		return 0, 0, false, fmt.Errorf("streak hmset %s: %w", userID, err)
	}

	// Update Postgres longest_streak and current_streak
	const q = `
		INSERT INTO gaming_profiles (user_id, current_streak, longest_streak, last_study_date)
		VALUES ($1, $2, $2, CURRENT_DATE)
		ON CONFLICT (user_id) DO UPDATE
		SET current_streak = $2,
		    longest_streak = GREATEST(gaming_profiles.longest_streak, $2),
		    last_study_date = CURRENT_DATE
		RETURNING longest_streak`

	var longestDB int
	err = s.db.QueryRow(ctx, q, userID, currentStreak).Scan(&longestDB)
	if err != nil {
		return 0, 0, false, fmt.Errorf("streak upsert %s: %w", userID, err)
	}

	return currentStreak, longestDB, reset, nil
}

// leaderboardTopN returns the top n entries by score from the given sorted set key,
// plus the rank of userID (omitted when empty or not present).
func (s *Store) leaderboardTopN(ctx context.Context, key, userID string, n int) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	members, err := s.rdb.ZRevRangeWithScores(ctx, key, 0, int64(n-1)).Result()
	if err != nil {
		return nil, nil, fmt.Errorf("zrevrange %s: %w", key, err)
	}

	entries := make([]model.LeaderboardEntry, len(members))
	for i, m := range members {
		entries[i] = model.LeaderboardEntry{
			UserID: m.Member.(string),
			XP:     m.Score,
			Rank:   int64(i + 1),
		}
	}

	var userEntry *model.LeaderboardEntry
	if userID != "" {
		rank, err := s.rdb.ZRevRank(ctx, key, userID).Result()
		if err == nil {
			score, _ := s.rdb.ZScore(ctx, key, userID).Result()
			userEntry = &model.LeaderboardEntry{
				UserID: userID,
				XP:     score,
				Rank:   rank + 1,
			}
		}
	}

	return entries, userEntry, nil
}

// weeklyKey returns the Redis key for the current ISO week's leaderboard.
func weeklyKey() string {
	y, w := time.Now().UTC().ISOWeek()
	return fmt.Sprintf("leaderboard:weekly:%d-W%02d", y, w)
}

// monthlyKey returns the Redis key for the current month's leaderboard.
func monthlyKey() string {
	now := time.Now().UTC()
	return fmt.Sprintf("leaderboard:monthly:%d-%02d", now.Year(), int(now.Month()))
}

// courseKey returns the Redis key for a course-scoped leaderboard.
func courseKey(courseID string) string {
	return "leaderboard:course:" + courseID
}

// LeaderboardUpdate sets a user's XP score in the global, weekly, and monthly leaderboards.
func (s *Store) LeaderboardUpdate(ctx context.Context, userID string, xp int64) error {
	z := redis.Z{Score: float64(xp), Member: userID}
	wKey := weeklyKey()
	mKey := monthlyKey()

	pipe := s.rdb.Pipeline()
	pipe.ZAdd(ctx, leaderboardKey, z)
	pipe.ZAdd(ctx, wKey, z)
	pipe.Expire(ctx, wKey, weeklyTTL)
	pipe.ZAdd(ctx, mKey, z)
	pipe.Expire(ctx, mKey, monthlyTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("leaderboard update %s: %w", userID, err)
	}
	return nil
}

// LeaderboardUpdateCourse sets a user's XP score in a course-scoped leaderboard.
func (s *Store) LeaderboardUpdateCourse(ctx context.Context, userID, courseID string, xp int64) error {
	key := courseKey(courseID)
	err := s.rdb.ZAdd(ctx, key, redis.Z{Score: float64(xp), Member: userID}).Err()
	if err != nil {
		return fmt.Errorf("leaderboard course update %s/%s: %w", courseID, userID, err)
	}
	return nil
}

// LeaderboardTop10 returns the global top-10 plus the requesting user's rank.
func (s *Store) LeaderboardTop10(ctx context.Context, userID string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	return s.leaderboardTopN(ctx, leaderboardKey, userID, 10)
}

// LeaderboardGetPeriod returns the top-10 for the given period ("weekly" or "monthly"),
// plus the requesting user's rank. Falls back to global for "all_time" or unknown values.
func (s *Store) LeaderboardGetPeriod(ctx context.Context, userID, period string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	var key string
	switch period {
	case model.PeriodWeekly:
		key = weeklyKey()
	case model.PeriodMonthly:
		key = monthlyKey()
	default:
		key = leaderboardKey
	}
	return s.leaderboardTopN(ctx, key, userID, 10)
}

// LeaderboardGetCourse returns the top-10 for a course-scoped board plus the user's rank.
func (s *Store) LeaderboardGetCourse(ctx context.Context, userID, courseID string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	return s.leaderboardTopN(ctx, courseKey(courseID), userID, 10)
}

// LeaderboardGetFriends returns all provided friend IDs (plus the caller) ranked by XP
// on the global leaderboard, along with the caller's rank in that group.
func (s *Store) LeaderboardGetFriends(ctx context.Context, userID string, friendIDs []string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	members := make([]string, 0, len(friendIDs)+1)
	members = append(members, userID)
	members = append(members, friendIDs...)

	scores, err := s.rdb.ZMScore(ctx, leaderboardKey, members...).Result()
	if err != nil {
		return nil, nil, fmt.Errorf("zmscores friends: %w", err)
	}

	type scored struct {
		userID string
		xp     float64
	}
	items := make([]scored, len(members))
	for i, uid := range members {
		items[i] = scored{uid, scores[i]}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].xp > items[j].xp })

	entries := make([]model.LeaderboardEntry, len(items))
	var userRank *model.LeaderboardEntry
	for i, it := range items {
		e := model.LeaderboardEntry{UserID: it.userID, XP: it.xp, Rank: int64(i + 1)}
		entries[i] = e
		if it.userID == userID {
			cp := e
			userRank = &cp
		}
	}
	return entries, userRank, nil
}

// RandomQuote fetches a random row from scifi_quotes.
func (s *Store) RandomQuote(ctx context.Context) (*model.Quote, error) {
	const q = `
		SELECT id, quote, attribution, context
		FROM scifi_quotes
		ORDER BY RANDOM()
		LIMIT 1`

	quote := &model.Quote{}
	err := s.db.QueryRow(ctx, q).Scan(&quote.ID, &quote.Quote, &quote.Attribution, &quote.Context)
	if err != nil {
		return nil, fmt.Errorf("random quote: %w", err)
	}
	return quote, nil
}
