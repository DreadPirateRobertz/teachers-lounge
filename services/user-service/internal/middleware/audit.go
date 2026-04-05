package middleware

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/store"
)

// AuditStorer is the minimal store interface the AuditLog middleware needs.
// Satisfied by *store.Store and by test doubles.
type AuditStorer interface {
	WriteAuditLog(ctx context.Context, p store.AuditLogParams) error
}

// statusRecorder wraps ResponseWriter to capture the HTTP status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

// WriteHeader captures the status and delegates to the wrapped writer.
func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// AuditLog returns a Chi middleware that writes a FERPA audit trail entry
// after the request completes, but only when the response is 2xx.
//
// accessorID is read from the authenticated user in context (set by Authenticate).
// studentID is read from the chi URL parameter named "id", if present.
func AuditLog(s AuditStorer, action, dataAccessed, purpose string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			// Only audit successful reads.
			if rec.status < 200 || rec.status >= 300 {
				return
			}

			accessorID := accessorFromCtx(r.Context())
			studentID := studentFromURL(r)

			_ = s.WriteAuditLog(r.Context(), store.AuditLogParams{
				AccessorID:   accessorID,
				StudentID:    studentID,
				Action:       action,
				DataAccessed: dataAccessed,
				Purpose:      purpose,
				IPAddress:    realIPFromRequest(r),
			})
		})
	}
}

// accessorFromCtx returns the authenticated user ID from context, or nil.
func accessorFromCtx(ctx context.Context) *uuid.UUID {
	id, ok := UserIDFromCtx(ctx)
	if !ok {
		return nil
	}
	return &id
}

// studentFromURL returns the UUID from the chi "id" URL parameter, or nil.
func studentFromURL(r *http.Request) *uuid.UUID {
	raw := chi.URLParam(r, "id")
	if raw == "" {
		return nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil
	}
	return &id
}

// realIPFromRequest returns the request's real IP, preferring X-Forwarded-For.
func realIPFromRequest(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	return r.RemoteAddr
}
