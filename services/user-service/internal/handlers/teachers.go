package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/middleware"
	"github.com/teacherslounge/user-service/internal/models"
	"github.com/teacherslounge/user-service/internal/store"
)

// TeachersHandler handles teacher-specific endpoints (profile, classes, students, materials).
type TeachersHandler struct {
	store store.Storer
}

// NewTeachersHandler creates a TeachersHandler backed by the given store.
func NewTeachersHandler(s store.Storer) *TeachersHandler {
	return &TeachersHandler{store: s}
}

// POST /users/{id}/teacher-profile
func (h *TeachersHandler) CreateTeacherProfile(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req models.CreateTeacherProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	profile, err := h.store.CreateTeacherProfile(r.Context(), store.CreateTeacherProfileParams{
		UserID:     userID,
		SchoolName: req.SchoolName,
		Bio:        req.Bio,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create teacher profile")
		return
	}

	writeJSON(w, http.StatusCreated, profile)
}

// GET /users/{id}/teacher-profile
func (h *TeachersHandler) GetTeacherProfile(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	profile, err := h.store.GetTeacherProfile(r.Context(), userID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "teacher profile not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

// ============================================================
// CLASSES
// ============================================================

// POST /teachers/{id}/classes
func (h *TeachersHandler) CreateClass(w http.ResponseWriter, r *http.Request) {
	teacherID, err := parseUserIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid teacher id")
		return
	}

	var req models.CreateClassRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	class, err := h.store.CreateClass(r.Context(), store.CreateClassParams{
		TeacherID:   teacherID,
		Name:        req.Name,
		Subject:     req.Subject,
		Description: req.Description,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create class")
		return
	}

	writeJSON(w, http.StatusCreated, class)
}

// GET /teachers/{id}/classes
func (h *TeachersHandler) ListClasses(w http.ResponseWriter, r *http.Request) {
	teacherID, err := parseUserIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid teacher id")
		return
	}

	classes, err := h.store.ListClasses(r.Context(), teacherID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if classes == nil {
		classes = []*models.TeacherClass{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"classes": classes})
}

// GET /teachers/{id}/classes/{class_id}
func (h *TeachersHandler) GetClass(w http.ResponseWriter, r *http.Request) {
	teacherID, classID, ok := parseTeacherClassParams(w, r)
	if !ok {
		return
	}

	class, err := h.store.GetClass(r.Context(), classID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "class not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if class.TeacherID != teacherID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	writeJSON(w, http.StatusOK, class)
}

// PATCH /teachers/{id}/classes/{class_id}
func (h *TeachersHandler) UpdateClass(w http.ResponseWriter, r *http.Request) {
	teacherID, classID, ok := parseTeacherClassParams(w, r)
	if !ok {
		return
	}

	existing, err := h.store.GetClass(r.Context(), classID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "class not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing.TeacherID != teacherID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	var req models.UpdateClassRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updated, err := h.store.UpdateClass(r.Context(), classID, store.UpdateClassParams{
		Name:        req.Name,
		Subject:     req.Subject,
		Description: req.Description,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update class")
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

// DELETE /teachers/{id}/classes/{class_id}
func (h *TeachersHandler) DeleteClass(w http.ResponseWriter, r *http.Request) {
	teacherID, classID, ok := parseTeacherClassParams(w, r)
	if !ok {
		return
	}

	existing, err := h.store.GetClass(r.Context(), classID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "class not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing.TeacherID != teacherID {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	if err := h.store.DeleteClass(r.Context(), classID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete class")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ============================================================
// ROSTER
// ============================================================

// POST /teachers/{id}/classes/{class_id}/students
func (h *TeachersHandler) AddStudent(w http.ResponseWriter, r *http.Request) {
	teacherID, classID, ok := parseTeacherClassParams(w, r)
	if !ok {
		return
	}
	if err := h.assertClassOwner(r, classID, teacherID); err != nil {
		writeClassOwnerError(w, err)
		return
	}

	var req models.AddStudentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	studentID, err := h.resolveStudent(r, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.store.AddStudentToClass(r.Context(), classID, studentID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add student")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /teachers/{id}/classes/{class_id}/students/{student_id}
func (h *TeachersHandler) RemoveStudent(w http.ResponseWriter, r *http.Request) {
	teacherID, classID, ok := parseTeacherClassParams(w, r)
	if !ok {
		return
	}
	if err := h.assertClassOwner(r, classID, teacherID); err != nil {
		writeClassOwnerError(w, err)
		return
	}

	studentID, err := uuid.Parse(chi.URLParam(r, "student_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid student id")
		return
	}

	if err := h.store.RemoveStudentFromClass(r.Context(), classID, studentID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove student")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /teachers/{id}/classes/{class_id}/students
func (h *TeachersHandler) ListRoster(w http.ResponseWriter, r *http.Request) {
	teacherID, classID, ok := parseTeacherClassParams(w, r)
	if !ok {
		return
	}
	if err := h.assertClassOwner(r, classID, teacherID); err != nil {
		writeClassOwnerError(w, err)
		return
	}

	roster, err := h.store.ListClassRoster(r.Context(), classID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if roster == nil {
		roster = []*models.StudentSummary{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"students": roster})
}

// ============================================================
// STUDENT PROGRESS
// ============================================================

// GET /teachers/{id}/classes/{class_id}/students/{student_id}/progress
func (h *TeachersHandler) GetStudentProgress(w http.ResponseWriter, r *http.Request) {
	teacherID, classID, ok := parseTeacherClassParams(w, r)
	if !ok {
		return
	}
	if err := h.assertClassOwner(r, classID, teacherID); err != nil {
		writeClassOwnerError(w, err)
		return
	}

	studentID, err := uuid.Parse(chi.URLParam(r, "student_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid student id")
		return
	}

	// Audit: teacher accessing student data
	claims := middleware.ClaimsFromCtx(r.Context())
	if claims != nil {
		_ = h.store.WriteAuditLog(r.Context(), store.AuditLogParams{
			AccessorID:   &teacherID,
			StudentID:    &studentID,
			Action:       "teacher_progress_view",
			DataAccessed: "gaming_profile,learning_profile,quiz_results",
			Purpose:      "teacher_class_management",
			IPAddress:    realIP(r),
		})
	}

	progress, err := h.store.GetStudentProgress(r.Context(), studentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "student not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, progress)
}

// ============================================================
// MATERIAL ASSIGNMENTS
// ============================================================

// POST /teachers/{id}/classes/{class_id}/materials
func (h *TeachersHandler) AssignMaterial(w http.ResponseWriter, r *http.Request) {
	teacherID, classID, ok := parseTeacherClassParams(w, r)
	if !ok {
		return
	}
	if err := h.assertClassOwner(r, classID, teacherID); err != nil {
		writeClassOwnerError(w, err)
		return
	}

	var req models.AssignMaterialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	materialID, err := uuid.Parse(req.MaterialID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid material_id")
		return
	}

	if err := h.store.AssignMaterialToClass(r.Context(), store.AssignMaterialParams{
		ClassID:    classID,
		MaterialID: materialID,
		DueDate:    req.DueDate,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to assign material")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DELETE /teachers/{id}/classes/{class_id}/materials/{material_id}
func (h *TeachersHandler) UnassignMaterial(w http.ResponseWriter, r *http.Request) {
	teacherID, classID, ok := parseTeacherClassParams(w, r)
	if !ok {
		return
	}
	if err := h.assertClassOwner(r, classID, teacherID); err != nil {
		writeClassOwnerError(w, err)
		return
	}

	materialID, err := uuid.Parse(chi.URLParam(r, "material_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid material id")
		return
	}

	if err := h.store.UnassignMaterialFromClass(r.Context(), classID, materialID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to unassign material")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /teachers/{id}/classes/{class_id}/materials
func (h *TeachersHandler) ListAssignedMaterials(w http.ResponseWriter, r *http.Request) {
	teacherID, classID, ok := parseTeacherClassParams(w, r)
	if !ok {
		return
	}
	if err := h.assertClassOwner(r, classID, teacherID); err != nil {
		writeClassOwnerError(w, err)
		return
	}

	materials, err := h.store.ListClassMaterials(r.Context(), classID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if materials == nil {
		materials = []*models.ClassMaterialAssignment{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"materials": materials})
}

// ============================================================
// HELPERS
// ============================================================

func parseTeacherClassParams(w http.ResponseWriter, r *http.Request) (teacherID, classID uuid.UUID, ok bool) {
	var err error
	teacherID, err = parseUserIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid teacher id")
		return uuid.Nil, uuid.Nil, false
	}
	classID, err = uuid.Parse(chi.URLParam(r, "class_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid class id")
		return uuid.Nil, uuid.Nil, false
	}
	return teacherID, classID, true
}

func (h *TeachersHandler) assertClassOwner(r *http.Request, classID, teacherID uuid.UUID) error {
	class, err := h.store.GetClass(r.Context(), classID)
	if err != nil {
		return err
	}
	if class.TeacherID != teacherID {
		return errForbidden
	}
	return nil
}

var errForbidden = errors.New("forbidden")

func writeClassOwnerError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "class not found")
	} else if errors.Is(err, errForbidden) {
		writeError(w, http.StatusForbidden, "forbidden")
	} else {
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

// resolveStudent resolves a student from the AddStudentRequest (by ID or email).
func (h *TeachersHandler) resolveStudent(r *http.Request, req models.AddStudentRequest) (uuid.UUID, error) {
	if req.StudentID != nil {
		id, err := uuid.Parse(*req.StudentID)
		if err != nil {
			return uuid.Nil, errors.New("invalid student_id")
		}
		return id, nil
	}
	if req.Email != nil {
		user, err := h.store.GetUserByEmail(r.Context(), *req.Email)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return uuid.Nil, errors.New("student not found")
			}
			return uuid.Nil, errors.New("internal error")
		}
		return user.ID, nil
	}
	return uuid.Nil, errors.New("student_id or email is required")
}
