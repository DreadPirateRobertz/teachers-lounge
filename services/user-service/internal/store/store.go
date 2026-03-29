package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/teacherslounge/user-service/internal/models"

	"github.com/google/uuid"
)

// Store is the Postgres data layer for the User Service.
type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing database URL: %w", err)
	}
	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating pool: %w", err)
	}
	if err = pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

// SetCurrentUser sets the RLS session variable so row-level security policies apply.
// Call this before any query that touches student-data tables.
func (s *Store) SetCurrentUser(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxKeyUserID{}, userID)
}

type ctxKeyUserID struct{}

// execWithRLS runs fn with the RLS user ID set on the connection.
// Services use direct pool queries (BYPASSRLS role) for internal operations;
// this is for future use when exposing queries to user-scoped contexts.
func (s *Store) execWithRLS(ctx context.Context, userID uuid.UUID, fn func(ctx context.Context) error) error {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, "SET LOCAL app.current_user_id = $1", userID.String())
	if err != nil {
		return err
	}
	return fn(ctx)
}

// ============================================================
// USER QUERIES
// ============================================================

func (s *Store) CreateUser(ctx context.Context, p CreateUserParams) (*models.User, error) {
	const q = `
		INSERT INTO users (email, password_hash, display_name, avatar_emoji, account_type, date_of_birth, guardian_email)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, email, password_hash, display_name, avatar_emoji, account_type,
		          date_of_birth, guardian_email, guardian_consent_at, created_at, updated_at`

	row := s.pool.QueryRow(ctx, q,
		p.Email, p.PasswordHash, p.DisplayName, p.AvatarEmoji, p.AccountType,
		p.DateOfBirth, p.GuardianEmail,
	)
	return scanUser(row)
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	const q = `
		SELECT id, email, password_hash, display_name, avatar_emoji, account_type,
		       date_of_birth, guardian_email, guardian_consent_at, created_at, updated_at
		FROM users WHERE email = $1`
	row := s.pool.QueryRow(ctx, q, email)
	return scanUser(row)
}

func (s *Store) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	const q = `
		SELECT id, email, password_hash, display_name, avatar_emoji, account_type,
		       date_of_birth, guardian_email, guardian_consent_at, created_at, updated_at
		FROM users WHERE id = $1`
	row := s.pool.QueryRow(ctx, q, id)
	return scanUser(row)
}

func (s *Store) UpdateUser(ctx context.Context, id uuid.UUID, p UpdateUserParams) (*models.User, error) {
	const q = `
		UPDATE users
		SET display_name = COALESCE($2, display_name),
		    avatar_emoji  = COALESCE($3, avatar_emoji),
		    updated_at    = NOW()
		WHERE id = $1
		RETURNING id, email, password_hash, display_name, avatar_emoji, account_type,
		          date_of_birth, guardian_email, guardian_consent_at, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, id, p.DisplayName, p.AvatarEmoji)
	return scanUser(row)
}

// DeleteUser cascades via FK constraints: all related rows are deleted automatically.
func (s *Store) DeleteUser(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}

// InitLearningProfile creates an empty learning profile for a new user.
func (s *Store) InitLearningProfile(ctx context.Context, userID uuid.UUID) error {
	const q = `
		INSERT INTO learning_profiles (user_id) VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING`
	_, err := s.pool.Exec(ctx, q, userID)
	return err
}

// InitGamingProfile creates a default gaming profile for a new user.
func (s *Store) InitGamingProfile(ctx context.Context, userID uuid.UUID) error {
	const q = `
		INSERT INTO gaming_profiles (user_id) VALUES ($1)
		ON CONFLICT (user_id) DO NOTHING`
	_, err := s.pool.Exec(ctx, q, userID)
	return err
}

func (s *Store) GetLearningProfile(ctx context.Context, userID uuid.UUID) (*models.LearningProfile, error) {
	const q = `
		SELECT user_id, felder_silverman_dials, learning_style_preferences,
		       misconception_log, explanation_preferences, updated_at
		FROM learning_profiles WHERE user_id = $1`
	row := s.pool.QueryRow(ctx, q, userID)
	return scanLearningProfile(row)
}

func (s *Store) UpdateLearningProfile(ctx context.Context, userID uuid.UUID, p UpdateProfileParams) error {
	const q = `
		UPDATE learning_profiles SET
			felder_silverman_dials    = COALESCE($2, felder_silverman_dials),
			learning_style_preferences = COALESCE($3, learning_style_preferences),
			explanation_preferences   = COALESCE($4, explanation_preferences),
			updated_at = NOW()
		WHERE user_id = $1`
	_, err := s.pool.Exec(ctx, q, userID,
		p.FelderSilvermanDials, p.LearningStylePreferences, p.ExplanationPreferences)
	return err
}

// ============================================================
// REFRESH TOKEN QUERIES
// ============================================================

func (s *Store) CreateRefreshToken(ctx context.Context, p CreateTokenParams) error {
	const q = `
		INSERT INTO auth_tokens (user_id, token_hash, device_info, expires_at)
		VALUES ($1, $2, $3, $4)`
	_, err := s.pool.Exec(ctx, q, p.UserID, p.TokenHash, p.DeviceInfo, p.ExpiresAt)
	return err
}

func (s *Store) GetRefreshToken(ctx context.Context, tokenHash string) (*models.AuthToken, error) {
	const q = `
		SELECT id, user_id, token_hash, device_info, expires_at, revoked_at, created_at
		FROM auth_tokens
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()`
	row := s.pool.QueryRow(ctx, q, tokenHash)
	return scanAuthToken(row)
}

func (s *Store) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE auth_tokens SET revoked_at = NOW() WHERE token_hash = $1`, tokenHash)
	return err
}

// RevokeAllUserTokens revokes all refresh tokens for a user (used on password change / account deletion).
func (s *Store) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE auth_tokens SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

// ============================================================
// SUBSCRIPTION QUERIES
// ============================================================

func (s *Store) CreateSubscription(ctx context.Context, p CreateSubscriptionParams) (*models.Subscription, error) {
	const q = `
		INSERT INTO subscriptions (user_id, stripe_customer_id, plan, status, trial_end)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, stripe_customer_id, stripe_subscription_id, plan, status,
		          current_period_start, current_period_end, trial_end, cancelled_at, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, p.UserID, p.StripeCustomerID, p.Plan, p.Status, p.TrialEnd)
	return scanSubscription(row)
}

func (s *Store) GetSubscriptionByUserID(ctx context.Context, userID uuid.UUID) (*models.Subscription, error) {
	const q = `
		SELECT id, user_id, stripe_customer_id, stripe_subscription_id, plan, status,
		       current_period_start, current_period_end, trial_end, cancelled_at, created_at, updated_at
		FROM subscriptions WHERE user_id = $1`
	row := s.pool.QueryRow(ctx, q, userID)
	return scanSubscription(row)
}

func (s *Store) GetSubscriptionByStripeID(ctx context.Context, stripeSubID string) (*models.Subscription, error) {
	const q = `
		SELECT id, user_id, stripe_customer_id, stripe_subscription_id, plan, status,
		       current_period_start, current_period_end, trial_end, cancelled_at, created_at, updated_at
		FROM subscriptions WHERE stripe_subscription_id = $1`
	row := s.pool.QueryRow(ctx, q, stripeSubID)
	return scanSubscription(row)
}

func (s *Store) UpdateSubscription(ctx context.Context, p UpdateSubscriptionParams) error {
	const q = `
		UPDATE subscriptions SET
			stripe_subscription_id = COALESCE($2, stripe_subscription_id),
			plan                   = COALESCE($3, plan),
			status                 = COALESCE($4, status),
			current_period_start   = COALESCE($5, current_period_start),
			current_period_end     = COALESCE($6, current_period_end),
			trial_end              = COALESCE($7, trial_end),
			cancelled_at           = COALESCE($8, cancelled_at),
			updated_at             = NOW()
		WHERE stripe_subscription_id = $1`
	_, err := s.pool.Exec(ctx, q,
		p.StripeSubscriptionID, p.NewStripeSubscriptionID, p.Plan, p.Status,
		p.CurrentPeriodStart, p.CurrentPeriodEnd, p.TrialEnd, p.CancelledAt,
	)
	return err
}

// ============================================================
// AUDIT LOG
// ============================================================

func (s *Store) WriteAuditLog(ctx context.Context, p AuditLogParams) error {
	const q = `
		INSERT INTO audit_log (accessor_id, student_id, action, data_accessed, purpose, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := s.pool.Exec(ctx, q, p.AccessorID, p.StudentID, p.Action, p.DataAccessed, p.Purpose, p.IPAddress)
	return err
}

// ============================================================
// EXPORT JOBS
// ============================================================

func (s *Store) CreateExportJob(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx,
		`INSERT INTO export_jobs (user_id) VALUES ($1) RETURNING id`, userID,
	).Scan(&id)
	return id, err
}
