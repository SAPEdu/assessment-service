package repositories

import (
	"context"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"gorm.io/gorm"
)

// QuestionRepository interface for question-specific operations
type QuestionRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, tx *gorm.DB, question *models.Question) error
	GetByID(ctx context.Context, tx *gorm.DB, id uint) (*models.Question, error)
	GetByIDWithDetails(ctx context.Context, tx *gorm.DB, id uint) (*models.Question, error) // Include attachments, category
	Update(ctx context.Context, tx *gorm.DB, question *models.Question) error
	Delete(ctx context.Context, tx *gorm.DB, id uint) error

	// Bulk operations
	CreateBatch(ctx context.Context, tx *gorm.DB, questions []*models.Question) error
	UpdateBatch(ctx context.Context, tx *gorm.DB, questions []*models.Question) error
	GetByIDs(ctx context.Context, tx *gorm.DB, ids []uint) ([]*models.Question, error)
	DeleteBatch(ctx context.Context, tx *gorm.DB, ids []uint) error

	// Query operations
	List(ctx context.Context, tx *gorm.DB, filters QuestionFilters) ([]*models.Question, int64, error)
	GetByCreator(ctx context.Context, tx *gorm.DB, creatorID string, filters QuestionFilters) ([]*models.Question, int64, error)
	GetByCategory(ctx context.Context, tx *gorm.DB, categoryID uint, filters QuestionFilters) ([]*models.Question, error)
	GetByType(ctx context.Context, tx *gorm.DB, questionType models.QuestionType, filters QuestionFilters) ([]*models.Question, error)
	GetByDifficulty(ctx context.Context, tx *gorm.DB, difficulty models.DifficultyLevel, limit, offset int) ([]*models.Question, error)
	Search(ctx context.Context, tx *gorm.DB, query string, filters QuestionFilters) ([]*models.Question, int64, error)

	// Assessment-specific queries
	GetByAssessment(ctx context.Context, tx *gorm.DB, assessmentID uint) ([]*models.Question, error)
	GetRandomQuestions(ctx context.Context, tx *gorm.DB, filters RandomQuestionFilters) ([]*models.Question, error)
	GetQuestionBank(ctx context.Context, tx *gorm.DB, creatorID string, filters QuestionBankFilters) ([]*models.Question, int64, error)

	// Advanced filtering
	GetByTags(ctx context.Context, tx *gorm.DB, tags []string, filters QuestionFilters) ([]*models.Question, error)
	GetSimilarQuestions(ctx context.Context, tx *gorm.DB, questionID uint, limit int) ([]*models.Question, error)

	// Statistics and analytics
	GetQuestionStats(ctx context.Context, tx *gorm.DB, id uint) (*QuestionStats, error)
	GetUsageStats(ctx context.Context, tx *gorm.DB, creatorID string) (*QuestionUsageStats, error)
	GetPerformanceStats(ctx context.Context, tx *gorm.DB, questionID uint) (*QuestionPerformanceStats, error)

	// Validation and checks
	ExistsByText(ctx context.Context, tx *gorm.DB, text string, creatorID string, excludeID *uint) (bool, error)
	IsUsedInAssessments(ctx context.Context, tx *gorm.DB, id uint) (bool, error)
	GetUsageCount(ctx context.Context, tx *gorm.DB, id uint) (int, error)

	// Content management
	UpdateContent(ctx context.Context, tx *gorm.DB, id uint, content interface{}) error

	// Question bank operations
	GetByBank(ctx context.Context, bankID uint, filters QuestionFilters) ([]*models.Question, int64, error)
	AddToBank(ctx context.Context, questionID, bankID uint) error
	RemoveFromBank(ctx context.Context, questionID, bankID uint) error
}

// QuestionCategoryRepository interface for question category operations
type QuestionCategoryRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, tx *gorm.DB, category *models.QuestionCategory) error
	GetByID(ctx context.Context, tx *gorm.DB, id uint) (*models.QuestionCategory, error)
	GetByIDWithChildren(ctx context.Context, tx *gorm.DB, id uint) (*models.QuestionCategory, error)
	Update(ctx context.Context, tx *gorm.DB, category *models.QuestionCategory) error
	Delete(ctx context.Context, tx *gorm.DB, id uint) error

	// Hierarchy operations
	GetByCreator(ctx context.Context, tx *gorm.DB, creatorID string) ([]*models.QuestionCategory, error)
	GetRootCategories(ctx context.Context, tx *gorm.DB, creatorID string) ([]*models.QuestionCategory, error)
	GetChildren(ctx context.Context, tx *gorm.DB, parentID uint) ([]*models.QuestionCategory, error)
	GetHierarchy(ctx context.Context, tx *gorm.DB, creatorID string) ([]*models.QuestionCategory, error)
	GetPath(ctx context.Context, tx *gorm.DB, categoryID uint) ([]*models.QuestionCategory, error)

	// Tree operations
	MoveCategory(ctx context.Context, tx *gorm.DB, categoryID uint, newParentID *uint) error
	GetDescendants(ctx context.Context, tx *gorm.DB, categoryID uint) ([]*models.QuestionCategory, error)
	UpdatePath(ctx context.Context, tx *gorm.DB, categoryID uint) error

	// Validation
	ExistsByName(ctx context.Context, tx *gorm.DB, name string, creatorID string, parentID *uint) (bool, error)
	HasQuestions(ctx context.Context, tx *gorm.DB, id uint) (bool, error)
	HasChildren(ctx context.Context, tx *gorm.DB, id uint) (bool, error)
	ValidateHierarchy(ctx context.Context, tx *gorm.DB, categoryID uint, parentID *uint) error

	// Statistics
	GetCategoryStats(ctx context.Context, tx *gorm.DB, categoryID uint) (*CategoryStats, error)
	GetCategoriesWithCounts(ctx context.Context, tx *gorm.DB, creatorID string) ([]*CategoryWithCount, error)
}

// QuestionAttachmentRepository interface for question attachment operations
type QuestionAttachmentRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, tx *gorm.DB, attachment *models.QuestionAttachment) error
	GetByID(ctx context.Context, tx *gorm.DB, id uint) (*models.QuestionAttachment, error)
	Update(ctx context.Context, tx *gorm.DB, attachment *models.QuestionAttachment) error
	Delete(ctx context.Context, tx *gorm.DB, id uint) error

	// Query operations
	GetByQuestion(ctx context.Context, tx *gorm.DB, questionID uint) ([]*models.QuestionAttachment, error)
	GetByQuestions(ctx context.Context, tx *gorm.DB, questionIDs []uint) (map[uint][]*models.QuestionAttachment, error)

	// Bulk operations
	CreateBatch(ctx context.Context, tx *gorm.DB, attachments []*models.QuestionAttachment) error
	DeleteByQuestion(ctx context.Context, tx *gorm.DB, questionID uint) error

	// File management
	GetOrphanedAttachments(ctx context.Context, tx *gorm.DB) ([]*models.QuestionAttachment, error)
	UpdateOrder(ctx context.Context, tx *gorm.DB, questionID uint, attachmentOrders []AttachmentOrder) error
}

// QuestionBankRepository interface for question bank operations
type QuestionBankRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, tx *gorm.DB, bank *models.QuestionBank) error
	GetByID(ctx context.Context, tx *gorm.DB, id uint) (*models.QuestionBank, error)
	GetByIDWithDetails(ctx context.Context, tx *gorm.DB, id uint) (*models.QuestionBank, error)
	Update(ctx context.Context, tx *gorm.DB, bank *models.QuestionBank) error
	Delete(ctx context.Context, tx *gorm.DB, id uint) error

	// Query operations
	List(ctx context.Context, tx *gorm.DB, filters QuestionBankFilters) ([]*models.QuestionBank, int64, error)
	GetByCreator(ctx context.Context, tx *gorm.DB, creatorID string, filters QuestionBankFilters) ([]*models.QuestionBank, int64, error)
	GetPublicBanks(ctx context.Context, tx *gorm.DB, filters QuestionBankFilters) ([]*models.QuestionBank, int64, error)
	GetSharedWithUser(ctx context.Context, tx *gorm.DB, userID string, filters QuestionBankFilters) ([]*models.QuestionBank, int64, error)
	Search(ctx context.Context, tx *gorm.DB, query string, filters QuestionBankFilters) ([]*models.QuestionBank, int64, error)

	// Sharing operations
	ShareBank(ctx context.Context, tx *gorm.DB, share *models.QuestionBankShare) error
	UnshareBank(ctx context.Context, tx *gorm.DB, bankID uint, userID string) error
	UpdateSharePermissions(ctx context.Context, tx *gorm.DB, bankID uint, userID string, canEdit, canDelete bool) error
	GetBankShares(ctx context.Context, tx *gorm.DB, bankID uint) ([]*models.QuestionBankShare, error)
	GetUserShares(ctx context.Context, tx *gorm.DB, userID string, filters QuestionBankShareFilters) ([]*models.QuestionBankShare, int64, error)

	// Question-Bank relationship operations
	AddQuestions(ctx context.Context, tx *gorm.DB, bankID uint, questionIDs []uint) error
	RemoveQuestions(ctx context.Context, tx *gorm.DB, bankID uint, questionIDs []uint) error
	GetBankQuestions(ctx context.Context, tx *gorm.DB, bankID uint, filters QuestionFilters) ([]*models.Question, int64, error)
	IsQuestionInBank(ctx context.Context, tx *gorm.DB, questionID, bankID uint) (bool, error)

	// Permission checks
	CanAccess(ctx context.Context, tx *gorm.DB, bankID uint, userID string) (bool, error)
	CanEdit(ctx context.Context, tx *gorm.DB, bankID uint, userID string) (bool, error)
	CanDelete(ctx context.Context, tx *gorm.DB, bankID uint, userID string) (bool, error)
	IsOwner(ctx context.Context, tx *gorm.DB, bankID uint, userID string) (bool, error)

	// Validation
	ExistsByName(ctx context.Context, tx *gorm.DB, name string, creatorID string) (bool, error)
	HasQuestions(ctx context.Context, tx *gorm.DB, bankID uint) (bool, error)

	// Statistics
	CountQuestionsInBank(ctx context.Context, tx *gorm.DB, bankID uint) (int, error)
	GetBankStats(ctx context.Context, tx *gorm.DB, bankID uint) (*QuestionBankStats, error)
	GetUsageCount(ctx context.Context, tx *gorm.DB, bankID uint) (int, error)
	UpdateUsage(ctx context.Context, tx *gorm.DB, bankID uint) error
}

// ===== ADDITIONAL FILTER STRUCTS =====

type AttachmentOrder struct {
	AttachmentID uint `json:"attachment_id"`
	Order        int  `json:"order"`
}

// ===== ADDITIONAL STATISTICS STRUCTS =====

type QuestionPerformanceStats struct {
	TotalAttempts      int            `json:"total_attempts"`
	CorrectAnswers     int            `json:"correct_answers"`
	CorrectRate        float64        `json:"correct_rate"`
	AverageScore       float64        `json:"average_score"`
	AverageTimeSpent   int            `json:"average_time_spent"`
	DifficultyActual   float64        `json:"difficulty_actual"`
	AnswerDistribution map[string]int `json:"answer_distribution"` // For MC questions
}

type CategoryStats struct {
	QuestionCount    int                            `json:"question_count"`
	SubcategoryCount int                            `json:"subcategory_count"`
	QuestionsByType  map[models.QuestionType]int    `json:"questions_by_type"`
	QuestionsByDiff  map[models.DifficultyLevel]int `json:"questions_by_difficulty"`
	TotalUsage       int                            `json:"total_usage"`
}

type CategoryWithCount struct {
	*models.QuestionCategory
	QuestionCount int `json:"question_count"`
	DirectCount   int `json:"direct_count"` // Questions directly in this category
	TotalCount    int `json:"total_count"`  // Including subcategories
}
