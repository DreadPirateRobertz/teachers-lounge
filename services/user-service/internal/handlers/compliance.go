package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/cache"
	"github.com/teacherslounge/user-service/internal/middleware"
	"github.com/teacherslounge/user-service/internal/models"
	"github.com/teacherslounge/user-service/internal/store"
)

const (
	auditQueryRateLimit    = 30            // max requests per window
	auditQueryRateWindow   = 60 * time.Second // 1 minute window
	auditQueryRateLimitKey = "ratelimit:admin:audit:%s"
)

// ComplianceHandler serves FERPA/GDPR admin endpoints.
type ComplianceHandler struct {
	store store.Storer
	cache cache.Cacher
}

// NewComplianceHandler creates a ComplianceHandler.
func NewComplianceHandler(s store.Storer, c cache.Cacher) *ComplianceHandler {
	return &ComplianceHandler{store: s, cache: c}
}

// GetAuditLog handles GET /admin/audit.
//
// Query parameters:
//   - student_id (required): UUID of the student whose audit log to query
//   - from (optional): RFC3339 start date
//   - to   (optional): RFC3339 end date
//
// Requires teacher profile (enforced by RequireTeacherProfile middleware).
// Rate-limited to 30 requests/minute per accessor. Each call is itself logged.
func (h *ComplianceHandler) GetAuditLog(w http.ResponseWriter, r *http.Request) {
	accessorID, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Rate limit: 30 requests/minute per accessor
	rateLimitKey := "ratelimit:admin:audit:" + accessorID.String()
	count, err := h.cache.IncrWithTTL(r.Context(), rateLimitKey, auditQueryRateWindow)
	if err == nil && count > auditQueryRateLimit {
		writeError(w, http.StatusTooManyRequests, "too many requests — try again in a minute")
		return
	}

	// Parse query params
	q := r.URL.Query()

	studentIDStr := q.Get("student_id")
	if studentIDStr == "" {
		writeError(w, http.StatusBadRequest, "student_id is required")
		return
	}
	studentID, err := uuid.Parse(studentIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid student_id")
		return
	}

	p := store.AuditLogQueryParams{
		StudentID: &studentID,
	}

	if fromStr := q.Get("from"); fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid 'from' date — use RFC3339 format")
			return
		}
		p.From = &t
	}
	if toStr := q.Get("to"); toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid 'to' date — use RFC3339 format")
			return
		}
		p.To = &t
	}

	entries, err := h.store.QueryAuditLog(r.Context(), p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// FERPA: log this admin access itself
	_ = h.store.WriteAuditLog(r.Context(), store.AuditLogParams{
		AccessorID:   &accessorID,
		StudentID:    &studentID,
		Action:       models.AuditActionAdminAccess,
		DataAccessed: "audit_log",
		Purpose:      "ferpa_audit_review",
		IPAddress:    realIP(r),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"entries": entries,
		"count":   len(entries),
	})
}
