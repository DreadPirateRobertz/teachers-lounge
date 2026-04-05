package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/handlers"
	"github.com/teacherslounge/user-service/internal/models"
	"github.com/teacherslounge/user-service/internal/store"
)

// ============================================================
// ADMIN HANDLER TESTS
// ============================================================

// TestGetAuditLog_ReturnsEntries verifies a successful query returns entries.
func TestGetAuditLog_ReturnsEntries(t *testing.T) {
	sid := uuid.New()
	ms := newMockStore()
	ms.auditEntries = []*models.AuditEntry{
		{
			ID:           uuid.New(),
			Timestamp:    time.Now(),
			StudentID:    &sid,
			Action:       models.AuditActionReadProfile,
			DataAccessed: "user_profile",
			Purpose:      "ferpa_compliance",
		},
	}

	h := handlers.NewAdminHandler(ms)
	req := httptest.NewRequest(http.MethodGet, "/admin/audit", nil)
	rec := httptest.NewRecorder()

	h.GetAuditLog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	entries, ok := resp["entries"]
	if !ok {
		t.Fatal("expected entries key in response")
	}
	arr, ok := entries.([]any)
	if !ok || len(arr) != 1 {
		t.Errorf("expected 1 entry, got %v", entries)
	}
}

// TestGetAuditLog_EmptySliceWhenNoEntries verifies an empty array (not null) is returned.
func TestGetAuditLog_EmptySliceWhenNoEntries(t *testing.T) {
	ms := newMockStore()
	ms.auditEntries = []*models.AuditEntry{}

	h := handlers.NewAdminHandler(ms)
	req := httptest.NewRequest(http.MethodGet, "/admin/audit", nil)
	rec := httptest.NewRecorder()

	h.GetAuditLog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	arr, _ := resp["entries"].([]any)
	// JSON null decodes as nil; we expect an empty array (length 0)
	if len(arr) != 0 {
		t.Errorf("expected empty entries, got %v", arr)
	}
}

// TestGetAuditLog_BadStudentID returns 400 for an invalid UUID.
func TestGetAuditLog_BadStudentID(t *testing.T) {
	ms := newMockStore()
	h := handlers.NewAdminHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/admin/audit?student_id=not-a-uuid", nil)
	rec := httptest.NewRecorder()

	h.GetAuditLog(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// TestGetAuditLog_BadFromTimestamp returns 400 for a non-RFC3339 from param.
func TestGetAuditLog_BadFromTimestamp(t *testing.T) {
	ms := newMockStore()
	h := handlers.NewAdminHandler(ms)

	req := httptest.NewRequest(http.MethodGet, "/admin/audit?from=yesterday", nil)
	rec := httptest.NewRecorder()

	h.GetAuditLog(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// TestGetAuditLog_FiltersByStudentID verifies the student_id param is forwarded to the store.
func TestGetAuditLog_FiltersByStudentID(t *testing.T) {
	sid := uuid.New()
	ms := newMockStore()
	ms.auditEntries = []*models.AuditEntry{}

	h := handlers.NewAdminHandler(ms)
	req := httptest.NewRequest(http.MethodGet, "/admin/audit?student_id="+sid.String(), nil)
	rec := httptest.NewRecorder()

	h.GetAuditLog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ms.lastAuditQuery == nil {
		t.Fatal("expected store.QueryAuditLog to be called")
	}
	if ms.lastAuditQuery.StudentID == nil || *ms.lastAuditQuery.StudentID != sid {
		t.Errorf("expected student_id=%s, got %v", sid, ms.lastAuditQuery.StudentID)
	}
}

// ============================================================
// MOCK STORE ADDITIONS (appended to existing mockStore in auth_test.go)
// ============================================================

// Additions are in mockStore (same package: handlers_test) — see auth_test.go for base.
// We add fields and new method overrides here via the mock extension pattern.

func (m *mockStore) QueryAuditLog(_ context.Context, p store.QueryAuditLogParams) ([]*models.AuditEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastAuditQuery = &p
	if m.auditEntries == nil {
		return []*models.AuditEntry{}, nil
	}
	return m.auditEntries, nil
}

func (m *mockStore) GetExportJob(_ context.Context, jobID, _ uuid.UUID) (*models.ExportJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.exportJobs[jobID]
	if !ok {
		return nil, store.ErrNotFound
	}
	return job, nil
}

func (m *mockStore) BuildUserExport(_ context.Context, jobID, userID uuid.UUID) (*models.UserExport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	export := &models.UserExport{
		ExportedAt:   time.Now(),
		Interactions: []models.InteractionExport{},
		QuizResults:  []models.QuizResultExport{},
	}
	if u, ok := m.byID[userID]; ok {
		export.User = u
	}
	completed := models.ExportJobStatus("complete")
	m.exportJobs[jobID] = &models.ExportJob{
		ID:         jobID,
		UserID:     userID,
		Status:     completed,
		ResultData: export,
	}
	return export, nil
}

func (m *mockStore) UpdateGuardianConsent(_ context.Context, userID uuid.UUID, guardianEmail string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.byID[userID]
	if !ok {
		return store.ErrNotFound
	}
	if u.GuardianEmail == nil || *u.GuardianEmail != guardianEmail {
		return store.ErrNotFound
	}
	now := time.Now()
	u.GuardianConsentAt = &now
	return nil
}
