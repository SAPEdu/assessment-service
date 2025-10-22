package repositories

import (
	"context"

	"github.com/SAP-F-2025/assessment-service/internal/models"
)

// UserFilters defines filters for user queries
type UserFilters struct {
	Query  string // Search query for name or email
	Limit  int    // Page size
	Offset int    // Offset for pagination
}

// UserRepository interface for user operations (minimal for assessment service)
type UserRepository interface {
	// Basic read operations (assessment service is not owner of user data)
	GetByID(ctx context.Context, id string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByIDs(ctx context.Context, ids []string) ([]*models.User, error)

	// List and search operations
	List(ctx context.Context, filters UserFilters) ([]*models.User, int64, error)
	Search(ctx context.Context, query string, filters UserFilters) ([]*models.User, int64, error)

	// Validation and checks
	ExistsByID(ctx context.Context, id string) (bool, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	// IsActive(ctx context.Context, id string) (bool, error)
	HasRole(ctx context.Context, id string, role models.UserRole) (bool, error)
}
