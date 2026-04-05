package store_test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/teacherslounge/gaming-service/internal/quest"
	"github.com/teacherslounge/gaming-service/internal/store"
)

// questStore creates a Store with only the Redis client wired up.
// Postgres methods are not called in these tests.
func questStore(t *testing.T, mr *miniredis.Miniredis) *store.Store {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return store.New(nil, rdb)
}

func TestGetDailyQuests_FreshUser(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	s := questStore(t, mr)
	quests, err := s.GetDailyQuests(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetDailyQuests: %v", err)
	}

	if len(quests) != 3 {
		t.Fatalf("expected 3 quests, got %d", len(quests))
	}

	for _, q := range quests {
		if q.Progress != 0 {
			t.Errorf("quest %q: fresh user should have Progress=0, got %d", q.ID, q.Progress)
		}
		if q.Completed {
			t.Errorf("quest %q: fresh user should not be completed", q.ID)
		}
	}
}

func TestUpdateQuestProgress_AdvancesProgress(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	s := questStore(t, mr)
	ctx := context.Background()
	userID := "user-advance"

	// "question_answered" advances "questions_answered" (target: 5)
	quests, xpEarned, gemsEarned, err := s.UpdateQuestProgress(ctx, userID, "question_answered")
	if err != nil {
		t.Fatalf("UpdateQuestProgress: %v", err)
	}

	if xpEarned != 0 || gemsEarned != 0 {
		t.Errorf("first progress should not earn rewards yet, got xp=%d gems=%d", xpEarned, gemsEarned)
	}

	var found bool
	for _, q := range quests {
		if q.ID == "questions_answered" {
			found = true
			if q.Progress != 1 {
				t.Errorf("expected progress=1, got %d", q.Progress)
			}
			if q.Completed {
				t.Errorf("quest should not be complete after 1/5")
			}
		}
	}
	if !found {
		t.Error("questions_answered quest not in response")
	}
}

func TestUpdateQuestProgress_CompletionDetection(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	s := questStore(t, mr)
	ctx := context.Background()
	userID := "user-complete"

	def := quest.ByID("questions_answered")
	if def == nil {
		t.Fatal("questions_answered quest definition not found")
	}

	var lastXP, lastGems int
	for i := 0; i < def.Target; i++ {
		_, lastXP, lastGems, _ = s.UpdateQuestProgress(ctx, userID, "question_answered")
	}

	if lastXP != def.XPReward {
		t.Errorf("completion should award %d XP, got %d", def.XPReward, lastXP)
	}
	if lastGems != def.GemsReward {
		t.Errorf("completion should award %d gems, got %d", def.GemsReward, lastGems)
	}

	// Verify completed flag is set
	quests, err := s.GetDailyQuests(ctx, userID)
	if err != nil {
		t.Fatalf("GetDailyQuests: %v", err)
	}
	for _, q := range quests {
		if q.ID == "questions_answered" {
			if !q.Completed {
				t.Error("quest should be marked completed")
			}
			if q.Progress < def.Target {
				t.Errorf("progress should be >= %d, got %d", def.Target, q.Progress)
			}
		}
	}
}

func TestUpdateQuestProgress_NoDoubleReward(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	s := questStore(t, mr)
	ctx := context.Background()
	userID := "user-nodbl"

	// Complete the single-step quest
	if _, _, _, err := s.UpdateQuestProgress(ctx, userID, "streak_checkin"); err != nil {
		t.Fatalf("first UpdateQuestProgress: %v", err)
	}

	// Call again — should not award again
	_, xpEarned, gemsEarned, err := s.UpdateQuestProgress(ctx, userID, "streak_checkin")
	if err != nil {
		t.Fatalf("second UpdateQuestProgress: %v", err)
	}
	if xpEarned != 0 || gemsEarned != 0 {
		t.Errorf("double-reward: got xp=%d gems=%d, want 0,0", xpEarned, gemsEarned)
	}
}

func TestUpdateQuestProgress_UnknownAction(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	s := questStore(t, mr)
	ctx := context.Background()

	quests, xpEarned, gemsEarned, err := s.UpdateQuestProgress(ctx, "user-unk", "invalid_action")
	if err != nil {
		t.Fatalf("UpdateQuestProgress with unknown action: %v", err)
	}
	if xpEarned != 0 || gemsEarned != 0 {
		t.Errorf("unknown action should not award rewards, got xp=%d gems=%d", xpEarned, gemsEarned)
	}
	if len(quests) != 3 {
		t.Errorf("expected 3 quests in response, got %d", len(quests))
	}
}

func TestUpdateQuestProgress_AllQuestTypes(t *testing.T) {
	tests := []struct {
		action  string
		questID string
	}{
		{"question_answered", "questions_answered"},
		{"streak_checkin", "keep_streak_alive"},
		{"concept_mastered", "master_new_concept"},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			mr, err := miniredis.Run()
			if err != nil {
				t.Fatalf("miniredis: %v", err)
			}
			defer mr.Close()

			s := questStore(t, mr)
			ctx := context.Background()

			def := quest.ByID(tt.questID)
			if def == nil {
				t.Fatalf("quest definition %q not found", tt.questID)
			}

			var totalXP, totalGems int
			for i := 0; i < def.Target; i++ {
				_, xp, gems, err := s.UpdateQuestProgress(ctx, "user-"+tt.action, tt.action)
				if err != nil {
					t.Fatalf("UpdateQuestProgress: %v", err)
				}
				totalXP += xp
				totalGems += gems
			}

			if totalXP != def.XPReward {
				t.Errorf("total XP = %d, want %d", totalXP, def.XPReward)
			}
			if totalGems != def.GemsReward {
				t.Errorf("total gems = %d, want %d", totalGems, def.GemsReward)
			}
		})
	}
}
