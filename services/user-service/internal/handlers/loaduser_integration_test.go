package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/auth"
	"github.com/teacherslounge/user-service/internal/handlers"
	"github.com/teacherslounge/user-service/internal/middleware"
	"github.com/teacherslounge/user-service/internal/models"
)

// fetchCountingStore wraps mockStore and records GetUserByID hits so the
// integration test can assert that a single request through the full
// Authenticate → LoadUser → handler chain only touches the store once.
type fetchCountingStore struct {
	*mockStore
	getUserByIDCalls atomic.Int32
}

func (s *fetchCountingStore) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	s.getUserByIDCalls.Add(1)
	return s.mockStore.GetUserByID(ctx, id)
}

// buildAuthRouter wires the production chain (Authenticate + LoadUser +
// RequireSelf) so the test exercises the same middleware order as
// cmd/server/main.go.
func buildAuthRouter(t *testing.T, s *fetchCountingStore, tokenUserID uuid.UUID) (http.Handler, string) {
	t.Helper()

	jwtMgr := auth.NewJWTManager(testJWTSecret, 15*time.Minute, 24*time.Hour)
	mc := newMockCache()
	usersH := handlers.NewUsersHandler(s, mc)

	tokenUser := &models.User{
		ID:          tokenUserID,
		Email:       "owner@example.com",
		AccountType: models.AccountTypeStandard,
	}
	tok, err := jwtMgr.IssueAccessToken(tokenUser, "trialing")
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	r := chi.NewRouter()
	r.Route("/users/{id}", func(r chi.Router) {
		r.Use(middleware.Authenticate(jwtMgr))
		r.Use(middleware.RequireSelf(func(req *http.Request) string {
			return chi.URLParam(req, "id")
		}))
		r.Use(middleware.LoadUser(s))
		r.Get("/profile", usersH.GetProfile)
		r.Get("/export", usersH.GetFullExport)
	})

	return r, tok
}

// TestLoadUser_GetProfile_SingleStoreFetch is the regression guard for the
// PR #238 review issue: the middleware fetches the user and the handler
// reuses that cached record instead of issuing a second GetUserByID.
func TestLoadUser_GetProfile_SingleStoreFetch(t *testing.T) {
	ms := newMockStore()
	uid := uuid.New()
	ms.byID[uid] = &models.User{ID: uid, Email: "owner@example.com", AccountType: models.AccountTypeStandard}
	ms.users["owner@example.com"] = ms.byID[uid]
	cs := &fetchCountingStore{mockStore: ms}

	router, tok := buildAuthRouter(t, cs, uid)

	req := httptest.NewRequest(http.MethodGet, "/users/"+uid.String()+"/profile", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := cs.getUserByIDCalls.Load(); got != 1 {
		t.Errorf("want exactly 1 GetUserByID per request, got %d", got)
	}
}

// TestLoadUser_GetFullExport_SingleStoreFetch confirms the same guarantee
// holds for the GDPR export path that motivated the original review note.
func TestLoadUser_GetFullExport_SingleStoreFetch(t *testing.T) {
	ms := newMockStore()
	uid := uuid.New()
	ms.byID[uid] = &models.User{ID: uid, Email: "owner@example.com", AccountType: models.AccountTypeStandard}
	ms.users["owner@example.com"] = ms.byID[uid]
	cs := &fetchCountingStore{mockStore: ms}

	router, tok := buildAuthRouter(t, cs, uid)

	req := httptest.NewRequest(http.MethodGet, "/users/"+uid.String()+"/export", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if got := cs.getUserByIDCalls.Load(); got != 1 {
		t.Errorf("want exactly 1 GetUserByID across LoadUser + GetFullExport + BuildUserExport, got %d", got)
	}
}

// TestLoadUser_DeactivatedUserGets401 covers the TOCTOU window: a token that
// was issued before the account was deleted must be rejected with 401 on the
// next request rather than returning 200 from a handler that never checks.
func TestLoadUser_DeactivatedUserGets401(t *testing.T) {
	ms := newMockStore()
	uid := uuid.New()
	// Mint the JWT for a user record that never enters the store, modelling
	// the "JWT issued, then DeleteUser ran" sequence.
	cs := &fetchCountingStore{mockStore: ms}

	router, tok := buildAuthRouter(t, cs, uid)

	req := httptest.NewRequest(http.MethodGet, "/users/"+uid.String()+"/profile", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401 for deactivated-user JWT, got %d: %s", rr.Code, rr.Body.String())
	}
}
