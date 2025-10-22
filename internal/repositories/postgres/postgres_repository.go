package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/SAP-F-2025/assessment-service/internal/cache"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/SAP-F-2025/assessment-service/internal/repositories/casdoor"
)

// PostgreSQLRepository implements the main Repository interface
type PostgreSQLRepository struct {
	db           *gorm.DB
	redisClient  *redis.Client
	cacheManager *cache.CacheManager

	// Repository instances
	assessment         repositories.AssessmentRepository
	assessmentSettings repositories.AssessmentSettingsRepository
	question           repositories.QuestionRepository
	questionCategory   repositories.QuestionCategoryRepository
	questionAttachment repositories.QuestionAttachmentRepository
	questionBank       repositories.QuestionBankRepository
	assessmentQuestion repositories.AssessmentQuestionRepository
	attempt            repositories.AttemptRepository
	answer             repositories.AnswerRepository
	user               repositories.UserRepository
	dashboard          repositories.DashboardRepository
}

// RepositoryConfig holds configuration for repository initialization
type RepositoryConfig struct {
	DB            *gorm.DB
	RedisClient   *redis.Client
	CasdoorConfig casdoor.CasdoorConfig
}

// NewPostgreSQLRepository creates a new repository manager with all sub-repositories
func NewPostgreSQLRepository(config RepositoryConfig) repositories.Repository {
	cacheManager := cache.NewCacheManager(config.RedisClient)

	repo := &PostgreSQLRepository{
		db:           config.DB,
		redisClient:  config.RedisClient,
		cacheManager: cacheManager,
	}

	// Initialize sub-repositories with caching
	repo.assessment = NewAssessmentPostgreSQL(config.DB, config.RedisClient)
	repo.question = NewQuestionPostgreSQL(config.DB, config.RedisClient)
	repo.questionBank = NewQuestionBankRepository(config.DB)
	repo.assessmentQuestion = NewAssessmentQuestionPostgreSQL(config.DB, config.RedisClient)
	repo.attempt = NewAttemptPostgreSQL(config.DB, config.RedisClient)

	// User repository uses Casdoor
	repo.user = casdoor.NewUserCasdoor(config.CasdoorConfig, config.RedisClient)

	// Dashboard repository
	repo.dashboard = NewDashboardRepository(config.DB)

	// TODO: Initialize other repositories
	repo.assessmentSettings = NewAssessmentSettingsPostgreSQL(config.DB, cacheManager)
	// repo.questionCategory = NewQuestionCategoryPostgreSQL(config.DB, config.RedisClient)
	// repo.questionAttachment = NewQuestionAttachmentPostgreSQL(config.DB, config.RedisClient)
	repo.answer = NewAnswerPostgreSQL(config.DB, config.RedisClient)

	return repo
}

// Assessment returns the assessment repository
func (r *PostgreSQLRepository) Assessment() repositories.AssessmentRepository {
	return r.assessment
}

// AssessmentSettings returns the assessment settings repository
func (r *PostgreSQLRepository) AssessmentSettings() repositories.AssessmentSettingsRepository {
	return r.assessmentSettings
}

// Question returns the question repository
func (r *PostgreSQLRepository) Question() repositories.QuestionRepository {
	return r.question
}

// QuestionCategory returns the question category repository
func (r *PostgreSQLRepository) QuestionCategory() repositories.QuestionCategoryRepository {
	return r.questionCategory
}

// QuestionAttachment returns the question attachment repository
func (r *PostgreSQLRepository) QuestionAttachment() repositories.QuestionAttachmentRepository {
	return r.questionAttachment
}

// QuestionBank returns the question bank repository
func (r *PostgreSQLRepository) QuestionBank() repositories.QuestionBankRepository {
	return r.questionBank
}

// AssessmentQuestion returns the assessment-question repository
func (r *PostgreSQLRepository) AssessmentQuestion() repositories.AssessmentQuestionRepository {
	return r.assessmentQuestion
}

// Attempt returns the attempt repository
func (r *PostgreSQLRepository) Attempt() repositories.AttemptRepository {
	return r.attempt
}

// Answer returns the answer repository
func (r *PostgreSQLRepository) Answer() repositories.AnswerRepository {
	return r.answer
}

// User returns the user repository
func (r *PostgreSQLRepository) User() repositories.UserRepository {
	return r.user
}

// Dashboard returns the dashboard repository
func (r *PostgreSQLRepository) Dashboard() repositories.DashboardRepository {
	return r.dashboard
}

// WithTransaction executes a function within a database transaction
func (r *PostgreSQLRepository) WithTransaction(ctx context.Context, fn func(repositories.Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create a new repository instance with the transaction
		txRepo := &PostgreSQLRepository{
			db:           tx,
			redisClient:  r.redisClient,
			cacheManager: r.cacheManager,
		}

		// Initialize sub-repositories with transaction
		txRepo.assessment = NewAssessmentPostgreSQL(tx, r.redisClient)
		txRepo.question = NewQuestionPostgreSQL(tx, r.redisClient)
		txRepo.questionBank = NewQuestionBankRepository(tx)
		txRepo.assessmentQuestion = NewAssessmentQuestionPostgreSQL(tx, r.redisClient)
		txRepo.attempt = NewAttemptPostgreSQL(tx, r.redisClient)

		// User repository doesn't need transaction (it's external)
		txRepo.user = r.user

		// Dashboard repository with transaction
		txRepo.dashboard = NewDashboardRepository(tx)

		return fn(txRepo)
	})
}

// Ping checks the health of database and cache connections
func (r *PostgreSQLRepository) Ping(ctx context.Context) error {
	// Check database connection
	sqlDB, err := r.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Check cache connection
	if r.redisClient != nil {
		if err := r.cacheManager.HealthCheck(ctx); err != nil {
			return fmt.Errorf("cache ping failed: %w", err)
		}
	}

	return nil
}

// Close closes all connections
func (r *PostgreSQLRepository) Close() error {
	// Close database connection
	sqlDB, err := r.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}

	// Close Redis connection
	if r.redisClient != nil {
		if err := r.redisClient.Close(); err != nil {
			return fmt.Errorf("failed to close Redis: %w", err)
		}
	}

	return nil
}

// RepositoryManager implements the RepositoryManager interface
type RepositoryManager struct {
	config RepositoryConfig
	repo   repositories.Repository
}

// NewRepositoryManager creates a new repository manager
func NewRepositoryManager(config RepositoryConfig) repositories.RepositoryManager {
	return &RepositoryManager{
		config: config,
	}
}

// Initialize initializes all repositories and connections
func (rm *RepositoryManager) Initialize() error {
	// Validate configuration
	if rm.config.DB == nil {
		return fmt.Errorf("database connection is required")
	}

	// Test database connection
	sqlDB, err := rm.config.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}

	// Test Redis connection if provided
	if rm.config.RedisClient != nil {
		if _, err := rm.config.RedisClient.Ping(ctx).Result(); err != nil {
			return fmt.Errorf("Redis connection failed: %w", err)
		}
	}

	// Initialize repository
	rm.repo = NewPostgreSQLRepository(rm.config)

	return nil
}

// GetRepository returns the repository instance
func (rm *RepositoryManager) GetRepository() repositories.Repository {
	return rm.repo
}

// HealthCheck checks the health of all repository connections
func (rm *RepositoryManager) HealthCheck(ctx context.Context) error {
	if rm.repo == nil {
		return fmt.Errorf("repository not initialized")
	}

	return rm.repo.Ping(ctx)
}

// Shutdown gracefully shuts down all repository connections
func (rm *RepositoryManager) Shutdown(ctx context.Context) error {
	if rm.repo == nil {
		return nil
	}

	return rm.repo.Close()
}

// CacheStats returns cache statistics for monitoring
func (r *PostgreSQLRepository) CacheStats(ctx context.Context) (map[string]interface{}, error) {
	if r.redisClient == nil {
		return map[string]interface{}{
			"cache_enabled": false,
		}, nil
	}

	stats := make(map[string]interface{})
	stats["cache_enabled"] = true

	// Get Redis info
	info, err := r.redisClient.Info(ctx, "memory", "stats").Result()
	if err != nil {
		return stats, fmt.Errorf("failed to get cache info: %w", err)
	}

	stats["redis_info"] = info

	// Get key counts by prefix
	prefixes := []string{"assessment:", "question:", "user:", "stats:", "exists:", "fast:"}
	for _, prefix := range prefixes {
		keys, err := r.redisClient.Keys(ctx, prefix+"*").Result()
		if err == nil {
			stats[prefix+"count"] = len(keys)
		}
	}

	return stats, nil
}

// WarmupCache preloads frequently accessed data into cache
func (r *PostgreSQLRepository) WarmupCache(ctx context.Context) error {
	if r.cacheManager == nil {
		return nil
	}

	return r.cacheManager.WarmupCache(ctx)
}

// ClearCache clears all cache data (use with caution)
func (r *PostgreSQLRepository) ClearCache(ctx context.Context) error {
	if r.cacheManager == nil {
		return nil
	}

	return r.cacheManager.ClearAll(ctx)
}
