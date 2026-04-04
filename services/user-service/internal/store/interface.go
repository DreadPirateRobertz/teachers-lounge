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

	// Teacher profiles
	CreateTeacherProfile(ctx context.Context, p CreateTeacherProfileParams) (*models.TeacherProfile, error)
	GetTeacherProfile(ctx context.Context, userID uuid.UUID) (*models.TeacherProfile, error)

	// Teacher classes
	CreateClass(ctx context.Context, p CreateClassParams) (*models.TeacherClass, error)
	GetClass(ctx context.Context, id uuid.UUID) (*models.TeacherClass, error)
	ListClasses(ctx context.Context, teacherID uuid.UUID) ([]*models.TeacherClass, error)
	UpdateClass(ctx context.Context, id uuid.UUID, p UpdateClassParams) (*models.TeacherClass, error)
	DeleteClass(ctx context.Context, id uuid.UUID) error

	// Class enrollment (roster)
	AddStudentToClass(ctx context.Context, classID, studentID uuid.UUID) error
	RemoveStudentFromClass(ctx context.Context, classID, studentID uuid.UUID) error
	ListClassRoster(ctx context.Context, classID uuid.UUID) ([]*models.StudentSummary, error)

	// Student progress (teacher-visible view)
	GetStudentProgress(ctx context.Context, studentID uuid.UUID) (*models.StudentProgress, error)

	// Class material assignments
	AssignMaterialToClass(ctx context.Context, p AssignMaterialParams) error
	UnassignMaterialFromClass(ctx context.Context, classID, materialID uuid.UUID) error
	ListClassMaterials(ctx context.Context, classID uuid.UUID) ([]*models.ClassMaterialAssignment, error)
}

// Ensure *Store satisfies Storer at compile time.
var _ Storer = (*Store)(nil)
