package handlers_test

// handlers_coverage_test.go — additional tests to bring handlers package to ≥90% coverage.
// Covers: teachers, subscriptions, webhooks, and users handler gaps.
// Builds on mockStore/mockCache defined in auth_test.go (same package).

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/billing"
	"github.com/teacherslounge/user-service/internal/handlers"
	"github.com/teacherslounge/user-service/internal/models"
	"github.com/teacherslounge/user-service/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// ============================================================
// HELPERS
// ============================================================

// chiReq creates a request with chi URL parameters injected into context.
func chiReq(method, path string, body io.Reader, params map[string]string) *http.Request {
	req := httptest.NewRequest(method, path, body)
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// jsonBody serialises v and returns a reader.
func jsonBody(v any) io.Reader {
	b, _ := json.Marshal(v)
	return bytes.NewReader(b)
}

// serve calls handler with the request and returns the recorder.
func serve(handler http.HandlerFunc, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	handler(w, r)
	return w
}

// validTeacherID / validClassID are stable UUIDs for tests that need ownership to match.
var (
	validTeacherID = uuid.MustParse("aaaaaaaa-0000-0000-0000-000000000001")
	validClassID   = uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000002")
	validStudentID = uuid.MustParse("cccccccc-0000-0000-0000-000000000003")
	validMaterialID = uuid.MustParse("dddddddd-0000-0000-0000-000000000004")
)

// ============================================================
// STORE OVERRIDES FOR TEACHERS
// ============================================================

// classOwnerStore overrides GetClass so assertClassOwner returns the configured result.
type classOwnerStore struct {
	*mockStore
	classTeacherID uuid.UUID // owner returned by GetClass
	getClassErr    error     // if non-nil, GetClass returns this error
}

func newClassOwnerStore(ownerID uuid.UUID) *classOwnerStore {
	return &classOwnerStore{mockStore: newMockStore(), classTeacherID: ownerID}
}

func (s *classOwnerStore) GetClass(_ context.Context, id uuid.UUID) (*models.TeacherClass, error) {
	if s.getClassErr != nil {
		return nil, s.getClassErr
	}
	return &models.TeacherClass{ID: id, TeacherID: s.classTeacherID}, nil
}

// errStore wraps mockStore and injects errors into specific operations.
type errStore struct {
	*mockStore
	createTeacherProfileErr   error
	getTeacherProfileErr      error
	createClassErr            error
	listClassesErr            error
	updateClassErr            error
	deleteClassErr            error
	addStudentErr             error
	removeStudentErr          error
	listRosterErr             error
	getStudentProgressErr     error
	assignMaterialErr         error
	unassignMaterialErr       error
	listClassMaterialsErr     error
	getSubscriptionErr        error
	updateSubscriptionUserErr error
	createExportJobErr        error
	getExportJobResult        *models.ExportJob
	getExportJobErr           error
	buildUserExportErr        error
	updateUserErr             error
	updateLearningProfileErr  error
}

func (s *errStore) CreateTeacherProfile(ctx context.Context, p store.CreateTeacherProfileParams) (*models.TeacherProfile, error) {
	if s.createTeacherProfileErr != nil {
		return nil, s.createTeacherProfileErr
	}
	return s.mockStore.CreateTeacherProfile(ctx, p)
}

func (s *errStore) GetTeacherProfile(ctx context.Context, id uuid.UUID) (*models.TeacherProfile, error) {
	if s.getTeacherProfileErr != nil {
		return nil, s.getTeacherProfileErr
	}
	return s.mockStore.GetTeacherProfile(ctx, id)
}

func (s *errStore) CreateClass(ctx context.Context, p store.CreateClassParams) (*models.TeacherClass, error) {
	if s.createClassErr != nil {
		return nil, s.createClassErr
	}
	return &models.TeacherClass{ID: validClassID, TeacherID: p.TeacherID, Name: p.Name}, nil
}

func (s *errStore) ListClasses(ctx context.Context, id uuid.UUID) ([]*models.TeacherClass, error) {
	if s.listClassesErr != nil {
		return nil, s.listClassesErr
	}
	return s.mockStore.ListClasses(ctx, id)
}

func (s *errStore) GetClass(ctx context.Context, id uuid.UUID) (*models.TeacherClass, error) {
	return &models.TeacherClass{ID: id, TeacherID: validTeacherID}, nil
}

func (s *errStore) UpdateClass(ctx context.Context, id uuid.UUID, p store.UpdateClassParams) (*models.TeacherClass, error) {
	if s.updateClassErr != nil {
		return nil, s.updateClassErr
	}
	return &models.TeacherClass{ID: id}, nil
}

func (s *errStore) DeleteClass(ctx context.Context, id uuid.UUID) error {
	if s.deleteClassErr != nil {
		return s.deleteClassErr
	}
	return nil
}

func (s *errStore) AddStudentToClass(ctx context.Context, classID, studentID uuid.UUID) error {
	if s.addStudentErr != nil {
		return s.addStudentErr
	}
	return nil
}

func (s *errStore) RemoveStudentFromClass(ctx context.Context, classID, studentID uuid.UUID) error {
	if s.removeStudentErr != nil {
		return s.removeStudentErr
	}
	return nil
}

func (s *errStore) ListClassRoster(ctx context.Context, classID uuid.UUID) ([]*models.StudentSummary, error) {
	if s.listRosterErr != nil {
		return nil, s.listRosterErr
	}
	return s.mockStore.ListClassRoster(ctx, classID)
}

func (s *errStore) GetStudentProgress(ctx context.Context, studentID uuid.UUID) (*models.StudentProgress, error) {
	if s.getStudentProgressErr != nil {
		return nil, s.getStudentProgressErr
	}
	return &models.StudentProgress{}, nil
}

func (s *errStore) AssignMaterialToClass(ctx context.Context, p store.AssignMaterialParams) error {
	if s.assignMaterialErr != nil {
		return s.assignMaterialErr
	}
	return nil
}

func (s *errStore) UnassignMaterialFromClass(ctx context.Context, classID, materialID uuid.UUID) error {
	if s.unassignMaterialErr != nil {
		return s.unassignMaterialErr
	}
	return nil
}

func (s *errStore) ListClassMaterials(ctx context.Context, classID uuid.UUID) ([]*models.ClassMaterialAssignment, error) {
	if s.listClassMaterialsErr != nil {
		return nil, s.listClassMaterialsErr
	}
	return s.mockStore.ListClassMaterials(ctx, classID)
}

func (s *errStore) GetSubscriptionByUserID(ctx context.Context, userID uuid.UUID) (*models.Subscription, error) {
	if s.getSubscriptionErr != nil {
		return nil, s.getSubscriptionErr
	}
	return s.mockStore.GetSubscriptionByUserID(ctx, userID)
}

func (s *errStore) UpdateSubscriptionByUserID(ctx context.Context, userID uuid.UUID, p store.UpdateSubscriptionParams) error {
	if s.updateSubscriptionUserErr != nil {
		return s.updateSubscriptionUserErr
	}
	return nil
}

func (s *errStore) CreateExportJob(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	if s.createExportJobErr != nil {
		return uuid.Nil, s.createExportJobErr
	}
	return s.mockStore.CreateExportJob(ctx, userID)
}

func (s *errStore) GetExportJob(ctx context.Context, jobID, userID uuid.UUID) (*models.ExportJob, error) {
	if s.getExportJobErr != nil {
		return nil, s.getExportJobErr
	}
	if s.getExportJobResult != nil {
		return s.getExportJobResult, nil
	}
	return &models.ExportJob{ID: jobID, UserID: userID, Status: models.ExportStatusPending}, nil
}

func (s *errStore) BuildUserExport(ctx context.Context, jobID, userID uuid.UUID) (*models.UserExport, error) {
	if s.buildUserExportErr != nil {
		return nil, s.buildUserExportErr
	}
	return &models.UserExport{}, nil
}

func (s *errStore) UpdateUser(ctx context.Context, id uuid.UUID, p store.UpdateUserParams) (*models.User, error) {
	if s.updateUserErr != nil {
		return nil, s.updateUserErr
	}
	return s.mockStore.UpdateUser(ctx, id, p)
}

func (s *errStore) UpdateLearningProfile(ctx context.Context, id uuid.UUID, p store.UpdateProfileParams) error {
	if s.updateLearningProfileErr != nil {
		return s.updateLearningProfileErr
	}
	return nil
}

// ============================================================
// MOCK SUBSCRIPTION MANAGER
// ============================================================

type mockSubscriptionManager struct {
	cancelResult      *models.Subscription
	cancelErr         error
	reactivateResult  *models.Subscription
	reactivateErr     error
}

func (m *mockSubscriptionManager) CancelSubscription(_ context.Context, sub *models.Subscription) (*models.Subscription, error) {
	if m.cancelErr != nil {
		return nil, m.cancelErr
	}
	if m.cancelResult != nil {
		return m.cancelResult, nil
	}
	c := *sub
	c.Status = models.StatusCancelled
	return &c, nil
}

func (m *mockSubscriptionManager) ReactivateSubscription(_ context.Context, sub *models.Subscription) (*models.Subscription, error) {
	if m.reactivateErr != nil {
		return nil, m.reactivateErr
	}
	if m.reactivateResult != nil {
		return m.reactivateResult, nil
	}
	r := *sub
	r.Status = models.StatusActive
	return &r, nil
}

// ============================================================
// HELPERS: teacher handler with chi params
// ============================================================

func newTeachersHandler(s store.Storer) *handlers.TeachersHandler {
	return handlers.NewTeachersHandler(s)
}

func teacherReq(method string, body any, params map[string]string) *http.Request {
	var r io.Reader
	if body != nil {
		r = jsonBody(body)
	}
	req := chiReq(method, "/", r, params)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// ============================================================
// TESTS: CreateTeacherProfile
// ============================================================

func TestCreateTeacherProfile_Success(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newTeachersHandler(s)

	r := teacherReq(http.MethodPost,
		models.CreateTeacherProfileRequest{SchoolName: "Lincoln High", Bio: "Bio text"},
		map[string]string{"id": validTeacherID.String()},
	)
	w := serve(h.CreateTeacherProfile, r)
	if w.Code != http.StatusCreated {
		t.Errorf("want 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateTeacherProfile_InvalidUserID(t *testing.T) {
	h := newTeachersHandler(&errStore{mockStore: newMockStore()})
	r := teacherReq(http.MethodPost, models.CreateTeacherProfileRequest{}, map[string]string{"id": "not-a-uuid"})
	w := serve(h.CreateTeacherProfile, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCreateTeacherProfile_InvalidJSON(t *testing.T) {
	h := newTeachersHandler(&errStore{mockStore: newMockStore()})
	req := chiReq(http.MethodPost, "/", bytes.NewBufferString("{invalid}"), map[string]string{"id": validTeacherID.String()})
	w := serve(h.CreateTeacherProfile, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCreateTeacherProfile_StoreError(t *testing.T) {
	s := &errStore{mockStore: newMockStore(), createTeacherProfileErr: errors.New("db error")}
	h := newTeachersHandler(s)
	r := teacherReq(http.MethodPost,
		models.CreateTeacherProfileRequest{SchoolName: "School"},
		map[string]string{"id": validTeacherID.String()},
	)
	w := serve(h.CreateTeacherProfile, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

// ============================================================
// TESTS: GetTeacherProfile
// ============================================================

func TestGetTeacherProfile_NotFound(t *testing.T) {
	// mockStore.GetTeacherProfile always returns ErrNotFound — expect 404.
	s := &errStore{mockStore: newMockStore()}
	h := newTeachersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.GetTeacherProfile, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetTeacherProfile_InvalidUserID(t *testing.T) {
	h := newTeachersHandler(&errStore{mockStore: newMockStore()})
	r := chiReq(http.MethodGet, "/", nil, map[string]string{"id": "bad"})
	w := serve(h.GetTeacherProfile, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestGetTeacherProfile_StoreError(t *testing.T) {
	s := &errStore{mockStore: newMockStore(), getTeacherProfileErr: errors.New("db error")}
	h := newTeachersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.GetTeacherProfile, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

// ============================================================
// TESTS: CreateClass
// ============================================================

func TestCreateClass_Success(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newTeachersHandler(s)
	r := teacherReq(http.MethodPost,
		models.CreateClassRequest{Name: "Algebra", Subject: "Math"},
		map[string]string{"id": validTeacherID.String()},
	)
	w := serve(h.CreateClass, r)
	if w.Code != http.StatusCreated {
		t.Errorf("want 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateClass_InvalidTeacherID(t *testing.T) {
	h := newTeachersHandler(&errStore{mockStore: newMockStore()})
	r := teacherReq(http.MethodPost, models.CreateClassRequest{Name: "X"}, map[string]string{"id": "bad"})
	w := serve(h.CreateClass, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCreateClass_InvalidJSON(t *testing.T) {
	h := newTeachersHandler(&errStore{mockStore: newMockStore()})
	req := chiReq(http.MethodPost, "/", bytes.NewBufferString("{bad}"), map[string]string{"id": validTeacherID.String()})
	w := serve(h.CreateClass, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCreateClass_MissingName(t *testing.T) {
	h := newTeachersHandler(&errStore{mockStore: newMockStore()})
	r := teacherReq(http.MethodPost, models.CreateClassRequest{Subject: "Math"}, map[string]string{"id": validTeacherID.String()})
	w := serve(h.CreateClass, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for missing name, got %d", w.Code)
	}
}

func TestCreateClass_StoreError(t *testing.T) {
	s := &errStore{mockStore: newMockStore(), createClassErr: errors.New("db error")}
	h := newTeachersHandler(s)
	r := teacherReq(http.MethodPost, models.CreateClassRequest{Name: "Algebra"}, map[string]string{"id": validTeacherID.String()})
	w := serve(h.CreateClass, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

// ============================================================
// TESTS: ListClasses
// ============================================================

func TestListClasses_Success(t *testing.T) {
	h := newTeachersHandler(&errStore{mockStore: newMockStore()})
	r := chiReq(http.MethodGet, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.ListClasses, r)
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListClasses_InvalidTeacherID(t *testing.T) {
	h := newTeachersHandler(&errStore{mockStore: newMockStore()})
	r := chiReq(http.MethodGet, "/", nil, map[string]string{"id": "bad"})
	w := serve(h.ListClasses, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestListClasses_StoreError(t *testing.T) {
	s := &errStore{mockStore: newMockStore(), listClassesErr: errors.New("db error")}
	h := newTeachersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.ListClasses, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

// ============================================================
// TESTS: GetClass
// ============================================================

func TestGetClass_Success(t *testing.T) {
	// classOwnerStore returns a class owned by validTeacherID
	s := newClassOwnerStore(validTeacherID)
	h := newTeachersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id":       validTeacherID.String(),
		"class_id": validClassID.String(),
	})
	w := serve(h.GetClass, r)
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetClass_InvalidClassID(t *testing.T) {
	s := newClassOwnerStore(validTeacherID)
	h := newTeachersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id":       validTeacherID.String(),
		"class_id": "not-uuid",
	})
	w := serve(h.GetClass, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestGetClass_NotFound(t *testing.T) {
	s := newClassOwnerStore(validTeacherID)
	s.getClassErr = store.ErrNotFound
	h := newTeachersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id":       validTeacherID.String(),
		"class_id": validClassID.String(),
	})
	w := serve(h.GetClass, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestGetClass_Forbidden(t *testing.T) {
	differentTeacher := uuid.New()
	s := newClassOwnerStore(differentTeacher)
	h := newTeachersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id":       validTeacherID.String(),
		"class_id": validClassID.String(),
	})
	w := serve(h.GetClass, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", w.Code)
	}
}

func TestGetClass_StoreError(t *testing.T) {
	s := newClassOwnerStore(validTeacherID)
	s.getClassErr = errors.New("db error")
	h := newTeachersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id":       validTeacherID.String(),
		"class_id": validClassID.String(),
	})
	w := serve(h.GetClass, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

// ============================================================
// TESTS: UpdateClass
// ============================================================

func TestUpdateClass_Success(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newTeachersHandler(s)
	name := "Updated"
	r := teacherReq(http.MethodPatch,
		models.UpdateClassRequest{Name: &name},
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()},
	)
	w := serve(h.UpdateClass, r)
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateClass_NotFound(t *testing.T) {
	s := newClassOwnerStore(validTeacherID)
	s.getClassErr = store.ErrNotFound
	h := newTeachersHandler(s)
	name := "X"
	r := teacherReq(http.MethodPatch, models.UpdateClassRequest{Name: &name},
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()},
	)
	w := serve(h.UpdateClass, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestUpdateClass_Forbidden(t *testing.T) {
	s := newClassOwnerStore(uuid.New()) // different owner
	h := newTeachersHandler(s)
	name := "X"
	r := teacherReq(http.MethodPatch, models.UpdateClassRequest{Name: &name},
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()},
	)
	w := serve(h.UpdateClass, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", w.Code)
	}
}

func TestUpdateClass_InvalidJSON(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newTeachersHandler(s)
	req := chiReq(http.MethodPatch, "/", bytes.NewBufferString("{bad}"),
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()})
	w := serve(h.UpdateClass, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestUpdateClass_UpdateStoreError(t *testing.T) {
	s := &errStore{mockStore: newMockStore(), updateClassErr: errors.New("db error")}
	h := newTeachersHandler(s)
	name := "X"
	r := teacherReq(http.MethodPatch, models.UpdateClassRequest{Name: &name},
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()},
	)
	w := serve(h.UpdateClass, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

// ============================================================
// TESTS: DeleteClass
// ============================================================

func TestDeleteClass_Success(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newTeachersHandler(s)
	r := chiReq(http.MethodDelete, "/", nil,
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()})
	w := serve(h.DeleteClass, r)
	if w.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d", w.Code)
	}
}

func TestDeleteClass_NotFound(t *testing.T) {
	s := newClassOwnerStore(validTeacherID)
	s.getClassErr = store.ErrNotFound
	h := newTeachersHandler(s)
	r := chiReq(http.MethodDelete, "/", nil,
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()})
	w := serve(h.DeleteClass, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestDeleteClass_Forbidden(t *testing.T) {
	s := newClassOwnerStore(uuid.New())
	h := newTeachersHandler(s)
	r := chiReq(http.MethodDelete, "/", nil,
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()})
	w := serve(h.DeleteClass, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", w.Code)
	}
}

func TestDeleteClass_DeleteStoreError(t *testing.T) {
	s := &errStore{mockStore: newMockStore(), deleteClassErr: errors.New("db error")}
	h := newTeachersHandler(s)
	r := chiReq(http.MethodDelete, "/", nil,
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()})
	w := serve(h.DeleteClass, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

// ============================================================
// TESTS: AddStudent
// ============================================================

func TestAddStudent_ByStudentID(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newTeachersHandler(s)
	sid := validStudentID.String()
	r := teacherReq(http.MethodPost,
		models.AddStudentRequest{StudentID: &sid},
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()},
	)
	w := serve(h.AddStudent, r)
	if w.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAddStudent_ByEmail(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	// Add a student to the mockStore by email
	s.users["student@school.edu"] = &models.User{ID: validStudentID, Email: "student@school.edu"}
	s.byID[validStudentID] = s.users["student@school.edu"]
	h := newTeachersHandler(s)
	email := "student@school.edu"
	r := teacherReq(http.MethodPost,
		models.AddStudentRequest{Email: &email},
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()},
	)
	w := serve(h.AddStudent, r)
	if w.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAddStudent_InvalidClassParams(t *testing.T) {
	h := newTeachersHandler(&errStore{mockStore: newMockStore()})
	r := teacherReq(http.MethodPost, models.AddStudentRequest{},
		map[string]string{"id": "bad", "class_id": validClassID.String()})
	w := serve(h.AddStudent, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestAddStudent_AssertOwnerError(t *testing.T) {
	s := newClassOwnerStore(uuid.New()) // different owner
	h := newTeachersHandler(s)
	sid := validStudentID.String()
	r := teacherReq(http.MethodPost,
		models.AddStudentRequest{StudentID: &sid},
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()},
	)
	w := serve(h.AddStudent, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", w.Code)
	}
}

func TestAddStudent_InvalidBody(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newTeachersHandler(s)
	req := chiReq(http.MethodPost, "/", bytes.NewBufferString("{bad}"),
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()})
	w := serve(h.AddStudent, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestAddStudent_NeitherIDNorEmail(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newTeachersHandler(s)
	r := teacherReq(http.MethodPost, models.AddStudentRequest{},
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()})
	w := serve(h.AddStudent, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAddStudent_InvalidStudentIDFormat(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newTeachersHandler(s)
	bad := "not-a-uuid"
	r := teacherReq(http.MethodPost, models.AddStudentRequest{StudentID: &bad},
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()})
	w := serve(h.AddStudent, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestAddStudent_EmailNotFound(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newTeachersHandler(s)
	email := "nobody@school.edu"
	r := teacherReq(http.MethodPost, models.AddStudentRequest{Email: &email},
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()})
	w := serve(h.AddStudent, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAddStudent_StoreError(t *testing.T) {
	s := &errStore{mockStore: newMockStore(), addStudentErr: errors.New("db error")}
	h := newTeachersHandler(s)
	sid := validStudentID.String()
	r := teacherReq(http.MethodPost, models.AddStudentRequest{StudentID: &sid},
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()})
	w := serve(h.AddStudent, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

// ============================================================
// TESTS: RemoveStudent
// ============================================================

func TestRemoveStudent_Success(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newTeachersHandler(s)
	r := chiReq(http.MethodDelete, "/", nil, map[string]string{
		"id": validTeacherID.String(), "class_id": validClassID.String(), "student_id": validStudentID.String(),
	})
	w := serve(h.RemoveStudent, r)
	if w.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d", w.Code)
	}
}

func TestRemoveStudent_InvalidStudentID(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newTeachersHandler(s)
	r := chiReq(http.MethodDelete, "/", nil, map[string]string{
		"id": validTeacherID.String(), "class_id": validClassID.String(), "student_id": "bad",
	})
	w := serve(h.RemoveStudent, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// ============================================================
// TESTS: ListRoster
// ============================================================

func TestListRoster_Success(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newTeachersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id": validTeacherID.String(), "class_id": validClassID.String(),
	})
	w := serve(h.ListRoster, r)
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListRoster_StoreError(t *testing.T) {
	s := &errStore{mockStore: newMockStore(), listRosterErr: errors.New("db error")}
	h := newTeachersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id": validTeacherID.String(), "class_id": validClassID.String(),
	})
	w := serve(h.ListRoster, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

// ============================================================
// TESTS: GetStudentProgress
// ============================================================

func TestGetStudentProgress_Success(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newTeachersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id": validTeacherID.String(), "class_id": validClassID.String(), "student_id": validStudentID.String(),
	})
	w := serve(h.GetStudentProgress, r)
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetStudentProgress_InvalidStudentID(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newTeachersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id": validTeacherID.String(), "class_id": validClassID.String(), "student_id": "bad",
	})
	w := serve(h.GetStudentProgress, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestGetStudentProgress_NotFound(t *testing.T) {
	s := &errStore{mockStore: newMockStore(), getStudentProgressErr: store.ErrNotFound}
	h := newTeachersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id": validTeacherID.String(), "class_id": validClassID.String(), "student_id": validStudentID.String(),
	})
	w := serve(h.GetStudentProgress, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestGetStudentProgress_StoreError(t *testing.T) {
	s := &errStore{mockStore: newMockStore(), getStudentProgressErr: errors.New("db error")}
	h := newTeachersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id": validTeacherID.String(), "class_id": validClassID.String(), "student_id": validStudentID.String(),
	})
	w := serve(h.GetStudentProgress, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

// ============================================================
// TESTS: AssignMaterial
// ============================================================

func TestAssignMaterial_Success(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newTeachersHandler(s)
	r := teacherReq(http.MethodPost,
		models.AssignMaterialRequest{MaterialID: validMaterialID.String()},
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()},
	)
	w := serve(h.AssignMaterial, r)
	if w.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAssignMaterial_InvalidMaterialID(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newTeachersHandler(s)
	r := teacherReq(http.MethodPost,
		models.AssignMaterialRequest{MaterialID: "bad-uuid"},
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()},
	)
	w := serve(h.AssignMaterial, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestAssignMaterial_StoreError(t *testing.T) {
	s := &errStore{mockStore: newMockStore(), assignMaterialErr: errors.New("db error")}
	h := newTeachersHandler(s)
	r := teacherReq(http.MethodPost,
		models.AssignMaterialRequest{MaterialID: validMaterialID.String()},
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()},
	)
	w := serve(h.AssignMaterial, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

// ============================================================
// TESTS: UnassignMaterial
// ============================================================

func TestUnassignMaterial_Success(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newTeachersHandler(s)
	r := chiReq(http.MethodDelete, "/", nil, map[string]string{
		"id": validTeacherID.String(), "class_id": validClassID.String(), "material_id": validMaterialID.String(),
	})
	w := serve(h.UnassignMaterial, r)
	if w.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d", w.Code)
	}
}

func TestUnassignMaterial_InvalidMaterialID(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newTeachersHandler(s)
	r := chiReq(http.MethodDelete, "/", nil, map[string]string{
		"id": validTeacherID.String(), "class_id": validClassID.String(), "material_id": "bad",
	})
	w := serve(h.UnassignMaterial, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestUnassignMaterial_StoreError(t *testing.T) {
	s := &errStore{mockStore: newMockStore(), unassignMaterialErr: errors.New("db error")}
	h := newTeachersHandler(s)
	r := chiReq(http.MethodDelete, "/", nil, map[string]string{
		"id": validTeacherID.String(), "class_id": validClassID.String(), "material_id": validMaterialID.String(),
	})
	w := serve(h.UnassignMaterial, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

// ============================================================
// TESTS: ListAssignedMaterials
// ============================================================

func TestListAssignedMaterials_Success(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newTeachersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id": validTeacherID.String(), "class_id": validClassID.String(),
	})
	w := serve(h.ListAssignedMaterials, r)
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListAssignedMaterials_StoreError(t *testing.T) {
	s := &errStore{mockStore: newMockStore(), listClassMaterialsErr: errors.New("db error")}
	h := newTeachersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id": validTeacherID.String(), "class_id": validClassID.String(),
	})
	w := serve(h.ListAssignedMaterials, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

// ============================================================
// TESTS: writeClassOwnerError — assertClassOwner internal store error
// ============================================================

func TestAssertClassOwner_InternalError(t *testing.T) {
	s := newClassOwnerStore(validTeacherID)
	s.getClassErr = errors.New("db boom")
	h := newTeachersHandler(s)
	// AddStudent calls assertClassOwner after parseTeacherClassParams
	sid := validStudentID.String()
	r := teacherReq(http.MethodPost, models.AddStudentRequest{StudentID: &sid},
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()})
	w := serve(h.AddStudent, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ============================================================
// TESTS: SubscriptionsHandler
// ============================================================

func newSubsHandler(s store.Storer, bm billing.SubscriptionManager) *handlers.SubscriptionsHandler {
	return handlers.NewSubscriptionsHandler(s, bm)
}

func subsStore(withSub bool, subErr error) *errStore {
	s := &errStore{mockStore: newMockStore()}
	if withSub {
		s.subs[validTeacherID] = &models.Subscription{
			ID:     uuid.New(),
			UserID: validTeacherID,
			Plan:   models.PlanTrial,
			Status: models.StatusActive,
		}
	}
	if subErr != nil {
		s.getSubscriptionErr = subErr
	}
	return s
}

func TestGetSubscription_Success(t *testing.T) {
	s := subsStore(true, nil)
	h := newSubsHandler(s, &mockSubscriptionManager{})
	r := chiReq(http.MethodGet, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.GetSubscription, r)
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetSubscription_InvalidUserID(t *testing.T) {
	h := newSubsHandler(&errStore{mockStore: newMockStore()}, &mockSubscriptionManager{})
	r := chiReq(http.MethodGet, "/", nil, map[string]string{"id": "bad"})
	w := serve(h.GetSubscription, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestGetSubscription_NotFound(t *testing.T) {
	s := subsStore(false, store.ErrNotFound)
	h := newSubsHandler(s, &mockSubscriptionManager{})
	r := chiReq(http.MethodGet, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.GetSubscription, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestGetSubscription_StoreError(t *testing.T) {
	s := subsStore(false, errors.New("db error"))
	h := newSubsHandler(s, &mockSubscriptionManager{})
	r := chiReq(http.MethodGet, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.GetSubscription, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

func TestCancelSubscription_Success(t *testing.T) {
	s := subsStore(true, nil)
	bm := &mockSubscriptionManager{}
	h := newSubsHandler(s, bm)
	r := chiReq(http.MethodPost, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.CancelSubscription, r)
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCancelSubscription_InvalidUserID(t *testing.T) {
	h := newSubsHandler(&errStore{mockStore: newMockStore()}, &mockSubscriptionManager{})
	r := chiReq(http.MethodPost, "/", nil, map[string]string{"id": "bad"})
	w := serve(h.CancelSubscription, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCancelSubscription_NotFound(t *testing.T) {
	s := subsStore(false, store.ErrNotFound)
	h := newSubsHandler(s, &mockSubscriptionManager{})
	r := chiReq(http.MethodPost, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.CancelSubscription, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestCancelSubscription_NotActive(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	s.subs[validTeacherID] = &models.Subscription{
		ID:     uuid.New(),
		UserID: validTeacherID,
		Status: models.StatusCancelled, // not active
		Plan:   models.PlanTrial,
	}
	h := newSubsHandler(s, &mockSubscriptionManager{})
	r := chiReq(http.MethodPost, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.CancelSubscription, r)
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("want 422, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCancelSubscription_BillingError(t *testing.T) {
	s := subsStore(true, nil)
	bm := &mockSubscriptionManager{cancelErr: errors.New("stripe error")}
	h := newSubsHandler(s, bm)
	r := chiReq(http.MethodPost, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.CancelSubscription, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

func TestCancelSubscription_PersistError(t *testing.T) {
	s := subsStore(true, nil)
	s.updateSubscriptionUserErr = errors.New("persist error")
	bm := &mockSubscriptionManager{}
	h := newSubsHandler(s, bm)
	r := chiReq(http.MethodPost, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.CancelSubscription, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

func TestReactivateSubscription_Success(t *testing.T) {
	s := subsStore(true, nil)
	bm := &mockSubscriptionManager{}
	h := newSubsHandler(s, bm)
	r := chiReq(http.MethodPost, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.ReactivateSubscription, r)
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestReactivateSubscription_NotFound(t *testing.T) {
	s := subsStore(false, store.ErrNotFound)
	h := newSubsHandler(s, &mockSubscriptionManager{})
	r := chiReq(http.MethodPost, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.ReactivateSubscription, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestReactivateSubscription_BillingError(t *testing.T) {
	s := subsStore(true, nil)
	bm := &mockSubscriptionManager{reactivateErr: errors.New("stripe error")}
	h := newSubsHandler(s, bm)
	r := chiReq(http.MethodPost, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.ReactivateSubscription, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

func TestReactivateSubscription_PersistError(t *testing.T) {
	s := subsStore(true, nil)
	s.updateSubscriptionUserErr = errors.New("persist error")
	h := newSubsHandler(s, &mockSubscriptionManager{})
	r := chiReq(http.MethodPost, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.ReactivateSubscription, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

func TestToSubscriptionResponse_WithTrialAndPeriodEnd(t *testing.T) {
	// Test via GetSubscription to exercise toSubscriptionResponse with both dates set.
	s := &errStore{mockStore: newMockStore()}
	trialEnd := time.Now().Add(24 * time.Hour)
	periodEnd := time.Now().Add(30 * 24 * time.Hour)
	s.subs[validTeacherID] = &models.Subscription{
		ID:               uuid.New(),
		UserID:           validTeacherID,
		Plan:             models.PlanTrial,
		Status:           models.StatusTrialing,
		TrialEnd:         &trialEnd,
		CurrentPeriodEnd: &periodEnd,
	}
	h := newSubsHandler(s, &mockSubscriptionManager{})
	r := chiReq(http.MethodGet, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.GetSubscription, r)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["trial_ends_at"] == nil {
		t.Error("expected trial_ends_at in response")
	}
	if body["next_billing_date"] == nil {
		t.Error("expected next_billing_date in response")
	}
}

// ============================================================
// TESTS: WebhookHandler
// ============================================================

func TestStripeWebhook_MissingSignatureHeader(t *testing.T) {
	// nil billing client is fine — we return before calling HandleWebhook
	h := handlers.NewWebhookHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", bytes.NewBufferString(`{}`))
	// No Stripe-Signature header set
	w := serve(h.StripeWebhook, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for missing Stripe-Signature, got %d: %s", w.Code, w.Body.String())
	}
}

func TestStripeWebhook_InvalidSignature(t *testing.T) {
	// A billing.Client with a bad webhook secret will fail signature validation.
	bc := billing.NewClient("sk_test_dummy", billing.PlanPrices{}, "whsec_test", nil)
	h := handlers.NewWebhookHandler(bc)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", bytes.NewBufferString(`{}`))
	req.Header.Set("Stripe-Signature", "t=1,v1=invalid")
	w := serve(h.StripeWebhook, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for invalid signature, got %d: %s", w.Code, w.Body.String())
	}
}

// ============================================================
// TESTS: UsersHandler gaps
// ============================================================

func newUsersHandler(s store.Storer) *handlers.UsersHandler {
	return handlers.NewUsersHandler(s, newMockCache())
}

func TestUpdatePreferences_Success(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	s.byID[validTeacherID] = &models.User{ID: validTeacherID, DisplayName: "Old"}
	h := newUsersHandler(s)

	name := "New Name"
	r := teacherReq(http.MethodPatch,
		models.UpdatePreferencesRequest{DisplayName: &name},
		map[string]string{"id": validTeacherID.String()},
	)
	w := serve(h.UpdatePreferences, r)
	if w.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdatePreferences_InvalidUserID(t *testing.T) {
	h := newUsersHandler(&errStore{mockStore: newMockStore()})
	r := teacherReq(http.MethodPatch, models.UpdatePreferencesRequest{}, map[string]string{"id": "bad"})
	w := serve(h.UpdatePreferences, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestUpdatePreferences_InvalidJSON(t *testing.T) {
	h := newUsersHandler(&errStore{mockStore: newMockStore()})
	req := chiReq(http.MethodPatch, "/", bytes.NewBufferString("{bad}"), map[string]string{"id": validTeacherID.String()})
	w := serve(h.UpdatePreferences, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestUpdatePreferences_UpdateUserError(t *testing.T) {
	s := &errStore{mockStore: newMockStore(), updateUserErr: errors.New("db error")}
	// Need user to exist so UpdateUser is called
	s.byID[validTeacherID] = &models.User{ID: validTeacherID}
	h := newUsersHandler(s)
	name := "X"
	r := teacherReq(http.MethodPatch,
		models.UpdatePreferencesRequest{DisplayName: &name},
		map[string]string{"id": validTeacherID.String()},
	)
	w := serve(h.UpdatePreferences, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

func TestUpdatePreferences_UpdateLearningProfileError(t *testing.T) {
	s := &errStore{mockStore: newMockStore(), updateLearningProfileErr: errors.New("db error")}
	h := newUsersHandler(s)
	prefs := map[string]float64{"visual": 0.8}
	r := teacherReq(http.MethodPatch,
		models.UpdatePreferencesRequest{LearningStylePrefs: prefs},
		map[string]string{"id": validTeacherID.String()},
	)
	w := serve(h.UpdatePreferences, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

func TestUpdatePreferences_WithLearningPrefs_Success(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newUsersHandler(s)
	prefs := map[string]float64{"visual": 0.8}
	r := teacherReq(http.MethodPatch,
		models.UpdatePreferencesRequest{LearningStylePrefs: prefs},
		map[string]string{"id": validTeacherID.String()},
	)
	w := serve(h.UpdatePreferences, r)
	if w.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestExportData_Success(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newUsersHandler(s)
	r := chiReq(http.MethodPost, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.ExportData, r)
	if w.Code != http.StatusAccepted {
		t.Errorf("want 202, got %d: %s", w.Code, w.Body.String())
	}
}

func TestExportData_InvalidUserID(t *testing.T) {
	h := newUsersHandler(&errStore{mockStore: newMockStore()})
	r := chiReq(http.MethodPost, "/", nil, map[string]string{"id": "bad"})
	w := serve(h.ExportData, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestExportData_StoreError(t *testing.T) {
	s := &errStore{mockStore: newMockStore(), createExportJobErr: errors.New("db error")}
	h := newUsersHandler(s)
	r := chiReq(http.MethodPost, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.ExportData, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

func TestGetExport_Success_Pending(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	jobID := uuid.New()
	s.getExportJobResult = &models.ExportJob{ID: jobID, UserID: validTeacherID, Status: models.ExportStatusPending}
	h := newUsersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id": validTeacherID.String(), "jobID": jobID.String(),
	})
	w := serve(h.GetExport, r)
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetExport_Success_Complete(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	jobID := uuid.New()
	export := &models.UserExport{}
	s.getExportJobResult = &models.ExportJob{
		ID:         jobID,
		UserID:     validTeacherID,
		Status:     models.ExportStatusComplete,
		ResultData: export,
	}
	h := newUsersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id": validTeacherID.String(), "jobID": jobID.String(),
	})
	w := serve(h.GetExport, r)
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetExport_InvalidUserID(t *testing.T) {
	h := newUsersHandler(&errStore{mockStore: newMockStore()})
	r := chiReq(http.MethodGet, "/", nil, map[string]string{"id": "bad", "jobID": uuid.New().String()})
	w := serve(h.GetExport, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestGetExport_InvalidJobID(t *testing.T) {
	h := newUsersHandler(&errStore{mockStore: newMockStore()})
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id": validTeacherID.String(), "jobID": "bad",
	})
	w := serve(h.GetExport, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestGetExport_NotFound(t *testing.T) {
	s := &errStore{mockStore: newMockStore(), getExportJobErr: store.ErrNotFound}
	h := newUsersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id": validTeacherID.String(), "jobID": uuid.New().String(),
	})
	w := serve(h.GetExport, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestGetExport_GetJobStoreError(t *testing.T) {
	s := &errStore{mockStore: newMockStore(), getExportJobErr: errors.New("db error")}
	h := newUsersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id": validTeacherID.String(), "jobID": uuid.New().String(),
	})
	w := serve(h.GetExport, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

func TestGetExport_BuildError(t *testing.T) {
	s := &errStore{mockStore: newMockStore(), buildUserExportErr: errors.New("build error")}
	jobID := uuid.New()
	s.getExportJobResult = &models.ExportJob{ID: jobID, UserID: validTeacherID, Status: models.ExportStatusPending}
	h := newUsersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id": validTeacherID.String(), "jobID": jobID.String(),
	})
	w := serve(h.GetExport, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

// ============================================================
// TESTS: Auth handler gaps
// ============================================================

func TestRegister_InvalidJSON(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString("{bad}"))
	req.Header.Set("Content-Type", "application/json")
	w := serve(h.Register, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRegister_MissingEmail(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)
	w := postJSON(t, h.Register, "/auth/register", models.RegisterRequest{
		Password:    "Password123!",
		DisplayName: "User",
		// Email omitted
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for missing email, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRegister_MissingDisplayName(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)
	w := postJSON(t, h.Register, "/auth/register", models.RegisterRequest{
		Email:    "a@b.com",
		Password: "Password123!",
		// DisplayName omitted
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for missing display_name, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRegister_RateLimit(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)
	// httptest.NewRequest sets RemoteAddr to "192.0.2.1:1234"
	// Register uses GetLoginAttempts and blocks when attempts >= MaxLoginAttempts (10).
	c.mu.Lock()
	c.attempts["ratelimit:login:192.0.2.1:1234"] = 10
	c.mu.Unlock()
	w := postJSON(t, h.Register, "/auth/register", models.RegisterRequest{
		Email:       "x@x.com",
		Password:    "Password123!",
		DisplayName: "X",
	})
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("want 429 for rate limit, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLogin_InvalidJSON(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString("{bad}"))
	req.Header.Set("Content-Type", "application/json")
	w := serve(h.Login, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLogin_RateLimit(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)
	// Login uses IncrLoginAttempts then checks > MaxLoginAttempts (10).
	// Pre-set to 10 so the handler's increment makes it 11 > 10.
	c.mu.Lock()
	c.attempts["ratelimit:login:192.0.2.1:1234"] = 10
	c.mu.Unlock()
	w := postJSON(t, h.Login, "/auth/login", models.LoginRequest{
		Email:    "a@b.com",
		Password: "pass",
	})
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("want 429 for rate limit, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRealIP_XForwardedFor(t *testing.T) {
	// Exercise the X-Forwarded-For branch of realIP indirectly via Register
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)

	b, _ := json.Marshal(models.RegisterRequest{
		Email:       "ip@example.com",
		Password:    "Password123!",
		DisplayName: "IP Test",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	w := httptest.NewRecorder()
	h.Register(w, req)
	// Should succeed (201) — we're just checking realIP doesn't panic
	if w.Code != http.StatusCreated {
		t.Errorf("want 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRealIP_XRealIP(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)

	b, _ := json.Marshal(models.RegisterRequest{
		Email:       "realip@example.com",
		Password:    "Password123!",
		DisplayName: "Real IP Test",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Real-Ip", "9.9.9.9")
	w := httptest.NewRecorder()
	h.Register(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("want 201, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRegisterLogin_InvalidDOBFormat verifies that a malformed date_of_birth
// in the register request is rejected with 400.
func TestRegisterLogin_InvalidDOBFormat(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)
	dob := "not-a-date"
	w := postJSON(t, h.Register, "/auth/register", models.RegisterRequest{
		Email:       "dob@example.com",
		Password:    "Password123!",
		DisplayName: "DOB User",
		DateOfBirth: &dob,
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for invalid DOB, got %d: %s", w.Code, w.Body.String())
	}
}

// ============================================================
// TESTS: nilIfEmpty / nilIfEmptyStr (exercise via UpdatePreferences)
// ============================================================

func TestNilIfEmpty_EmptyMapBecomesNil(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newUsersHandler(s)
	// Empty LearningStylePrefs → nilIfEmpty returns nil → UpdateLearningProfile NOT called
	r := teacherReq(http.MethodPatch,
		models.UpdatePreferencesRequest{LearningStylePrefs: map[string]float64{}},
		map[string]string{"id": validTeacherID.String()},
	)
	w := serve(h.UpdatePreferences, r)
	// With empty map, the `if req.LearningStylePrefs != nil` branch is taken but
	// nilIfEmpty returns nil — UpdateLearningProfile is still called.
	// We just verify no panic / 204.
	if w.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestNilIfEmptyStr_EmptyMapBecomesNil(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newUsersHandler(s)
	r := teacherReq(http.MethodPatch,
		models.UpdatePreferencesRequest{ExplanationPreferences: map[string]string{}},
		map[string]string{"id": validTeacherID.String()},
	)
	w := serve(h.UpdatePreferences, r)
	if w.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d: %s", w.Code, w.Body.String())
	}
}

// ============================================================
// TESTS: GetProfile gaps
// ============================================================

func TestGetProfile_InvalidUserID(t *testing.T) {
	h := newUsersHandler(&errStore{mockStore: newMockStore()})
	r := chiReq(http.MethodGet, "/", nil, map[string]string{"id": "bad"})
	w := serve(h.GetProfile, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestGetProfile_UserNotFound(t *testing.T) {
	h := newUsersHandler(&errStore{mockStore: newMockStore()})
	// validTeacherID is not in the store, so GetUserByID returns ErrNotFound.
	r := chiReq(http.MethodGet, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.GetProfile, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d: %s", w.Code, w.Body.String())
	}
}

// getUserByIDErrStore returns a specific error from GetUserByID.
type getUserByIDErrStore struct {
	*errStore
	getUserByIDErr error
}

func (s *getUserByIDErrStore) GetUserByID(_ context.Context, _ uuid.UUID) (*models.User, error) {
	return nil, s.getUserByIDErr
}

func TestGetProfile_StoreError(t *testing.T) {
	s := &getUserByIDErrStore{
		errStore:       &errStore{mockStore: newMockStore()},
		getUserByIDErr: errors.New("db error"),
	}
	h := newUsersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.GetProfile, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

// getLearningProfileErrStore injects an error into GetLearningProfile.
type getLearningProfileErrStore struct {
	*errStore
	learningProfileErr error
}

func (s *getLearningProfileErrStore) GetLearningProfile(_ context.Context, _ uuid.UUID) (*models.LearningProfile, error) {
	return nil, s.learningProfileErr
}

func TestGetProfile_LearningProfileError(t *testing.T) {
	base := &errStore{mockStore: newMockStore()}
	base.byID[validTeacherID] = &models.User{ID: validTeacherID, Email: "a@b.com"}
	s := &getLearningProfileErrStore{
		errStore:           base,
		learningProfileErr: errors.New("db error"), // non-ErrNotFound
	}
	h := newUsersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.GetProfile, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ============================================================
// TESTS: CompleteOnboarding gaps
// ============================================================

func TestCompleteOnboarding_InvalidUserID(t *testing.T) {
	h := newUsersHandler(&errStore{mockStore: newMockStore()})
	r := chiReq(http.MethodPatch, "/", nil, map[string]string{"id": "bad"})
	w := serve(h.CompleteOnboarding, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// completeOnboardingErrStore returns error from CompleteOnboarding.
type completeOnboardingErrStore struct {
	*errStore
	completeOnboardingErr error
}

func (s *completeOnboardingErrStore) CompleteOnboarding(_ context.Context, _ uuid.UUID) error {
	return s.completeOnboardingErr
}

func TestCompleteOnboarding_StoreError(t *testing.T) {
	s := &completeOnboardingErrStore{
		errStore:              &errStore{mockStore: newMockStore()},
		completeOnboardingErr: errors.New("db error"),
	}
	h := newUsersHandler(s)
	r := chiReq(http.MethodPatch, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.CompleteOnboarding, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

// ============================================================
// TESTS: DeleteAccount gaps
// ============================================================

func TestDeleteAccount_InvalidUserID(t *testing.T) {
	h := newUsersHandler(&errStore{mockStore: newMockStore()})
	r := chiReq(http.MethodDelete, "/", nil, map[string]string{"id": "bad"})
	w := serve(h.DeleteAccount, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// deleteUserErrStore returns error from DeleteUser.
type deleteUserErrStore struct {
	*errStore
	deleteUserErr error
}

func (s *deleteUserErrStore) DeleteUser(_ context.Context, _ uuid.UUID) error {
	return s.deleteUserErr
}

func TestDeleteAccount_StoreError(t *testing.T) {
	s := &deleteUserErrStore{
		errStore:      &errStore{mockStore: newMockStore()},
		deleteUserErr: errors.New("db error"),
	}
	h := newUsersHandler(s)
	r := chiReq(http.MethodDelete, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.DeleteAccount, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

// ============================================================
// TESTS: Refresh error paths
// ============================================================

// revokeTokenErrStore injects error into RevokeRefreshToken.
type revokeTokenErrStore struct {
	*mockStore
	revokeErr error
}

func (s *revokeTokenErrStore) RevokeRefreshToken(_ context.Context, _ string) error {
	return s.revokeErr
}

func TestRefresh_RevokeError(t *testing.T) {
	base := newMockStore()
	c := newMockCache()
	// Use a normal handler to do mustRegister (populates the mockStore).
	normalH := newTestAuthHandler(base, c)
	_, refreshCookie := mustRegister(t, normalH, base)

	// Now swap to a store that errors on RevokeRefreshToken.
	s := &revokeTokenErrStore{mockStore: base, revokeErr: errors.New("db error")}
	h := newTestAuthHandler(s, c)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(refreshCookie)
	w := serve(h.Refresh, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500 for revoke error, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRefresh_UserNotFoundAfterRevoke(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)

	// Register to get a valid refresh token
	_, refreshCookie := mustRegister(t, h, s)

	// Now remove the user from the store so GetUserByID fails
	for id := range s.byID {
		delete(s.byID, id)
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(refreshCookie)
	w := serve(h.Refresh, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401 for user not found after revoke, got %d: %s", w.Code, w.Body.String())
	}
}

// ============================================================
// TESTS: Teachers assertClassOwner forbidden paths (via classOwnerStore)
// ============================================================

func TestRemoveStudent_Forbidden(t *testing.T) {
	s := newClassOwnerStore(uuid.New()) // different owner
	h := newTeachersHandler(s)
	r := chiReq(http.MethodDelete, "/", nil, map[string]string{
		"id": validTeacherID.String(), "class_id": validClassID.String(), "student_id": validStudentID.String(),
	})
	w := serve(h.RemoveStudent, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", w.Code)
	}
}

func TestRemoveStudent_AssertOwnerNotFound(t *testing.T) {
	s := newClassOwnerStore(validTeacherID)
	s.getClassErr = store.ErrNotFound
	h := newTeachersHandler(s)
	r := chiReq(http.MethodDelete, "/", nil, map[string]string{
		"id": validTeacherID.String(), "class_id": validClassID.String(), "student_id": validStudentID.String(),
	})
	w := serve(h.RemoveStudent, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestRemoveStudent_StoreError(t *testing.T) {
	s := &errStore{mockStore: newMockStore(), removeStudentErr: errors.New("db error")}
	h := newTeachersHandler(s)
	r := chiReq(http.MethodDelete, "/", nil, map[string]string{
		"id": validTeacherID.String(), "class_id": validClassID.String(), "student_id": validStudentID.String(),
	})
	w := serve(h.RemoveStudent, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

func TestListRoster_Forbidden(t *testing.T) {
	s := newClassOwnerStore(uuid.New())
	h := newTeachersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id": validTeacherID.String(), "class_id": validClassID.String(),
	})
	w := serve(h.ListRoster, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", w.Code)
	}
}

func TestListRoster_AssertOwnerNotFound(t *testing.T) {
	s := newClassOwnerStore(validTeacherID)
	s.getClassErr = store.ErrNotFound
	h := newTeachersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id": validTeacherID.String(), "class_id": validClassID.String(),
	})
	w := serve(h.ListRoster, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestAssignMaterial_Forbidden(t *testing.T) {
	s := newClassOwnerStore(uuid.New())
	h := newTeachersHandler(s)
	r := teacherReq(http.MethodPost, models.AssignMaterialRequest{MaterialID: validMaterialID.String()},
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()})
	w := serve(h.AssignMaterial, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", w.Code)
	}
}

func TestAssignMaterial_InvalidJSON(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newTeachersHandler(s)
	req := chiReq(http.MethodPost, "/", bytes.NewBufferString("{bad}"),
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()})
	w := serve(h.AssignMaterial, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestUnassignMaterial_Forbidden(t *testing.T) {
	s := newClassOwnerStore(uuid.New())
	h := newTeachersHandler(s)
	r := chiReq(http.MethodDelete, "/", nil, map[string]string{
		"id": validTeacherID.String(), "class_id": validClassID.String(), "material_id": validMaterialID.String(),
	})
	w := serve(h.UnassignMaterial, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", w.Code)
	}
}

func TestListAssignedMaterials_Forbidden(t *testing.T) {
	s := newClassOwnerStore(uuid.New())
	h := newTeachersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id": validTeacherID.String(), "class_id": validClassID.String(),
	})
	w := serve(h.ListAssignedMaterials, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", w.Code)
	}
}

func TestGetStudentProgress_Forbidden(t *testing.T) {
	s := newClassOwnerStore(uuid.New())
	h := newTeachersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id": validTeacherID.String(), "class_id": validClassID.String(), "student_id": validStudentID.String(),
	})
	w := serve(h.GetStudentProgress, r)
	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", w.Code)
	}
}

// TestWriteClassOwnerError_NotFound exercises the ErrNotFound branch of writeClassOwnerError.
func TestWriteClassOwnerError_NotFound(t *testing.T) {
	s := newClassOwnerStore(validTeacherID)
	s.getClassErr = store.ErrNotFound
	h := newTeachersHandler(s)
	sid := validStudentID.String()
	r := teacherReq(http.MethodPost, models.AddStudentRequest{StudentID: &sid},
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()})
	w := serve(h.AddStudent, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ============================================================
// TESTS: Subscription gaps
// ============================================================

func TestReactivateSubscription_InvalidUserID(t *testing.T) {
	h := newSubsHandler(&errStore{mockStore: newMockStore()}, &mockSubscriptionManager{})
	r := chiReq(http.MethodPost, "/", nil, map[string]string{"id": "bad"})
	w := serve(h.ReactivateSubscription, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestReactivateSubscription_StoreError(t *testing.T) {
	s := subsStore(false, errors.New("db error"))
	h := newSubsHandler(s, &mockSubscriptionManager{})
	r := chiReq(http.MethodPost, "/", nil, map[string]string{"id": validTeacherID.String()})
	w := serve(h.ReactivateSubscription, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

// ============================================================
// TESTS: issueTokenPair error path
// ============================================================

// createRefreshTokenErrStore injects error on CreateRefreshToken.
type createRefreshTokenErrStore struct {
	*mockStore
	createTokenErr error
}

func (s *createRefreshTokenErrStore) CreateRefreshToken(_ context.Context, _ store.CreateTokenParams) error {
	return s.createTokenErr
}

func TestRegister_IssueTokenPairError(t *testing.T) {
	base := newMockStore()
	s := &createRefreshTokenErrStore{mockStore: base, createTokenErr: errors.New("db error")}
	c := newMockCache()
	h := newTestAuthHandler(s, c)
	w := postJSON(t, h.Register, "/auth/register", models.RegisterRequest{
		Email:       "token@fail.com",
		Password:    "Password123!",
		DisplayName: "Token Fail",
	})
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500 for issueTokenPair error, got %d: %s", w.Code, w.Body.String())
	}
}

// ============================================================
// TESTS: nilIfEmptyStr non-empty path
// ============================================================

func TestNilIfEmptyStr_NonEmptyReturnsPointer(t *testing.T) {
	s := &errStore{mockStore: newMockStore()}
	h := newUsersHandler(s)
	r := teacherReq(http.MethodPatch,
		models.UpdatePreferencesRequest{ExplanationPreferences: map[string]string{"tone": "formal"}},
		map[string]string{"id": validTeacherID.String()},
	)
	w := serve(h.UpdatePreferences, r)
	if w.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d: %s", w.Code, w.Body.String())
	}
}

// ============================================================
// TESTS: AdminHandler gaps (complement compliance_test.go)
// ============================================================

func TestGetAuditLog_InvalidTo(t *testing.T) {
	h := handlers.NewAdminHandler(newMockStore())
	req := httptest.NewRequest(http.MethodGet, "/admin/audit?to=bad-date", nil)
	w := httptest.NewRecorder()
	h.GetAuditLog(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for invalid to, got %d", w.Code)
	}
}

func TestGetAuditLog_InvalidLimit(t *testing.T) {
	h := handlers.NewAdminHandler(newMockStore())
	req := httptest.NewRequest(http.MethodGet, "/admin/audit?limit=bad", nil)
	w := httptest.NewRecorder()
	h.GetAuditLog(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for invalid limit, got %d", w.Code)
	}
}

func TestGetAuditLog_ZeroLimit(t *testing.T) {
	h := handlers.NewAdminHandler(newMockStore())
	req := httptest.NewRequest(http.MethodGet, "/admin/audit?limit=0", nil)
	w := httptest.NewRecorder()
	h.GetAuditLog(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for zero limit, got %d", w.Code)
	}
}

func TestGetAuditLog_InvalidOffset(t *testing.T) {
	h := handlers.NewAdminHandler(newMockStore())
	req := httptest.NewRequest(http.MethodGet, "/admin/audit?offset=bad", nil)
	w := httptest.NewRecorder()
	h.GetAuditLog(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400 for invalid offset, got %d", w.Code)
	}
}

func TestGetAuditLog_ValidLimitAndOffset(t *testing.T) {
	h := handlers.NewAdminHandler(newMockStore())
	req := httptest.NewRequest(http.MethodGet, "/admin/audit?limit=50&offset=10", nil)
	w := httptest.NewRecorder()
	h.GetAuditLog(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetAuditLog_ValidToDate(t *testing.T) {
	h := handlers.NewAdminHandler(newMockStore())
	req := httptest.NewRequest(http.MethodGet, "/admin/audit?to=2026-01-01T00:00:00Z", nil)
	w := httptest.NewRecorder()
	h.GetAuditLog(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d: %s", w.Code, w.Body.String())
	}
}

// queryAuditLogErrStore injects error into QueryAuditLog.
type queryAuditLogErrStore struct {
	*mockStore
	queryErr error
}

func (s *queryAuditLogErrStore) QueryAuditLog(_ context.Context, _ store.QueryAuditLogParams) ([]*models.AuditEntry, error) {
	return nil, s.queryErr
}

func TestGetAuditLog_QueryStoreError(t *testing.T) {
	s := &queryAuditLogErrStore{mockStore: newMockStore(), queryErr: errors.New("db error")}
	h := handlers.NewAdminHandler(s)
	req := httptest.NewRequest(http.MethodGet, "/admin/audit", nil)
	w := httptest.NewRecorder()
	h.GetAuditLog(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500 for store error, got %d", w.Code)
	}
}

// ============================================================
// TESTS: issueTokenPair nil subscription path
// ============================================================

func TestLogin_NoSubscription_IssuesToken(t *testing.T) {
	// A user with no subscription: GetSubscriptionByUserID returns ErrNotFound,
	// sub is nil, issueTokenPair is called with nil sub (subStatus = "").
	s := newMockStore()
	c := newMockCache()

	// Create user directly without a subscription
	hash, _ := bcrypt.GenerateFromPassword([]byte("Password123!"), bcrypt.MinCost)
	u := &models.User{
		ID:           uuid.New(),
		Email:        "nosub@example.com",
		PasswordHash: string(hash),
		DisplayName:  "NoSub",
		AccountType:  models.AccountTypeStandard,
	}
	s.users[u.Email] = u
	s.byID[u.ID] = u
	// No subscription created — subs map is empty

	h := newTestAuthHandler(s, c)
	w := postJSON(t, h.Login, "/auth/login", models.LoginRequest{
		Email:    "nosub@example.com",
		Password: "Password123!",
	})
	if w.Code != http.StatusOK {
		t.Errorf("want 200 for login with nil sub, got %d: %s", w.Code, w.Body.String())
	}
}

// ============================================================
// TESTS: GetClass/UpdateClass/DeleteClass internal store error
// ============================================================

func TestGetClass_InternalStoreError(t *testing.T) {
	s := newClassOwnerStore(validTeacherID)
	s.getClassErr = errors.New("db error") // non-ErrNotFound
	h := newTeachersHandler(s)
	r := chiReq(http.MethodGet, "/", nil, map[string]string{
		"id": validTeacherID.String(), "class_id": validClassID.String(),
	})
	w := serve(h.GetClass, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

func TestUpdateClass_GetClassInternalError(t *testing.T) {
	s := newClassOwnerStore(validTeacherID)
	s.getClassErr = errors.New("db error")
	h := newTeachersHandler(s)
	name := "X"
	r := teacherReq(http.MethodPatch, models.UpdateClassRequest{Name: &name},
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()},
	)
	w := serve(h.UpdateClass, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

func TestDeleteClass_GetClassInternalError(t *testing.T) {
	s := newClassOwnerStore(validTeacherID)
	s.getClassErr = errors.New("db error")
	h := newTeachersHandler(s)
	r := chiReq(http.MethodDelete, "/", nil,
		map[string]string{"id": validTeacherID.String(), "class_id": validClassID.String()})
	w := serve(h.DeleteClass, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
}

// ============================================================
// TESTS: UpdateConsent gaps
// ============================================================

func TestUpdateConsent_InvalidUserID(t *testing.T) {
	h := newUsersHandler(&errStore{mockStore: newMockStore()})
	r := chiReq(http.MethodPatch, "/", jsonBody(map[string]string{}), map[string]string{"id": "bad"})
	r.Header.Set("Content-Type", "application/json")
	w := serve(h.UpdateConsent, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// updateConsentErrStore returns error from UpdateConsent.
type updateConsentErrStore struct {
	*errStore
	updateConsentErr error
}

func (s *updateConsentErrStore) UpdateConsent(_ context.Context, _ uuid.UUID, _ store.UpdateConsentParams) error {
	return s.updateConsentErr
}

func TestUpdateConsent_StoreError(t *testing.T) {
	s := &updateConsentErrStore{
		errStore:         &errStore{mockStore: newMockStore()},
		updateConsentErr: errors.New("db error"),
	}
	h := newUsersHandler(s)
	r := chiReq(http.MethodPatch, "/",
		jsonBody(map[string]bool{"tutoring": true}),
		map[string]string{"id": validTeacherID.String()})
	r.Header.Set("Content-Type", "application/json")
	w := serve(h.UpdateConsent, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d: %s", w.Code, w.Body.String())
	}
}

// ============================================================
// Suppress unused import warning for fmt
// ============================================================
var _ = fmt.Sprintf
