package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/auth"
	"github.com/teacherslounge/user-service/internal/billing"
	"github.com/teacherslounge/user-service/internal/cache"
	"github.com/teacherslounge/user-service/internal/config"
	"github.com/teacherslounge/user-service/internal/models"
	"github.com/teacherslounge/user-service/internal/store"
	"golang.org/x/crypto/bcrypt"

	"github.com/teacherslounge/user-service/internal/rediskeys"
)

// AuthHandler handles auth endpoints: register, login, token refresh, logout.
type AuthHandler struct {
	store      store.Storer
	cache      cache.Cacher
	jwt        *auth.JWTManager
	billing    *billing.Client
	cfg        *config.Config
}

// NewAuthHandler creates an AuthHandler wired to the given dependencies.
func NewAuthHandler(s store.Storer, c cache.Cacher, j *auth.JWTManager, b *billing.Client, cfg *config.Config) *AuthHandler {
	return &AuthHandler{store: s, cache: c, jwt: j, billing: b, cfg: cfg}
}

// POST /auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := validateRegisterRequest(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check login rate limit (by IP)
	ip := realIP(r)
	attempts, _ := h.cache.GetLoginAttempts(r.Context(), rediskeys.RateLimitLogin(ip))
	if attempts >= rediskeys.MaxLoginAttempts {
		writeError(w, http.StatusTooManyRequests, "too many attempts — try again later")
		return
	}

	// Check email not already registered
	existing, err := h.store.GetUserByEmail(r.Context(), req.Email)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing != nil {
		writeError(w, http.StatusConflict, "email already registered")
		return
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Determine account type from date of birth (K-12 hook)
	accountType := models.AccountTypeStandard
	var dob *time.Time
	if req.DateOfBirth != nil {
		t, err := time.Parse("2006-01-02", *req.DateOfBirth)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid date_of_birth format (use YYYY-MM-DD)")
			return
		}
		dob = &t
		age := time.Since(t).Hours() / (24 * 365.25)
		if age < 18 {
			accountType = models.AccountTypeMinor
			if req.GuardianEmail == nil {
				writeError(w, http.StatusBadRequest, "guardian_email required for users under 18")
				return
			}
		}
	}

	// Create user
	user, err := h.store.CreateUser(r.Context(), store.CreateUserParams{
		Email:         req.Email,
		PasswordHash:  string(hash),
		DisplayName:   req.DisplayName,
		AvatarEmoji:   "🎓",
		AccountType:   accountType,
		DateOfBirth:   dob,
		GuardianEmail: req.GuardianEmail,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	// Bootstrap learning + gaming profiles + consent records
	_ = h.store.InitLearningProfile(r.Context(), user.ID)
	_ = h.store.InitGamingProfile(r.Context(), user.ID)
	_ = h.store.InitConsent(r.Context(), user.ID, realIP(r), r.UserAgent())

	// Create Stripe customer + start trial (non-fatal — can be retried async)
	var stripeCustomerID string
	if h.billing != nil {
		stripeCustomerID, _ = h.billing.CreateCustomer(r.Context(), user, h.cfg.TrialDays)
	}

	trialEnd := time.Now().AddDate(0, 0, h.cfg.TrialDays)
	sub, err := h.store.CreateSubscription(r.Context(), store.CreateSubscriptionParams{
		UserID:           user.ID,
		StripeCustomerID: stripeCustomerID,
		Plan:             models.PlanTrial,
		Status:           models.StatusTrialing,
		TrialEnd:         &trialEnd,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create subscription")
		return
	}

	// Issue tokens
	accessToken, refreshRaw, err := h.issueTokenPair(r, user, sub)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue tokens")
		return
	}

	setRefreshCookie(w, refreshRaw, h.cfg.RefreshTokenDuration)
	writeJSON(w, http.StatusCreated, models.AuthResponse{
		AccessToken: accessToken,
		User:        toUserResponse(user, sub),
	})
}

// POST /auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Rate limit by IP
	ip := realIP(r)
	attempts, _ := h.cache.IncrLoginAttempts(r.Context(),
		rediskeys.RateLimitLogin(ip),
		time.Duration(rediskeys.TTLRateLimitLogin)*time.Second,
	)
	if attempts > rediskeys.MaxLoginAttempts {
		writeError(w, http.StatusTooManyRequests, "too many login attempts — try again in 15 minutes")
		return
	}

	user, err := h.store.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		// Return generic error to avoid user enumeration
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	sub, _ := h.store.GetSubscriptionByUserID(r.Context(), user.ID)

	accessToken, refreshRaw, err := h.issueTokenPair(r, user, sub)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue tokens")
		return
	}

	setRefreshCookie(w, refreshRaw, h.cfg.RefreshTokenDuration)
	writeJSON(w, http.StatusOK, models.AuthResponse{
		AccessToken: accessToken,
		User:        toUserResponse(user, sub),
	})
}

// POST /auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	rawToken, err := r.Cookie("refresh_token")
	if err != nil {
		writeError(w, http.StatusUnauthorized, "missing refresh token")
		return
	}

	tokenHash := auth.HashToken(rawToken.Value)

	// Distributed lock: prevent concurrent refresh races (token rotation)
	lockKey := rediskeys.SessionRefreshLock(tokenHash)
	lockVal := uuid.NewString()
	locked, _ := h.cache.AcquireRefreshLock(r.Context(), lockKey, lockVal,
		time.Duration(rediskeys.TTLRefreshLock)*time.Second)
	if !locked {
		writeError(w, http.StatusConflict, "concurrent refresh in progress — retry in a moment")
		return
	}
	defer func() { _ = h.cache.ReleaseRefreshLock(r.Context(), lockKey) }()

	token, err := h.store.GetRefreshToken(r.Context(), tokenHash)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	// Rotate: revoke old, issue new
	if err := h.store.RevokeRefreshToken(r.Context(), tokenHash); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	user, err := h.store.GetUserByID(r.Context(), token.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "user not found")
		return
	}

	sub, _ := h.store.GetSubscriptionByUserID(r.Context(), user.ID)

	accessToken, refreshRaw, err := h.issueTokenPair(r, user, sub)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue tokens")
		return
	}

	setRefreshCookie(w, refreshRaw, h.cfg.RefreshTokenDuration)
	writeJSON(w, http.StatusOK, models.AuthResponse{
		AccessToken: accessToken,
		User:        toUserResponse(user, sub),
	})
}

// POST /auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err == nil {
		tokenHash := auth.HashToken(cookie.Value)
		_ = h.store.RevokeRefreshToken(r.Context(), tokenHash)
	}
	// Clear the cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

// ============================================================
// HELPERS
// ============================================================

func (h *AuthHandler) issueTokenPair(r *http.Request, user *models.User, sub *models.Subscription) (accessToken, refreshRaw string, err error) {
	subStatus := ""
	if sub != nil {
		subStatus = string(sub.Status)
	}

	accessToken, err = h.jwt.IssueAccessToken(user, subStatus)
	if err != nil {
		return
	}

	var refreshHashed string
	refreshRaw, refreshHashed, err = auth.GenerateRefreshToken()
	if err != nil {
		return
	}

	err = h.store.CreateRefreshToken(r.Context(), store.CreateTokenParams{
		UserID:    user.ID,
		TokenHash: refreshHashed,
		DeviceInfo: map[string]string{
			"user_agent": r.UserAgent(),
			"ip":         realIP(r),
		},
		ExpiresAt: time.Now().Add(h.cfg.RefreshTokenDuration),
	})
	return
}

func setRefreshCookie(w http.ResponseWriter, token string, duration time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		MaxAge:   int(duration.Seconds()),
		HttpOnly: true,
		Secure:   true,
		Path:     "/auth/refresh",
		SameSite: http.SameSiteStrictMode,
	})
}

func validateRegisterRequest(req *models.RegisterRequest) error {
	if req.Email == "" {
		return errors.New("email is required")
	}
	if len(req.Password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	if req.DisplayName == "" {
		return errors.New("display_name is required")
	}
	return nil
}

func toUserResponse(user *models.User, sub *models.Subscription) *models.UserResponse {
	resp := &models.UserResponse{
		ID:                     user.ID,
		Email:                  user.Email,
		DisplayName:            user.DisplayName,
		AvatarEmoji:            user.AvatarEmoji,
		AccountType:            user.AccountType,
		HasCompletedOnboarding: user.HasCompletedOnboarding,
	}
	if sub != nil {
		ss := &models.SubscriptionSummary{
			Plan:     sub.Plan,
			Status:   sub.Status,
			TrialEnd: sub.TrialEnd,
		}
		resp.Subscription = ss
	}
	return resp
}

func realIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
