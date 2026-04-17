package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/models"
	"github.com/teacherslounge/user-service/internal/store"
)

// fetchCounterStore records every GetUserByID call. The fake exists so tests
// can assert that LoadUser issues exactly one fetch per request and that
// downstream handlers reuse the cached record via UserFromCtx.
type fetchCounterStore struct {
	calls atomic.Int32
	user  *models.User
	err   error
}

func (s *fetchCounterStore) GetUserByID(_ context.Context, id uuid.UUID) (*models.User, error) {
	s.calls.Add(1)
	if s.err != nil {
		return nil, s.err
	}
	if s.user == nil || s.user.ID != id {
		return nil, store.ErrNotFound
	}
	return s.user, nil
}

func newAuthedRequest(userID uuid.UUID) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	return req.WithContext(WithUserIDForTest(req.Context(), userID))
}

// TestLoadUser_PopulatesContext_SingleFetch covers the redundant double-fetch
// fix: middleware fetches once, the handler reads the same record from
// UserFromCtx instead of issuing a second GetUserByID.
func TestLoadUser_PopulatesContext_SingleFetch(t *testing.T) {
	uid := uuid.New()
	st := &fetchCounterStore{user: &models.User{ID: uid, Email: "a@b.com"}}

	var seen *models.User
	mw := LoadUser(st)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = UserFromCtx(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newAuthedRequest(uid))

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := st.calls.Load(); got != 1 {
		t.Errorf("want exactly 1 store fetch, got %d", got)
	}
	if seen == nil || seen.ID != uid {
		t.Errorf("handler did not see the cached user via UserFromCtx (got %+v)", seen)
	}
}

// TestLoadUser_DeactivatedReturns401 covers the TOCTOU fix: an unexpired
// access JWT for a user whose record was removed from the store must be
// rejected with 401 before the handler runs.
func TestLoadUser_DeactivatedReturns401(t *testing.T) {
	uid := uuid.New()
	st := &fetchCounterStore{} // no user seeded → ErrNotFound on lookup

	called := false
	handler := LoadUser(st)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newAuthedRequest(uid))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401 for deleted-account JWT, got %d: %s", rec.Code, rec.Body.String())
	}
	if called {
		t.Error("handler must not run when the user no longer exists")
	}
}

// TestLoadUser_StoreError500 verifies non-NotFound store errors do not get
// silently turned into 401 — they surface as 500 so operators can investigate.
func TestLoadUser_StoreError500(t *testing.T) {
	uid := uuid.New()
	st := &fetchCounterStore{err: errors.New("db down")}

	rec := httptest.NewRecorder()
	LoadUser(st)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, newAuthedRequest(uid))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500 on transient store error, got %d", rec.Code)
	}
}

// TestLoadUser_MissingUserIDReturns401 verifies LoadUser refuses to fetch
// when Authenticate did not run earlier in the chain.
func TestLoadUser_MissingUserIDReturns401(t *testing.T) {
	st := &fetchCounterStore{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)

	LoadUser(st)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("handler must not run without an authenticated user")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401 when user ID is absent from ctx, got %d", rec.Code)
	}
	if got := st.calls.Load(); got != 0 {
		t.Errorf("want zero store fetches when ctx is unauthenticated, got %d", got)
	}
}

// TestRequireAdmin_ReusesCachedUser verifies that when LoadUser ran first,
// RequireAdmin does not issue a second GetUserByID.
func TestRequireAdmin_ReusesCachedUser(t *testing.T) {
	uid := uuid.New()
	st := &fetchCounterStore{user: &models.User{ID: uid, IsAdmin: true}}

	chain := LoadUser(st)(RequireAdmin(st)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, newAuthedRequest(uid))

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := st.calls.Load(); got != 1 {
		t.Errorf("want exactly 1 store fetch across LoadUser+RequireAdmin, got %d", got)
	}
}

// TestRequireAdmin_NonAdminForbidden documents that the admin gate still
// fires for cached non-admin users (defense in depth).
func TestRequireAdmin_NonAdminForbidden(t *testing.T) {
	uid := uuid.New()
	st := &fetchCounterStore{user: &models.User{ID: uid, IsAdmin: false}}

	chain := LoadUser(st)(RequireAdmin(st)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("handler must not run for non-admin")
	})))

	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, newAuthedRequest(uid))

	if rec.Code != http.StatusForbidden {
		t.Errorf("want 403 for non-admin, got %d", rec.Code)
	}
}

// TestRequireAdmin_DeactivatedReturns401 covers admin routes that bypass the
// per-user LoadUser (RequireAdmin must perform its own existence check when
// no cached user is present).
func TestRequireAdmin_DeactivatedReturns401(t *testing.T) {
	uid := uuid.New()
	st := &fetchCounterStore{} // no user → ErrNotFound

	rec := httptest.NewRecorder()
	RequireAdmin(st)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("handler must not run when admin's account no longer exists")
	})).ServeHTTP(rec, newAuthedRequest(uid))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401 for deleted admin, got %d", rec.Code)
	}
}

// TestRequireAdmin_MissingUserIDReturns401 verifies RequireAdmin rejects
// requests that reach it without an authenticated user ID (i.e., when
// Authenticate was not wired earlier in the chain).
func TestRequireAdmin_MissingUserIDReturns401(t *testing.T) {
	st := &fetchCounterStore{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)

	RequireAdmin(st)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("handler must not run without an authenticated user")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("want 401 when user ID is absent from ctx, got %d", rec.Code)
	}
	if got := st.calls.Load(); got != 0 {
		t.Errorf("want zero store fetches when ctx is unauthenticated, got %d", got)
	}
}

// TestRequireAdmin_StoreError500 verifies that transient store failures
// surface as 500 (not 401) so operators can investigate rather than seeing a
// burst of spurious auth failures.
func TestRequireAdmin_StoreError500(t *testing.T) {
	uid := uuid.New()
	st := &fetchCounterStore{err: errors.New("db down")}

	rec := httptest.NewRecorder()
	RequireAdmin(st)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("handler must not run when the store is unavailable")
	})).ServeHTTP(rec, newAuthedRequest(uid))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500 on transient store error, got %d", rec.Code)
	}
}
