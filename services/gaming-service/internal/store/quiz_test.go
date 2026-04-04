package store_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// hintKey mirrors the private hintKeyFmt constant.
func hintKey(sessionID, questionID string) string {
	return fmt.Sprintf("quiz:hints:%s:%s", sessionID, questionID)
}

// simulateGetHintIndex reads the hint counter from Redis, replicating GetHintIndex logic.
func simulateGetHintIndex(ctx context.Context, rdb *redis.Client, sessionID, questionID string) (int, error) {
	key := hintKey(sessionID, questionID)
	val, err := rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	n, _ := strconv.Atoi(val)
	return n, nil
}

// simulateIncrHintIndex replicates the Redis side of IncrHintIndex (gem deduction is Postgres-side).
func simulateIncrHintIndex(ctx context.Context, rdb *redis.Client, sessionID, questionID string) (newIndex int, err error) {
	key := hintKey(sessionID, questionID)
	newVal, err := rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	return int(newVal) - 1, nil // 0-based index
}

func TestHintIndex_FirstRequest(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	idx, err := simulateGetHintIndex(ctx, rdb, "sess-1", "q-1")
	if err != nil {
		t.Fatal(err)
	}
	if idx != 0 {
		t.Errorf("first request: want idx=0, got %d", idx)
	}
}

func TestHintIndex_IncrementSequence(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	for want := 0; want < 3; want++ {
		got, err := simulateIncrHintIndex(ctx, rdb, "sess-1", "q-1")
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("hint %d: want idx=%d, got %d", want+1, want, got)
		}
	}

	// GetHintIndex should now return 3 (three hints consumed).
	idx, err := simulateGetHintIndex(ctx, rdb, "sess-1", "q-1")
	if err != nil {
		t.Fatal(err)
	}
	if idx != 3 {
		t.Errorf("after 3 incrs: want idx=3, got %d", idx)
	}
}

func TestHintIndex_IsolatedPerQuestion(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	// Consume 2 hints for q-1.
	simulateIncrHintIndex(ctx, rdb, "sess-1", "q-1")
	simulateIncrHintIndex(ctx, rdb, "sess-1", "q-1")

	// q-2 in same session should start at 0.
	idx, err := simulateGetHintIndex(ctx, rdb, "sess-1", "q-2")
	if err != nil {
		t.Fatal(err)
	}
	if idx != 0 {
		t.Errorf("q-2 hint index should be 0, got %d", idx)
	}
}

func TestHintIndex_IsolatedPerSession(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	ctx := context.Background()

	// Use 1 hint in session A.
	simulateIncrHintIndex(ctx, rdb, "sess-A", "q-1")

	// Session B for the same question should start at 0.
	idx, err := simulateGetHintIndex(ctx, rdb, "sess-B", "q-1")
	if err != nil {
		t.Fatal(err)
	}
	if idx != 0 {
		t.Errorf("sess-B hint index should be 0, got %d", idx)
	}
}
