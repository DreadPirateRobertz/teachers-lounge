package handlers_test

// Tests for UsersHandler.DownloadExport — GET /users/{id}/export (tl-82h).
//
// Coverage targets:
//   - Full export shape: 200 with JSON body containing expected top-level fields,
//     Content-Disposition attachment header set.
//   - 403 when the JWT caller does not match the URL {id} param.
//   - 404 when the user does not exist in the store.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/handlers"
	"github.com/teacherslounge/user-service/internal/middleware"
	"github.com/teacherslounge/user-service/internal/models"
	"github.com/teacherslounge/user-service/internal/store"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// exportReq builds a GET /users/{id}/export request with chi params and
// optionally injects a caller ID into the context (simulating RequireSelf).
func exportReq(userID uuid.UUID, callerID *uuid.UUID) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/users/"+userID.String()+"/export", nil)

	// Inject chi URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", userID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Inject JWT caller ID when provided
	if callerID != nil {
		req = req.WithContext(middleware.WithUserIDForTest(req.Context(), *callerID))
	}
	return req
}

// ── DownloadExport tests ──────────────────────────────────────────────────────

// TestDownloadExport_FullShape verifies that a successful request returns 200,
// a Content-Disposition attachment header, and a JSON body with the top-level
// fields expected by the GDPR spec (exported_at, user, interactions, quiz_results).
func TestDownloadExport_FullShape(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := handlers.NewUsersHandler(s, c)

	// Create a real user so GetUserByID succeeds.
	user, err := s.CreateUser(context.Background(), store.CreateUserParams{
		Email:        "export@test.com",
		PasswordHash: "hash",
		DisplayName:  "Export User",
		AvatarEmoji:  "📦",
		AccountType:  models.AccountTypeStandard,
	})
	if err != nil {
		t.Fatalf("setup: create user: %v", err)
	}

	req := exportReq(user.ID, &user.ID) // caller == owner
	rr := httptest.NewRecorder()
	h.DownloadExport(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Content-Disposition must be an attachment.
	cd := rr.Header().Get("Content-Disposition")
	if !strings.HasPrefix(cd, "attachment;") {
		t.Errorf("expected Content-Disposition to start with 'attachment;', got %q", cd)
	}
	if !strings.Contains(cd, user.ID.String()) {
		t.Errorf("Content-Disposition should include user ID, got %q", cd)
	}

	// Body must decode to a UserExport with the expected fields.
	var export models.UserExport
	if err := json.NewDecoder(rr.Body).Decode(&export); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if export.ExportedAt.IsZero() {
		t.Error("expected non-zero exported_at")
	}
	if export.Interactions == nil {
		t.Error("expected interactions field (may be empty slice, not nil)")
	}
	if export.QuizResults == nil {
		t.Error("expected quiz_results field (may be empty slice, not nil)")
	}
}

// TestDownloadExport_ForbiddenOtherUser verifies that a caller whose JWT user ID
// differs from the URL {id} param receives 403 Forbidden.
func TestDownloadExport_ForbiddenOtherUser(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := handlers.NewUsersHandler(s, c)

	ownerID := uuid.New()
	otherID := uuid.New()

	req := exportReq(ownerID, &otherID) // caller ≠ owner
	rr := httptest.NewRecorder()
	h.DownloadExport(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestDownloadExport_ForbiddenNoAuth verifies 403 when no JWT context is present.
func TestDownloadExport_ForbiddenNoAuth(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := handlers.NewUsersHandler(s, c)

	req := exportReq(uuid.New(), nil) // no caller ID
	rr := httptest.NewRecorder()
	h.DownloadExport(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestDownloadExport_NotFound verifies 404 when the user does not exist.
func TestDownloadExport_NotFound(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := handlers.NewUsersHandler(s, c)

	missingID := uuid.New() // not in the store
	req := exportReq(missingID, &missingID)
	rr := httptest.NewRecorder()
	h.DownloadExport(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestDownloadExport_InvalidUserID verifies 400 for a malformed UUID in the path.
func TestDownloadExport_InvalidUserID(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := handlers.NewUsersHandler(s, c)

	req := httptest.NewRequest(http.MethodGet, "/users/not-a-uuid/export", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "not-a-uuid")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.DownloadExport(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// TestDownloadExport_BuildError verifies 500 when BuildUserExport fails.
func TestDownloadExport_BuildError(t *testing.T) {
	s := &errStore{mockStore: newMockStore(), buildUserExportErr: errBuildFailed}
	c := newMockCache()
	h := handlers.NewUsersHandler(s, c)

	// Pre-create user so GetUserByID succeeds.
	user, _ := s.CreateUser(context.Background(), store.CreateUserParams{
		Email: "build-err@test.com", PasswordHash: "h",
		DisplayName: "Err", AvatarEmoji: "x", AccountType: models.AccountTypeStandard,
	})

	req := exportReq(user.ID, &user.ID)
	rr := httptest.NewRecorder()
	h.DownloadExport(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

// errBuildFailed is a sentinel error injected via errStore.
var errBuildFailed = errStoreError("build export failed")

type errStoreError string

func (e errStoreError) Error() string { return string(e) }
