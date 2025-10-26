package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/SAP-F-2025/assessment-service/internal/validator"
	"gorm.io/gorm"
)

// ===== REQUEST/RESPONSE DTOs =====

// Use business validator types
type CreateAssessmentRequest = validator.AssessmentCreateRequest
type UpdateAssessmentRequest = validator.AssessmentUpdateRequest
type AssessmentSettingsRequest = validator.AssessmentSettingsRequest

// Use business validator types
type AssessmentQuestionRequest = validator.AssessmentQuestionRequest

type AssessmentResponse struct {
	*models.Assessment
	CanEdit   bool `json:"can_edit"`
	CanDelete bool `json:"can_delete"`
	CanTake   bool `json:"can_take"`
}

type AssessmentListResponse struct {
	Assessments []*AssessmentResponse `json:"assessments"`
	Total       int64                 `json:"total"`
	Page        int                   `json:"page"`
	Size        int                   `json:"size"`
}

type UpdateStatusRequest struct {
	Status models.AssessmentStatus `json:"status" validate:"required,oneof=Draft Active Expired Archived"`
	Reason *string                 `json:"reason" validate:"omitempty,max=500"`
}

type UpdateAssessmentQuestionRequest struct {
	QuestionId uint `json:"question_id"`
	Points     int  `json:"points" validate:"required,min=1,max=100"`       // Required: Actual points for this question in the assessment
	TimeLimit  *int `json:"time_limit" validate:"omitempty,min=5,max=3600"` // DEPRECATED: Not used in timing logic
}

type ReorderQuestionsRequest struct {
	QuestionOrders []repositories.QuestionOrder `json:"question_orders"`
}

// ===== ATTEMPT RELATED DTOs =====

type StartAttemptRequest struct {
	AssessmentID uint `json:"assessment_id" validate:"required"`
}

type SubmitAnswerRequest struct {
	QuestionID uint        `json:"question_id" validate:"required"`
	AnswerData interface{} `json:"answer" validate:"required"`
	TimeSpent  *int        `json:"time_spent"`
}

type SubmitAttemptRequest struct {
	AttemptID uint                  `json:"attempt_id" validate:"required"`
	Answers   []SubmitAnswerRequest `json:"answers" validate:"required,dive"`
	TimeSpent *int                  `json:"time_spent"`
	EndReason string                `json:"end_reason"`
}

type AttemptResponse struct {
	*models.AssessmentAttempt
	CanSubmit      bool                 `json:"can_submit"`
	CanResume      bool                 `json:"can_resume"`
	IsPendingGrade bool                 `json:"is_pending_grade"`
	Questions      []QuestionForAttempt `json:"questions,omitempty"`
}

type QuestionForAttempt struct {
	*models.Question
	IsLast  bool `json:"is_last"`
	IsFirst bool `json:"is_first"`
}

// ===== QUESTION RELATED DTOs =====

// Use business validator types
type CreateQuestionRequest = validator.QuestionCreateRequest

type UpdateQuestionRequest struct {
	Text        *string                 `json:"text" validate:"omitempty,max=2000"`
	Content     interface{}             `json:"content"`
	Points      *int                    `json:"points" validate:"omitempty,min=1,max=100"`
	TimeLimit   *int                    `json:"time_limit" validate:"omitempty,min=5,max=3600"` // DEPRECATED: Not used in timing logic
	Difficulty  *models.DifficultyLevel `json:"difficulty"`
	CategoryID  *uint                   `json:"category_id"`
	Tags        []string                `json:"tags"`
	Explanation *string                 `json:"explanation" validate:"omitempty,max=1000"`
}

type QuestionResponse struct {
	*models.Question
	CanEdit    bool `json:"can_edit"`
	CanDelete  bool `json:"can_delete"`
	UsageCount int  `json:"usage_count"`
}

type QuestionListResponse struct {
	Questions []*QuestionResponse `json:"questions"`
	Total     int64               `json:"total"`
	Page      int                 `json:"page"`
	Size      int                 `json:"size"`
}

// ===== GRADING RELATED DTOs =====

type GradingResult struct {
	AnswerID      uint      `json:"answer_id"`
	QuestionID    uint      `json:"question_id"`
	Score         float64   `json:"score"`
	MaxScore      float64   `json:"max_score"`
	IsCorrect     bool      `json:"is_correct"`
	PartialCredit bool      `json:"partial_credit"`
	Feedback      *string   `json:"feedback"`
	GradedAt      time.Time `json:"graded_at"`
	GradedBy      *string   `json:"graded_by"`
}

type AttemptGradingResult struct {
	AttemptID  uint            `json:"attempt_id"`
	TotalScore float64         `json:"total_score"`
	MaxScore   float64         `json:"max_score"`
	Percentage float64         `json:"percentage"`
	IsPassing  bool            `json:"is_passing"`
	Grade      *string         `json:"grade"`
	Questions  []GradingResult `json:"questions"`
	GradedAt   time.Time       `json:"graded_at"`
	GradedBy   string          `json:"graded_by"`
}

// ===== QUESTION BANK RELATED DTOs =====

type CreateQuestionBankRequest struct {
	Name        string  `json:"name" validate:"required,max=200"`
	Description *string `json:"description" validate:"omitempty,max=2000"`
	IsPublic    bool    `json:"is_public"`
	IsShared    bool    `json:"is_shared"`
}

type UpdateQuestionBankRequest struct {
	Name        *string `json:"name" validate:"omitempty,max=200"`
	Description *string `json:"description" validate:"omitempty,max=2000"`
	IsPublic    *bool   `json:"is_public"`
	IsShared    *bool   `json:"is_shared"`
}

type ShareQuestionBankRequest struct {
	UserID    string `json:"user_id" validate:"required"`
	CanEdit   bool   `json:"can_edit"`
	CanDelete bool   `json:"can_delete"`
}

type QuestionBankResponse struct {
	*models.QuestionBank
	CanEdit       bool   `json:"can_edit"`
	CanDelete     bool   `json:"can_delete"`
	QuestionCount int    `json:"question_count"`
	ShareCount    int    `json:"share_count"`
	IsOwner       bool   `json:"is_owner"`
	AccessLevel   string `json:"access_level"` // "owner", "editor", "viewer"
}

type QuestionBankListResponse struct {
	Banks []*QuestionBankResponse `json:"banks"`
	Total int64                   `json:"total"`
	Page  int                     `json:"page"`
	Size  int                     `json:"size"`
}

type QuestionBankShareResponse struct {
	*models.QuestionBankShare
	CanModify bool `json:"can_modify"`
}

type AddQuestionsTobankRequest struct {
	QuestionIDs []uint `json:"question_ids" validate:"required,min=1"`
}

// ===== SERVICE INTERFACES =====

type AssessmentService interface {
	// Core CRUD operations
	Create(ctx context.Context, req *CreateAssessmentRequest, creatorID string) (*AssessmentResponse, error)
	GetByID(ctx context.Context, id uint, userID string) (*AssessmentResponse, error)
	GetByIDWithDetails(ctx context.Context, id uint, userID string) (*AssessmentResponse, error)
	Update(ctx context.Context, id uint, req *UpdateAssessmentRequest, userID string) (*AssessmentResponse, error)
	Delete(ctx context.Context, id uint, userID string) error

	// List and search operations
	List(ctx context.Context, filters repositories.AssessmentFilters, userID string) (*AssessmentListResponse, error)
	GetByCreator(ctx context.Context, creatorID string, filters repositories.AssessmentFilters) (*AssessmentListResponse, error)
	Search(ctx context.Context, query string, filters repositories.AssessmentFilters, userID string) (*AssessmentListResponse, error)

	// Status management
	UpdateStatus(ctx context.Context, id uint, req *UpdateStatusRequest, userID string) error
	Publish(ctx context.Context, id uint, userID string) error
	Archive(ctx context.Context, id uint, userID string) error

	// Question management
	AddQuestion(ctx context.Context, assessmentID, questionID uint, order int, points int, userID string) error
	AddQuestions(ctx context.Context, assessmentID uint, questionsId []uint, userID string) error // Deprecated: Use AddQuestionsBatch
	AddQuestionsBatch(ctx context.Context, assessmentID uint, questions []AssessmentQuestionRequest, userID string) error
	AutoAssignQuestions(ctx context.Context, assessmentID uint, questionIDs []uint, userID string) error // Auto-calculate and assign points evenly
	RemoveQuestion(ctx context.Context, assessmentID, questionID uint, userID string) error
	RemoveQuestions(ctx context.Context, assessmentID uint, questionsId []uint, userID string) error
	ReorderQuestions(ctx context.Context, assessmentID uint, orders []repositories.QuestionOrder, userID string) error
	UpdateAssessmentQuestionBatch(ctx context.Context, assessmentID uint, reqs []UpdateAssessmentQuestionRequest, userID string) error
	UpdateAssessmentQuestion(ctx context.Context, assessmentID, questionID uint, req *UpdateAssessmentQuestionRequest, userID string) error

	// Statistics and analytics
	GetStats(ctx context.Context, id uint, userID string) (*repositories.AssessmentStats, error)
	GetCreatorStats(ctx context.Context, creatorID string) (*repositories.CreatorStats, error)

	// Permission checks
	CanAccess(ctx context.Context, assessmentID uint, userID string) (bool, error)
	CanEdit(ctx context.Context, assessmentID uint, userID string) (bool, error)
	CanDelete(ctx context.Context, assessmentID uint, userID string) (bool, error)
	CanTake(ctx context.Context, assessmentID uint, userID string) (bool, error)
}

type QuestionService interface {
	// Core CRUD operations
	Create(ctx context.Context, req *CreateQuestionRequest, creatorID string) (*QuestionResponse, error)
	GetByID(ctx context.Context, id uint, userID string) (*QuestionResponse, error)
	GetByIDWithDetails(ctx context.Context, id uint, userID string) (*QuestionResponse, error)
	Update(ctx context.Context, id uint, req *UpdateQuestionRequest, userID string) (*QuestionResponse, error)
	Delete(ctx context.Context, id uint, userID string) error

	// List and search operations
	List(ctx context.Context, filters repositories.QuestionFilters, userID string) (*QuestionListResponse, error)
	GetByCreator(ctx context.Context, creatorID string, filters repositories.QuestionFilters) (*QuestionListResponse, error)
	Search(ctx context.Context, query string, filters repositories.QuestionFilters, userID string) (*QuestionListResponse, error)
	GetRandomQuestions(ctx context.Context, filters repositories.RandomQuestionFilters, userID string) ([]*models.Question, error)

	// Bulk operations
	CreateBatch(ctx context.Context, questions []*CreateQuestionRequest, creatorID string) ([]*QuestionResponse, []error)
	UpdateBatch(ctx context.Context, updates map[uint]*UpdateQuestionRequest, userID string) (map[uint]*QuestionResponse, map[uint]error)

	// Question banking
	GetByBank(ctx context.Context, bankID uint, filters repositories.QuestionFilters, userID string) (*QuestionListResponse, error)
	AddToBank(ctx context.Context, questionID, bankID uint, userID string) error
	RemoveFromBank(ctx context.Context, questionID, bankID uint, userID string) error

	// Statistics
	GetStats(ctx context.Context, questionID uint, userID string) (*repositories.QuestionStats, error)
	GetUsageStats(ctx context.Context, creatorID string) (*repositories.QuestionUsageStats, error)

	// Permission checks
	CanAccess(ctx context.Context, questionID uint, userID string) (bool, error)
	CanEdit(ctx context.Context, questionID uint, userID string) (bool, error)
	CanDelete(ctx context.Context, questionID uint, userID string) (bool, error)
}

type QuestionBankService interface {
	// Core CRUD operations
	Create(ctx context.Context, req *CreateQuestionBankRequest, creatorID string) (*QuestionBankResponse, error)
	GetByID(ctx context.Context, id uint, userID string) (*QuestionBankResponse, error)
	GetByIDWithDetails(ctx context.Context, id uint, userID string) (*QuestionBankResponse, error)
	Update(ctx context.Context, id uint, req *UpdateQuestionBankRequest, userID string) (*QuestionBankResponse, error)
	Delete(ctx context.Context, id uint, userID string) error

	// List and search operations
	List(ctx context.Context, filters repositories.QuestionBankFilters, userID string) (*QuestionBankListResponse, error)
	GetByCreator(ctx context.Context, creatorID string, filters repositories.QuestionBankFilters) (*QuestionBankListResponse, error)
	GetPublic(ctx context.Context, filters repositories.QuestionBankFilters) (*QuestionBankListResponse, error)
	GetSharedWithUser(ctx context.Context, userID string, filters repositories.QuestionBankFilters) (*QuestionBankListResponse, error)
	Search(ctx context.Context, query string, filters repositories.QuestionBankFilters, userID string) (*QuestionBankListResponse, error)

	// Sharing operations
	ShareBank(ctx context.Context, bankID uint, req *ShareQuestionBankRequest, sharerID string) error
	UnshareBank(ctx context.Context, bankID uint, userID string, sharerID string) error
	UpdateSharePermissions(ctx context.Context, bankID uint, userID string, canEdit, canDelete bool, sharerID string) error
	GetBankShares(ctx context.Context, bankID uint, userID string) ([]*QuestionBankShareResponse, error)
	GetUserShares(ctx context.Context, userID string, filters repositories.QuestionBankShareFilters) ([]*QuestionBankShareResponse, int64, error)

	// Question management
	AddQuestions(ctx context.Context, bankID uint, req *AddQuestionsTobankRequest, userID string) error
	RemoveQuestions(ctx context.Context, bankID uint, questionIDs []uint, userID string) error
	GetBankQuestions(ctx context.Context, bankID uint, filters repositories.QuestionFilters, userID string) (*QuestionListResponse, error)

	// Statistics
	GetStats(ctx context.Context, bankID uint, userID string) (*repositories.QuestionBankStats, error)

	// Permission checks
	CanAccess(ctx context.Context, bankID uint, userID string) (bool, error)
	CanEdit(ctx context.Context, bankID uint, userID string) (bool, error)
	CanDelete(ctx context.Context, bankID uint, userID string) (bool, error)
	IsOwner(ctx context.Context, bankID uint, userID string) (bool, error)
}

type AttemptService interface {
	// Core attempt operations
	Start(ctx context.Context, req *StartAttemptRequest, studentID string) (*AttemptResponse, error)
	Resume(ctx context.Context, attemptID uint, studentID string) (*AttemptResponse, error)
	Submit(ctx context.Context, req *SubmitAttemptRequest, studentID string) (*AttemptResponse, error)
	SubmitAnswer(ctx context.Context, attemptID uint, req *SubmitAnswerRequest, studentID string) error

	// Get operations
	GetByID(ctx context.Context, id uint, userID string) (*AttemptResponse, error)
	GetByIDWithDetails(ctx context.Context, id uint, userID string) (*AttemptResponse, error)
	GetCurrentAttempt(ctx context.Context, assessmentID uint, studentID string) (*AttemptResponse, error)

	// List operations
	List(ctx context.Context, filters repositories.AttemptFilters, userID string) ([]*AttemptResponse, int64, error)
	GetByStudent(ctx context.Context, studentID string, filters repositories.AttemptFilters) ([]*AttemptResponse, int64, error)
	GetByAssessment(ctx context.Context, assessmentID uint, filters repositories.AttemptFilters, userID string) ([]*AttemptResponse, int64, error)

	// Time management
	GetTimeRemaining(ctx context.Context, attemptID uint, studentID string) (int, error) // seconds
	ExtendTime(ctx context.Context, attemptID uint, minutes int, userID string) error
	HandleTimeout(ctx context.Context, attemptID uint) error

	// Validation
	CanStart(ctx context.Context, assessmentID uint, studentID string) (bool, error)
	GetAttemptCount(ctx context.Context, assessmentID uint, studentID string) (int, error)
	IsAttemptActive(ctx context.Context, attemptID uint) (bool, error)
	HasPendingManualGrading(ctx context.Context, tx *gorm.DB, attemptID uint) (bool, error)

	// Statistics
	GetStats(ctx context.Context, assessmentID uint, userID string) (*repositories.AttemptStats, error)
}

type GradingService interface {
	// Manual grading
	GradeAnswer(ctx context.Context, answerID uint, score float64, feedback *string, graderID string) (*GradingResult, error)
	GradeAttempt(ctx context.Context, attemptID uint, graderID string) (*AttemptGradingResult, error)
	GradeMultipleAnswers(ctx context.Context, grades []repositories.AnswerGrade, graderID string) ([]GradingResult, error)

	// Auto grading
	AutoGradeAnswer(ctx context.Context, answerID uint) (*GradingResult, error)
	AutoGradeAttempt(ctx context.Context, attemptID uint) (*AttemptGradingResult, error)
	AutoGradeAssessment(ctx context.Context, assessmentID uint) (map[uint]*AttemptGradingResult, error)

	// Grading utilities
	CalculateScore(ctx context.Context, questionType models.QuestionType, questionContent json.RawMessage, studentAnswer json.RawMessage) (float64, bool, error)
	GenerateFeedback(ctx context.Context, questionType models.QuestionType, questionContent json.RawMessage, studentAnswer json.RawMessage, isCorrect bool) (*string, error)

	// Bulk operations
	ReGradeQuestion(ctx context.Context, questionID uint, userID string) ([]GradingResult, error)
	ReGradeAssessment(ctx context.Context, assessmentID uint, userID string) (map[uint]*AttemptGradingResult, error)

	// Statistics
	GetGradingOverview(ctx context.Context, assessmentID uint, userID string) (*repositories.GradingStats, error)
}

// ===== SERVICE MANAGER =====

type ServiceManager interface {
	// Core service getters
	Assessment() AssessmentService
	Question() QuestionService
	QuestionBank() QuestionBankService
	Attempt() AttemptService
	Grading() GradingService
	Dashboard() DashboardService
	Student() StudentService

	// Additional service getters
	ImportExport() ImportExportService
	// Notification() NotificationService
	// Analytics() AnalyticsService

	// Health and lifecycle
	Initialize(ctx context.Context) error
	HealthCheck(ctx context.Context) error
	Shutdown(ctx context.Context) error
}
