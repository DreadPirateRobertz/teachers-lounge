package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/store"
)

// stubAuditStore records the most recent WriteAuditLog call.
type stubAuditStore struct {
	last *store.AuditLogParams
	err  error
}

func (s *stubAuditStore) WriteAuditLog(_ context.Context, p store.AuditLogParams) error {
	s.last = &p
	return s.err
}

func newAuditRequest(method, target, userID, routeID string) *http.Request {
	r := httptest.NewRequest(method, target, nil)

	// Inject authenticated user into context (as Authenticate middleware would).
	if userID != "" {
		ctx := context.WithValue(r.Context(), ctxKeyUserID{}, userID)
		r = r.WithContext(ctx)
	}

	// Inject chi URL parameter "id" (as the router would).
	if routeID != "" {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", routeID)
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	}

	return r
}

// TestAuditLog_WritesOnSuccess verifies an audit entry is written for 200 responses.
func TestAuditLog_WritesOnSuccess(t *testing.T) {
	st := &stubAuditStore{}
	uid := uuid.New()
	mw := AuditLog(st, "read_profile", "user_profile", "ferpa_compliance")

	req := newAuditRequest(http.MethodGet, "/users/"+uid.String()+"/profile", uid.String(), uid.String())
	rec := httptest.NewRecorder()

	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if st.last == nil {
		t.Fatal("expected audit log entry, got none")
	}
	if st.last.Action != "read_profile" {
		t.Errorf("expected action=read_profile, got %q", st.last.Action)
	}
	if st.last.AccessorID == nil || *st.last.AccessorID != uid {
		t.Errorf("expected accessor_id=%s, got %v", uid, st.last.AccessorID)
	}
	if st.last.StudentID == nil || *st.last.StudentID != uid {
		t.Errorf("expected student_id=%s, got %v", uid, st.last.StudentID)
	}
}

// TestAuditLog_SkipsOnError verifies no audit entry is written when the handler returns 4xx/5xx.
func TestAuditLog_SkipsOnError(t *testing.T) {
	st := &stubAuditStore{}
	mw := AuditLog(st, "read_profile", "user_profile", "ferpa_compliance")

	req := newAuditRequest(http.MethodGet, "/users/x/profile", "", "")
	rec := httptest.NewRecorder()

	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})).ServeHTTP(rec, req)

	if st.last != nil {
		t.Errorf("expected no audit entry for 404, got %+v", st.last)
	}
}

// TestAuditLog_NilAccessorWhenUnauthenticated verifies accessor_id is nil when no JWT context.
func TestAuditLog_NilAccessorWhenUnauthenticated(t *testing.T) {
	st := &stubAuditStore{}
	uid := uuid.New()
	mw := AuditLog(st, "read_profile", "user_profile", "ferpa_compliance")

	// No user in context — studentID still extracted from URL.
	req := newAuditRequest(http.MethodGet, "/users/"+uid.String()+"/profile", "", uid.String())
	rec := httptest.NewRecorder()

	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if st.last == nil {
		t.Fatal("expected audit entry")
	}
	if st.last.AccessorID != nil {
		t.Errorf("expected nil accessor_id, got %v", st.last.AccessorID)
	}
	if st.last.StudentID == nil || *st.last.StudentID != uid {
		t.Errorf("expected student_id=%s, got %v", uid, st.last.StudentID)
	}
}

// TestAuditLog_StoreErrorDoesNotFailRequest verifies audit errors do not propagate.
func TestAuditLog_StoreErrorDoesNotFailRequest(t *testing.T) {
	st := &stubAuditStore{err: context.DeadlineExceeded}
	mw := AuditLog(st, "read_profile", "user_profile", "ferpa_compliance")

	req := newAuditRequest(http.MethodGet, "/users/x/profile", uuid.New().String(), "")
	rec := httptest.NewRecorder()

	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 despite audit error, got %d", rec.Code)
	}
}

// TestAuditLog_ImplicitOKStatus verifies that a handler not calling WriteHeader is treated as 200.
func TestAuditLog_ImplicitOKStatus(t *testing.T) {
	st := &stubAuditStore{}
	mw := AuditLog(st, "read_profile", "user_profile", "ferpa_compliance")

	req := newAuditRequest(http.MethodGet, "/users/x/profile", uuid.New().String(), "")
	rec := httptest.NewRecorder()

	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// deliberate: no WriteHeader call → Go default is 200
		_, _ = w.Write([]byte("ok"))
	})).ServeHTTP(rec, req)

	if st.last == nil {
		t.Fatal("expected audit entry for implicit 200")
	}
}
