package store_test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/teacherslounge/gaming-service/internal/rival"
	"github.com/teacherslounge/gaming-service/internal/store"
)

// newRivalStore creates a Store backed by miniredis with a nil Postgres pool,
// suitable for testing Redis-only operations like SeedRivals and TickRivals.
func newRivalStore(t *testing.T) (*store.Store, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return store.New(nil, rdb), mr
}

func TestSeedRivals_InsertsAtBaseXP(t *testing.T) {
	s, mr := newRivalStore(t)
	defer mr.Close()
	ctx := context.Background()

	if err := s.SeedRivals(ctx, rival.Roster); err != nil {
		t.Fatalf("SeedRivals: %v", err)
	}

	entries, _, err := s.LeaderboardTop10(ctx, "")
	if err != nil {
		t.Fatalf("LeaderboardTop10: %v", err)
	}
	if len(entries) != len(rival.Roster) {
		t.Fatalf("want %d entries after seed, got %d", len(rival.Roster), len(entries))
	}

	byID := make(map[string]float64, len(entries))
	for _, e := range entries {
		byID[e.UserID] = e.XP
	}
	for _, r := range rival.Roster {
		got, ok := byID[r.ID]
		if !ok {
			t.Errorf("rival %q missing from leaderboard after seed", r.ID)
			continue
		}
		if int(got) != r.BaseXP {
			t.Errorf("rival %q: want BaseXP %d, got %g", r.ID, r.BaseXP, got)
		}
	}
}

func TestSeedRivals_Idempotent(t *testing.T) {
	s, mr := newRivalStore(t)
	defer mr.Close()
	ctx := context.Background()

	if err := s.SeedRivals(ctx, rival.Roster); err != nil {
		t.Fatalf("first SeedRivals: %v", err)
	}

	// Simulate rivals gaining XP after the first seed.
	if err := s.TickRivals(ctx, rival.Roster); err != nil {
		t.Fatalf("TickRivals: %v", err)
	}

	entries1, _, err := s.LeaderboardTop10(ctx, "")
	if err != nil {
		t.Fatalf("LeaderboardTop10 after tick: %v", err)
	}
	xpAfterTick := make(map[string]float64, len(entries1))
	for _, e := range entries1 {
		xpAfterTick[e.UserID] = e.XP
	}

	// A second seed must not overwrite the ticked XP.
	if err := s.SeedRivals(ctx, rival.Roster); err != nil {
		t.Fatalf("second SeedRivals: %v", err)
	}

	entries2, _, err := s.LeaderboardTop10(ctx, "")
	if err != nil {
		t.Fatalf("LeaderboardTop10 after second seed: %v", err)
	}
	for _, e := range entries2 {
		if e.XP != xpAfterTick[e.UserID] {
			t.Errorf("rival %q: second seed overwrote XP (want %g, got %g)",
				e.UserID, xpAfterTick[e.UserID], e.XP)
		}
	}
}

func TestTickRivals_IncreasesScore(t *testing.T) {
	s, mr := newRivalStore(t)
	defer mr.Close()
	ctx := context.Background()

	if err := s.SeedRivals(ctx, rival.Roster); err != nil {
		t.Fatalf("SeedRivals: %v", err)
	}

	before, _, err := s.LeaderboardTop10(ctx, "")
	if err != nil {
		t.Fatalf("LeaderboardTop10 before tick: %v", err)
	}
	xpBefore := make(map[string]float64, len(before))
	for _, e := range before {
		xpBefore[e.UserID] = e.XP
	}

	if err := s.TickRivals(ctx, rival.Roster); err != nil {
		t.Fatalf("TickRivals: %v", err)
	}

	after, _, err := s.LeaderboardTop10(ctx, "")
	if err != nil {
		t.Fatalf("LeaderboardTop10 after tick: %v", err)
	}
	for _, e := range after {
		if !rival.IsRival(e.UserID) {
			continue
		}
		if e.XP <= xpBefore[e.UserID] {
			t.Errorf("rival %q: XP did not increase after tick (before %g, after %g)",
				e.UserID, xpBefore[e.UserID], e.XP)
		}
	}
}

func TestTickRivals_GainWithinRange(t *testing.T) {
	s, mr := newRivalStore(t)
	defer mr.Close()
	ctx := context.Background()

	if err := s.SeedRivals(ctx, rival.Roster); err != nil {
		t.Fatalf("SeedRivals: %v", err)
	}

	before, _, err := s.LeaderboardTop10(ctx, "")
	if err != nil {
		t.Fatalf("LeaderboardTop10 before: %v", err)
	}
	xpBefore := make(map[string]float64, len(before))
	for _, e := range before {
		xpBefore[e.UserID] = e.XP
	}

	if err := s.TickRivals(ctx, rival.Roster); err != nil {
		t.Fatalf("TickRivals: %v", err)
	}

	after, _, err := s.LeaderboardTop10(ctx, "")
	if err != nil {
		t.Fatalf("LeaderboardTop10 after: %v", err)
	}
	xpAfter := make(map[string]float64, len(after))
	for _, e := range after {
		xpAfter[e.UserID] = e.XP
	}

	for _, r := range rival.Roster {
		gain := int(xpAfter[r.ID] - xpBefore[r.ID])
		if gain < r.DailyGainMin || gain > r.DailyGainMax {
			t.Errorf("rival %q: gain %d out of range [%d, %d]",
				r.ID, gain, r.DailyGainMin, r.DailyGainMax)
		}
	}
}

func TestLeaderboardTop10_MarksRivals(t *testing.T) {
	s, mr := newRivalStore(t)
	defer mr.Close()
	ctx := context.Background()

	if err := s.SeedRivals(ctx, rival.Roster); err != nil {
		t.Fatalf("SeedRivals: %v", err)
	}
	// Add a real user alongside the rivals.
	if err := s.LeaderboardUpdate(ctx, "user-alice", 999); err != nil {
		t.Fatalf("LeaderboardUpdate: %v", err)
	}

	entries, userRank, err := s.LeaderboardTop10(ctx, "user-alice")
	if err != nil {
		t.Fatalf("LeaderboardTop10: %v", err)
	}

	for _, e := range entries {
		wantRival := rival.IsRival(e.UserID)
		if e.IsRival != wantRival {
			t.Errorf("entry %q: IsRival=%v, want %v", e.UserID, e.IsRival, wantRival)
		}
	}

	if userRank == nil {
		t.Fatal("user-alice rank is nil")
	}
	if userRank.IsRival {
		t.Error("user-alice should not be marked as rival")
	}
}

func TestSeedRivals_EmptySlice(t *testing.T) {
	s, mr := newRivalStore(t)
	defer mr.Close()
	ctx := context.Background()

	if err := s.SeedRivals(ctx, nil); err != nil {
		t.Errorf("SeedRivals(nil) should be a no-op, got error: %v", err)
	}
	if err := s.SeedRivals(ctx, []rival.Rival{}); err != nil {
		t.Errorf("SeedRivals(empty) should be a no-op, got error: %v", err)
	}
}
