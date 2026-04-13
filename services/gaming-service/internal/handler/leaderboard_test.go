package handler_test

// Handler-layer tests for the leaderboard endpoints. Store-side coverage
// lives in internal/store/leaderboard_test.go; this file exercises the
// HTTP branches — period routing, course_id optional path, friend-list
// parsing, caller filtering, auth gates, and store-error → 500 mapping.
//
// Uses leaderboardStore, a minimal Storer fake that embeds noopStore
// (defined in flashcard_test.go) and overrides only the leaderboard
// methods. All other Storer methods remain no-ops.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/handler"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/taunt"
)

// ── leaderboardStore fake ─────────────────────────────────────────────────────

// leaderboardStore overrides the 6 Storer methods that back the four
// leaderboard HTTP handlers. Every override records its invocation so tests
// can assert the handler called the right one with the right args.
type leaderboardStore struct {
	noopStore

	// Configurable returns.
	updateErr       error
	updateCourseErr error

	top10Entries   []model.LeaderboardEntry
	top10UserRank  *model.LeaderboardEntry
	top10Err       error

	periodEntries  []model.LeaderboardEntry
	periodUserRank *model.LeaderboardEntry
	periodErr      error

	courseEntries  []model.LeaderboardEntry
	courseUserRank *model.LeaderboardEntry
	courseErr      error

	friendEntries  []model.LeaderboardEntry
	friendUserRank *model.LeaderboardEntry
	friendErr      error

	// Captured invocations.
	updateCalls       int
	updateLastUser    string
	updateLastXP     int64
	updateCourseCalls int
	updateCourseLast  struct {
		userID, courseID string
		xp               int64
	}
	top10Calls    int
	periodCalls   int
	periodLast    string
	courseCalls   int
	courseLast    string
	friendCalls   int
	friendLastIDs []string
}

func (s *leaderboardStore) LeaderboardUpdate(_ context.Context, userID string, xp int64) error {
	s.updateCalls++
	s.updateLastUser = userID
	s.updateLastXP = xp
	return s.updateErr
}

func (s *leaderboardStore) LeaderboardUpdateCourse(_ context.Context, userID, courseID string, xp int64) error {
	s.updateCourseCalls++
	s.updateCourseLast.userID = userID
	s.updateCourseLast.courseID = courseID
	s.updateCourseLast.xp = xp
	return s.updateCourseErr
}

func (s *leaderboardStore) LeaderboardTop10(_ context.Context, _ string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	s.top10Calls++
	return s.top10Entries, s.top10UserRank, s.top10Err
}

func (s *leaderboardStore) LeaderboardGetPeriod(_ context.Context, _, period string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	s.periodCalls++
	s.periodLast = period
	return s.periodEntries, s.periodUserRank, s.periodErr
}

func (s *leaderboardStore) LeaderboardGetCourse(_ context.Context, _, courseID string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	s.courseCalls++
	s.courseLast = courseID
	return s.courseEntries, s.courseUserRank, s.courseErr
}

func (s *leaderboardStore) LeaderboardGetFriends(_ context.Context, _ string, friendIDs []string) ([]model.LeaderboardEntry, *model.LeaderboardEntry, error) {
	s.friendCalls++
	s.friendLastIDs = friendIDs
	return s.friendEntries, s.friendUserRank, s.friendErr
}

// ── helpers ───────────────────────────────────────────────────────────────────

func newLeaderboardHandler(s *leaderboardStore) *handler.Handler {
	return handler.New(s, taunt.StaticGenerator{}, zap.NewNop())
}

// withCourseParam injects a chi URL param (courseId) into the request so
// the handler can chi.URLParam it out.
func withCourseParam(r *http.Request, courseID string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("courseId", courseID)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func decodeLeaderboard(t *testing.T, rr *httptest.ResponseRecorder) model.LeaderboardResponse {
	t.Helper()
	var resp model.LeaderboardResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode leaderboard response: %v", err)
	}
	return resp
}

// ── LeaderboardUpdate (POST /gaming/leaderboard/update) ───────────────────────

func TestLeaderboardUpdate_BadJSON_Returns400(t *testing.T) {
	h := newLeaderboardHandler(&leaderboardStore{})

	req := httptest.NewRequest(http.MethodPost, "/gaming/leaderboard/update", strings.NewReader("{not json"))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.LeaderboardUpdate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestLeaderboardUpdate_MissingUserID_Returns400(t *testing.T) {
	h := newLeaderboardHandler(&leaderboardStore{})

	body, _ := json.Marshal(model.LeaderboardUpdateRequest{XP: 100})
	req := httptest.NewRequest(http.MethodPost, "/gaming/leaderboard/update", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.LeaderboardUpdate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestLeaderboardUpdate_CallerMismatch_Returns403(t *testing.T) {
	// Request says user=u2 but caller is u1 — forbidden.
	h := newLeaderboardHandler(&leaderboardStore{})

	body, _ := json.Marshal(model.LeaderboardUpdateRequest{UserID: "u2", XP: 100})
	req := httptest.NewRequest(http.MethodPost, "/gaming/leaderboard/update", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.LeaderboardUpdate(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestLeaderboardUpdate_NoCaller_Returns403(t *testing.T) {
	h := newLeaderboardHandler(&leaderboardStore{})

	body, _ := json.Marshal(model.LeaderboardUpdateRequest{UserID: "u1", XP: 100})
	req := httptest.NewRequest(http.MethodPost, "/gaming/leaderboard/update", bytes.NewBuffer(body))
	// No WithUserID — caller is unauthenticated.
	rr := httptest.NewRecorder()
	h.LeaderboardUpdate(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestLeaderboardUpdate_NoCourse_OnlyGlobalBoardUpdated(t *testing.T) {
	s := &leaderboardStore{}
	h := newLeaderboardHandler(s)

	body, _ := json.Marshal(model.LeaderboardUpdateRequest{UserID: "u1", XP: 250})
	req := httptest.NewRequest(http.MethodPost, "/gaming/leaderboard/update", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.LeaderboardUpdate(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
	if s.updateCalls != 1 {
		t.Errorf("expected 1 LeaderboardUpdate call, got %d", s.updateCalls)
	}
	if s.updateLastUser != "u1" || s.updateLastXP != 250 {
		t.Errorf("update args: got (%q, %d), want (u1, 250)", s.updateLastUser, s.updateLastXP)
	}
	if s.updateCourseCalls != 0 {
		t.Errorf("expected 0 course updates when course_id empty, got %d", s.updateCourseCalls)
	}
}

func TestLeaderboardUpdate_WithCourse_BothBoardsUpdated(t *testing.T) {
	s := &leaderboardStore{}
	h := newLeaderboardHandler(s)

	body, _ := json.Marshal(model.LeaderboardUpdateRequest{UserID: "u1", XP: 250, CourseID: "chem101"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/leaderboard/update", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.LeaderboardUpdate(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if s.updateCalls != 1 {
		t.Errorf("expected 1 global update, got %d", s.updateCalls)
	}
	if s.updateCourseCalls != 1 {
		t.Errorf("expected 1 course update, got %d", s.updateCourseCalls)
	}
	if s.updateCourseLast.courseID != "chem101" || s.updateCourseLast.xp != 250 {
		t.Errorf("course args: got (%q, %d), want (chem101, 250)", s.updateCourseLast.courseID, s.updateCourseLast.xp)
	}
}

func TestLeaderboardUpdate_StoreError_Returns500(t *testing.T) {
	s := &leaderboardStore{updateErr: errors.New("redis down")}
	h := newLeaderboardHandler(s)

	body, _ := json.Marshal(model.LeaderboardUpdateRequest{UserID: "u1", XP: 100})
	req := httptest.NewRequest(http.MethodPost, "/gaming/leaderboard/update", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.LeaderboardUpdate(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestLeaderboardUpdate_CourseStoreError_Returns500(t *testing.T) {
	s := &leaderboardStore{updateCourseErr: errors.New("course board down")}
	h := newLeaderboardHandler(s)

	body, _ := json.Marshal(model.LeaderboardUpdateRequest{UserID: "u1", XP: 100, CourseID: "c1"})
	req := httptest.NewRequest(http.MethodPost, "/gaming/leaderboard/update", bytes.NewBuffer(body))
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.LeaderboardUpdate(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// ── GetLeaderboard (GET /gaming/leaderboard?period=...) ───────────────────────

func TestGetLeaderboard_NoPeriod_UsesTop10(t *testing.T) {
	s := &leaderboardStore{top10Entries: []model.LeaderboardEntry{{UserID: "a", XP: 1000, Rank: 1}}}
	h := newLeaderboardHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/gaming/leaderboard", nil)
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.GetLeaderboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if s.top10Calls != 1 || s.periodCalls != 0 {
		t.Errorf("empty period must use Top10: top10Calls=%d periodCalls=%d", s.top10Calls, s.periodCalls)
	}
	resp := decodeLeaderboard(t, rr)
	if len(resp.Top10) != 1 || resp.Top10[0].UserID != "a" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestGetLeaderboard_AllTime_UsesTop10(t *testing.T) {
	s := &leaderboardStore{}
	h := newLeaderboardHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/gaming/leaderboard?period="+model.PeriodAllTime, nil)
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.GetLeaderboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if s.top10Calls != 1 || s.periodCalls != 0 {
		t.Errorf("all_time must use Top10: top10Calls=%d periodCalls=%d", s.top10Calls, s.periodCalls)
	}
}

func TestGetLeaderboard_Weekly_UsesGetPeriod(t *testing.T) {
	s := &leaderboardStore{}
	h := newLeaderboardHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/gaming/leaderboard?period="+model.PeriodWeekly, nil)
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.GetLeaderboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if s.periodCalls != 1 || s.top10Calls != 0 {
		t.Errorf("weekly must use GetPeriod: periodCalls=%d top10Calls=%d", s.periodCalls, s.top10Calls)
	}
	if s.periodLast != model.PeriodWeekly {
		t.Errorf("period arg: got %q, want %q", s.periodLast, model.PeriodWeekly)
	}
}

func TestGetLeaderboard_Monthly_UsesGetPeriod(t *testing.T) {
	s := &leaderboardStore{}
	h := newLeaderboardHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/gaming/leaderboard?period="+model.PeriodMonthly, nil)
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.GetLeaderboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if s.periodLast != model.PeriodMonthly {
		t.Errorf("period arg: got %q, want %q", s.periodLast, model.PeriodMonthly)
	}
}

func TestGetLeaderboard_StoreError_Returns500(t *testing.T) {
	s := &leaderboardStore{top10Err: errors.New("redis down")}
	h := newLeaderboardHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/gaming/leaderboard", nil)
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.GetLeaderboard(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// ── GetCourseLeaderboard (GET /gaming/leaderboard/course/{courseId}) ──────────

func TestGetCourseLeaderboard_MissingCourseID_Returns400(t *testing.T) {
	h := newLeaderboardHandler(&leaderboardStore{})

	req := httptest.NewRequest(http.MethodGet, "/gaming/leaderboard/course/", nil)
	req = withUser(req, "u1")
	// No courseId URL param set.
	rr := httptest.NewRecorder()
	h.GetCourseLeaderboard(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestGetCourseLeaderboard_HappyPath_ForwardsCourseID(t *testing.T) {
	s := &leaderboardStore{courseEntries: []model.LeaderboardEntry{{UserID: "a", XP: 500, Rank: 1}}}
	h := newLeaderboardHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/gaming/leaderboard/course/chem101", nil)
	req = withUser(req, "u1")
	req = withCourseParam(req, "chem101")
	rr := httptest.NewRecorder()
	h.GetCourseLeaderboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if s.courseCalls != 1 || s.courseLast != "chem101" {
		t.Errorf("course args: calls=%d last=%q", s.courseCalls, s.courseLast)
	}
	resp := decodeLeaderboard(t, rr)
	if len(resp.Top10) != 1 || resp.Top10[0].UserID != "a" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestGetCourseLeaderboard_StoreError_Returns500(t *testing.T) {
	s := &leaderboardStore{courseErr: errors.New("db down")}
	h := newLeaderboardHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/gaming/leaderboard/course/chem101", nil)
	req = withUser(req, "u1")
	req = withCourseParam(req, "chem101")
	rr := httptest.NewRecorder()
	h.GetCourseLeaderboard(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// ── GetFriendLeaderboard (GET /gaming/leaderboard/friends?friends=...) ────────

func TestGetFriendLeaderboard_NoCaller_Returns403(t *testing.T) {
	h := newLeaderboardHandler(&leaderboardStore{})

	req := httptest.NewRequest(http.MethodGet, "/gaming/leaderboard/friends?friends=a,b", nil)
	// No WithUserID.
	rr := httptest.NewRecorder()
	h.GetFriendLeaderboard(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestGetFriendLeaderboard_ParsesCommaSeparatedList(t *testing.T) {
	s := &leaderboardStore{}
	h := newLeaderboardHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/gaming/leaderboard/friends?friends=alice,bob,carol", nil)
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.GetFriendLeaderboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	want := []string{"alice", "bob", "carol"}
	if len(s.friendLastIDs) != len(want) {
		t.Fatalf("friendLastIDs: got %v, want %v", s.friendLastIDs, want)
	}
	for i, id := range want {
		if s.friendLastIDs[i] != id {
			t.Errorf("friendLastIDs[%d]: got %q, want %q", i, s.friendLastIDs[i], id)
		}
	}
}

func TestGetFriendLeaderboard_TrimsWhitespace(t *testing.T) {
	s := &leaderboardStore{}
	h := newLeaderboardHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/gaming/leaderboard/friends?friends="+
		"%20alice%20,bob%20,%20carol", nil)
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.GetFriendLeaderboard(rr, req)

	want := []string{"alice", "bob", "carol"}
	for i, id := range want {
		if i >= len(s.friendLastIDs) || s.friendLastIDs[i] != id {
			t.Errorf("friendLastIDs[%d]: got %v, want %q (full: %v)", i, s.friendLastIDs, id, s.friendLastIDs)
		}
	}
}

func TestGetFriendLeaderboard_FiltersOutCaller(t *testing.T) {
	// Caller should be stripped from the friend list so the store query
	// does not duplicate their row — the handler surfaces the caller via
	// UserRank separately.
	s := &leaderboardStore{}
	h := newLeaderboardHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/gaming/leaderboard/friends?friends=alice,u1,bob", nil)
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.GetFriendLeaderboard(rr, req)

	for _, id := range s.friendLastIDs {
		if id == "u1" {
			t.Errorf("caller u1 should be filtered out of friendIDs, got %v", s.friendLastIDs)
		}
	}
	if len(s.friendLastIDs) != 2 {
		t.Errorf("expected 2 friends (alice, bob), got %v", s.friendLastIDs)
	}
}

func TestGetFriendLeaderboard_EmptyFriendParam_CallsStoreWithNil(t *testing.T) {
	s := &leaderboardStore{}
	h := newLeaderboardHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/gaming/leaderboard/friends", nil)
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.GetFriendLeaderboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if s.friendCalls != 1 {
		t.Errorf("expected 1 store call, got %d", s.friendCalls)
	}
	if len(s.friendLastIDs) != 0 {
		t.Errorf("expected nil/empty friendIDs, got %v", s.friendLastIDs)
	}
}

func TestGetFriendLeaderboard_HappyPath_ReturnsFriendResponseShape(t *testing.T) {
	// Response must use FriendLeaderboardResponse ({friends, user_rank}) —
	// not LeaderboardResponse ({top_10, user_rank}). Pin the shape so a
	// future rename doesn't silently break the frontend.
	s := &leaderboardStore{
		friendEntries: []model.LeaderboardEntry{{UserID: "alice", XP: 300, Rank: 2}},
		friendUserRank: &model.LeaderboardEntry{UserID: "u1", XP: 500, Rank: 1},
	}
	h := newLeaderboardHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/gaming/leaderboard/friends?friends=alice", nil)
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.GetFriendLeaderboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp model.FriendLeaderboardResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Friends) != 1 || resp.Friends[0].UserID != "alice" {
		t.Errorf("Friends: got %+v", resp.Friends)
	}
	if resp.UserRank == nil || resp.UserRank.UserID != "u1" {
		t.Errorf("UserRank: got %+v", resp.UserRank)
	}
}

func TestGetFriendLeaderboard_StoreError_Returns500(t *testing.T) {
	s := &leaderboardStore{friendErr: errors.New("db down")}
	h := newLeaderboardHandler(s)

	req := httptest.NewRequest(http.MethodGet, "/gaming/leaderboard/friends?friends=alice", nil)
	req = withUser(req, "u1")
	rr := httptest.NewRecorder()
	h.GetFriendLeaderboard(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}
