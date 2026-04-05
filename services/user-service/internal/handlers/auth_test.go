package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/auth"
	"github.com/teacherslounge/user-service/internal/cache"
	"github.com/teacherslounge/user-service/internal/config"
	"github.com/teacherslounge/user-service/internal/handlers"
	"github.com/teacherslounge/user-service/internal/models"
	"github.com/teacherslounge/user-service/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// ============================================================
// MOCK STORE
// ============================================================

type mockStore struct {
	mu    sync.Mutex
	users map[string]*models.User     // keyed by email
	byID  map[uuid.UUID]*models.User
	subs  map[uuid.UUID]*models.Subscription
	toks  map[string]*models.AuthToken // keyed by token hash
	// FERPA/GDPR test helpers
	auditEntries   []*models.AuditEntry
	lastAuditQuery *store.QueryAuditLogParams
	exportJobs     map[uuid.UUID]*models.ExportJob
}

func newMockStore() *mockStore {
	return &mockStore{
		users:      map[string]*models.User{},
		byID:       map[uuid.UUID]*models.User{},
		subs:       map[uuid.UUID]*models.Subscription{},
		toks:       map[string]*models.AuthToken{},
		exportJobs: map[uuid.UUID]*models.ExportJob{},
	}
}

func (m *mockStore) CreateUser(_ context.Context, p store.CreateUserParams) (*models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.users[p.Email]; exists {
		return nil, fmt.Errorf("duplicate email")
	}
	u := &models.User{
		ID:          uuid.New(),
		Email:       p.Email,
		PasswordHash: p.PasswordHash,
		DisplayName: p.DisplayName,
		AvatarEmoji: p.AvatarEmoji,
		AccountType: p.AccountType,
		DateOfBirth: p.DateOfBirth,
		GuardianEmail: p.GuardianEmail,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.users[p.Email] = u
	m.byID[u.ID] = u
	return u, nil
}

func (m *mockStore) GetUserByEmail(_ context.Context, email string) (*models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[email]
	if !ok {
		return nil, store.ErrNotFound
	}
	return u, nil
}

func (m *mockStore) GetUserByID(_ context.Context, id uuid.UUID) (*models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.byID[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return u, nil
}

func (m *mockStore) UpdateUser(_ context.Context, id uuid.UUID, p store.UpdateUserParams) (*models.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.byID[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	if p.DisplayName != nil {
		u.DisplayName = *p.DisplayName
	}
	return u, nil
}

func (m *mockStore) DeleteUser(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.byID[id]
	if !ok {
		return nil
	}
	delete(m.users, u.Email)
	delete(m.byID, id)
	return nil
}

func (m *mockStore) InitLearningProfile(_ context.Context, _ uuid.UUID) error { return nil }
func (m *mockStore) InitGamingProfile(_ context.Context, _ uuid.UUID) error   { return nil }

func (m *mockStore) GetLearningProfile(_ context.Context, _ uuid.UUID) (*models.LearningProfile, error) {
	return &models.LearningProfile{}, nil
}
func (m *mockStore) UpdateLearningProfile(_ context.Context, _ uuid.UUID, _ store.UpdateProfileParams) error {
	return nil
}

func (m *mockStore) CreateRefreshToken(_ context.Context, p store.CreateTokenParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.toks[p.TokenHash] = &models.AuthToken{
		ID:        uuid.New(),
		UserID:    p.UserID,
		TokenHash: p.TokenHash,
		ExpiresAt: p.ExpiresAt,
		CreatedAt: time.Now(),
	}
	return nil
}

func (m *mockStore) GetRefreshToken(_ context.Context, hash string) (*models.AuthToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.toks[hash]
	if !ok || t.RevokedAt != nil || t.ExpiresAt.Before(time.Now()) {
		return nil, store.ErrNotFound
	}
	return t, nil
}

func (m *mockStore) RevokeRefreshToken(_ context.Context, hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.toks[hash]; ok {
		now := time.Now()
		t.RevokedAt = &now
	}
	return nil
}

func (m *mockStore) RevokeAllUserTokens(_ context.Context, _ uuid.UUID) error { return nil }

func (m *mockStore) CreateSubscription(_ context.Context, p store.CreateSubscriptionParams) (*models.Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := &models.Subscription{
		ID:               uuid.New(),
		UserID:           p.UserID,
		StripeCustomerID: p.StripeCustomerID,
		Plan:             p.Plan,
		Status:           p.Status,
		TrialEnd:         p.TrialEnd,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	m.subs[p.UserID] = s
	return s, nil
}

func (m *mockStore) GetSubscriptionByUserID(_ context.Context, userID uuid.UUID) (*models.Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.subs[userID]
	if !ok {
		return nil, store.ErrNotFound
	}
	return s, nil
}

func (m *mockStore) UpdateSubscription(_ context.Context, _ store.UpdateSubscriptionParams) error {
	return nil
}

func (m *mockStore) UpdateSubscriptionByUserID(_ context.Context, _ uuid.UUID, _ store.UpdateSubscriptionParams) error {
	return nil
}

func (m *mockStore) WriteAuditLog(_ context.Context, _ store.AuditLogParams) error { return nil }
func (m *mockStore) CreateExportJob(_ context.Context, _ uuid.UUID) (uuid.UUID, error) {
	return uuid.New(), nil
}
func (m *mockStore) CreateErasureJob(_ context.Context, _ uuid.UUID, _ map[string]any) (uuid.UUID, error) {
	return uuid.New(), nil
}

// Consent stubs
func (m *mockStore) InitConsent(_ context.Context, _ uuid.UUID, _, _ string) error { return nil }
func (m *mockStore) GetConsent(_ context.Context, _ uuid.UUID) (*models.ConsentBundle, error) {
	return &models.ConsentBundle{}, nil
}
func (m *mockStore) UpdateConsent(_ context.Context, _ uuid.UUID, _ store.UpdateConsentParams) error {
	return nil
}

// Teacher profile stubs
func (m *mockStore) CreateTeacherProfile(_ context.Context, _ store.CreateTeacherProfileParams) (*models.TeacherProfile, error) {
	return nil, nil
}
func (m *mockStore) GetTeacherProfile(_ context.Context, _ uuid.UUID) (*models.TeacherProfile, error) {
	return nil, store.ErrNotFound
}

// Class stubs
func (m *mockStore) CreateClass(_ context.Context, _ store.CreateClassParams) (*models.TeacherClass, error) {
	return nil, nil
}
func (m *mockStore) GetClass(_ context.Context, _ uuid.UUID) (*models.TeacherClass, error) {
	return nil, store.ErrNotFound
}
func (m *mockStore) ListClasses(_ context.Context, _ uuid.UUID) ([]*models.TeacherClass, error) {
	return nil, nil
}
func (m *mockStore) UpdateClass(_ context.Context, _ uuid.UUID, _ store.UpdateClassParams) (*models.TeacherClass, error) {
	return nil, nil
}
func (m *mockStore) DeleteClass(_ context.Context, _ uuid.UUID) error { return nil }

// Roster stubs
func (m *mockStore) AddStudentToClass(_ context.Context, _, _ uuid.UUID) error { return nil }
func (m *mockStore) RemoveStudentFromClass(_ context.Context, _, _ uuid.UUID) error { return nil }
func (m *mockStore) ListClassRoster(_ context.Context, _ uuid.UUID) ([]*models.StudentSummary, error) {
	return nil, nil
}

// Progress stubs
func (m *mockStore) GetStudentProgress(_ context.Context, _ uuid.UUID) (*models.StudentProgress, error) {
	return nil, nil
}

// Material assignment stubs
func (m *mockStore) AssignMaterialToClass(_ context.Context, _ store.AssignMaterialParams) error {
	return nil
}
func (m *mockStore) UnassignMaterialFromClass(_ context.Context, _, _ uuid.UUID) error { return nil }
func (m *mockStore) ListClassMaterials(_ context.Context, _ uuid.UUID) ([]*models.ClassMaterialAssignment, error) {
	return nil, nil
}

// ============================================================
// MOCK CACHE
// ============================================================

type mockCache struct {
	mu       sync.Mutex
	attempts map[string]int64
	locks    map[string]string
}

func newMockCache() *mockCache {
	return &mockCache{
		attempts: map[string]int64{},
		locks:    map[string]string{},
	}
}

func (m *mockCache) GetLoginAttempts(_ context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.attempts[key], nil
}

func (m *mockCache) IncrLoginAttempts(_ context.Context, key string, _ time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attempts[key]++
	return m.attempts[key], nil
}

func (m *mockCache) AcquireRefreshLock(_ context.Context, key, value string, _ time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.locks[key]; exists {
		return false, nil
	}
	m.locks[key] = value
	return true, nil
}

func (m *mockCache) ReleaseRefreshLock(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.locks, key)
	return nil
}

func (m *mockCache) DeleteSession(_ context.Context, _ string) error    { return nil }
func (m *mockCache) DeleteUserKeys(_ context.Context, _ string) error  { return nil }
func (m *mockCache) IncrWithTTL(_ context.Context, key string, _ time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attempts[key]++
	return m.attempts[key], nil
}

// ============================================================
// HELPERS
// ============================================================

const testJWTSecret = "super-secret-jwt-key-at-least-32-chars!!"

func testConfig() *config.Config {
	trialEnd := time.Now().AddDate(0, 0, 14)
	_ = trialEnd
	return &config.Config{
		JWTSecret:             testJWTSecret,
		AccessTokenDuration:   15 * time.Minute,
		RefreshTokenDuration:  30 * 24 * time.Hour,
		TrialDays:             14,
		StripePriceMonthly:    "price_monthly",
		StripePriceQuarterly:  "price_quarterly",
		StripePriceSemesterly: "price_semesterly",
	}
}

// billingClientAdapter wraps mockBillingClient so it satisfies the interface AuthHandler expects.
// AuthHandler calls billing.Client.CreateCustomer — we stub this by embedding a real *billing.Client
// with empty keys (Stripe calls won't happen in unit tests since we override CreateCustomer).
// Simplest: pass nil and let AuthHandler handle nil billing gracefully (it already does: "Non-fatal: log and proceed").

func newTestAuthHandler(s store.Storer, c cache.Cacher) *handlers.AuthHandler {
	cfg := testConfig()
	jwtMgr := auth.NewJWTManager(cfg.JWTSecret, cfg.AccessTokenDuration, cfg.RefreshTokenDuration)
	// Pass nil billing — AuthHandler already treats billing errors as non-fatal on register
	return handlers.NewAuthHandler(s, c, jwtMgr, nil, cfg)
}

func postJSON(t *testing.T, handler http.HandlerFunc, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

func mustRegister(t *testing.T, h *handlers.AuthHandler, s *mockStore) (string, *http.Cookie) {
	t.Helper()
	hash, _ := bcrypt.GenerateFromPassword([]byte("Password123!"), bcrypt.MinCost)
	u, _ := s.CreateUser(context.Background(), store.CreateUserParams{
		Email:        "test@example.com",
		PasswordHash: string(hash),
		DisplayName:  "Test User",
		AvatarEmoji:  "🎓",
		AccountType:  models.AccountTypeStandard,
	})
	trialEnd := time.Now().AddDate(0, 0, 14)
	s.CreateSubscription(context.Background(), store.CreateSubscriptionParams{
		UserID:           u.ID,
		StripeCustomerID: "cus_test",
		Plan:             models.PlanTrial,
		Status:           models.StatusTrialing,
		TrialEnd:         &trialEnd,
	})

	w := postJSON(t, h.Login, "/auth/login", models.LoginRequest{
		Email:    "test@example.com",
		Password: "Password123!",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}
	var resp models.AuthResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)

	var refreshCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "refresh_token" {
			refreshCookie = c
		}
	}
	return resp.AccessToken, refreshCookie
}

// ============================================================
// TESTS: REGISTER
// ============================================================

func TestRegister_Success(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)

	w := postJSON(t, h.Register, "/auth/register", models.RegisterRequest{
		Email:       "new@example.com",
		Password:    "Password123!",
		DisplayName: "New User",
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp models.AuthResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.AccessToken == "" {
		t.Error("expected access token in response")
	}
	if resp.User.Email != "new@example.com" {
		t.Errorf("unexpected email: %s", resp.User.Email)
	}

	// Refresh token cookie must be set
	var found bool
	for _, cookie := range w.Result().Cookies() {
		if cookie.Name == "refresh_token" {
			found = true
			if !cookie.HttpOnly {
				t.Error("refresh_token cookie must be HttpOnly")
			}
		}
	}
	if !found {
		t.Error("refresh_token cookie not set")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)

	body := models.RegisterRequest{Email: "dup@example.com", Password: "Password123!", DisplayName: "A"}
	postJSON(t, h.Register, "/auth/register", body)
	w := postJSON(t, h.Register, "/auth/register", body)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 for duplicate, got %d", w.Code)
	}
}

func TestRegister_WeakPassword(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)

	w := postJSON(t, h.Register, "/auth/register", models.RegisterRequest{
		Email:       "a@b.com",
		Password:    "short",
		DisplayName: "A",
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for short password, got %d", w.Code)
	}
}

// ============================================================
// TESTS: K-12 AGE GATE
// ============================================================

func TestRegister_MinorRequiresGuardianEmail(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)

	dob := "2015-01-01" // 11 years old
	w := postJSON(t, h.Register, "/auth/register", models.RegisterRequest{
		Email:       "kid@school.edu",
		Password:    "Password123!",
		DisplayName: "Kid User",
		DateOfBirth: &dob,
		// GuardianEmail intentionally omitted
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when minor has no guardian_email, got %d", w.Code)
	}
}

func TestRegister_MinorSetsAccountType(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)

	dob := "2015-06-15"
	guardianEmail := "parent@example.com"
	w := postJSON(t, h.Register, "/auth/register", models.RegisterRequest{
		Email:         "minor@school.edu",
		Password:      "Password123!",
		DisplayName:   "Minor User",
		DateOfBirth:   &dob,
		GuardianEmail: &guardianEmail,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for minor with guardian, got %d: %s", w.Code, w.Body.String())
	}
	var resp models.AuthResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.User.AccountType != models.AccountTypeMinor {
		t.Errorf("expected account_type=minor, got %s", resp.User.AccountType)
	}
}

func TestRegister_AdultHasStandardAccountType(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)

	dob := "1995-01-01" // 30 years old
	w := postJSON(t, h.Register, "/auth/register", models.RegisterRequest{
		Email:       "adult@example.com",
		Password:    "Password123!",
		DisplayName: "Adult User",
		DateOfBirth: &dob,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp models.AuthResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.User.AccountType != models.AccountTypeStandard {
		t.Errorf("expected account_type=standard, got %s", resp.User.AccountType)
	}
}

// ============================================================
// TESTS: LOGIN
// ============================================================

func TestLogin_Success(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)

	mustRegister(t, h, s) // seeds store with hashed password, returns tokens we don't need here

	w := postJSON(t, h.Login, "/auth/login", models.LoginRequest{
		Email:    "test@example.com",
		Password: "Password123!",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp models.AuthResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.AccessToken == "" {
		t.Error("expected access token")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)

	mustRegister(t, h, s)

	w := postJSON(t, h.Login, "/auth/login", models.LoginRequest{
		Email:    "test@example.com",
		Password: "WrongPassword!",
	})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong password, got %d", w.Code)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)

	w := postJSON(t, h.Login, "/auth/login", models.LoginRequest{
		Email:    "ghost@example.com",
		Password: "Password123!",
	})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// ============================================================
// TESTS: REFRESH (token rotation)
// ============================================================

func TestRefresh_Success(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)
	_, cookie := mustRegister(t, h, s)

	// Use the cookie to refresh
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	h.Refresh(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp models.AuthResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp.AccessToken == "" {
		t.Error("expected new access token after refresh")
	}
}

func TestRefresh_TokenRotation_OldTokenRevoked(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)
	_, cookie := mustRegister(t, h, s)

	// First refresh — should succeed and revoke the old token
	req1 := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req1.AddCookie(cookie)
	w1 := httptest.NewRecorder()
	h.Refresh(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first refresh failed: %d", w1.Code)
	}

	// Second refresh with same old token — must fail (revoked)
	req2 := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req2.AddCookie(cookie)
	w2 := httptest.NewRecorder()
	h.Refresh(w2, req2)
	if w2.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 on reuse of revoked token, got %d", w2.Code)
	}
}

func TestRefresh_ConcurrentRace_OnlyOneWins(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)
	_, cookie := mustRegister(t, h, s)

	// Fire two concurrent refresh requests with the same token.
	// The distributed lock ensures exactly one succeeds (200) and the other
	// gets 409 Conflict (lock already held).
	results := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
			req.AddCookie(cookie)
			w := httptest.NewRecorder()
			h.Refresh(w, req)
			results <- w.Code
		}()
	}
	wg.Wait()
	close(results)

	codes := make([]int, 0, 2)
	for code := range results {
		codes = append(codes, code)
	}

	ok200 := 0
	for _, code := range codes {
		if code == http.StatusOK {
			ok200++
		}
	}
	// Exactly one refresh must succeed. The rejected request gets 409 (lock held)
	// or 401 (token already revoked by the winner) — both are valid.
	if ok200 != 1 {
		t.Errorf("expected exactly one 200 for concurrent refresh, got codes: %v", codes)
	}
}

func TestRefresh_MissingCookie(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	w := httptest.NewRecorder()
	h.Refresh(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing cookie, got %d", w.Code)
	}
}

// ============================================================
// TESTS: LOGOUT
// ============================================================

func TestLogout_ClearsCookie(t *testing.T) {
	s := newMockStore()
	c := newMockCache()
	h := newTestAuthHandler(s, c)
	_, cookie := mustRegister(t, h, s)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	h.Logout(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}

	var cleared bool
	for _, c := range w.Result().Cookies() {
		if c.Name == "refresh_token" && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("expected refresh_token cookie to be cleared (MaxAge < 0)")
	}
}

