package store_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/teacherslounge/gaming-service/internal/model"
)

// leaderboardStore is a thin wrapper around the Redis client that replicates
// the store's leaderboard logic so we can test it without Postgres.
type leaderboardStore struct {
	rdb *redis.Client
}

const globalKey = "leaderboard:global"

func (s *leaderboardStore) update(ctx context.Context, userID string, xp float64) error {
	y, w := time.Now().UTC().ISOWeek()
	wKey := fmt.Sprintf("leaderboard:weekly:%d-W%02d", y, w)
	now := time.Now().UTC()
	mKey := fmt.Sprintf("leaderboard:monthly:%d-%02d", now.Year(), int(now.Month()))

	z := redis.Z{Score: xp, Member: userID}
	pipe := s.rdb.Pipeline()
	pipe.ZAdd(ctx, globalKey, z)
	pipe.ZAdd(ctx, wKey, z)
	pipe.Expire(ctx, wKey, 14*24*time.Hour)
	pipe.ZAdd(ctx, mKey, z)
	pipe.Expire(ctx, mKey, 62*24*time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *leaderboardStore) updateCourse(ctx context.Context, userID, courseID string, xp float64) error {
	return s.rdb.ZAdd(ctx, "leaderboard:course:"+courseID, redis.Z{Score: xp, Member: userID}).Err()
}

func (s *leaderboardStore) topN(ctx context.Context, key string, n int) ([]model.LeaderboardEntry, error) {
	members, err := s.rdb.ZRevRangeWithScores(ctx, key, 0, int64(n-1)).Result()
	if err != nil {
		return nil, err
	}
	entries := make([]model.LeaderboardEntry, len(members))
	for i, m := range members {
		entries[i] = model.LeaderboardEntry{
			UserID: m.Member.(string),
			XP:     m.Score,
			Rank:   int64(i + 1),
		}
	}
	return entries, nil
}

func (s *leaderboardStore) userRank(ctx context.Context, key, userID string) *model.LeaderboardEntry {
	rank, err := s.rdb.ZRevRank(ctx, key, userID).Result()
	if err != nil {
		return nil
	}
	score, _ := s.rdb.ZScore(ctx, key, userID).Result()
	return &model.LeaderboardEntry{UserID: userID, XP: score, Rank: rank + 1}
}

func newLeaderboardStore(t *testing.T) (*leaderboardStore, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return &leaderboardStore{rdb: rdb}, mr
}

func TestLeaderboardUpdate_GlobalBoard(t *testing.T) {
	s, mr := newLeaderboardStore(t)
	defer mr.Close()
	ctx := context.Background()

	if err := s.update(ctx, "alice", 500); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := s.update(ctx, "bob", 800); err != nil {
		t.Fatalf("update: %v", err)
	}

	entries, err := s.topN(ctx, globalKey, 10)
	if err != nil {
		t.Fatalf("topN: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	if entries[0].UserID != "bob" || entries[0].Rank != 1 {
		t.Errorf("rank 1: want bob, got %s", entries[0].UserID)
	}
	if entries[1].UserID != "alice" || entries[1].Rank != 2 {
		t.Errorf("rank 2: want alice, got %s", entries[1].UserID)
	}
}

func TestLeaderboardUpdate_PopulatesWeeklyAndMonthly(t *testing.T) {
	s, mr := newLeaderboardStore(t)
	defer mr.Close()
	ctx := context.Background()

	if err := s.update(ctx, "alice", 100); err != nil {
		t.Fatalf("update: %v", err)
	}

	y, w := time.Now().UTC().ISOWeek()
	wKey := fmt.Sprintf("leaderboard:weekly:%d-W%02d", y, w)
	now := time.Now().UTC()
	mKey := fmt.Sprintf("leaderboard:monthly:%d-%02d", now.Year(), int(now.Month()))

	for _, key := range []string{wKey, mKey} {
		entries, err := s.topN(ctx, key, 10)
		if err != nil {
			t.Fatalf("topN %s: %v", key, err)
		}
		if len(entries) != 1 || entries[0].UserID != "alice" {
			t.Errorf("%s: want alice, got %v", key, entries)
		}
	}
}

func TestLeaderboardUpdate_WeeklyKeyExpires(t *testing.T) {
	s, mr := newLeaderboardStore(t)
	defer mr.Close()
	ctx := context.Background()

	if err := s.update(ctx, "alice", 100); err != nil {
		t.Fatalf("update: %v", err)
	}

	y, w := time.Now().UTC().ISOWeek()
	wKey := fmt.Sprintf("leaderboard:weekly:%d-W%02d", y, w)

	ttl := mr.TTL(wKey)
	if ttl <= 0 {
		t.Errorf("weekly key has no TTL set, got %v", ttl)
	}
}

func TestLeaderboardUserRank(t *testing.T) {
	s, mr := newLeaderboardStore(t)
	defer mr.Close()
	ctx := context.Background()

	for _, tc := range []struct{ id string; xp float64 }{
		{"alice", 300},
		{"bob", 500},
		{"carol", 200},
	} {
		if err := s.update(ctx, tc.id, tc.xp); err != nil {
			t.Fatalf("update %s: %v", tc.id, err)
		}
	}

	rank := s.userRank(ctx, globalKey, "alice")
	if rank == nil {
		t.Fatal("alice rank is nil")
	}
	if rank.Rank != 2 {
		t.Errorf("alice rank: want 2, got %d", rank.Rank)
	}

	rank = s.userRank(ctx, globalKey, "carol")
	if rank.Rank != 3 {
		t.Errorf("carol rank: want 3, got %d", rank.Rank)
	}
}

func TestLeaderboardUserRank_NotPresent(t *testing.T) {
	s, mr := newLeaderboardStore(t)
	defer mr.Close()
	ctx := context.Background()

	rank := s.userRank(ctx, globalKey, "nobody")
	if rank != nil {
		t.Errorf("want nil rank for unknown user, got %+v", rank)
	}
}

func TestLeaderboardCourseBoard(t *testing.T) {
	s, mr := newLeaderboardStore(t)
	defer mr.Close()
	ctx := context.Background()

	const course = "course-go-101"
	if err := s.updateCourse(ctx, "alice", course, 400); err != nil {
		t.Fatalf("updateCourse alice: %v", err)
	}
	if err := s.updateCourse(ctx, "bob", course, 250); err != nil {
		t.Fatalf("updateCourse bob: %v", err)
	}
	// dave is in a different course — should not appear
	if err := s.updateCourse(ctx, "dave", "course-python-201", 9999); err != nil {
		t.Fatalf("updateCourse dave: %v", err)
	}

	entries, err := s.topN(ctx, "leaderboard:course:"+course, 10)
	if err != nil {
		t.Fatalf("topN course: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2, got %d", len(entries))
	}
	if entries[0].UserID != "alice" {
		t.Errorf("rank 1: want alice, got %s", entries[0].UserID)
	}
	if entries[1].UserID != "bob" {
		t.Errorf("rank 2: want bob, got %s", entries[1].UserID)
	}
}

func TestLeaderboardTop10_Limit(t *testing.T) {
	s, mr := newLeaderboardStore(t)
	defer mr.Close()
	ctx := context.Background()

	for i := 0; i < 15; i++ {
		if err := s.update(ctx, fmt.Sprintf("user-%d", i), float64(i*10)); err != nil {
			t.Fatalf("update user-%d: %v", i, err)
		}
	}

	entries, err := s.topN(ctx, globalKey, 10)
	if err != nil {
		t.Fatalf("topN: %v", err)
	}
	if len(entries) != 10 {
		t.Errorf("want 10, got %d", len(entries))
	}
	// Highest XP is user-14 (score 140)
	if entries[0].UserID != "user-14" {
		t.Errorf("rank 1: want user-14, got %s", entries[0].UserID)
	}
}

func TestLeaderboardFriends_RankedByXP(t *testing.T) {
	s, mr := newLeaderboardStore(t)
	defer mr.Close()
	ctx := context.Background()

	for _, tc := range []struct{ id string; xp float64 }{
		{"alice", 800},
		{"bob", 600},
		{"carol", 400},
		{"dave", 200},
	} {
		if err := s.update(ctx, tc.id, tc.xp); err != nil {
			t.Fatalf("update %s: %v", tc.id, err)
		}
	}

	// Simulate LeaderboardGetFriends: caller=alice, friends=[bob, carol, dave]
	members := []string{"alice", "bob", "carol", "dave"}
	scores, err := s.rdb.ZMScore(ctx, globalKey, members...).Result()
	if err != nil {
		t.Fatalf("ZMScore: %v", err)
	}

	type scored struct{ id string; xp float64 }
	items := make([]scored, len(members))
	for i, id := range members {
		items[i] = scored{id, scores[i]}
	}
	// sort descending
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].xp > items[i].xp {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	if items[0].id != "alice" {
		t.Errorf("rank 1: want alice, got %s", items[0].id)
	}
	if items[3].id != "dave" {
		t.Errorf("rank 4: want dave, got %s", items[3].id)
	}
}

func TestLeaderboardFriends_UserNotInGlobal(t *testing.T) {
	s, mr := newLeaderboardStore(t)
	defer mr.Close()
	ctx := context.Background()

	// Only alice has a score; bob is unknown
	if err := s.update(ctx, "alice", 100); err != nil {
		t.Fatalf("update: %v", err)
	}

	scores, err := s.rdb.ZMScore(ctx, globalKey, "alice", "bob").Result()
	if err != nil {
		t.Fatalf("ZMScore: %v", err)
	}
	if scores[0] != 100 {
		t.Errorf("alice score: want 100, got %f", scores[0])
	}
	if scores[1] != 0 {
		t.Errorf("bob (unknown) score: want 0, got %f", scores[1])
	}
}
