package models

import (
	"time"

	"github.com/google/uuid"
)

// ============================================================
// USERS
// ============================================================

type AccountType string

const (
	AccountTypeStandard AccountType = "standard"
	AccountTypeMinor    AccountType = "minor"
)

type User struct {
	ID           uuid.UUID   `json:"id"`
	Email        string      `json:"email"`
	PasswordHash string      `json:"-"`
	DisplayName  string      `json:"display_name"`
	AvatarEmoji  string      `json:"avatar_emoji"`
	AccountType  AccountType `json:"account_type"`
	IsAdmin      bool        `json:"is_admin"`
	DateOfBirth  *time.Time  `json:"date_of_birth,omitempty"`
	// K-12 / FERPA: guardian consent for minors
	GuardianEmail     *string    `json:"guardian_email,omitempty"`
	GuardianConsentAt *time.Time `json:"guardian_consent_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// IsMinor returns true if the user's date of birth indicates they are under 18.
func (u *User) IsMinor() bool {
	if u.DateOfBirth == nil {
		return false
	}
	age := time.Since(*u.DateOfBirth).Hours() / (24 * 365.25)
	return age < 18
}

// ============================================================
// AUTH TOKENS (refresh tokens)
// ============================================================

type AuthToken struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	TokenHash  string
	DeviceInfo map[string]string
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

// ============================================================
// SUBSCRIPTIONS
// ============================================================

type SubscriptionPlan string

const (
	PlanTrial      SubscriptionPlan = "trial"
	PlanMonthly    SubscriptionPlan = "monthly"
	PlanQuarterly  SubscriptionPlan = "quarterly"
	PlanSemesterly SubscriptionPlan = "semesterly"
)

type SubscriptionStatus string

const (
	StatusTrialing SubscriptionStatus = "trialing"
	StatusActive   SubscriptionStatus = "active"
	StatusPastDue  SubscriptionStatus = "past_due"
	StatusCancelled SubscriptionStatus = "cancelled"
	StatusExpired  SubscriptionStatus = "expired"
)

type Subscription struct {
	ID                   uuid.UUID          `json:"id"`
	UserID               uuid.UUID          `json:"user_id"`
	StripeCustomerID     string             `json:"stripe_customer_id"`
	StripeSubscriptionID *string            `json:"stripe_subscription_id,omitempty"`
	Plan                 SubscriptionPlan   `json:"plan"`
	Status               SubscriptionStatus `json:"status"`
	CurrentPeriodStart   *time.Time         `json:"current_period_start,omitempty"`
	CurrentPeriodEnd     *time.Time         `json:"current_period_end,omitempty"`
	TrialEnd             *time.Time         `json:"trial_end,omitempty"`
	CancelledAt          *time.Time         `json:"cancelled_at,omitempty"`
	CreatedAt            time.Time          `json:"created_at"`
	UpdatedAt            time.Time          `json:"updated_at"`
}

func (s *Subscription) IsActive() bool {
	return s.Status == StatusTrialing || s.Status == StatusActive
}

// ============================================================
// LEARNING PROFILE
// ============================================================

type LearningProfile struct {
	UserID                   uuid.UUID          `json:"user_id"`
	FelderSilvermanDials     map[string]float64 `json:"felder_silverman_dials"`
	LearningStylePreferences map[string]float64 `json:"learning_style_preferences"`
	MisconceptionLog         []Misconception    `json:"misconception_log"`
	ExplanationPreferences   map[string]string  `json:"explanation_preferences"`
	UpdatedAt                time.Time          `json:"updated_at"`
}

type Misconception struct {
	Topic        string `json:"topic"`
	Misconception string `json:"misconception"`
	SeenCount    int    `json:"seen_count"`
}

// ============================================================
// REQUEST / RESPONSE DTOs
// ============================================================

type RegisterRequest struct {
	Email       string  `json:"email"`
	Password    string  `json:"password"`
	DisplayName string  `json:"display_name"`
	// K-12 hook: optional date of birth for age gate
	DateOfBirth *string `json:"date_of_birth,omitempty"` // "YYYY-MM-DD"
	// K-12 hook: optional guardian email (required if minor)
	GuardianEmail *string `json:"guardian_email,omitempty"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	AccessToken string        `json:"access_token"`
	User        *UserResponse `json:"user"`
	// Refresh token is set as HTTP-only cookie, not in body
}

type UserResponse struct {
	ID           uuid.UUID          `json:"id"`
	Email        string             `json:"email"`
	DisplayName  string             `json:"display_name"`
	AvatarEmoji  string             `json:"avatar_emoji"`
	AccountType  AccountType        `json:"account_type"`
	Subscription *SubscriptionSummary `json:"subscription,omitempty"`
}

type SubscriptionSummary struct {
	Plan   SubscriptionPlan   `json:"plan"`
	Status SubscriptionStatus `json:"status"`
	TrialEnd *time.Time       `json:"trial_end,omitempty"`
}

type UpdatePreferencesRequest struct {
	DisplayName            *string            `json:"display_name,omitempty"`
	AvatarEmoji            *string            `json:"avatar_emoji,omitempty"`
	LearningStylePrefs     map[string]float64 `json:"learning_style_preferences,omitempty"`
	FelderSilvermanDials   map[string]float64 `json:"felder_silverman_dials,omitempty"`
	ExplanationPreferences map[string]string  `json:"explanation_preferences,omitempty"`
}

// ============================================================
// TEACHER ACCOUNTS
// ============================================================

// TeacherProfile marks a user as a teacher and holds teacher metadata.
type TeacherProfile struct {
	UserID     uuid.UUID `json:"user_id"`
	SchoolName string    `json:"school_name"`
	Bio        string    `json:"bio"`
	CreatedAt  time.Time `json:"created_at"`
}

// TeacherClass represents a class managed by a teacher.
type TeacherClass struct {
	ID          uuid.UUID `json:"id"`
	TeacherID   uuid.UUID `json:"teacher_id"`
	Name        string    `json:"name"`
	Subject     string    `json:"subject"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// StudentSummary is the roster entry view of a student.
type StudentSummary struct {
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
	AvatarEmoji string    `json:"avatar_emoji"`
	Email       string    `json:"email"`
	EnrolledAt  time.Time `json:"enrolled_at"`
}

// GamingProfileSummary is a teacher-visible snapshot of a student's gaming progress.
type GamingProfileSummary struct {
	Level          int   `json:"level"`
	XP             int64 `json:"xp"`
	CurrentStreak  int   `json:"current_streak"`
	LongestStreak  int   `json:"longest_streak"`
	BossesDefeated int   `json:"bosses_defeated"`
}

// QuizStats aggregates quiz performance for a student.
type QuizStats struct {
	TotalQuestions int     `json:"total_questions"`
	CorrectAnswers int     `json:"correct_answers"`
	AccuracyPct    float64 `json:"accuracy_pct"`
}

// StudentProgress is the full progress view a teacher sees for a student.
type StudentProgress struct {
	Student  *StudentSummary       `json:"student"`
	Gaming   *GamingProfileSummary `json:"gaming_profile,omitempty"`
	Learning *LearningProfile      `json:"learning_profile,omitempty"`
	Quiz     *QuizStats            `json:"quiz_stats,omitempty"`
}

// ============================================================
// COMPLIANCE (FERPA / GDPR)
// ============================================================

// ConsentType enumerates the consent categories collected at registration.
type ConsentType string

const (
	ConsentTutoring  ConsentType = "tutoring"
	ConsentAnalytics ConsentType = "analytics"
	ConsentMarketing ConsentType = "marketing"
)

// ConsentRecord is one row of the consent_records table.
type ConsentRecord struct {
	ID          uuid.UUID   `json:"id"`
	UserID      uuid.UUID   `json:"user_id"`
	ConsentType ConsentType `json:"consent_type"`
	Granted     bool        `json:"granted"`
	GrantedAt   *time.Time  `json:"granted_at,omitempty"`
	IPAddress   string      `json:"ip_address,omitempty"`
	UserAgent   string      `json:"user_agent,omitempty"`
}

// ConsentBundle is the full consent state for a user (all three types).
type ConsentBundle struct {
	Tutoring  *ConsentRecord `json:"tutoring"`
	Analytics *ConsentRecord `json:"analytics"`
	Marketing *ConsentRecord `json:"marketing"`
}

// ConsentUpdateRequest carries optional updates for each consent type.
type ConsentUpdateRequest struct {
	Tutoring  *bool `json:"tutoring,omitempty"`
	Analytics *bool `json:"analytics,omitempty"`
	Marketing *bool `json:"marketing,omitempty"`
}

// AuditLogEntry is a row returned from the admin audit query.
type AuditLogEntry struct {
	ID           uuid.UUID  `json:"id"`
	Timestamp    time.Time  `json:"timestamp"`
	AccessorID   *uuid.UUID `json:"accessor_id,omitempty"`
	StudentID    *uuid.UUID `json:"student_id,omitempty"`
	Action       string     `json:"action"`
	DataAccessed string     `json:"data_accessed"`
	Purpose      string     `json:"purpose"`
	IPAddress    string     `json:"ip_address"`
}

// FERPA audit action constants used across user-service and tutoring-service.
const (
	AuditActionReadProfile      = "READ_PROFILE"
	AuditActionReadInteractions = "READ_INTERACTIONS"
	AuditActionReadQuizResults  = "READ_QUIZ_RESULTS"
	AuditActionUpdateProfile    = "UPDATE_PROFILE"
	AuditActionDeleteAccount    = "DELETE_ACCOUNT"
	AuditActionExportData       = "EXPORT_DATA"
	AuditActionAdminAccess      = "ADMIN_ACCESS"
)

// ============================================================
// CLASS MATERIAL ASSIGNMENTS
// ============================================================

// ClassMaterialAssignment is a material assigned to a class.
type ClassMaterialAssignment struct {
	ClassID    uuid.UUID  `json:"class_id"`
	MaterialID uuid.UUID  `json:"material_id"`
	Filename   string     `json:"filename"`
	AssignedAt time.Time  `json:"assigned_at"`
	DueDate    *time.Time `json:"due_date,omitempty"`
}

// ============================================================
// TEACHER REQUEST / RESPONSE DTOs
// ============================================================

type CreateTeacherProfileRequest struct {
	SchoolName string `json:"school_name"`
	Bio        string `json:"bio"`
}

type CreateClassRequest struct {
	Name        string `json:"name"`
	Subject     string `json:"subject"`
	Description string `json:"description"`
}

type UpdateClassRequest struct {
	Name        *string `json:"name,omitempty"`
	Subject     *string `json:"subject,omitempty"`
	Description *string `json:"description,omitempty"`
}

type AddStudentRequest struct {
	// StudentID or Email — at least one must be provided
	StudentID *string `json:"student_id,omitempty"`
	Email     *string `json:"email,omitempty"`
}

type AssignMaterialRequest struct {
	MaterialID string     `json:"material_id"`
	DueDate    *time.Time `json:"due_date,omitempty"`
}

// ============================================================
// FERPA AUDIT LOG
// ============================================================

// AuditEntry is a single record from the audit_log table.
type AuditEntry struct {
	ID           uuid.UUID  `json:"id"`
	Timestamp    time.Time  `json:"timestamp"`
	AccessorID   *uuid.UUID `json:"accessor_id,omitempty"`
	StudentID    *uuid.UUID `json:"student_id,omitempty"`
	Action       string     `json:"action"`
	DataAccessed string     `json:"data_accessed"`
	Purpose      string     `json:"purpose"`
	IPAddress    string     `json:"ip_address,omitempty"`
}

// Audit action constants used across user-service and tutoring-service.
const (
	AuditActionReadProfile      = "read_profile"
	AuditActionReadInteractions = "read_interactions"
	AuditActionReadSubscription = "read_subscription"
	AuditActionExportRequest    = "export_request"
	AuditActionExportView       = "export_view"
	AuditActionAccountDelete    = "account_delete"
	AuditActionTeacherView      = "teacher_progress_view"
	AuditActionQueryAuditLog    = "query_audit_log"
	AuditActionConsentGiven     = "guardian_consent_given"
)

// ============================================================
// GDPR DATA EXPORT
// ============================================================

// ExportJobStatus mirrors the export_status Postgres enum.
type ExportJobStatus string

const (
	ExportStatusPending    ExportJobStatus = "pending"
	ExportStatusProcessing ExportJobStatus = "processing"
	ExportStatusComplete   ExportJobStatus = "complete"
	ExportStatusFailed     ExportJobStatus = "failed"
)

// ExportJob is a row from the export_jobs table.
type ExportJob struct {
	ID          uuid.UUID       `json:"id"`
	UserID      uuid.UUID       `json:"user_id"`
	Status      ExportJobStatus `json:"status"`
	GCSPath     *string         `json:"gcs_path,omitempty"`
	ResultData  *UserExport     `json:"result_data,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}

// UserExport is the full GDPR data package for a user.
type UserExport struct {
	ExportedAt      time.Time              `json:"exported_at"`
	User            *User                  `json:"user"`
	LearningProfile *LearningProfile       `json:"learning_profile,omitempty"`
	Subscription    *Subscription          `json:"subscription,omitempty"`
	Interactions    []InteractionExport    `json:"interactions"`
	QuizResults     []QuizResultExport     `json:"quiz_results"`
}

// InteractionExport is a single chat message for the export payload.
type InteractionExport struct {
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// QuizResultExport is a single quiz answer for the export payload.
type QuizResultExport struct {
	QuestionID  uuid.UUID `json:"question_id"`
	Correct     bool      `json:"correct"`
	AnswerGiven string    `json:"answer_given"`
	XPEarned    int       `json:"xp_earned"`
	AnsweredAt  time.Time `json:"answered_at"`
}

// ============================================================
// CONSENT MANAGEMENT
// ============================================================

// ConsentStatus is the response payload for GET /users/{id}/consent.
type ConsentStatus struct {
	IsMinor           bool       `json:"is_minor"`
	GuardianEmail     *string    `json:"guardian_email,omitempty"`
	GuardianConsentAt *time.Time `json:"guardian_consent_at,omitempty"`
	ConsentRequired   bool       `json:"consent_required"`
	ConsentGiven      bool       `json:"consent_given"`
}

// UpdateConsentRequest is the body for PATCH /users/{id}/consent.
type UpdateConsentRequest struct {
	// GuardianEmail must match the guardian_email on the account.
	GuardianEmail string `json:"guardian_email"`
}
