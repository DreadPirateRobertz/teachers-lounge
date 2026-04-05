package store

// White-box tests for the leaderboard snapshot cache.
// Using package store (not store_test) to access unexported helpers.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/teacherslounge/gaming-service/internal/model"
)

func newCacheTestStore(t *testing.T) (*Store, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	// DB is nil — leaderboardTopNSnapshot only uses Redis, not Postgres.
	s := &Store{db: nil, rdb: rdb}
	return s, mr
}

// seedGlobal seeds the global leaderboard sorted set directly in miniredis.
func seedGlobal(t *testing.T, rdb redis.Cmdable, users map[string]float64) {
	t.Helper()
	ctx := context.Background()
	for uid, xp := range users {
		if err := rdb.ZAdd(ctx, leaderboardKey, redis.Z{Score: xp, Member: uid}).Err(); err != nil {
			t.Fatalf("seed ZAdd %s: %v", uid, err)
		}
	}
}

func TestLeaderboardSnapshot_CacheMissPopulatesCache(t *testing.T) {
	s, mr := newCacheTestStore(t)
	defer mr.Close()
	ctx := context.Background()

	seedGlobal(t, s.rdb, map[string]float64{"alice": 500, "bob": 800})

	// First call — cache miss, must fetch from sorted set and populate cache
	entries, err := s.leaderboardTopNSnapshot(ctx, leaderboardKey, 10)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	if entries[0].UserID != "bob" {
		t.Errorf("rank 1: want bob, got %s", entries[0].UserID)
	}

	// Verify the snapshot key now exists in Redis
	snapKey := leaderboardSnapshotPrefix + leaderboardKey
	if !mr.Exists(snapKey) {
		t.Error("snapshot key should exist in Redis after cache miss")
	}
	ttl := mr.TTL(snapKey)
	if ttl <= 0 || ttl > leaderboardSnapshotTTL {
		t.Errorf("snapshot TTL: want 0 < ttl <= %v, got %v", leaderboardSnapshotTTL, ttl)
	}
}

func TestLeaderboardSnapshot_CacheHitReturnsCached(t *testing.T) {
	s, mr := newCacheTestStore(t)
	defer mr.Close()
	ctx := context.Background()

	seedGlobal(t, s.rdb, map[string]float64{"alice": 500, "bob": 800})

	// Warm the cache
	if _, err := s.leaderboardTopNSnapshot(ctx, leaderboardKey, 10); err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Now mutate the sorted set — cache should still return the old snapshot
	if err := s.rdb.ZAdd(ctx, leaderboardKey, redis.Z{Score: 9999, Member: "carol"}).Err(); err != nil {
		t.Fatalf("ZAdd carol: %v", err)
	}

	entries, err := s.leaderboardTopNSnapshot(ctx, leaderboardKey, 10)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	// carol must NOT appear — she was added after the cache was written
	for _, e := range entries {
		if e.UserID == "carol" {
			t.Error("carol should not appear in cached snapshot")
		}
	}
}

func TestLeaderboardSnapshot_CacheExpiry(t *testing.T) {
	s, mr := newCacheTestStore(t)
	defer mr.Close()
	ctx := context.Background()

	seedGlobal(t, s.rdb, map[string]float64{"alice": 500})

	// Warm the cache
	if _, err := s.leaderboardTopNSnapshot(ctx, leaderboardKey, 10); err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Expire the cache using miniredis time travel
	mr.FastForward(leaderboardSnapshotTTL + time.Second)

	// Add a new user after cache expiry
	if err := s.rdb.ZAdd(ctx, leaderboardKey, redis.Z{Score: 9999, Member: "dave"}).Err(); err != nil {
		t.Fatalf("ZAdd dave: %v", err)
	}

	entries, err := s.leaderboardTopNSnapshot(ctx, leaderboardKey, 10)
	if err != nil {
		t.Fatalf("post-expiry call: %v", err)
	}
	found := false
	for _, e := range entries {
		if e.UserID == "dave" {
			found = true
			break
		}
	}
	if !found {
		t.Error("dave should appear after cache expiry and re-fetch")
	}
}

func TestLeaderboardSnapshot_CorruptCacheFallsBackToRedis(t *testing.T) {
	s, mr := newCacheTestStore(t)
	defer mr.Close()
	ctx := context.Background()

	seedGlobal(t, s.rdb, map[string]float64{"alice": 500})

	// Manually write corrupt JSON into the snapshot key
	snapKey := leaderboardSnapshotPrefix + leaderboardKey
	if err := s.rdb.Set(ctx, snapKey, []byte("not-json{{{"), leaderboardSnapshotTTL).Err(); err != nil {
		t.Fatalf("set corrupt cache: %v", err)
	}

	entries, err := s.leaderboardTopNSnapshot(ctx, leaderboardKey, 10)
	if err != nil {
		t.Fatalf("should fall back to live data: %v", err)
	}
	if len(entries) != 1 || entries[0].UserID != "alice" {
		t.Errorf("fallback: want [alice], got %+v", entries)
	}
}

func TestLeaderboardSnapshot_EmptySet(t *testing.T) {
	s, mr := newCacheTestStore(t)
	defer mr.Close()
	ctx := context.Background()

	entries, err := s.leaderboardTopNSnapshot(ctx, leaderboardKey, 10)
	if err != nil {
		t.Fatalf("empty set: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("want 0 entries, got %d", len(entries))
	}

	// Snapshot for empty set should be an empty JSON array, not absent
	snapKey := leaderboardSnapshotPrefix + leaderboardKey
	raw, _ := s.rdb.Get(ctx, snapKey).Bytes()
	var check []model.LeaderboardEntry
	if err := json.Unmarshal(raw, &check); err != nil {
		t.Errorf("empty snapshot should be valid JSON array, got %q: %v", raw, err)
	}
}
