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
	DateOfBirth  *time.Time  `json:"date_of_birth,omitempty"`
	// K-12 skeleton — not active at launch
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
