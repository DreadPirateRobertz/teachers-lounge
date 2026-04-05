package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/middleware"
	"github.com/teacherslounge/user-service/internal/models"
	"github.com/teacherslounge/user-service/internal/store"
)

// AdminHandler handles FERPA compliance admin endpoints.
type AdminHandler struct {
	store store.Storer
}

// NewAdminHandler creates an AdminHandler backed by the given store.
func NewAdminHandler(s store.Storer) *AdminHandler {
	return &AdminHandler{store: s}
}

// GetAuditLog handles GET /admin/audit.
//
// Query parameters:
//   - student_id (UUID, optional) — filter by student
//   - from       (RFC3339, optional) — earliest timestamp
//   - to         (RFC3339, optional) — latest timestamp
//   - limit      (int, optional)    — max results, clamped to 500, default 100
//   - offset     (int, optional)    — pagination offset
//
// Writes an audit entry for the admin's own query so access is self-documenting.
func (h *AdminHandler) GetAuditLog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	p := store.QueryAuditLogParams{
		Limit:  100,
		Offset: 0,
	}

	if raw := q.Get("student_id"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid student_id")
			return
		}
		p.StudentID = &id
	}

	if raw := q.Get("from"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid from timestamp (use RFC3339)")
			return
		}
		p.From = &t
	}

	if raw := q.Get("to"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid to timestamp (use RFC3339)")
			return
		}
		p.To = &t
	}

	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		p.Limit = n
	}

	if raw := q.Get("offset"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "invalid offset")
			return
		}
		p.Offset = n
	}

	entries, err := h.store.QueryAuditLog(r.Context(), p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query audit log")
		return
	}

	// Record this admin access in the audit log itself.
	accessorID := adminAccessorID(r)
	_ = h.store.WriteAuditLog(r.Context(), store.AuditLogParams{
		AccessorID:   accessorID,
		StudentID:    p.StudentID,
		Action:       models.AuditActionQueryAuditLog,
		DataAccessed: "audit_log",
		Purpose:      "ferpa_compliance_review",
		IPAddress:    realIP(r),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"entries": entries,
		"count":   len(entries),
	})
}

// adminAccessorID extracts the authenticated user's UUID from context.
func adminAccessorID(r *http.Request) *uuid.UUID {
	id, ok := middleware.UserIDFromCtx(r.Context())
	if !ok {
		return nil
	}
	return &id
}
