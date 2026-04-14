package store_test

import (
	"context"
	"testing"

	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/rival"
	"github.com/teacherslounge/gaming-service/internal/store"
)

func newLBStore(t *testing.T) *store.Store {
	t.Helper()
	return newMasteryStore(t, &txDB{})
}

func addXP(t *testing.T, s *store.Store, userID string, xp int64) {
	t.Helper()
	if err := s.LeaderboardUpdate(context.Background(), userID, xp); err != nil {
		t.Fatalf("LeaderboardUpdate(%q, %d): %v", userID, xp, err)
	}
}

func TestLeaderboardUpdateCourse_AppearsInCourseBoard(t *testing.T) {
	s := newLBStore(t)
	ctx := context.Background()
	if err := s.LeaderboardUpdateCourse(ctx, "alice", "course-go-101", 500); err != nil {
		t.Fatalf("LeaderboardUpdateCourse: %v", err)
	}
	entries, _, err := s.LeaderboardGetCourse(ctx, "alice", "course-go-101")
	if err != nil {
		t.Fatalf("LeaderboardGetCourse: %v", err)
	}
	if len(entries) != 1 || entries[0].UserID != "alice" {
		t.Errorf("expected alice in course board, got %v", entries)
	}
}

func TestLeaderboardUpdateCourse_IsolatedFromGlobal(t *testing.T) {
	s := newLBStore(t)
	ctx := context.Background()
	if err := s.LeaderboardUpdateCourse(ctx, "bob", "course-python-201", 1000); err != nil {
		t.Fatalf("LeaderboardUpdateCourse: %v", err)
	}
	global, _, err := s.LeaderboardTop10(ctx, "bob")
	if err != nil {
		t.Fatalf("LeaderboardTop10: %v", err)
	}
	for _, e := range global {
		if e.UserID == "bob" {
			t.Errorf("bob should not appear in global board from course update")
		}
	}
}

func TestLeaderboardGetPeriod_WeeklyReturnsCurrentWeek(t *testing.T) {
	s := newLBStore(t)
	ctx := context.Background()
	addXP(t, s, "alice", 300)
	addXP(t, s, "bob", 200)
	entries, _, err := s.LeaderboardGetPeriod(ctx, "alice", model.PeriodWeekly)
	if err != nil {
		t.Fatalf("LeaderboardGetPeriod weekly: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected entries in weekly board")
	}
	if entries[0].UserID != "alice" {
		t.Errorf("rank 1: want alice, got %s", entries[0].UserID)
	}
}

func TestLeaderboardGetPeriod_MonthlyReturnsCurrentMonth(t *testing.T) {
	s := newLBStore(t)
	ctx := context.Background()
	addXP(t, s, "carol", 800)
	addXP(t, s, "dave", 400)
	entries, _, err := s.LeaderboardGetPeriod(ctx, "carol", model.PeriodMonthly)
	if err != nil {
		t.Fatalf("LeaderboardGetPeriod monthly: %v", err)
	}
	if len(entries) == 0 || entries[0].UserID != "carol" {
		t.Errorf("want carol as rank 1, got %v", entries)
	}
}

func TestLeaderboardGetPeriod_AllTimeFallsBackToGlobal(t *testing.T) {
	s := newLBStore(t)
	ctx := context.Background()
	addXP(t, s, "eve", 600)
	entries, _, err := s.LeaderboardGetPeriod(ctx, "eve", "all_time")
	if err != nil {
		t.Fatalf("LeaderboardGetPeriod all_time: %v", err)
	}
	found := false
	for _, e := range entries {
		if e.UserID == "eve" {
			found = true
			break
		}
	}
	if !found {
		t.Error("eve not found in all_time leaderboard")
	}
}

func TestLeaderboardGetPeriod_UserRankIncluded(t *testing.T) {
	s := newLBStore(t)
	ctx := context.Background()
	addXP(t, s, "alice", 500)
	addXP(t, s, "bob", 300)
	_, userRank, err := s.LeaderboardGetPeriod(ctx, "bob", model.PeriodWeekly)
	if err != nil {
		t.Fatalf("LeaderboardGetPeriod: %v", err)
	}
	if userRank == nil || userRank.Rank != 2 {
		t.Errorf("bob rank: want 2, got %v", userRank)
	}
}

func TestLeaderboardGetCourse_RankedByXP(t *testing.T) {
	s := newLBStore(t)
	ctx := context.Background()
	const course = "course-chem-101"
	for _, tc := range []struct{ id string; xp float64 }{
		{"alice", 700}, {"bob", 400}, {"carol", 900},
	} {
		if err := s.LeaderboardUpdateCourse(ctx, tc.id, course, int64(tc.xp)); err != nil {
			t.Fatalf("course update %s: %v", tc.id, err)
		}
	}
	entries, _, err := s.LeaderboardGetCourse(ctx, "alice", course)
	if err != nil {
		t.Fatalf("LeaderboardGetCourse: %v", err)
	}
	if len(entries) < 3 || entries[0].UserID != "carol" {
		t.Errorf("expected carol at rank 1, got %v", entries)
	}
}

func TestLeaderboardGetCourse_EmptyBoard(t *testing.T) {
	s := newLBStore(t)
	entries, _, err := s.LeaderboardGetCourse(context.Background(), "x", "course-empty")
	if err != nil {
		t.Fatalf("LeaderboardGetCourse: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty board, got %d", len(entries))
	}
}

func TestLeaderboardGetFriends_RankedByGlobalXP(t *testing.T) {
	s := newLBStore(t)
	ctx := context.Background()
	addXP(t, s, "alice", 800)
	addXP(t, s, "bob", 600)
	addXP(t, s, "carol", 400)
	entries, userRank, err := s.LeaderboardGetFriends(ctx, "alice", []string{"bob", "carol"})
	if err != nil {
		t.Fatalf("LeaderboardGetFriends: %v", err)
	}
	if len(entries) != 3 || entries[0].UserID != "alice" {
		t.Errorf("want alice at rank 1, got %v", entries)
	}
	if userRank == nil || userRank.Rank != 1 {
		t.Errorf("alice rank: want 1, got %v", userRank)
	}
}

func TestLeaderboardGetFriends_UnknownUser_ScoreZero(t *testing.T) {
	s := newLBStore(t)
	ctx := context.Background()
	addXP(t, s, "alice", 500)
	entries, _, err := s.LeaderboardGetFriends(ctx, "alice", []string{"nobody"})
	if err != nil {
		t.Fatalf("LeaderboardGetFriends: %v", err)
	}
	for _, e := range entries {
		if e.UserID == "nobody" && e.XP != 0 {
			t.Errorf("unknown user XP: want 0, got %v", e.XP)
		}
	}
}

func TestLeaderboardGetFriends_NoFriends_CallerOnly(t *testing.T) {
	s := newLBStore(t)
	ctx := context.Background()
	addXP(t, s, "solo", 300)
	entries, userRank, err := s.LeaderboardGetFriends(ctx, "solo", nil)
	if err != nil {
		t.Fatalf("LeaderboardGetFriends: %v", err)
	}
	if len(entries) != 1 || entries[0].UserID != "solo" {
		t.Errorf("want [solo], got %v", entries)
	}
	if userRank == nil || userRank.Rank != 1 {
		t.Errorf("userRank: want rank 1, got %v", userRank)
	}
}

func TestSeedRivals_EmptySlice_NoOp(t *testing.T) {
	s := newLBStore(t)
	if err := s.SeedRivals(context.Background(), nil); err != nil {
		t.Fatalf("SeedRivals(nil): %v", err)
	}
}

func TestSeedRivals_PreservesExistingScore(t *testing.T) {
	s := newLBStore(t)
	ctx := context.Background()
	r := rival.Rival{ID: "rival:test-rival", BaseXP: 100, DailyGainMin: 5, DailyGainMax: 10}
	if err := s.SeedRivals(ctx, []rival.Rival{r}); err != nil {
		t.Fatalf("first SeedRivals: %v", err)
	}
	if err := s.LeaderboardUpdate(ctx, r.ID, 500); err != nil {
		t.Fatalf("LeaderboardUpdate: %v", err)
	}
	if err := s.SeedRivals(ctx, []rival.Rival{r}); err != nil {
		t.Fatalf("second SeedRivals: %v", err)
	}
	entries, _, err := s.LeaderboardTop10(ctx, "")
	if err != nil {
		t.Fatalf("LeaderboardTop10: %v", err)
	}
	for _, e := range entries {
		if e.UserID == r.ID && e.XP != 500 {
			t.Errorf("rival score: want 500, got %v", e.XP)
		}
	}
}

func TestTickRivals_EmptySlice_NoOp(t *testing.T) {
	s := newLBStore(t)
	if err := s.TickRivals(context.Background(), nil); err != nil {
		t.Fatalf("TickRivals(nil): %v", err)
	}
}
