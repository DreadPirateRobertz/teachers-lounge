package handler_test

// Tests for gaming-service → notification-service achievement push hooks.
//
// The hooks fire in fire-and-forget goroutines; tests use a buffered channel
// in recordingNotifier to observe dispatches without sleep-based races. A
// small helper waits on the channel with a timeout so the test fails fast
// if no push was dispatched.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/handler"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/notifier"
)

// levelUpCall records a NotifyLevelUp invocation.
type levelUpCall struct {
	userID   string
	newLevel int
}

// questCompleteCall records a NotifyQuestComplete invocation.
type questCompleteCall struct {
	userID, title string
	xpReward      int
}

// recordingNotifier is a test double that pushes each invocation onto a
// buffered channel so tests can observe dispatches deterministically.
type recordingNotifier struct {
	levelUp  chan levelUpCall
	quest    chan questCompleteCall
	errLevel error
	errQuest error
}

func newRecordingNotifier() *recordingNotifier {
	return &recordingNotifier{
		levelUp: make(chan levelUpCall, 8),
		quest:   make(chan questCompleteCall, 8),
	}
}

func (r *recordingNotifier) NotifyLevelUp(_ context.Context, userID string, newLevel int) error {
	r.levelUp <- levelUpCall{userID: userID, newLevel: newLevel}
	return r.errLevel
}

func (r *recordingNotifier) NotifyQuestComplete(_ context.Context, userID, title string, xpReward int) error {
	r.quest <- questCompleteCall{userID: userID, title: title, xpReward: xpReward}
	return r.errQuest
}

// waitLevelUp blocks for up to 500ms for a level-up dispatch, failing the
// test if nothing arrives.
func waitLevelUp(t *testing.T, r *recordingNotifier) levelUpCall {
	t.Helper()
	select {
	case c := <-r.levelUp:
		return c
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected NotifyLevelUp dispatch, none received within 500ms")
		return levelUpCall{}
	}
}

// waitQuestComplete blocks for up to 500ms for a quest-complete dispatch.
func waitQuestComplete(t *testing.T, r *recordingNotifier) questCompleteCall {
	t.Helper()
	select {
	case c := <-r.quest:
		return c
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected NotifyQuestComplete dispatch, none received within 500ms")
		return questCompleteCall{}
	}
}

// assertNoLevelUp drains a brief window to confirm no dispatch occurred.
func assertNoLevelUp(t *testing.T, r *recordingNotifier) {
	t.Helper()
	select {
	case c := <-r.levelUp:
		t.Fatalf("unexpected NotifyLevelUp dispatch: %+v", c)
	case <-time.After(75 * time.Millisecond):
	}
}

func assertNoQuestComplete(t *testing.T, r *recordingNotifier) {
	t.Helper()
	select {
	case c := <-r.quest:
		t.Fatalf("unexpected NotifyQuestComplete dispatch: %+v", c)
	case <-time.After(75 * time.Millisecond):
	}
}

// newHandlerWithNotifier builds a handler wired to the given notifier.
func newHandlerWithNotifier(s handler.Storer, n notifier.Notifier) *handler.Handler {
	return handler.New(s, nil, zap.NewNop(), handler.WithNotifier(n))
}

// ── GainXP → NotifyLevelUp ────────────────────────────────────────────────────

// TestGainXP_FiresLevelUpPush verifies a level-up push is dispatched when the
// new XP crosses a level boundary.
func TestGainXP_FiresLevelUpPush(t *testing.T) {
	// Start at XP 95, level 1 — an amount of 50 pushes past the L2 threshold (100 XP).
	s := &xpStore{
		getXPFn: func(_ context.Context, _ string) (int64, int, error) { return 490, 1, nil },
	}
	rec := newRecordingNotifier()
	h := newHandlerWithNotifier(s, rec)

	body, _ := json.Marshal(model.GainXPRequest{UserID: "u-leveler", Amount: 50})
	req := httptest.NewRequest(http.MethodPost, "/gaming/xp", bytes.NewBuffer(body))
	req = withUser(req, "u-leveler")
	rr := httptest.NewRecorder()

	h.GainXP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body)
	}
	got := waitLevelUp(t, rec)
	if got.userID != "u-leveler" {
		t.Errorf("user_id = %q, want u-leveler", got.userID)
	}
	if got.newLevel < 2 {
		t.Errorf("new_level = %d, want ≥ 2", got.newLevel)
	}
}

// TestGainXP_NoLevelUpNoPush verifies no push is dispatched when XP does not
// cross a level boundary.
func TestGainXP_NoLevelUpNoPush(t *testing.T) {
	s := &xpStore{
		getXPFn: func(_ context.Context, _ string) (int64, int, error) { return 0, 1, nil },
	}
	rec := newRecordingNotifier()
	h := newHandlerWithNotifier(s, rec)

	body, _ := json.Marshal(model.GainXPRequest{UserID: "u1", Amount: 5})
	req := httptest.NewRequest(http.MethodPost, "/gaming/xp", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()

	h.GainXP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	assertNoLevelUp(t, rec)
}

// TestGainXP_UpsertFailureSuppressesPush ensures we don't fire a push when
// the store write fails (the user didn't actually level up from the server's POV).
func TestGainXP_UpsertFailureSuppressesPush(t *testing.T) {
	s := &xpStore{
		getXPFn:    func(_ context.Context, _ string) (int64, int, error) { return 95, 1, nil },
		upsertXPFn: func(_ context.Context, _ string, _ int64, _ int) error { return errors.New("db fail") },
	}
	rec := newRecordingNotifier()
	h := newHandlerWithNotifier(s, rec)

	body, _ := json.Marshal(model.GainXPRequest{UserID: "u1", Amount: 50})
	req := httptest.NewRequest(http.MethodPost, "/gaming/xp", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()

	h.GainXP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on upsert error, got %d", rr.Code)
	}
	assertNoLevelUp(t, rec)
}

// TestGainXP_NoNotifier_NoPanic verifies the handler runs without a notifier.
func TestGainXP_NoNotifier_NoPanic(t *testing.T) {
	s := &xpStore{
		getXPFn: func(_ context.Context, _ string) (int64, int, error) { return 490, 1, nil },
	}
	// No WithNotifier option — notifier is nil.
	h := handler.New(s, nil, zap.NewNop())

	body, _ := json.Marshal(model.GainXPRequest{UserID: "u1", Amount: 50})
	req := httptest.NewRequest(http.MethodPost, "/gaming/xp", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()

	h.GainXP(rr, req) // must not panic

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

// ── UpdateQuestProgress → NotifyQuestComplete ─────────────────────────────────

// TestUpdateQuestProgress_FiresQuestCompletePush verifies a push per quest
// that transitions from incomplete → complete during this call.
func TestUpdateQuestProgress_FiresQuestCompletePush(t *testing.T) {
	s := &questStore{
		getDailyQuestsFn: func(_ context.Context, _ string) ([]model.QuestState, error) {
			// Pre-state: q1 not yet complete.
			return []model.QuestState{
				{ID: "q1", Title: "Answer 5", Progress: 4, Target: 5, Completed: false, XPReward: 100},
			}, nil
		},
		updateQuestFn: func(_ context.Context, _, _ string) ([]model.QuestState, int, int, error) {
			// Post-state: q1 now complete; 100 XP earned.
			return []model.QuestState{
				{ID: "q1", Title: "Answer 5", Progress: 5, Target: 5, Completed: true, XPReward: 100},
			}, 100, 0, nil
		},
	}
	rec := newRecordingNotifier()
	h := newHandlerWithNotifier(s, rec)

	body, _ := json.Marshal(model.QuestProgressRequest{UserID: "u1", Action: "question_answered"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quests/progress", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()

	h.UpdateQuestProgress(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body)
	}
	got := waitQuestComplete(t, rec)
	if got.userID != "u1" || got.title != "Answer 5" || got.xpReward != 100 {
		t.Errorf("dispatch = %+v, want {u1 Answer 5 100}", got)
	}
}

// TestUpdateQuestProgress_SkipsAlreadyCompletedQuests verifies that a quest
// already completed before the call does NOT re-fire a push.
func TestUpdateQuestProgress_SkipsAlreadyCompletedQuests(t *testing.T) {
	s := &questStore{
		getDailyQuestsFn: func(_ context.Context, _ string) ([]model.QuestState, error) {
			return []model.QuestState{
				{ID: "q1", Title: "Already Done", Completed: true, XPReward: 100},
				{ID: "q2", Title: "Still Working", Progress: 3, Target: 5, Completed: false, XPReward: 50},
			}, nil
		},
		updateQuestFn: func(_ context.Context, _, _ string) ([]model.QuestState, int, int, error) {
			// q1 still completed (unchanged), q2 advances but doesn't finish.
			return []model.QuestState{
				{ID: "q1", Title: "Already Done", Completed: true, XPReward: 100},
				{ID: "q2", Title: "Still Working", Progress: 4, Target: 5, Completed: false, XPReward: 50},
			}, 0, 0, nil
		},
	}
	rec := newRecordingNotifier()
	h := newHandlerWithNotifier(s, rec)

	body, _ := json.Marshal(model.QuestProgressRequest{UserID: "u1", Action: "question_answered"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quests/progress", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()

	h.UpdateQuestProgress(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	assertNoQuestComplete(t, rec)
}

// TestUpdateQuestProgress_MultipleNewCompletions_FiresOnePerQuest verifies
// that two quests completing in the same call produce two separate pushes.
func TestUpdateQuestProgress_MultipleNewCompletions_FiresOnePerQuest(t *testing.T) {
	s := &questStore{
		getDailyQuestsFn: func(_ context.Context, _ string) ([]model.QuestState, error) {
			return []model.QuestState{
				{ID: "qA", Title: "Quest A", Progress: 4, Target: 5, Completed: false, XPReward: 50},
				{ID: "qB", Title: "Quest B", Progress: 0, Target: 1, Completed: false, XPReward: 30},
			}, nil
		},
		updateQuestFn: func(_ context.Context, _, _ string) ([]model.QuestState, int, int, error) {
			return []model.QuestState{
				{ID: "qA", Title: "Quest A", Progress: 5, Target: 5, Completed: true, XPReward: 50},
				{ID: "qB", Title: "Quest B", Progress: 1, Target: 1, Completed: true, XPReward: 30},
			}, 80, 0, nil
		},
		awardQuestRewardsFn: func(_ context.Context, _ string, _, _ int) (int64, int, bool, int, error) {
			return 180, 2, false, 0, nil
		},
	}
	rec := newRecordingNotifier()
	h := newHandlerWithNotifier(s, rec)

	body, _ := json.Marshal(model.QuestProgressRequest{UserID: "u1", Action: "streak_checkin"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quests/progress", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()

	h.UpdateQuestProgress(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body)
	}

	seen := map[string]int{}
	for i := 0; i < 2; i++ {
		got := waitQuestComplete(t, rec)
		seen[got.title] = got.xpReward
	}
	if seen["Quest A"] != 50 {
		t.Errorf("expected Quest A push with xp=50, got %v", seen)
	}
	if seen["Quest B"] != 30 {
		t.Errorf("expected Quest B push with xp=30, got %v", seen)
	}
	assertNoQuestComplete(t, rec)
}

// TestUpdateQuestProgress_FiresLevelUpOnRewardLevel verifies that when quest
// rewards cause a level-up, a level-up push fires too.
func TestUpdateQuestProgress_FiresLevelUpOnRewardLevel(t *testing.T) {
	s := &questStore{
		getDailyQuestsFn: func(_ context.Context, _ string) ([]model.QuestState, error) {
			return []model.QuestState{
				{ID: "q1", Title: "Final Push", Progress: 4, Target: 5, Completed: false, XPReward: 100},
			}, nil
		},
		updateQuestFn: func(_ context.Context, _, _ string) ([]model.QuestState, int, int, error) {
			return []model.QuestState{
				{ID: "q1", Title: "Final Push", Progress: 5, Target: 5, Completed: true, XPReward: 100},
			}, 100, 0, nil
		},
		awardQuestRewardsFn: func(_ context.Context, _ string, _, _ int) (int64, int, bool, int, error) {
			return 250, 3, true, 0, nil // leveledUp=true, newLevel=3
		},
	}
	rec := newRecordingNotifier()
	h := newHandlerWithNotifier(s, rec)

	body, _ := json.Marshal(model.QuestProgressRequest{UserID: "u1", Action: "question_answered"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quests/progress", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()

	h.UpdateQuestProgress(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	gotQuest := waitQuestComplete(t, rec)
	if gotQuest.title != "Final Push" {
		t.Errorf("quest push title = %q, want Final Push", gotQuest.title)
	}
	gotLevel := waitLevelUp(t, rec)
	if gotLevel.newLevel != 3 {
		t.Errorf("level push new_level = %d, want 3", gotLevel.newLevel)
	}
}

// TestUpdateQuestProgress_PreStateError_StillProcessesRequest verifies a
// failed pre-state snapshot degrades to "no pushes" without failing the request.
func TestUpdateQuestProgress_PreStateError_StillProcessesRequest(t *testing.T) {
	s := &questStore{
		getDailyQuestsFn: func(_ context.Context, _ string) ([]model.QuestState, error) {
			return nil, errors.New("redis down")
		},
		updateQuestFn: func(_ context.Context, _, _ string) ([]model.QuestState, int, int, error) {
			return []model.QuestState{
				{ID: "q1", Title: "A", Completed: true, XPReward: 50},
			}, 50, 0, nil
		},
		awardQuestRewardsFn: func(_ context.Context, _ string, _, _ int) (int64, int, bool, int, error) {
			return 100, 2, false, 0, nil
		},
	}
	rec := newRecordingNotifier()
	h := newHandlerWithNotifier(s, rec)

	body, _ := json.Marshal(model.QuestProgressRequest{UserID: "u1", Action: "question_answered"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quests/progress", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()

	h.UpdateQuestProgress(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 despite pre-state error, got %d", rr.Code)
	}
	// No pushes because pre-state diff was unavailable; this is the intended
	// degraded behavior (notification-service dedup would already suppress
	// duplicates if we fired blindly).
	assertNoQuestComplete(t, rec)
}

// TestUpdateQuestProgress_UpdateFailureSuppressesPush ensures that when the
// store write fails, no push is dispatched (the user didn't actually complete).
func TestUpdateQuestProgress_UpdateFailureSuppressesPush(t *testing.T) {
	s := &questStore{
		getDailyQuestsFn: func(_ context.Context, _ string) ([]model.QuestState, error) {
			return []model.QuestState{
				{ID: "q1", Title: "x", Completed: false, XPReward: 50},
			}, nil
		},
		updateQuestFn: func(_ context.Context, _, _ string) ([]model.QuestState, int, int, error) {
			return nil, 0, 0, errors.New("redis timeout")
		},
	}
	rec := newRecordingNotifier()
	h := newHandlerWithNotifier(s, rec)

	body, _ := json.Marshal(model.QuestProgressRequest{UserID: "u1", Action: "streak_checkin"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quests/progress", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()

	h.UpdateQuestProgress(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
	assertNoQuestComplete(t, rec)
}

// TestUpdateQuestProgress_AwardFailureSuppressesPushes ensures that when
// AwardQuestRewards fails after the quest advances, no pushes are fired
// (the 500 response means the user's state is ambiguous).
func TestUpdateQuestProgress_AwardFailureSuppressesPushes(t *testing.T) {
	s := &questStore{
		getDailyQuestsFn: func(_ context.Context, _ string) ([]model.QuestState, error) {
			return []model.QuestState{
				{ID: "q1", Title: "x", Progress: 4, Target: 5, Completed: false, XPReward: 50},
			}, nil
		},
		updateQuestFn: func(_ context.Context, _, _ string) ([]model.QuestState, int, int, error) {
			return []model.QuestState{
				{ID: "q1", Title: "x", Progress: 5, Target: 5, Completed: true, XPReward: 50},
			}, 50, 0, nil
		},
		awardQuestRewardsFn: func(_ context.Context, _ string, _, _ int) (int64, int, bool, int, error) {
			return 0, 0, false, 0, errors.New("ledger down")
		},
	}
	rec := newRecordingNotifier()
	h := newHandlerWithNotifier(s, rec)

	body, _ := json.Marshal(model.QuestProgressRequest{UserID: "u1", Action: "question_answered"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quests/progress", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()

	h.UpdateQuestProgress(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
	assertNoQuestComplete(t, rec)
	assertNoLevelUp(t, rec)
}

// TestNotifier_QuestCompleteErrorIsLoggedNotPropagated exercises the error
// path in notifyQuestComplete — the dispatch is attempted but the handler
// still returns 200.
func TestNotifier_QuestCompleteErrorIsLoggedNotPropagated(t *testing.T) {
	s := &questStore{
		getDailyQuestsFn: func(_ context.Context, _ string) ([]model.QuestState, error) {
			return []model.QuestState{
				{ID: "q1", Title: "x", Progress: 4, Target: 5, Completed: false, XPReward: 50},
			}, nil
		},
		updateQuestFn: func(_ context.Context, _, _ string) ([]model.QuestState, int, int, error) {
			return []model.QuestState{
				{ID: "q1", Title: "x", Progress: 5, Target: 5, Completed: true, XPReward: 50},
			}, 50, 0, nil
		},
		awardQuestRewardsFn: func(_ context.Context, _ string, _, _ int) (int64, int, bool, int, error) {
			return 100, 1, false, 0, nil
		},
	}
	rec := newRecordingNotifier()
	rec.errQuest = errors.New("notif-service 503")
	h := newHandlerWithNotifier(s, rec)

	body, _ := json.Marshal(model.QuestProgressRequest{UserID: "u1", Action: "question_answered"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/quests/progress", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()

	h.UpdateQuestProgress(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("notifier errors must not fail the request; got %d", rr.Code)
	}
	_ = waitQuestComplete(t, rec)
}

// TestNotifier_ErrorDoesNotBlockResponse verifies the handler returns 200
// even if the notifier returns an error (fire-and-forget semantics).
func TestNotifier_ErrorDoesNotBlockResponse(t *testing.T) {
	s := &xpStore{
		getXPFn: func(_ context.Context, _ string) (int64, int, error) { return 490, 1, nil },
	}
	rec := newRecordingNotifier()
	rec.errLevel = errors.New("notif-service 503")
	h := newHandlerWithNotifier(s, rec)

	body, _ := json.Marshal(model.GainXPRequest{UserID: "u1", Amount: 50})
	req := httptest.NewRequest(http.MethodPost, "/gaming/xp", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()

	h.GainXP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("notifier errors must not fail the request; got %d", rr.Code)
	}
	_ = waitLevelUp(t, rec) // push still attempted
}
