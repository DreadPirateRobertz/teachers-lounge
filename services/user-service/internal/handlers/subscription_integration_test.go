//go:build integration

package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/auth"
	"github.com/teacherslounge/user-service/internal/billing"
	"github.com/teacherslounge/user-service/internal/config"
	"github.com/teacherslounge/user-service/internal/handlers"
	"github.com/teacherslounge/user-service/internal/middleware"
	"github.com/teacherslounge/user-service/internal/models"
	"github.com/teacherslounge/user-service/internal/store"
)

// ============================================================
// MOCK BILLING — avoids real Stripe calls in integration tests
// ============================================================

type mockBilling struct{}

func (m *mockBilling) CancelSubscription(_ context.Context, sub *models.Subscription) (*models.Subscription, error) {
	sub.Status = models.StatusActive // stays active until period end
	return sub, nil
}

func (m *mockBilling) ReactivateSubscription(_ context.Context, sub *models.Subscription) (*models.Subscription, error) {
	if sub.Status != models.StatusActive {
		return nil, fmt.Errorf("can only reactivate an active subscription (current status: %s)", sub.Status)
	}
	return sub, nil
}

// Ensure mockBilling implements the interface.
var _ billing.SubscriptionManager = (*mockBilling)(nil)

// ============================================================
// TEST SUITE SETUP
// ============================================================

func testDBURL(t *testing.T) string {
	t.Helper()
	u := os.Getenv("TEST_DATABASE_URL")
	if u == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping integration tests")
	}
	return u
}

func newIntegrationStore(t *testing.T) *store.Store {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s, err := store.New(ctx, testDBURL(t))
	if err != nil {
		t.Fatalf("connecting to test database: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

const integrationJWTSecret = "integration-test-secret-32chars!!"

func newIntegrationRouter(t *testing.T, s *store.Store, jwt *auth.JWTManager) *chi.Mux {
	t.Helper()

	// Use mock cache — subscription endpoints don't need Redis for these tests.
	c := newMockCache()

	authH := handlers.NewAuthHandler(s, c, jwt, nil, &config.Config{
		TrialDays:            14,
		RefreshTokenDuration: 30 * 24 * time.Hour,
	})
	subsH := handlers.NewSubscriptionsHandler(s, &mockBilling{})

	r := chi.NewRouter()
	r.Post("/auth/register", authH.Register)
	r.Route("/users/{id}", func(r chi.Router) {
		r.Use(middleware.Authenticate(jwt))
		r.Use(middleware.RequireSelf(func(req *http.Request) string {
			return chi.URLParam(req, "id")
		}))
		r.Get("/subscription", subsH.GetSubscription)
		r.Post("/subscription/cancel", subsH.CancelSubscription)
		r.Post("/subscription/reactivate", subsH.ReactivateSubscription)
	})
	return r
}

// ============================================================
// HELPERS
// ============================================================

func integrationPost(t *testing.T, ts *httptest.Server, path string, body any) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := ts.Client().Post(ts.URL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func integrationGet(t *testing.T, ts *httptest.Server, path, token string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func integrationPostAuth(t *testing.T, ts *httptest.Server, path, token string, body any) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(http.MethodPost, ts.URL+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func integrationDecodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
}

func integrationUniqueEmail() string {
	return fmt.Sprintf("test-%s@example.com", uuid.New().String()[:8])
}

// registerUser registers a new user and returns (userID, accessToken).
func registerUser(t *testing.T, ts *httptest.Server) (string, string) {
	t.Helper()
	resp := integrationPost(t, ts, "/auth/register", map[string]any{
		"email":        integrationUniqueEmail(),
		"password":     "hunter12345",
		"display_name": "Integration Tester",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d", resp.StatusCode)
	}
	var body struct {
		AccessToken string `json:"access_token"`
		User        struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	integrationDecodeJSON(t, resp, &body)
	if body.AccessToken == "" || body.User.ID == "" {
		t.Fatal("register: missing access_token or user id")
	}
	return body.User.ID, body.AccessToken
}

// ============================================================
// TESTS
// ============================================================

// TestSubscriptionFlow_Integration covers the full lifecycle:
// register → get subscription → cancel → reactivate.
func TestSubscriptionFlow_Integration(t *testing.T) {
	s := newIntegrationStore(t)
	jwt := auth.NewJWTManager(integrationJWTSecret, 15*time.Minute, 30*24*time.Hour)
	ts := httptest.NewServer(newIntegrationRouter(t, s, jwt))
	defer ts.Close()

	userID, token := registerUser(t, ts)

	// ── GET /users/{id}/subscription ──────────────────────────
	subResp := integrationGet(t, ts, "/users/"+userID+"/subscription", token)
	if subResp.StatusCode != http.StatusOK {
		t.Fatalf("get subscription: expected 200, got %d", subResp.StatusCode)
	}

	var sub struct {
		Plan        string  `json:"plan"`
		Status      string  `json:"status"`
		TrialEndsAt *string `json:"trial_ends_at"`
	}
	integrationDecodeJSON(t, subResp, &sub)

	if sub.Plan != string(models.PlanTrial) {
		t.Errorf("expected plan=trial, got %q", sub.Plan)
	}
	if sub.Status != string(models.StatusTrialing) {
		t.Errorf("expected status=trialing, got %q", sub.Status)
	}
	if sub.TrialEndsAt == nil {
		t.Error("trial_ends_at should be set for a new trial subscription")
	}

	// ── POST /users/{id}/subscription/cancel ──────────────────
	// Trial subscriptions are considered active (IsActive() = true).
	// Mock billing sets status=active (pending cancel, stays active until period end).
	cancelResp := integrationPostAuth(t, ts, "/users/"+userID+"/subscription/cancel", token, nil)
	if cancelResp.StatusCode != http.StatusOK {
		t.Fatalf("cancel subscription: expected 200, got %d", cancelResp.StatusCode)
	}

	var cancelBody struct {
		Status string `json:"status"`
	}
	integrationDecodeJSON(t, cancelResp, &cancelBody)
	if cancelBody.Status != string(models.StatusActive) {
		t.Errorf("cancel: expected status=active (pending cancel), got %q", cancelBody.Status)
	}

	// ── POST /users/{id}/subscription/reactivate ──────────────
	// After mock-cancel the subscription is still active, so reactivate should succeed.
	reactivateResp := integrationPostAuth(t, ts, "/users/"+userID+"/subscription/reactivate", token, nil)
	if reactivateResp.StatusCode != http.StatusOK {
		t.Fatalf("reactivate subscription: expected 200, got %d", reactivateResp.StatusCode)
	}
}

// TestGetSubscription_NotFound verifies 404 for a user with no subscription.
func TestGetSubscription_NotFound(t *testing.T) {
	s := newIntegrationStore(t)
	jwt := auth.NewJWTManager(integrationJWTSecret, 15*time.Minute, 30*24*time.Hour)
	ts := httptest.NewServer(newIntegrationRouter(t, s, jwt))
	defer ts.Close()

	// Issue a token for a UUID that has no subscription in DB.
	ghostID := uuid.New()
	token, err := jwt.IssueAccessToken(&models.User{
		ID:          ghostID,
		Email:       "ghost@example.com",
		AccountType: models.AccountTypeStandard,
	}, "")
	if err != nil {
		t.Fatalf("issuing token: %v", err)
	}

	resp := integrationGet(t, ts, "/users/"+ghostID.String()+"/subscription", token)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// TestCancelSubscription_NotActive verifies 422 when subscription is not active.
func TestCancelSubscription_NotActive(t *testing.T) {
	s := newIntegrationStore(t)
	jwt := auth.NewJWTManager(integrationJWTSecret, 15*time.Minute, 30*24*time.Hour)
	ts := httptest.NewServer(newIntegrationRouter(t, s, jwt))
	defer ts.Close()

	// Register a user and manually mark their subscription as cancelled in DB.
	userID, token := registerUser(t, ts)
	uid, _ := uuid.Parse(userID)

	// UpdateSubscriptionByUserID works for trial subs that have no StripeSubscriptionID.
	cancelledStatus := models.StatusCancelled
	if err := s.UpdateSubscriptionByUserID(context.Background(), uid, store.UpdateSubscriptionParams{
		Status: &cancelledStatus,
	}); err != nil {
		t.Fatalf("could not set subscription to cancelled: %v", err)
	}

	// Cancel should now return 422 since IsActive() = false for cancelled status.
	resp := integrationPostAuth(t, ts, "/users/"+userID+"/subscription/cancel", token, nil)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp.StatusCode)
	}
}
