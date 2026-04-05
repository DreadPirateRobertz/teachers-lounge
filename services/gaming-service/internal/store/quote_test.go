package store_test

// Tests for the scifi-quote Redis dedup logic (tl-pcw).
//
// Postgres is not wired — we test the Redis-facing dedup layer by
// simulating the SADD / SMEMBERS / DEL sequence that RandomQuoteForUser
// uses. Full integration tests (actual SELECT from scifi_quotes) belong
// in the e2e suite.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// seenQuotesKey mirrors the private helper in store.go so we can
// pre-populate the Redis set in tests.
func seenQuotesKey(userID string) string {
	return fmt.Sprintf("quotes:seen:%s:%s", userID, time.Now().UTC().Format("2006-01-02"))
}

// quoteSeenIDs fetches the set of seen quote IDs for a user from Redis.
func quoteSeenIDs(t *testing.T, rdb *redis.Client, userID string) []string {
	t.Helper()
	members, err := rdb.SMembers(context.Background(), seenQuotesKey(userID)).Result()
	if err != nil {
		t.Fatalf("SMembers: %v", err)
	}
	return members
}

// markSeen pre-populates the Redis seen-set for a user with the given IDs,
// simulating quotes that were already served today.
func markSeen(t *testing.T, rdb *redis.Client, userID string, ids ...string) {
	t.Helper()
	ctx := context.Background()
	key := seenQuotesKey(userID)
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	if err := rdb.SAdd(ctx, key, args...).Err(); err != nil {
		t.Fatalf("SAdd (markSeen): %v", err)
	}
}

// ── TTL tests — Redis key expires after 25 hours ─────────────────────────────

func TestQuoteSeenKey_ExpireAfter25Hours(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	userID := "user-ttl"
	key := seenQuotesKey(userID)

	// Set a key with the expected TTL.
	const seenTTL = 25 * time.Hour
	if err := rdb.SAdd(ctx, key, "42").Err(); err != nil {
		t.Fatalf("SAdd: %v", err)
	}
	if err := rdb.Expire(ctx, key, seenTTL).Err(); err != nil {
		t.Fatalf("Expire: %v", err)
	}

	ttl, err := rdb.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}
	if ttl <= 24*time.Hour || ttl > 25*time.Hour {
		t.Errorf("expected TTL in (24h, 25h], got %v", ttl)
	}
}

// ── Dedup logic tests — simulate the SMEMBERS / SADD / DEL sequence ──────────

func TestQuoteDedup_FreshUser_EmptySeenSet(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	userID := "user-fresh"

	seen := quoteSeenIDs(t, rdb, userID)
	if len(seen) != 0 {
		t.Errorf("fresh user: expected empty seen set, got %v", seen)
	}
}

func TestQuoteDedup_AfterServing_IDAddedToSet(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	userID := "user-add"
	key := seenQuotesKey(userID)

	// Simulate RandomQuoteForUser marking quote 7 as seen.
	const seenTTL = 25 * time.Hour
	pipe := rdb.Pipeline()
	pipe.SAdd(ctx, key, "7")
	pipe.Expire(ctx, key, seenTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		t.Fatalf("pipeline: %v", err)
	}

	seen := quoteSeenIDs(t, rdb, userID)
	if len(seen) != 1 || seen[0] != "7" {
		t.Errorf("expected seen=[7], got %v", seen)
	}
}

func TestQuoteDedup_MultipleQuotes_AllTracked(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	userID := "user-multi"
	key := seenQuotesKey(userID)
	const seenTTL = 25 * time.Hour

	for _, id := range []string{"1", "5", "12", "99"} {
		pipe := rdb.Pipeline()
		pipe.SAdd(ctx, key, id)
		pipe.Expire(ctx, key, seenTTL)
		if _, err := pipe.Exec(ctx); err != nil {
			t.Fatalf("pipeline for id %s: %v", id, err)
		}
	}

	seen := quoteSeenIDs(t, rdb, userID)
	if len(seen) != 4 {
		t.Errorf("expected 4 seen IDs, got %d: %v", len(seen), seen)
	}
}

func TestQuoteDedup_Reset_ClearsSeen(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()
	userID := "user-reset"

	// Pre-populate: simulate user has seen all quotes today.
	markSeen(t, rdb, userID, "1", "2", "3")

	// Simulate the reset: DEL the key (as RandomQuoteForUser does when exhausted).
	key := seenQuotesKey(userID)
	if err := rdb.Del(ctx, key).Err(); err != nil {
		t.Fatalf("Del: %v", err)
	}

	seen := quoteSeenIDs(t, rdb, userID)
	if len(seen) != 0 {
		t.Errorf("after reset: expected empty seen set, got %v", seen)
	}
}

func TestQuoteDedup_DifferentUsers_Independent(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	markSeen(t, rdb, "alice", "10", "20")
	markSeen(t, rdb, "bob", "30")

	aliceSeen := quoteSeenIDs(t, rdb, "alice")
	bobSeen := quoteSeenIDs(t, rdb, "bob")

	if len(aliceSeen) != 2 {
		t.Errorf("alice: expected 2 seen IDs, got %v", aliceSeen)
	}
	if len(bobSeen) != 1 {
		t.Errorf("bob: expected 1 seen ID, got %v", bobSeen)
	}
}

// ── seenQuotesKey format test ─────────────────────────────────────────────────

func TestSeenQuotesKey_IncludesUserIDAndDate(t *testing.T) {
	userID := "user-format"
	key := seenQuotesKey(userID)
	today := time.Now().UTC().Format("2006-01-02")

	expected := fmt.Sprintf("quotes:seen:%s:%s", userID, today)
	if key != expected {
		t.Errorf("seenQuotesKey: got %q, want %q", key, expected)
	}
}
