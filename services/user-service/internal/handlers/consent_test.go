package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/handlers"
	"github.com/teacherslounge/user-service/internal/models"
)

// buildConsentRequest builds an httptest.Request with a chi route context for {id}.
func buildConsentRequest(method, userID string, body []byte) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, "/users/"+userID+"/consent", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, "/users/"+userID+"/consent", nil)
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", userID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	return req
}

// TestGetConsent_AdultUser verifies a non-minor returns consent_required=false.
func TestGetConsent_AdultUser(t *testing.T) {
	ms := newMockStore()
	uid := uuid.New()
	dob := time.Now().AddDate(-25, 0, 0) // 25 years old
	ms.byID[uid] = &models.User{
		ID:          uid,
		Email:       "adult@example.com",
		AccountType: models.AccountTypeStandard,
		DateOfBirth: &dob,
	}

	h := handlers.NewUsersHandler(ms, newMockCache())
	rec := httptest.NewRecorder()
	h.GetConsent(rec, buildConsentRequest(http.MethodGet, uid.String(), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp models.ConsentStatus
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.ConsentRequired {
		t.Error("expected consent_required=false for adult")
	}
	if resp.ConsentGiven {
		t.Error("expected consent_given=false for adult")
	}
}

// TestGetConsent_MinorWithoutConsent verifies a minor without consent returns correct flags.
func TestGetConsent_MinorWithoutConsent(t *testing.T) {
	ms := newMockStore()
	uid := uuid.New()
	dob := time.Now().AddDate(-14, 0, 0) // 14 years old
	guardianEmail := "parent@example.com"
	ms.byID[uid] = &models.User{
		ID:            uid,
		Email:         "kid@example.com",
		AccountType:   models.AccountTypeMinor,
		DateOfBirth:   &dob,
		GuardianEmail: &guardianEmail,
	}

	h := handlers.NewUsersHandler(ms, newMockCache())
	rec := httptest.NewRecorder()
	h.GetConsent(rec, buildConsentRequest(http.MethodGet, uid.String(), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp models.ConsentStatus
	json.NewDecoder(rec.Body).Decode(&resp)
	if !resp.IsMinor {
		t.Error("expected is_minor=true")
	}
	if !resp.ConsentRequired {
		t.Error("expected consent_required=true")
	}
	if resp.ConsentGiven {
		t.Error("expected consent_given=false before consent")
	}
}

// TestGetConsent_MinorWithConsent verifies a minor with consent given returns consent_given=true.
func TestGetConsent_MinorWithConsent(t *testing.T) {
	ms := newMockStore()
	uid := uuid.New()
	dob := time.Now().AddDate(-14, 0, 0)
	guardianEmail := "parent@example.com"
	consentAt := time.Now().Add(-24 * time.Hour)
	ms.byID[uid] = &models.User{
		ID:                uid,
		Email:             "kid@example.com",
		AccountType:       models.AccountTypeMinor,
		DateOfBirth:       &dob,
		GuardianEmail:     &guardianEmail,
		GuardianConsentAt: &consentAt,
	}

	h := handlers.NewUsersHandler(ms, newMockCache())
	rec := httptest.NewRecorder()
	h.GetConsent(rec, buildConsentRequest(http.MethodGet, uid.String(), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp models.ConsentStatus
	json.NewDecoder(rec.Body).Decode(&resp)
	if !resp.ConsentGiven {
		t.Error("expected consent_given=true")
	}
}

// TestUpdateConsent_SuccessForMinor verifies guardian consent is recorded for a minor.
func TestUpdateConsent_SuccessForMinor(t *testing.T) {
	ms := newMockStore()
	uid := uuid.New()
	dob := time.Now().AddDate(-14, 0, 0)
	guardianEmail := "parent@example.com"
	ms.byID[uid] = &models.User{
		ID:            uid,
		Email:         "kid@example.com",
		AccountType:   models.AccountTypeMinor,
		DateOfBirth:   &dob,
		GuardianEmail: &guardianEmail,
	}

	body, _ := json.Marshal(map[string]string{"guardian_email": "parent@example.com"})
	h := handlers.NewUsersHandler(ms, newMockCache())
	rec := httptest.NewRecorder()
	h.UpdateConsent(rec, buildConsentRequest(http.MethodPatch, uid.String(), body))

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestUpdateConsent_WrongGuardianEmail returns 422 when guardian_email doesn't match.
func TestUpdateConsent_WrongGuardianEmail(t *testing.T) {
	ms := newMockStore()
	uid := uuid.New()
	dob := time.Now().AddDate(-14, 0, 0)
	guardianEmail := "parent@example.com"
	ms.byID[uid] = &models.User{
		ID:            uid,
		Email:         "kid@example.com",
		AccountType:   models.AccountTypeMinor,
		DateOfBirth:   &dob,
		GuardianEmail: &guardianEmail,
	}

	body, _ := json.Marshal(map[string]string{"guardian_email": "wrong@example.com"})
	h := handlers.NewUsersHandler(ms, newMockCache())
	rec := httptest.NewRecorder()
	h.UpdateConsent(rec, buildConsentRequest(http.MethodPatch, uid.String(), body))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rec.Code)
	}
}

// TestUpdateConsent_AdultReturns422 verifies non-minor users get 422.
func TestUpdateConsent_AdultReturns422(t *testing.T) {
	ms := newMockStore()
	uid := uuid.New()
	dob := time.Now().AddDate(-30, 0, 0)
	ms.byID[uid] = &models.User{
		ID:          uid,
		Email:       "adult@example.com",
		AccountType: models.AccountTypeStandard,
		DateOfBirth: &dob,
	}

	body, _ := json.Marshal(map[string]string{"guardian_email": "anyone@example.com"})
	h := handlers.NewUsersHandler(ms, newMockCache())
	rec := httptest.NewRecorder()
	h.UpdateConsent(rec, buildConsentRequest(http.MethodPatch, uid.String(), body))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rec.Code)
	}
}
