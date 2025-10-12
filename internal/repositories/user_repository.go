package repositories

import (
	"context"

	"github.com/SAP-F-2025/assessment-service/internal/models"
)

// UserRepository interface for user operations (minimal for assessment service)
type UserRepository interface {
	// Basic read operations (assessment service is not owner of user data)
	GetByID(ctx context.Context, id string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByIDs(ctx context.Context, ids []string) ([]*models.User, error)

	// Validation and checks
	ExistsByID(ctx context.Context, id string) (bool, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	// IsActive(ctx context.Context, id string) (bool, error)
	HasRole(ctx context.Context, id string, role models.UserRole) (bool, error)
}
