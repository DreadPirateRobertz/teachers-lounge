package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/models"
)

// Storer is the interface the auth and user handlers depend on.
// Implemented by *Store; also implemented by test doubles.
type Storer interface {
	CreateUser(ctx context.Context, p CreateUserParams) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	UpdateUser(ctx context.Context, id uuid.UUID, p UpdateUserParams) (*models.User, error)
	DeleteUser(ctx context.Context, id uuid.UUID) error

	InitLearningProfile(ctx context.Context, userID uuid.UUID) error
	InitGamingProfile(ctx context.Context, userID uuid.UUID) error
	GetLearningProfile(ctx context.Context, userID uuid.UUID) (*models.LearningProfile, error)
	UpdateLearningProfile(ctx context.Context, userID uuid.UUID, p UpdateProfileParams) error

	CreateRefreshToken(ctx context.Context, p CreateTokenParams) error
	GetRefreshToken(ctx context.Context, tokenHash string) (*models.AuthToken, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
	RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error

	CreateSubscription(ctx context.Context, p CreateSubscriptionParams) (*models.Subscription, error)
	GetSubscriptionByUserID(ctx context.Context, userID uuid.UUID) (*models.Subscription, error)
	UpdateSubscription(ctx context.Context, p UpdateSubscriptionParams) error

	WriteAuditLog(ctx context.Context, p AuditLogParams) error
	CreateExportJob(ctx context.Context, userID uuid.UUID) (uuid.UUID, error)
}

// Ensure *Store satisfies Storer at compile time.
var _ Storer = (*Store)(nil)
