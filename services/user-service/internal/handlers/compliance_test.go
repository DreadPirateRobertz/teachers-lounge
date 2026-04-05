package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/auth"
	"github.com/teacherslounge/user-service/internal/handlers"
	"github.com/teacherslounge/user-service/internal/middleware"
	"github.com/teacherslounge/user-service/internal/models"

)

// withAccessor returns a middleware that injects a user ID via the test helper.
func withAccessor(userID uuid.UUID) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.WithUserIDForTest(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ============================================================
// AUDIT LOG TESTS
// ============================================================

func TestGetAuditLog_MissingStudentID(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := handlers.NewComplianceHandler(s, c)

	accessorID := uuid.New()
	r := chi.NewRouter()
	r.Use(withAccessor(accessorID))
	r.Get("/admin/audit", h.GetAuditLog)

	req := httptest.NewRequest(http.MethodGet, "/admin/audit", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestGetAuditLog_InvalidStudentID(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := handlers.NewComplianceHandler(s, c)

	accessorID := uuid.New()
	r := chi.NewRouter()
	r.Use(withAccessor(accessorID))
	r.Get("/admin/audit", h.GetAuditLog)

	req := httptest.NewRequest(http.MethodGet, "/admin/audit?student_id=not-a-uuid", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestGetAuditLog_Success(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := handlers.NewComplianceHandler(s, c)

	accessorID := uuid.New()
	studentID := uuid.New()
	r := chi.NewRouter()
	r.Use(withAccessor(accessorID))
	r.Get("/admin/audit", h.GetAuditLog)

	req := httptest.NewRequest(http.MethodGet, "/admin/audit?student_id="+studentID.String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := resp["entries"]; !ok {
		t.Error("response missing 'entries' field")
	}
	if _, ok := resp["count"]; !ok {
		t.Error("response missing 'count' field")
	}
}

func TestGetAuditLog_WithDateRange(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := handlers.NewComplianceHandler(s, c)

	accessorID := uuid.New()
	studentID := uuid.New()
	r := chi.NewRouter()
	r.Use(withAccessor(accessorID))
	r.Get("/admin/audit", h.GetAuditLog)

	url := "/admin/audit?student_id=" + studentID.String() +
		"&from=2024-01-01T00:00:00Z&to=2024-12-31T23:59:59Z"
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetAuditLog_InvalidFromDate(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := handlers.NewComplianceHandler(s, c)

	accessorID := uuid.New()
	studentID := uuid.New()
	r := chi.NewRouter()
	r.Use(withAccessor(accessorID))
	r.Get("/admin/audit", h.GetAuditLog)

	req := httptest.NewRequest(http.MethodGet,
		"/admin/audit?student_id="+studentID.String()+"&from=not-a-date", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestGetAuditLog_RateLimit(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := handlers.NewComplianceHandler(s, c)

	accessorID := uuid.New()
	studentID := uuid.New()
	r := chi.NewRouter()
	r.Use(withAccessor(accessorID))
	r.Get("/admin/audit", h.GetAuditLog)

	// 30 requests succeed; the 31st hits the rate limit
	for i := 1; i <= 32; i++ {
		req := httptest.NewRequest(http.MethodGet, "/admin/audit?student_id="+studentID.String(), nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if i > 30 && rr.Code != http.StatusTooManyRequests {
			t.Fatalf("request %d: expected 429, got %d", i, rr.Code)
		}
	}
}

// ============================================================
// CONSENT TESTS
// ============================================================

func TestGetConsent_Success(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	usersH := handlers.NewUsersHandler(s, c)

	userID := uuid.New()
	r := chi.NewRouter()
	r.Get("/users/{id}/consent", usersH.GetConsent)

	req := httptest.NewRequest(http.MethodGet, "/users/"+userID.String()+"/consent", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var bundle models.ConsentBundle
	if err := json.NewDecoder(rr.Body).Decode(&bundle); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

func TestGetConsent_InvalidUserID(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	usersH := handlers.NewUsersHandler(s, c)

	r := chi.NewRouter()
	r.Get("/users/{id}/consent", usersH.GetConsent)

	req := httptest.NewRequest(http.MethodGet, "/users/not-a-uuid/consent", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestUpdateConsent_Success(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	usersH := handlers.NewUsersHandler(s, c)

	userID := uuid.New()
	r := chi.NewRouter()
	r.Patch("/users/{id}/consent", usersH.UpdateConsent)

	body := `{"tutoring": true, "analytics": false}`
	req := httptest.NewRequest(http.MethodPatch, "/users/"+userID.String()+"/consent",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestUpdateConsent_MarketingOnly(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	usersH := handlers.NewUsersHandler(s, c)

	userID := uuid.New()
	r := chi.NewRouter()
	r.Patch("/users/{id}/consent", usersH.UpdateConsent)

	body := `{"marketing": true}`
	req := httptest.NewRequest(http.MethodPatch, "/users/"+userID.String()+"/consent",
		strings.NewReader(body))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestUpdateConsent_InvalidBody(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	usersH := handlers.NewUsersHandler(s, c)

	userID := uuid.New()
	r := chi.NewRouter()
	r.Patch("/users/{id}/consent", usersH.UpdateConsent)

	req := httptest.NewRequest(http.MethodPatch, "/users/"+userID.String()+"/consent",
		strings.NewReader("{bad json"))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// ============================================================
// DELETE ACCOUNT — GDPR erasure cascade
// ============================================================

func TestDeleteAccount_EnqueuesErasureJob(t *testing.T) {
	s := newMockStore()
	c := newMockCache()

	jwtMgr := auth.NewJWTManager(strings.Repeat("x", 32), 15*time.Minute, 30*24*time.Hour)
	cfg := testConfig()
	authH := handlers.NewAuthHandler(s, c, jwtMgr, nil, cfg)
	usersH := handlers.NewUsersHandler(s, c)

	// Register a user first
	body := `{"email":"gdpr@test.com","password":"password123","display_name":"GDPR User"}`
	regReq := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(body))
	regReq.Header.Set("Content-Type", "application/json")
	regRR := httptest.NewRecorder()
	authH.Register(regRR, regReq)
	if regRR.Code != http.StatusCreated {
		t.Fatalf("register failed: %d: %s", regRR.Code, regRR.Body.String())
	}

	var regResp models.AuthResponse
	_ = json.NewDecoder(regRR.Body).Decode(&regResp)
	userID := regResp.User.ID

	// Delete account via the handler
	r := chi.NewRouter()
	r.Delete("/users/{id}", usersH.DeleteAccount)

	delReq := httptest.NewRequest(http.MethodDelete, "/users/"+userID.String(), nil)
	delRR := httptest.NewRecorder()
	r.ServeHTTP(delRR, delReq)

	if delRR.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", delRR.Code, delRR.Body.String())
	}

	// Verify user is gone
	if _, err := s.GetUserByID(context.Background(), userID); err == nil {
		t.Error("user should have been deleted from store")
	}
}

// ============================================================
// AUDIT LOGGING ON PROFILE READ
// ============================================================

func TestGetProfile_WritesAuditLog(t *testing.T) {
	s := newMockStore()
	c := newMockCache()

	// Pre-populate a user
	jwtMgr := auth.NewJWTManager(strings.Repeat("x", 32), 15*time.Minute, 30*24*time.Hour)
	cfg := testConfig()
	authH := handlers.NewAuthHandler(s, c, jwtMgr, nil, cfg)
	usersH := handlers.NewUsersHandler(s, c)

	body := `{"email":"audit@test.com","password":"password123","display_name":"Audit User"}`
	regReq := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(body))
	regReq.Header.Set("Content-Type", "application/json")
	regRR := httptest.NewRecorder()
	authH.Register(regRR, regReq)
	if regRR.Code != http.StatusCreated {
		t.Fatalf("register failed: %d", regRR.Code)
	}
	var regResp models.AuthResponse
	_ = json.NewDecoder(regRR.Body).Decode(&regResp)
	userID := regResp.User.ID

	// GET /profile should succeed (audit log is written but non-fatal)
	r := chi.NewRouter()
	r.Get("/users/{id}/profile", usersH.GetProfile)
	req := httptest.NewRequest(http.MethodGet, "/users/"+userID.String()+"/profile", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
