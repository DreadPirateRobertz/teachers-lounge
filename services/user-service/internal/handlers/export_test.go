package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/auth"
	"github.com/teacherslounge/user-service/internal/handlers"
	"github.com/teacherslounge/user-service/internal/middleware"
	"github.com/teacherslounge/user-service/internal/models"
)

// buildExportRouter wires GET /users/{id}/export behind the standard
// Authenticate + RequireSelf middleware chain used by every other
// /users/{id}/* route. Returns a router and a token scoped to tokenUserID
// so tests can exercise happy, forbidden, and missing-user paths.
func buildExportRouter(t *testing.T, ms *mockStore, tokenUserID uuid.UUID) (http.Handler, string) {
	t.Helper()

	jwtMgr := auth.NewJWTManager(testJWTSecret, 15*time.Minute, 24*time.Hour)
	mc := newMockCache()
	usersH := handlers.NewUsersHandler(ms, mc)

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
		r.Get("/export", usersH.GetFullExport)
	})

	return r, tok
}

// TestGetFullExport_Success verifies that an authenticated owner receives the
// full GDPR export payload as JSON (UserExport shape).
func TestGetFullExport_Success(t *testing.T) {
	ms := newMockStore()
	userID := uuid.New()
	ms.byID[userID] = &models.User{
		ID:          userID,
		Email:       "owner@example.com",
		AccountType: models.AccountTypeStandard,
	}
	ms.users["owner@example.com"] = ms.byID[userID]

	router, tok := buildExportRouter(t, ms, userID)

	req := httptest.NewRequest(http.MethodGet, "/users/"+userID.String()+"/export", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var export models.UserExport
	if err := json.Unmarshal(rr.Body.Bytes(), &export); err != nil {
		t.Fatalf("response is not a UserExport JSON: %v\nbody: %s", err, rr.Body.String())
	}

	if export.User == nil || export.User.ID != userID {
		t.Errorf("export.User should be the owner; got %+v", export.User)
	}
	if export.ExportedAt.IsZero() {
		t.Error("export.ExportedAt should be set")
	}
	if export.Interactions == nil {
		t.Error("export.Interactions should be an empty slice, not nil")
	}
	if export.QuizResults == nil {
		t.Error("export.QuizResults should be an empty slice, not nil")
	}
}

// TestGetFullExport_Forbidden_NonOwner verifies that a token for one user
// cannot retrieve the export for a different user; RequireSelf should 403.
func TestGetFullExport_Forbidden_NonOwner(t *testing.T) {
	ms := newMockStore()
	ownerID := uuid.New()
	otherID := uuid.New()
	ms.byID[ownerID] = &models.User{ID: ownerID, Email: "owner@example.com", AccountType: models.AccountTypeStandard}
	ms.byID[otherID] = &models.User{ID: otherID, Email: "other@example.com", AccountType: models.AccountTypeStandard}
	ms.users["owner@example.com"] = ms.byID[ownerID]
	ms.users["other@example.com"] = ms.byID[otherID]

	// Token belongs to ownerID, but request targets otherID.
	router, tok := buildExportRouter(t, ms, ownerID)

	req := httptest.NewRequest(http.MethodGet, "/users/"+otherID.String()+"/export", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("want 403 for cross-user export attempt, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestGetFullExport_NotFound verifies that a valid JWT whose user has been
// deleted from the store (or never existed there) returns 404 rather than
// leaking a half-populated export.
func TestGetFullExport_NotFound(t *testing.T) {
	ms := newMockStore()
	userID := uuid.New()
	// Deliberately do NOT insert into ms.byID — the JWT is valid, but the
	// store has no record for userID. Mirrors the post-deletion / stale-token
	// scenario.

	router, tok := buildExportRouter(t, ms, userID)

	req := httptest.NewRequest(http.MethodGet, "/users/"+userID.String()+"/export", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("want 404 for missing user, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestGetFullExport_Unauthenticated verifies that a request without a bearer
// token is rejected with 401 before the handler runs.
func TestGetFullExport_Unauthenticated(t *testing.T) {
	ms := newMockStore()
	userID := uuid.New()
	ms.byID[userID] = &models.User{ID: userID, Email: "x@example.com", AccountType: models.AccountTypeStandard}
	ms.users["x@example.com"] = ms.byID[userID]

	router, _ := buildExportRouter(t, ms, userID)

	req := httptest.NewRequest(http.MethodGet, "/users/"+userID.String()+"/export", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401 for missing Authorization header, got %d", rr.Code)
	}
}
