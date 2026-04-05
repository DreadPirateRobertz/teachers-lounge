package store

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/teacherslounge/user-service/internal/models"
)

// ============================================================
// PARAM STRUCTS
// ============================================================

type CreateUserParams struct {
	Email        string
	PasswordHash string
	DisplayName  string
	AvatarEmoji  string
	AccountType  models.AccountType
	DateOfBirth  *time.Time
	GuardianEmail *string
}

type UpdateUserParams struct {
	DisplayName *string
	AvatarEmoji *string
}

type UpdateProfileParams struct {
	FelderSilvermanDials     *map[string]float64
	LearningStylePreferences *map[string]float64
	ExplanationPreferences   *map[string]string
}

type CreateTokenParams struct {
	UserID     uuid.UUID
	TokenHash  string
	DeviceInfo map[string]string
	ExpiresAt  time.Time
}

type CreateSubscriptionParams struct {
	UserID           uuid.UUID
	StripeCustomerID string
	Plan             models.SubscriptionPlan
	Status           models.SubscriptionStatus
	TrialEnd         *time.Time
}

type UpdateSubscriptionParams struct {
	StripeSubscriptionID    string
	NewStripeSubscriptionID *string
	Plan                    *models.SubscriptionPlan
	Status                  *models.SubscriptionStatus
	CurrentPeriodStart      *time.Time
	CurrentPeriodEnd        *time.Time
	TrialEnd                *time.Time
	CancelledAt             *time.Time
}

type AuditLogParams struct {
	AccessorID   *uuid.UUID
	StudentID    *uuid.UUID
	Action       string
	DataAccessed string
	Purpose      string
	IPAddress    string
}

// QueryAuditLogParams filters the audit_log query for the admin endpoint.
type QueryAuditLogParams struct {
	StudentID *uuid.UUID
	From      *time.Time
	To        *time.Time
	Limit     int // clamped to 500; default 100
	Offset    int
}

// ============================================================
// SCAN HELPERS
// ============================================================

type scanner interface {
	Scan(dest ...any) error
}

func scanUser(row scanner) (*models.User, error) {
	u := &models.User{}
	err := row.Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.AvatarEmoji,
		&u.AccountType, &u.IsAdmin, &u.DateOfBirth, &u.GuardianEmail, &u.GuardianConsentAt,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return u, nil
}

func scanAuthToken(row scanner) (*models.AuthToken, error) {
	t := &models.AuthToken{}
	var deviceInfoJSON []byte
	err := row.Scan(&t.ID, &t.UserID, &t.TokenHash, &deviceInfoJSON, &t.ExpiresAt, &t.RevokedAt, &t.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if deviceInfoJSON != nil {
		_ = json.Unmarshal(deviceInfoJSON, &t.DeviceInfo)
	}
	return t, nil
}

func scanSubscription(row scanner) (*models.Subscription, error) {
	s := &models.Subscription{}
	err := row.Scan(
		&s.ID, &s.UserID, &s.StripeCustomerID, &s.StripeSubscriptionID,
		&s.Plan, &s.Status, &s.CurrentPeriodStart, &s.CurrentPeriodEnd,
		&s.TrialEnd, &s.CancelledAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return s, nil
}

func scanLearningProfile(row scanner) (*models.LearningProfile, error) {
	p := &models.LearningProfile{}
	var (
		felderJSON     []byte
		styleJSON      []byte
		misconceptJSON []byte
		explanJSON     []byte
	)
	err := row.Scan(&p.UserID, &felderJSON, &styleJSON, &misconceptJSON, &explanJSON, &p.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	_ = json.Unmarshal(felderJSON, &p.FelderSilvermanDials)
	_ = json.Unmarshal(styleJSON, &p.LearningStylePreferences)
	_ = json.Unmarshal(misconceptJSON, &p.MisconceptionLog)
	_ = json.Unmarshal(explanJSON, &p.ExplanationPreferences)
	return p, nil
}

// ErrNotFound is returned when a query finds no rows.
var ErrNotFound = pgx.ErrNoRows

// ============================================================
// TEACHER PARAM STRUCTS
// ============================================================

type CreateTeacherProfileParams struct {
	UserID     uuid.UUID
	SchoolName string
	Bio        string
}

type CreateClassParams struct {
	TeacherID   uuid.UUID
	Name        string
	Subject     string
	Description string
}

type UpdateClassParams struct {
	Name        *string
	Subject     *string
	Description *string
}

type AssignMaterialParams struct {
	ClassID    uuid.UUID
	MaterialID uuid.UUID
	DueDate    *time.Time
}

// ============================================================
// TEACHER SCAN HELPERS
// ============================================================

func scanTeacherProfile(row scanner) (*models.TeacherProfile, error) {
	p := &models.TeacherProfile{}
	err := row.Scan(&p.UserID, &p.SchoolName, &p.Bio, &p.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

func scanTeacherClass(row scanner) (*models.TeacherClass, error) {
	c := &models.TeacherClass{}
	err := row.Scan(&c.ID, &c.TeacherID, &c.Name, &c.Subject, &c.Description, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return c, nil
}
