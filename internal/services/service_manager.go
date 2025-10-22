package services

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/SAP-F-2025/assessment-service/internal/validator"
	"gorm.io/gorm"
)

// ServiceManagerConfig holds configuration for the service manager
type ServiceManagerConfig struct {
	// Logging configuration
	EnableDebugLogging bool
	EnableMetrics      bool
	LogLevel           slog.Level

	// Service-specific configurations
	Assessment   ServiceConfig
	Question     ServiceConfig
	QuestionBank ServiceConfig
	Attempt      ServiceConfig
	Grading      ServiceConfig

	// Global settings
	DefaultTimeout    time.Duration
	MaxRetries        int
	CircuitBreaker    bool
	RateLimitingRules map[string]RateLimit
}

type ServiceConfig struct {
	Enabled         bool
	CacheEnabled    bool
	CacheTTL        time.Duration
	ValidationLevel ValidationLevel
	AuditingEnabled bool
	MetricsEnabled  bool
}

type ValidationLevel int

const (
	ValidationBasic ValidationLevel = iota
	ValidationStrict
	ValidationFull
)

type RateLimit struct {
	RequestsPerMinute int
	BurstSize         int
}

// serviceManager implements ServiceManager interface
type serviceManager struct {
	// Dependencies
	db        *gorm.DB
	repo      repositories.Repository
	logger    *slog.Logger
	validator *validator.Validator
	config    ServiceManagerConfig

	// Service instances
	assessmentService   AssessmentService
	questionService     QuestionService
	questionBankService QuestionBankService
	attemptService      AttemptService
	gradingService      GradingService
	dashboardService    DashboardService
	studentService      StudentService
	importExportService ImportExportService
	// notificationService NotificationService
	//analyticsService    AnalyticsService

	// Utilities
	//validationService *ValidationService

	// Lifecycle management
	initialized bool
	shutdown    bool
	mu          sync.RWMutex
}

// NewServiceManager creates a new service manager with all dependencies
func NewServiceManager(db *gorm.DB, repo repositories.Repository, logger *slog.Logger, validator *validator.Validator, config ServiceManagerConfig) ServiceManager {
	return &serviceManager{
		db:        db,
		repo:      repo,
		logger:    logger,
		validator: validator,
		config:    config,
	}
}

// NewDefaultServiceManager creates a service manager with default configuration
func NewDefaultServiceManager(db *gorm.DB, repo repositories.Repository, logger *slog.Logger, validator *validator.Validator) ServiceManager {
	config := ServiceManagerConfig{
		EnableDebugLogging: false,
		EnableMetrics:      true,
		LogLevel:           slog.LevelInfo,

		Assessment: ServiceConfig{
			Enabled:         true,
			CacheEnabled:    false,
			CacheTTL:        5 * time.Minute,
			ValidationLevel: ValidationStrict,
			AuditingEnabled: true,
			MetricsEnabled:  true,
		},
		Question: ServiceConfig{
			Enabled:         true,
			CacheEnabled:    true,
			CacheTTL:        10 * time.Minute,
			ValidationLevel: ValidationFull,
			AuditingEnabled: true,
			MetricsEnabled:  true,
		},
		Attempt: ServiceConfig{
			Enabled:         true,
			CacheEnabled:    false,
			CacheTTL:        1 * time.Minute,
			ValidationLevel: ValidationStrict,
			AuditingEnabled: true,
			MetricsEnabled:  true,
		},
		Grading: ServiceConfig{
			Enabled:         true,
			CacheEnabled:    false,
			CacheTTL:        0,
			ValidationLevel: ValidationBasic,
			AuditingEnabled: true,
			MetricsEnabled:  true,
		},
		QuestionBank: ServiceConfig{
			Enabled:         true,
			CacheEnabled:    false,
			CacheTTL:        10 * time.Minute,
			ValidationLevel: ValidationFull,
			AuditingEnabled: true,
			MetricsEnabled:  true,
		},

		DefaultTimeout:    30 * time.Second,
		MaxRetries:        3,
		CircuitBreaker:    true,
		RateLimitingRules: make(map[string]RateLimit),
	}

	return NewServiceManager(db, repo, logger, validator, config)
}

// Initialize sets up all services and their dependencies
func (sm *serviceManager) Initialize(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.initialized {
		return nil
	}

	sm.logger.Info("Initializing service manager")

	// Initialize individual services
	if err := sm.initializeServices(ctx); err != nil {
		return fmt.Errorf("failed to initialize services: %w", err)
	}

	// Validate all services are healthy
	if err := sm.validateServicesHealth(ctx); err != nil {
		return fmt.Errorf("service health check failed: %w", err)
	}

	sm.initialized = true
	sm.logger.Info("Service manager initialized successfully")

	return nil
}

func (sm *serviceManager) initializeServices(ctx context.Context) error {
	var initErrors []error

	// Initialize AssessmentService
	if sm.config.Assessment.Enabled {
		sm.assessmentService = NewAssessmentService(sm.repo, sm.db, sm.logger, sm.validator)
		sm.logger.Info("Assessment service initialized")
	}

	// Initialize QuestionService
	if sm.config.Question.Enabled {
		sm.questionService = NewQuestionService(sm.repo, sm.db, sm.logger, sm.validator)
		sm.logger.Info("Question service initialized")
	}

	// Initialize QuestionBankService
	if sm.config.QuestionBank.Enabled {
		sm.questionBankService = NewQuestionBankService(sm.repo, sm.db, sm.logger, sm.validator)
		sm.logger.Info("QuestionBank service initialized")
	}

	// Initialize AttemptService
	if sm.config.Attempt.Enabled {
		sm.attemptService = NewAttemptService(sm.repo, sm.db, sm.logger, sm.validator)
		sm.logger.Info("Attempt service initialized")
	}

	// Initialize GradingService
	if sm.config.Grading.Enabled {
		sm.gradingService = NewGradingService(sm.db, sm.repo, sm.logger, sm.validator)
		sm.logger.Info("Grading service initialized")
	}

	// Initialize DashboardService
	sm.dashboardService = NewDashboardService(sm.repo, sm.db, sm.logger)
	sm.logger.Info("Dashboard service initialized")

	// Initialize StudentService
	sm.studentService = NewStudentService(sm.repo, sm.db, sm.logger)
	sm.logger.Info("Student service initialized")

	// Initialize ImportExportService
	sm.importExportService = NewImportExportService(sm.repo, sm.logger, sm.validator)
	sm.logger.Info("ImportExport service initialized")

	// Initialize NotificationService
	//sm.notificationService = NewNotificationService(sm.repo, sm.logger, sm.validator)
	// sm.logger.Info("Notification service initialized")

	if len(initErrors) > 0 {
		return fmt.Errorf("service initialization failed with %d errors", len(initErrors))
	}

	return nil
}

func (sm *serviceManager) validateServicesHealth(ctx context.Context) error {
	// Perform basic health checks on all initialized services
	// This could include checking database connections, cache availability, etc.

	// For now, just check if repository is healthy
	//if err := sm.repo.(repositories.RepositoryManager).HealthCheck(ctx); err != nil {
	//	return fmt.Errorf("repository health check failed: %w", err)
	//}

	return nil
}

// Service getters
func (sm *serviceManager) Assessment() AssessmentService {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if !sm.initialized {
		panic("service manager not initialized")
	}

	if sm.config.Assessment.Enabled && sm.assessmentService != nil {
		return sm.assessmentService
	}

	panic("assessment service not enabled or not initialized")
}

func (sm *serviceManager) Question() QuestionService {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if !sm.initialized {
		panic("service manager not initialized")
	}

	if sm.config.Question.Enabled && sm.questionService != nil {
		return sm.questionService
	}

	panic("question service not enabled or not initialized")
}

func (sm *serviceManager) QuestionBank() QuestionBankService {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if !sm.initialized {
		panic("service manager not initialized")
	}

	if sm.config.QuestionBank.Enabled && sm.questionBankService != nil {
		return sm.questionBankService
	}

	panic("question bank service not enabled or not initialized")
}

func (sm *serviceManager) Attempt() AttemptService {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if !sm.initialized {
		panic("service manager not initialized")
	}

	if sm.config.Attempt.Enabled && sm.attemptService != nil {
		return sm.attemptService
	}

	panic("attempt service not enabled or not initialized")
}

func (sm *serviceManager) Grading() GradingService {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if !sm.initialized {
		panic("service manager not initialized")
	}

	if sm.config.Grading.Enabled && sm.gradingService != nil {
		return sm.gradingService
	}

	panic("grading service not enabled or not initialized")
}

func (sm *serviceManager) Dashboard() DashboardService {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if !sm.initialized {
		panic("service manager not initialized")
	}

	if sm.dashboardService != nil {
		return sm.dashboardService
	}

	panic("dashboard service not initialized")
}

func (sm *serviceManager) Student() StudentService {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if !sm.initialized {
		panic("service manager not initialized")
	}

	if sm.studentService != nil {
		return sm.studentService
	}

	panic("student service not initialized")
}

func (sm *serviceManager) ImportExport() ImportExportService {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if !sm.initialized {
		panic("service manager not initialized")
	}

	if sm.importExportService != nil {
		return sm.importExportService
	}

	panic("import/export service not initialized")
}

//func (sm *serviceManager) Notification() NotificationService {
//	sm.mu.RLock()
//	defer sm.mu.RUnlock()
//
//	if !sm.initialized {
//		panic("service manager not initialized")
//	}
//
//	if sm.notificationService != nil {
//		return sm.notificationService
//	}
//
//	panic("notification service not initialized")
//}

// Health and lifecycle
func (sm *serviceManager) HealthCheck(ctx context.Context) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if !sm.initialized {
		return fmt.Errorf("service manager not initialized")
	}

	if sm.shutdown {
		return fmt.Errorf("service manager is shut down")
	}

	// Check repository health
	if err := sm.repo.(repositories.RepositoryManager).HealthCheck(ctx); err != nil {
		return fmt.Errorf("repository health check failed: %w", err)
	}

	// Additional health checks could be added here
	// - Check cache connectivity
	// - Check external service dependencies
	// - Verify service configuration

	return nil
}

func (sm *serviceManager) Shutdown(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.shutdown {
		return nil
	}

	sm.logger.Info("Shutting down service manager")

	// Graceful shutdown of services
	// Services don't currently have explicit shutdown methods,
	// but this is where we would call them

	// Shutdown repository manager
	if repoManager, ok := sm.repo.(repositories.RepositoryManager); ok {
		if err := repoManager.Shutdown(ctx); err != nil {
			sm.logger.Error("Failed to shutdown repository manager", "error", err)
		}
	}

	sm.shutdown = true
	sm.logger.Info("Service manager shut down completed")

	return nil
}

// ===== UTILITY METHODS =====

// GetConfig returns the service manager configuration
func (sm *serviceManager) GetConfig() ServiceManagerConfig {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.config
}

// IsInitialized returns whether the service manager has been initialized
func (sm *serviceManager) IsInitialized() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.initialized
}

// IsShutdown returns whether the service manager has been shut down
func (sm *serviceManager) IsShutdown() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.shutdown
}

// ===== METRICS AND MONITORING =====

// GetServiceMetrics returns metrics for all services
func (sm *serviceManager) GetServiceMetrics(ctx context.Context) (map[string]interface{}, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if !sm.initialized {
		return nil, fmt.Errorf("service manager not initialized")
	}

	metrics := map[string]interface{}{
		"service_manager": map[string]interface{}{
			"initialized": sm.initialized,
			"shutdown":    sm.shutdown,
			"config":      sm.config,
		},
	}

	// Add service-specific metrics
	if sm.config.Assessment.MetricsEnabled {
		// Assessment service metrics would be collected here
		metrics["assessment_service"] = map[string]interface{}{
			"enabled": sm.config.Assessment.Enabled,
			"status":  "healthy",
		}
	}

	if sm.config.Question.MetricsEnabled {
		// Question service metrics would be collected here
		metrics["question_service"] = map[string]interface{}{
			"enabled": sm.config.Question.Enabled,
			"status":  "healthy",
		}
	}

	if sm.config.Attempt.MetricsEnabled {
		// Attempt service metrics would be collected here
		metrics["attempt_service"] = map[string]interface{}{
			"enabled": sm.config.Attempt.Enabled,
			"status":  "healthy",
		}
	}

	if sm.config.Grading.MetricsEnabled {
		// Grading service metrics would be collected here
		metrics["grading_service"] = map[string]interface{}{
			"enabled": sm.config.Grading.Enabled,
			"status":  "healthy",
		}
	}

	return metrics, nil
}

// ===== HELPER FUNCTIONS =====

// WithTimeout creates a context with the default timeout
func (sm *serviceManager) WithTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, sm.config.DefaultTimeout)
}

// WithDeadline creates a context with a specific deadline
func (sm *serviceManager) WithDeadline(parent context.Context, deadline time.Time) (context.Context, context.CancelFunc) {
	return context.WithDeadline(parent, deadline)
}

// ===== CONFIGURATION VALIDATION =====

// ValidateConfig validates the service manager configuration
func (config *ServiceManagerConfig) Validate() error {
	var errors []string

	// Validate timeouts
	if config.DefaultTimeout <= 0 {
		errors = append(errors, "default timeout must be positive")
	}

	if config.MaxRetries < 0 {
		errors = append(errors, "max retries cannot be negative")
	}

	// Validate service configurations
	if err := config.Assessment.validate("assessment"); err != nil {
		errors = append(errors, err.Error())
	}

	if err := config.Question.validate("question"); err != nil {
		errors = append(errors, err.Error())
	}

	if err := config.Attempt.validate("attempt"); err != nil {
		errors = append(errors, err.Error())
	}

	if err := config.Grading.validate("grading"); err != nil {
		errors = append(errors, err.Error())
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed: %v", errors)
	}

	return nil
}

func (sc *ServiceConfig) validate(serviceName string) error {
	var errors []string

	if sc.CacheTTL < 0 {
		errors = append(errors, fmt.Sprintf("%s: cache TTL cannot be negative", serviceName))
	}

	if sc.ValidationLevel < ValidationBasic || sc.ValidationLevel > ValidationFull {
		errors = append(errors, fmt.Sprintf("%s: invalid validation level", serviceName))
	}

	if len(errors) > 0 {
		return fmt.Errorf("%s", errors[0])
	}

	return nil
}

// ===== FACTORY FUNCTIONS =====

// CreateProductionServiceManager creates a service manager configured for production
func CreateProductionServiceManager(db *gorm.DB, repo repositories.Repository, logger *slog.Logger, validator *validator.Validator) ServiceManager {
	config := ServiceManagerConfig{
		EnableDebugLogging: false,
		EnableMetrics:      true,
		LogLevel:           slog.LevelInfo,

		Assessment: ServiceConfig{
			Enabled:         true,
			CacheEnabled:    true,
			CacheTTL:        10 * time.Minute,
			ValidationLevel: ValidationFull,
			AuditingEnabled: true,
			MetricsEnabled:  true,
		},
		Question: ServiceConfig{
			Enabled:         true,
			CacheEnabled:    true,
			CacheTTL:        30 * time.Minute,
			ValidationLevel: ValidationFull,
			AuditingEnabled: true,
			MetricsEnabled:  true,
		},
		Attempt: ServiceConfig{
			Enabled:         true,
			CacheEnabled:    false, // Real-time data
			CacheTTL:        0,
			ValidationLevel: ValidationStrict,
			AuditingEnabled: true,
			MetricsEnabled:  true,
		},
		Grading: ServiceConfig{
			Enabled:         true,
			CacheEnabled:    false,
			CacheTTL:        0,
			ValidationLevel: ValidationStrict,
			AuditingEnabled: true,
			MetricsEnabled:  true,
		},

		DefaultTimeout: 60 * time.Second,
		MaxRetries:     3,
		CircuitBreaker: true,
		RateLimitingRules: map[string]RateLimit{
			"assessment_create": {RequestsPerMinute: 60, BurstSize: 10},
			"attempt_start":     {RequestsPerMinute: 100, BurstSize: 20},
			"grading_submit":    {RequestsPerMinute: 200, BurstSize: 50},
		},
	}

	return NewServiceManager(db, repo, logger, validator, config)
}

// CreateDevelopmentServiceManager creates a service manager configured for development
func CreateDevelopmentServiceManager(db *gorm.DB, repo repositories.Repository, logger *slog.Logger, validator *validator.Validator) ServiceManager {
	config := ServiceManagerConfig{
		EnableDebugLogging: true,
		EnableMetrics:      false,
		LogLevel:           slog.LevelDebug,

		Assessment: ServiceConfig{
			Enabled:         true,
			CacheEnabled:    false,
			CacheTTL:        0,
			ValidationLevel: ValidationBasic,
			AuditingEnabled: false,
			MetricsEnabled:  false,
		},
		Question: ServiceConfig{
			Enabled:         true,
			CacheEnabled:    false,
			CacheTTL:        0,
			ValidationLevel: ValidationBasic,
			AuditingEnabled: false,
			MetricsEnabled:  false,
		},
		Attempt: ServiceConfig{
			Enabled:         true,
			CacheEnabled:    false,
			CacheTTL:        0,
			ValidationLevel: ValidationBasic,
			AuditingEnabled: false,
			MetricsEnabled:  false,
		},
		Grading: ServiceConfig{
			Enabled:         true,
			CacheEnabled:    false,
			CacheTTL:        0,
			ValidationLevel: ValidationBasic,
			AuditingEnabled: false,
			MetricsEnabled:  false,
		},

		DefaultTimeout:    10 * time.Second,
		MaxRetries:        1,
		CircuitBreaker:    false,
		RateLimitingRules: make(map[string]RateLimit),
	}

	return NewServiceManager(db, repo, logger, validator, config)
}
