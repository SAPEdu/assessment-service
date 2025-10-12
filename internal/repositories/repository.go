package repositories

import "context"

// Repository interface tổng hợp tất cả các repository interfaces
type Repository interface {
	// Assessment domain
	Assessment() AssessmentRepository
	AssessmentSettings() AssessmentSettingsRepository

	// Question domain
	Question() QuestionRepository
	QuestionCategory() QuestionCategoryRepository
	QuestionAttachment() QuestionAttachmentRepository
	QuestionBank() QuestionBankRepository

	// Assessment-Question relationship
	AssessmentQuestion() AssessmentQuestionRepository

	// Attempt domain
	Attempt() AttemptRepository
	Answer() AnswerRepository

	// User domain (read-only for assessment service)
	User() UserRepository

	// Dashboard domain
	Dashboard() DashboardRepository

	// Transaction support
	WithTransaction(ctx context.Context, fn func(Repository) error) error

	// Health check
	Ping(ctx context.Context) error

	// Close connections
	Close() error
}

// RepositoryManager interface for managing repository lifecycle
type RepositoryManager interface {
	// Initialize repositories with database connections
	Initialize() error

	// Get repository instance
	GetRepository() Repository

	// Health check for all repositories
	HealthCheck(ctx context.Context) error

	// Graceful shutdown
	Shutdown(ctx context.Context) error
}

// TransactionRepository interface for transaction management
type TransactionRepository interface {
	// Begin transaction
	Begin(ctx context.Context) (Repository, error)

	// Commit transaction
	Commit(ctx context.Context) error

	// Rollback transaction
	Rollback(ctx context.Context) error
}
