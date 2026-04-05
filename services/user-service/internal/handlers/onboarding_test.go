package handlers_test

import (
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

// newOnboardingJWT returns a JWTManager using the shared test secret so the
// test middleware can validate tokens produced in these tests.
func newOnboardingJWT() *auth.JWTManager {
	return auth.NewJWTManager(testJWTSecret, 15*time.Minute, 24*time.Hour)
}

// buildOnboardingRouter wires PATCH /users/{id}/onboarding with auth middleware.
func buildOnboardingRouter(t *testing.T, ms *mockStore, userID uuid.UUID) (http.Handler, string) {
	t.Helper()

	jwtMgr := newOnboardingJWT()
	mc := newMockCache()
	usersH := handlers.NewUsersHandler(ms, mc)

	accessToken, _, err := jwtMgr.Issue(userID.String(), "test@example.com", "trialing")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	r := chi.NewRouter()
	r.Route("/users/{id}", func(r chi.Router) {
		r.Use(middleware.Authenticate(jwtMgr))
		r.Use(middleware.RequireSelf(func(req *http.Request) string {
			return chi.URLParam(req, "id")
		}))
		r.Patch("/onboarding", usersH.CompleteOnboarding)
	})

	return r, accessToken
}

// ============================================================
// CompleteOnboarding
// ============================================================

// TestCompleteOnboarding_Success verifies that PATCH /users/{id}/onboarding
// returns 204 and sets HasCompletedOnboarding + OnboardedAt on the user.
func TestCompleteOnboarding_Success(t *testing.T) {
	ms := newMockStore()
	userID := uuid.New()
	ms.byID[userID] = &models.User{
		ID:    userID,
		Email: "wizard@example.com",
	}
	ms.users["wizard@example.com"] = ms.byID[userID]

	router, tok := buildOnboardingRouter(t, ms, userID)

	req := httptest.NewRequest(http.MethodPatch, "/users/"+userID.String()+"/onboarding", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d: %s", rr.Code, rr.Body.String())
	}

	u := ms.byID[userID]
	if !u.HasCompletedOnboarding {
		t.Error("HasCompletedOnboarding should be true after PATCH /onboarding")
	}
	if u.OnboardedAt == nil {
		t.Error("OnboardedAt should be non-nil after PATCH /onboarding")
	}
}

// TestCompleteOnboarding_Idempotent verifies that calling the endpoint twice
// returns 204 both times without error.
func TestCompleteOnboarding_Idempotent(t *testing.T) {
	ms := newMockStore()
	userID := uuid.New()
	ms.byID[userID] = &models.User{
		ID:                     userID,
		Email:                  "wizard@example.com",
		HasCompletedOnboarding: true,
	}
	ms.users["wizard@example.com"] = ms.byID[userID]

	router, tok := buildOnboardingRouter(t, ms, userID)

	for i := range 2 {
		req := httptest.NewRequest(http.MethodPatch, "/users/"+userID.String()+"/onboarding", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusNoContent {
			t.Errorf("call %d: want 204, got %d", i+1, rr.Code)
		}
	}
}

// TestCompleteOnboarding_RequiresSelf verifies that a user cannot complete
// onboarding for a different user's ID.
func TestCompleteOnboarding_RequiresSelf(t *testing.T) {
	ms := newMockStore()
	ownerID := uuid.New()
	otherID := uuid.New()
	ms.byID[ownerID] = &models.User{ID: ownerID, Email: "owner@example.com"}
	ms.users["owner@example.com"] = ms.byID[ownerID]

	// Token belongs to ownerID but request targets otherID.
	router, tok := buildOnboardingRouter(t, ms, ownerID)

	req := httptest.NewRequest(http.MethodPatch, "/users/"+otherID.String()+"/onboarding", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("want 403 for wrong user, got %d", rr.Code)
	}
}

// TestCompleteOnboarding_NoAuth verifies that unauthenticated requests are
// rejected with 401.
func TestCompleteOnboarding_NoAuth(t *testing.T) {
	ms := newMockStore()
	userID := uuid.New()
	ms.byID[userID] = &models.User{ID: userID, Email: "x@example.com"}
	ms.users["x@example.com"] = ms.byID[userID]

	router, _ := buildOnboardingRouter(t, ms, userID)

	req := httptest.NewRequest(http.MethodPatch, "/users/"+userID.String()+"/onboarding", nil)
	// No Authorization header
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", rr.Code)
	}
}
