package store

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/rival"
)

const (
	leaderboardKey    = "leaderboard:global"
	weeklyTTL         = 14 * 24 * time.Hour
	monthlyTTL        = 62 * 24 * time.Hour

	// leaderboardSnapshotTTL is the cache lifetime for pre-serialised top-10 snapshots.
	// The global leaderboard does not need real-time accuracy — 30s is imperceptible to users
	// and eliminates repeated ZRevRangeWithScores calls under high concurrent read load.
	leaderboardSnapshotTTL    = 30 * time.Second
	leaderboardSnapshotPrefix = "leaderboard:snapshot:"
)

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

// leaderboardTopN returns the top n entries by score from the given sorted set key,
// plus the rank of userID (omitted when empty or not present).
//
// The top-n list (without the per-user rank) is cached in Redis for
// leaderboardSnapshotTTL to reduce ZRevRangeWithScores calls under read load.
// The per-user rank is always fetched live so it remains accurate.
func (s *Store) leaderboardTopN(ctx context.Context, key, userID string, n int) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	entries, err := s.leaderboardTopNSnapshot(ctx, key, n)
	if err != nil {
		return nil, nil, err
	}

	var userEntry *model.LeaderboardEntry
	if userID != "" {
		rank, err := s.rdb.ZRevRank(ctx, key, userID).Result()
		if err == nil {
			score, _ := s.rdb.ZScore(ctx, key, userID).Result()
			userEntry = &model.LeaderboardEntry{
				UserID:  userID,
				XP:      score,
				Rank:    rank + 1,
				IsRival: rival.IsRival(userID),
			}
		}
	}

	return entries, userEntry, nil
}

// leaderboardTopNSnapshot returns the top n entries from a cached Redis snapshot.
// On cache miss it fetches from the sorted set and writes a new snapshot.
func (s *Store) leaderboardTopNSnapshot(ctx context.Context, key string, n int) ([]model.LeaderboardEntry, error) {
	snapKey := leaderboardSnapshotPrefix + key

	// Try cache hit first
	cached, err := s.rdb.Get(ctx, snapKey).Bytes()
	if err == nil {
		var entries []model.LeaderboardEntry
		if jsonErr := json.Unmarshal(cached, &entries); jsonErr == nil {
			return entries, nil
		}
	}

	// Cache miss — fetch from sorted set
	members, err := s.rdb.ZRevRangeWithScores(ctx, key, 0, int64(n-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("zrevrange %s: %w", key, err)
	}

	entries := make([]model.LeaderboardEntry, len(members))
	for i, m := range members {
		uid := m.Member.(string)
		entries[i] = model.LeaderboardEntry{
			UserID:  uid,
			XP:      m.Score,
			Rank:    int64(i + 1),
			IsRival: rival.IsRival(uid),
		}
	}

	// Write snapshot — ignore errors (cache is best-effort)
	if b, jsonErr := json.Marshal(entries); jsonErr == nil {
		_ = s.rdb.Set(ctx, snapKey, b, leaderboardSnapshotTTL).Err()
	}

	return entries, nil
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

// SeedRivals inserts each rival into the global leaderboard at its BaseXP using
// ZAddNX (add-if-not-exists), so existing scores accumulated since the last
// restart are preserved across service restarts.
func (s *Store) SeedRivals(ctx context.Context, rivals []rival.Rival) error {
	if len(rivals) == 0 {
		return nil
	}
	zs := make([]redis.Z, len(rivals))
	for i, r := range rivals {
		zs[i] = redis.Z{Score: float64(r.BaseXP), Member: r.ID}
	}
	if err := s.rdb.ZAddNX(ctx, leaderboardKey, zs...).Err(); err != nil {
		return fmt.Errorf("seed rivals: %w", err)
	}
	return nil
}

// TickRivals advances each rival's global leaderboard score by a random amount
// in [DailyGainMin, DailyGainMax], simulating a day of study activity.
// It is safe to call multiple times; each call applies one independent increment.
func (s *Store) TickRivals(ctx context.Context, rivals []rival.Rival) error {
	pipe := s.rdb.Pipeline()
	for _, r := range rivals {
		spread := r.DailyGainMax - r.DailyGainMin
		gain := r.DailyGainMin + rand.Intn(spread+1)
		pipe.ZIncrBy(ctx, leaderboardKey, float64(gain), r.ID)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("tick rivals: %w", err)
	}
	// Invalidate the leaderboard snapshot so the next read reflects updated scores.
	_ = s.rdb.Del(ctx, leaderboardSnapshotPrefix+leaderboardKey).Err()
	return nil
}
