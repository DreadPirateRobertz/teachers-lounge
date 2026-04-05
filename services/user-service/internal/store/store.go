package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/teacherslounge/user-service/internal/models"

	"github.com/google/uuid"
)

func marshalJSON(v any) ([]byte, error)   { return json.Marshal(v) }
func unmarshalJSON(b []byte, v any) error { return json.Unmarshal(b, v) }

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
		          is_admin, date_of_birth, guardian_email, guardian_consent_at, created_at, updated_at`

	row := s.pool.QueryRow(ctx, q,
		p.Email, p.PasswordHash, p.DisplayName, p.AvatarEmoji, p.AccountType,
		p.DateOfBirth, p.GuardianEmail,
	)
	return scanUser(row)
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	const q = `
		SELECT id, email, password_hash, display_name, avatar_emoji, account_type,
		       is_admin, date_of_birth, guardian_email, guardian_consent_at, created_at, updated_at
		FROM users WHERE email = $1`
	row := s.pool.QueryRow(ctx, q, email)
	return scanUser(row)
}

func (s *Store) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	const q = `
		SELECT id, email, password_hash, display_name, avatar_emoji, account_type,
		       is_admin, date_of_birth, guardian_email, guardian_consent_at, created_at, updated_at
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
		          is_admin, date_of_birth, guardian_email, guardian_consent_at, created_at, updated_at`
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

// UpdateSubscriptionByUserID updates the subscription row identified by user_id.
// Used for handler-initiated operations (cancel, reactivate) where the caller
// already holds the user ID from the auth context but the subscription may not
// yet have a stripe_subscription_id (e.g. trial plans).
func (s *Store) UpdateSubscriptionByUserID(ctx context.Context, userID uuid.UUID, p UpdateSubscriptionParams) error {
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
		WHERE user_id = $1`
	_, err := s.pool.Exec(ctx, q,
		userID, p.NewStripeSubscriptionID, p.Plan, p.Status,
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

// ============================================================
// ERASURE JOBS
// ============================================================

// CreateErasureJob records an async job to clean up external stores after user deletion.
// No FK to users — the user row is gone before this job is processed.
func (s *Store) CreateErasureJob(ctx context.Context, userID uuid.UUID, metadata map[string]any) (uuid.UUID, error) {
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return uuid.Nil, fmt.Errorf("marshalling erasure metadata: %w", err)
	}
	var id uuid.UUID
	err = s.pool.QueryRow(ctx,
		`INSERT INTO erasure_jobs (user_id, metadata) VALUES ($1, $2) RETURNING id`,
		userID, metaJSON,
	).Scan(&id)
	return id, err
}

// ============================================================
// AUDIT LOG QUERY (admin / FERPA endpoint)
// ============================================================

// QueryAuditLog returns audit log entries filtered by student_id and date range.
func (s *Store) QueryAuditLog(ctx context.Context, p AuditLogQueryParams) ([]*models.AuditLogEntry, error) {
	limit := p.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}

	const q = `
		SELECT id, timestamp, accessor_id, student_id, action, data_accessed, purpose,
		       COALESCE(ip_address::text, '')
		FROM audit_log
		WHERE ($1::uuid IS NULL OR student_id = $1)
		  AND ($2::timestamptz IS NULL OR timestamp >= $2)
		  AND ($3::timestamptz IS NULL OR timestamp <= $3)
		ORDER BY timestamp DESC
		LIMIT $4`

	rows, err := s.pool.Query(ctx, q, p.StudentID, p.From, p.To, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*models.AuditLogEntry
	for rows.Next() {
		e := &models.AuditLogEntry{}
		if err := rows.Scan(
			&e.ID, &e.Timestamp, &e.AccessorID, &e.StudentID,
			&e.Action, &e.DataAccessed, &e.Purpose, &e.IPAddress,
		); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ============================================================
// CONSENT MANAGEMENT
// ============================================================

// InitConsent creates default (granted=false) consent records for a new user.
func (s *Store) InitConsent(ctx context.Context, userID uuid.UUID, ip, userAgent string) error {
	const q = `
		INSERT INTO consent_records (user_id, consent_type, granted, ip_address, user_agent)
		VALUES
			($1, 'tutoring',  false, $2, $3),
			($1, 'analytics', false, $2, $3),
			($1, 'marketing', false, $2, $3)
		ON CONFLICT (user_id, consent_type) DO NOTHING`
	_, err := s.pool.Exec(ctx, q, userID, ip, userAgent)
	return err
}

// GetConsent retrieves all consent records for a user as a ConsentBundle.
func (s *Store) GetConsent(ctx context.Context, userID uuid.UUID) (*models.ConsentBundle, error) {
	const q = `
		SELECT id, user_id, consent_type, granted, granted_at,
		       COALESCE(ip_address::text, ''), COALESCE(user_agent, '')
		FROM consent_records WHERE user_id = $1`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bundle := &models.ConsentBundle{}
	for rows.Next() {
		r := &models.ConsentRecord{}
		var ct string
		if err := rows.Scan(
			&r.ID, &r.UserID, &ct, &r.Granted, &r.GrantedAt, &r.IPAddress, &r.UserAgent,
		); err != nil {
			return nil, err
		}
		r.ConsentType = models.ConsentType(ct)
		switch r.ConsentType {
		case models.ConsentTutoring:
			bundle.Tutoring = r
		case models.ConsentAnalytics:
			bundle.Analytics = r
		case models.ConsentMarketing:
			bundle.Marketing = r
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return bundle, nil
}

// UpdateConsent upserts consent records for the given user.
// Only the non-nil fields in p are written.
func (s *Store) UpdateConsent(ctx context.Context, userID uuid.UUID, p UpdateConsentParams) error {
	type update struct {
		ctype   string
		granted bool
	}
	var updates []update
	if p.Tutoring != nil {
		updates = append(updates, update{"tutoring", *p.Tutoring})
	}
	if p.Analytics != nil {
		updates = append(updates, update{"analytics", *p.Analytics})
	}
	if p.Marketing != nil {
		updates = append(updates, update{"marketing", *p.Marketing})
	}
	if len(updates) == 0 {
		return nil
	}

	const q = `
		INSERT INTO consent_records (user_id, consent_type, granted, granted_at, ip_address, user_agent)
		VALUES ($1, $2, $3, CASE WHEN $3 THEN NOW() ELSE NULL END, $4, $5)
		ON CONFLICT (user_id, consent_type) DO UPDATE SET
			granted    = EXCLUDED.granted,
			granted_at = CASE WHEN EXCLUDED.granted THEN NOW() ELSE NULL END,
			ip_address = EXCLUDED.ip_address,
			user_agent = EXCLUDED.user_agent`

	for _, u := range updates {
		if _, err := s.pool.Exec(ctx, q, userID, u.ctype, u.granted, p.IPAddress, p.UserAgent); err != nil {
			return err
		}
	}
	return nil
}

// ============================================================
// TEACHER PROFILES
// ============================================================

func (s *Store) CreateTeacherProfile(ctx context.Context, p CreateTeacherProfileParams) (*models.TeacherProfile, error) {
	const q = `
		INSERT INTO teacher_profiles (user_id, school_name, bio)
		VALUES ($1, $2, $3)
		RETURNING user_id, school_name, bio, created_at`
	row := s.pool.QueryRow(ctx, q, p.UserID, p.SchoolName, p.Bio)
	return scanTeacherProfile(row)
}

func (s *Store) GetTeacherProfile(ctx context.Context, userID uuid.UUID) (*models.TeacherProfile, error) {
	const q = `
		SELECT user_id, school_name, bio, created_at
		FROM teacher_profiles WHERE user_id = $1`
	row := s.pool.QueryRow(ctx, q, userID)
	return scanTeacherProfile(row)
}

// ============================================================
// TEACHER CLASSES
// ============================================================

func (s *Store) CreateClass(ctx context.Context, p CreateClassParams) (*models.TeacherClass, error) {
	const q = `
		INSERT INTO teacher_classes (teacher_id, name, subject, description)
		VALUES ($1, $2, $3, $4)
		RETURNING id, teacher_id, name, subject, description, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, p.TeacherID, p.Name, p.Subject, p.Description)
	return scanTeacherClass(row)
}

func (s *Store) GetClass(ctx context.Context, id uuid.UUID) (*models.TeacherClass, error) {
	const q = `
		SELECT id, teacher_id, name, subject, description, created_at, updated_at
		FROM teacher_classes WHERE id = $1`
	row := s.pool.QueryRow(ctx, q, id)
	return scanTeacherClass(row)
}

func (s *Store) ListClasses(ctx context.Context, teacherID uuid.UUID) ([]*models.TeacherClass, error) {
	const q = `
		SELECT id, teacher_id, name, subject, description, created_at, updated_at
		FROM teacher_classes WHERE teacher_id = $1
		ORDER BY created_at DESC`
	rows, err := s.pool.Query(ctx, q, teacherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var classes []*models.TeacherClass
	for rows.Next() {
		c, err := scanTeacherClass(rows)
		if err != nil {
			return nil, err
		}
		classes = append(classes, c)
	}
	return classes, rows.Err()
}

func (s *Store) UpdateClass(ctx context.Context, id uuid.UUID, p UpdateClassParams) (*models.TeacherClass, error) {
	const q = `
		UPDATE teacher_classes SET
			name        = COALESCE($2, name),
			subject     = COALESCE($3, subject),
			description = COALESCE($4, description),
			updated_at  = NOW()
		WHERE id = $1
		RETURNING id, teacher_id, name, subject, description, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, id, p.Name, p.Subject, p.Description)
	return scanTeacherClass(row)
}

func (s *Store) DeleteClass(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM teacher_classes WHERE id = $1`, id)
	return err
}

// ============================================================
// CLASS ENROLLMENT (ROSTER)
// ============================================================

func (s *Store) AddStudentToClass(ctx context.Context, classID, studentID uuid.UUID) error {
	const q = `
		INSERT INTO class_enrollments (class_id, student_id)
		VALUES ($1, $2)
		ON CONFLICT (class_id, student_id) DO NOTHING`
	_, err := s.pool.Exec(ctx, q, classID, studentID)
	return err
}

func (s *Store) RemoveStudentFromClass(ctx context.Context, classID, studentID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM class_enrollments WHERE class_id = $1 AND student_id = $2`,
		classID, studentID,
	)
	return err
}

func (s *Store) ListClassRoster(ctx context.Context, classID uuid.UUID) ([]*models.StudentSummary, error) {
	const q = `
		SELECT u.id, u.display_name, u.avatar_emoji, u.email, e.enrolled_at
		FROM class_enrollments e
		JOIN users u ON u.id = e.student_id
		WHERE e.class_id = $1
		ORDER BY e.enrolled_at ASC`
	rows, err := s.pool.Query(ctx, q, classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roster []*models.StudentSummary
	for rows.Next() {
		s := &models.StudentSummary{}
		if err := rows.Scan(&s.UserID, &s.DisplayName, &s.AvatarEmoji, &s.Email, &s.EnrolledAt); err != nil {
			return nil, err
		}
		roster = append(roster, s)
	}
	return roster, rows.Err()
}

// ============================================================
// STUDENT PROGRESS
// ============================================================

func (s *Store) GetStudentProgress(ctx context.Context, studentID uuid.UUID) (*models.StudentProgress, error) {
	// Student summary
	user, err := s.GetUserByID(ctx, studentID)
	if err != nil {
		return nil, err
	}
	progress := &models.StudentProgress{
		Student: &models.StudentSummary{
			UserID:      user.ID,
			DisplayName: user.DisplayName,
			AvatarEmoji: user.AvatarEmoji,
			Email:       user.Email,
		},
	}

	// Gaming profile
	var g models.GamingProfileSummary
	err = s.pool.QueryRow(ctx, `
		SELECT level, xp, current_streak, longest_streak, bosses_defeated
		FROM gaming_profiles WHERE user_id = $1`, studentID,
	).Scan(&g.Level, &g.XP, &g.CurrentStreak, &g.LongestStreak, &g.BossesDefeated)
	if err == nil {
		progress.Gaming = &g
	}

	// Learning profile
	learning, err := s.GetLearningProfile(ctx, studentID)
	if err == nil {
		progress.Learning = learning
	}

	// Quiz stats
	var qs models.QuizStats
	err = s.pool.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(SUM(CASE WHEN is_correct THEN 1 ELSE 0 END), 0)
		FROM quiz_results WHERE user_id = $1`, studentID,
	).Scan(&qs.TotalQuestions, &qs.CorrectAnswers)
	if err == nil && qs.TotalQuestions > 0 {
		qs.AccuracyPct = float64(qs.CorrectAnswers) / float64(qs.TotalQuestions) * 100
		progress.Quiz = &qs
	}

	return progress, nil
}

// ============================================================
// CLASS MATERIAL ASSIGNMENTS
// ============================================================

func (s *Store) AssignMaterialToClass(ctx context.Context, p AssignMaterialParams) error {
	const q = `
		INSERT INTO class_material_assignments (class_id, material_id, due_date)
		VALUES ($1, $2, $3)
		ON CONFLICT (class_id, material_id) DO UPDATE SET due_date = EXCLUDED.due_date`
	_, err := s.pool.Exec(ctx, q, p.ClassID, p.MaterialID, p.DueDate)
	return err
}

func (s *Store) UnassignMaterialFromClass(ctx context.Context, classID, materialID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM class_material_assignments WHERE class_id = $1 AND material_id = $2`,
		classID, materialID,
	)
	return err
}

func (s *Store) ListClassMaterials(ctx context.Context, classID uuid.UUID) ([]*models.ClassMaterialAssignment, error) {
	const q = `
		SELECT a.class_id, a.material_id, m.filename, a.assigned_at, a.due_date
		FROM class_material_assignments a
		JOIN materials m ON m.id = a.material_id
		WHERE a.class_id = $1
		ORDER BY a.assigned_at DESC`
	rows, err := s.pool.Query(ctx, q, classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assignments []*models.ClassMaterialAssignment
	for rows.Next() {
		a := &models.ClassMaterialAssignment{}
		if err := rows.Scan(&a.ClassID, &a.MaterialID, &a.Filename, &a.AssignedAt, &a.DueDate); err != nil {
			return nil, err
		}
		assignments = append(assignments, a)
	}
	return assignments, rows.Err()
}

// ============================================================
// AUDIT LOG — QUERY (admin)
// ============================================================

// QueryAuditLog returns audit entries filtered by the given params.
// Limit is clamped to 500; default is 100.
func (s *Store) QueryAuditLog(ctx context.Context, p QueryAuditLogParams) ([]*models.AuditEntry, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	const q = `
		SELECT id, timestamp, accessor_id, student_id, action, data_accessed, purpose, ip_address
		FROM audit_log
		WHERE ($1::uuid IS NULL OR student_id = $1)
		  AND ($2::timestamptz IS NULL OR timestamp >= $2)
		  AND ($3::timestamptz IS NULL OR timestamp <= $3)
		ORDER BY timestamp DESC
		LIMIT $4 OFFSET $5`

	rows, err := s.pool.Query(ctx, q, p.StudentID, p.From, p.To, limit, p.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*models.AuditEntry
	for rows.Next() {
		e := &models.AuditEntry{}
		var ip *string
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.AccessorID, &e.StudentID,
			&e.Action, &e.DataAccessed, &e.Purpose, &ip); err != nil {
			return nil, err
		}
		if ip != nil {
			e.IPAddress = *ip
		}
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []*models.AuditEntry{}
	}
	return entries, rows.Err()
}

// ============================================================
// EXPORT JOBS — GET + BUILD
// ============================================================

// GetExportJob fetches an export job by ID, scoped to the owning user.
func (s *Store) GetExportJob(ctx context.Context, jobID, userID uuid.UUID) (*models.ExportJob, error) {
	const q = `
		SELECT id, user_id, status, gcs_path, result_data, created_at, completed_at
		FROM export_jobs WHERE id = $1 AND user_id = $2`
	row := s.pool.QueryRow(ctx, q, jobID, userID)

	job := &models.ExportJob{}
	var resultJSON []byte
	err := row.Scan(&job.ID, &job.UserID, &job.Status, &job.GCSPath,
		&resultJSON, &job.CreatedAt, &job.CompletedAt)
	if err != nil {
		if err == ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if resultJSON != nil {
		job.ResultData = &models.UserExport{}
		if err := unmarshalJSON(resultJSON, job.ResultData); err != nil {
			return nil, fmt.Errorf("unmarshal result_data: %w", err)
		}
	}
	return job, nil
}

// BuildUserExport collects all PII for a user and stores it in the export_jobs row.
// It transitions the job from pending → processing → complete atomically.
func (s *Store) BuildUserExport(ctx context.Context, jobID, userID uuid.UUID) (*models.UserExport, error) {
	// Mark processing
	_, err := s.pool.Exec(ctx,
		`UPDATE export_jobs SET status = 'processing' WHERE id = $1 AND user_id = $2`,
		jobID, userID)
	if err != nil {
		return nil, fmt.Errorf("mark processing: %w", err)
	}

	export := &models.UserExport{ExportedAt: time.Now().UTC()}

	// User
	export.User, err = s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	// Learning profile
	export.LearningProfile, _ = s.GetLearningProfile(ctx, userID)

	// Subscription
	export.Subscription, _ = s.GetSubscriptionByUserID(ctx, userID)

	// Interactions (last 1000)
	export.Interactions, err = s.listInteractionsForExport(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get interactions: %w", err)
	}

	// Quiz results
	export.QuizResults, err = s.listQuizResultsForExport(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get quiz results: %w", err)
	}

	// Serialise and store
	resultBytes, err := marshalJSON(export)
	if err != nil {
		return nil, fmt.Errorf("marshal export: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE export_jobs
		SET status = 'complete', result_data = $3, completed_at = NOW()
		WHERE id = $1 AND user_id = $2`,
		jobID, userID, resultBytes)
	if err != nil {
		return nil, fmt.Errorf("store result: %w", err)
	}

	return export, nil
}

func (s *Store) listInteractionsForExport(ctx context.Context, userID uuid.UUID) ([]models.InteractionExport, error) {
	const q = `
		SELECT session_id, role, content, created_at
		FROM interactions WHERE user_id = $1
		ORDER BY created_at DESC LIMIT 1000`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.InteractionExport
	for rows.Next() {
		var e models.InteractionExport
		if err := rows.Scan(&e.SessionID, &e.Role, &e.Content, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if out == nil {
		out = []models.InteractionExport{}
	}
	return out, rows.Err()
}

func (s *Store) listQuizResultsForExport(ctx context.Context, userID uuid.UUID) ([]models.QuizResultExport, error) {
	const q = `
		SELECT question_id, correct, answer_given, xp_earned, answered_at
		FROM quiz_results WHERE user_id = $1
		ORDER BY answered_at DESC LIMIT 1000`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.QuizResultExport
	for rows.Next() {
		var e models.QuizResultExport
		if err := rows.Scan(&e.QuestionID, &e.Correct, &e.AnswerGiven, &e.XPEarned, &e.AnsweredAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if out == nil {
		out = []models.QuizResultExport{}
	}
	return out, rows.Err()
}

// ============================================================
// CONSENT MANAGEMENT
// ============================================================

// UpdateGuardianConsent records guardian consent for a minor user.
// guardianEmail must match the guardian_email stored on the account.
func (s *Store) UpdateGuardianConsent(ctx context.Context, userID uuid.UUID, guardianEmail string) error {
	result, err := s.pool.Exec(ctx, `
		UPDATE users
		SET guardian_consent_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND guardian_email = $2`,
		userID, guardianEmail)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
