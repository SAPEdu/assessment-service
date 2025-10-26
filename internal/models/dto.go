package models

import (
	"encoding/json"
	"time"
)

type AssessmentCreateRequest struct {
	Title        string                     `json:"title" validate:"required,min=1,max=200"`
	Description  *string                    `json:"description" validate:"omitempty,max=1000"`
	Duration     int                        `json:"duration" validate:"required,min=5,max=300"`
	PassingScore int                        `json:"passing_score" validate:"required,min=0,max=100"`
	MaxAttempts  int                        `json:"max_attempts" validate:"min=1,max=10"`
	TimeWarning  *int                       `json:"time_warning" validate:"omitempty,min=60,max=3600"`
	DueDate      *time.Time                 `json:"due_date"`
	Settings     *AssessmentSettingsRequest `json:"settings"`
	CategoryID   *uint                      `json:"category_id"`
}

type AssessmentUpdateRequest struct {
	Title        *string                    `json:"title" validate:"omitempty,min=1,max=200"`
	Description  *string                    `json:"description" validate:"omitempty,max=1000"`
	Duration     *int                       `json:"duration" validate:"omitempty,min=5,max=300"`
	PassingScore *int                       `json:"passing_score" validate:"omitempty,min=0,max=100"`
	MaxAttempts  *int                       `json:"max_attempts" validate:"omitempty,min=1,max=10"`
	TimeWarning  *int                       `json:"time_warning" validate:"omitempty,min=60,max=3600"`
	DueDate      *time.Time                 `json:"due_date"`
	Settings     *AssessmentSettingsRequest `json:"settings"`
	CategoryID   *uint                      `json:"category_id"`
}

type AssessmentSettingsRequest struct {
	RandomizeQuestions          *bool `json:"randomize_questions"`
	RandomizeOptions            *bool `json:"randomize_options"`
	ShowProgressBar             *bool `json:"show_progress_bar"`
	RequireWebcam               *bool `json:"require_webcam"`
	PreventTabSwitching         *bool `json:"prevent_tab_switching"`
	PreventRightClick           *bool `json:"prevent_right_click"`
	PreventCopyPaste            *bool `json:"prevent_copy_paste"`
	RequireIdentityVerification *bool `json:"require_identity_verification"`
	RequireFullScreen           *bool `json:"require_full_screen"`
	AllowScreenReader           *bool `json:"allow_screen_reader"`
	FontSizeAdjustment          *int  `json:"font_size_adjustment" validate:"omitempty,min=-2,max=2"`
	HighContrastMode            *bool `json:"high_contrast_mode"`
}

type QuestionCreateRequest struct {
	Type        QuestionType    `json:"type" validate:"required,oneof=multiple_choice true_false essay fill_blank matching ordering short_answer"`
	Text        string          `json:"text" validate:"required"`
	Points      int             `json:"points" validate:"min=1,max=100"`
	TimeLimit   *int            `json:"time_limit" validate:"omitempty,min=10,max=7200"` // DEPRECATED: Not used in timing logic
	Content     json.RawMessage `json:"content" validate:"required"`
	CategoryID  *uint           `json:"category_id"`
	Difficulty  DifficultyLevel `json:"difficulty" validate:"oneof=easy medium hard"`
	Tags        []string        `json:"tags"`
	Explanation *string         `json:"explanation"`
}

type QuestionUpdateRequest struct {
	Text        *string          `json:"text" validate:"omitempty,min=1"`
	Points      *int             `json:"points" validate:"omitempty,min=1,max=100"`
	TimeLimit   *int             `json:"time_limit" validate:"omitempty,min=10,max=7200"` // DEPRECATED: Not used in timing logic
	Content     json.RawMessage  `json:"content"`
	CategoryID  *uint            `json:"category_id"`
	Difficulty  *DifficultyLevel `json:"difficulty" validate:"omitempty,oneof=easy medium hard"`
	Tags        []string         `json:"tags"`
	Explanation *string          `json:"explanation"`
}

type QuestionBankCreateRequest struct {
	Name        string  `json:"name" validate:"required,min=1,max=200"`
	Description *string `json:"description" validate:"omitempty,max=1000"`
	IsPublic    bool    `json:"is_public"`
	IsShared    bool    `json:"is_shared"`
}

type CategoryCreateRequest struct {
	Name        string  `json:"name" validate:"required,min=1,max=100"`
	Description *string `json:"description" validate:"omitempty,max=500"`
	Color       string  `json:"color" validate:"omitempty,hexcolor"`
	Icon        *string `json:"icon" validate:"omitempty,max=50"`
	ParentID    *uint   `json:"parent_id"`
}

type AttemptStartRequest struct {
	AssessmentID uint `json:"assessment_id" validate:"required"`
}

type SubmitAnswerRequest struct {
	QuestionID uint            `json:"question_id" validate:"required"`
	Answer     json.RawMessage `json:"answer" validate:"required"`
	TimeSpent  int             `json:"time_spent" validate:"min=0"`
	Flagged    bool            `json:"flagged"`
}

type CompleteAttemptRequest struct {
	FinalAnswers []SubmitAnswerRequest `json:"final_answers"`
	TimeSpent    int                   `json:"time_spent" validate:"min=0"`
}

type GradeAnswerRequest struct {
	Score    float64 `json:"score" validate:"min=0"`
	Feedback *string `json:"feedback"`
}

// ===== PAGINATION & FILTERING =====

type ListAssessmentsParams struct {
	Page       int              `json:"page" validate:"min=0"`
	Size       int              `json:"size" validate:"min=1,max=100"`
	Status     AssessmentStatus `json:"status"`
	Search     string           `json:"search"`
	CategoryID *uint            `json:"category_id"`
	CreatedBy  *string          `json:"created_by"`
	SortBy     string           `json:"sort_by"`
	SortDir    string           `json:"sort_dir" validate:"omitempty,oneof=asc desc"`
	DateFrom   *time.Time       `json:"date_from"`
	DateTo     *time.Time       `json:"date_to"`
}

type ListQuestionsParams struct {
	Page       int             `json:"page" validate:"min=0"`
	Size       int             `json:"size" validate:"min=1,max=100"`
	Type       QuestionType    `json:"type"`
	CategoryID *uint           `json:"category_id"`
	Difficulty DifficultyLevel `json:"difficulty"`
	Tags       []string        `json:"tags"`
	Search     string          `json:"search"`
	BankID     *uint           `json:"bank_id"`
	SortBy     string          `json:"sort_by"`
	SortDir    string          `json:"sort_dir" validate:"omitempty,oneof=asc desc"`
}

type ListAttemptsParams struct {
	Page         int           `json:"page" validate:"min=0"`
	Size         int           `json:"size" validate:"min=1,max=100"`
	AssessmentID *uint         `json:"assessment_id"`
	StudentID    *string       `json:"student_id"`
	Status       AttemptStatus `json:"status"`
	DateFrom     *time.Time    `json:"date_from"`
	DateTo       *time.Time    `json:"date_to"`
	SortBy       string        `json:"sort_by"`
	SortDir      string        `json:"sort_dir" validate:"omitempty,oneof=asc desc"`
}

type PaginatedResponse struct {
	Content          interface{} `json:"content"`
	TotalElements    int64       `json:"total_elements"`
	TotalPages       int         `json:"total_pages"`
	Size             int         `json:"size"`
	Page             int         `json:"page"`
	First            bool        `json:"first"`
	Last             bool        `json:"last"`
	NumberOfElements int         `json:"number_of_elements"`
	Empty            bool        `json:"empty"`
}

// ===== STATISTICS & ANALYTICS DTOS =====

type ScoreBucket struct {
	Range string `json:"range"` // "0-10", "11-20", etc.
	Count int    `json:"count"`
}

type TimeBucket struct {
	Range string `json:"range"` // "0-5 min", "6-10 min", etc.
	Count int    `json:"count"`
}

type OptionStat struct {
	OptionID       string  `json:"option_id"`
	OptionText     string  `json:"option_text"`
	SelectionCount int     `json:"selection_count"`
	SelectionRate  float64 `json:"selection_rate"`
	IsCorrect      bool    `json:"is_correct"`
}

type AssessmentStats struct {
	TotalAttempts     int                 `json:"total_attempts"`
	CompletedAttempts int                 `json:"completed_attempts"`
	AverageScore      float64             `json:"average_score"`
	PassRate          float64             `json:"pass_rate"`
	AverageTime       int                 `json:"average_time"`
	ScoreDistribution []ScoreBucket       `json:"score_distribution"`
	QuestionStats     []QuestionStatsItem `json:"question_stats"`
	CategoryBreakdown []CategoryStatsItem `json:"category_breakdown"`
}

type QuestionStatsItem struct {
	QuestionID      uint    `json:"question_id"`
	QuestionText    string  `json:"question_text"`
	CorrectRate     float64 `json:"correct_rate"`
	AverageScore    float64 `json:"average_score"`
	DifficultyIndex float64 `json:"difficulty_index"`
}

type CategoryStatsItem struct {
	CategoryID    uint    `json:"category_id"`
	CategoryName  string  `json:"category_name"`
	QuestionCount int     `json:"question_count"`
	AverageScore  float64 `json:"average_score"`
}

type StudentProgress struct {
	StudentID         string     `json:"student_id"`
	StudentName       string     `json:"student_name"`
	StudentEmail      string     `json:"student_email"`
	AttemptNumber     int        `json:"attempt_number"`
	Status            string     `json:"status"`
	Score             float64    `json:"score"`
	Percentage        float64    `json:"percentage"`
	TimeSpent         int        `json:"time_spent"`
	StartedAt         time.Time  `json:"started_at"`
	CompletedAt       *time.Time `json:"completed_at"`
	QuestionsAnswered int        `json:"questions_answered"`
	TotalQuestions    int        `json:"total_questions"`
	ViolationCount    int        `json:"violation_count"`
}

// ===== ASSESSMENT STATUS MANAGEMENT =====

type ChangeStatusRequest struct {
	Status AssessmentStatus `json:"status" validate:"required,oneof=Draft Active Expired Archived"`
	Reason *string          `json:"reason" validate:"omitempty,max=500"`
}

type StatusChangeResponse struct {
	OldStatus AssessmentStatus `json:"old_status"`
	NewStatus AssessmentStatus `json:"new_status"`
	ChangedAt time.Time        `json:"changed_at"`
	ChangedBy uint             `json:"changed_by"`
	Reason    *string          `json:"reason"`
}

// ===== BULK OPERATIONS =====

type BulkDeleteRequest struct {
	AssessmentIDs []uint  `json:"assessment_ids" validate:"required,min=1,max=50"`
	Reason        *string `json:"reason" validate:"omitempty,max=500"`
}

type BulkStatusChangeRequest struct {
	AssessmentIDs []uint           `json:"assessment_ids" validate:"required,min=1,max=50"`
	Status        AssessmentStatus `json:"status" validate:"required,oneof=Draft Active Expired Archived"`
	Reason        *string          `json:"reason" validate:"omitempty,max=500"`
}

type BulkOperationResult struct {
	SuccessCount int                  `json:"success_count"`
	FailureCount int                  `json:"failure_count"`
	Errors       []BulkOperationError `json:"errors"`
	SuccessItems []uint               `json:"success_items"`
}

type BulkOperationError struct {
	AssessmentID uint   `json:"assessment_id"`
	Error        string `json:"error"`
}

// ===== IMPORT/EXPORT DTOs =====

type ImportQuestionsRequest struct {
	AssessmentID uint   `json:"assessment_id" validate:"required"`
	Format       string `json:"format" validate:"required,oneof=xlsx csv json"`
	Overwrite    bool   `json:"overwrite"`
	ValidateOnly bool   `json:"validate_only"`
}

type ExportQuestionsRequest struct {
	AssessmentID   uint     `json:"assessment_id" validate:"required"`
	Format         string   `json:"format" validate:"required,oneof=xlsx csv json pdf"`
	IncludeAnswers bool     `json:"include_answers"`
	IncludeStats   bool     `json:"include_stats"`
	Categories     []uint   `json:"categories"`
	Difficulties   []string `json:"difficulties"`
}

type ImportJobResponse struct {
	JobID               string                  `json:"job_id"`
	Status              string                  `json:"status"`
	Progress            int                     `json:"progress"`
	TotalRows           int                     `json:"total_rows"`
	ProcessedRows       int                     `json:"processed_rows"`
	Errors              []ImportValidationError `json:"errors"`
	CreatedAt           time.Time               `json:"created_at"`
	EstimatedCompletion *time.Time              `json:"estimated_completion"`
}

// ===== VALIDATION RESPONSES =====

type ValidationErrorResponse struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value"`
	Code    string `json:"code"`
}

type BusinessRuleViolation struct {
	Rule        string `json:"rule"`
	Message     string `json:"message"`
	Severity    string `json:"severity"` // error, warning, info
	CanOverride bool   `json:"can_override"`
}

// ===== ASSESSMENT SUMMARY DTOs =====

type AssessmentSummary struct {
	ID             uint             `json:"id"`
	Title          string           `json:"title"`
	Duration       int              `json:"duration"`
	Status         AssessmentStatus `json:"status"`
	DueDate        *time.Time       `json:"due_date"`
	CreatedBy      string           `json:"created_by"`
	CreatedAt      time.Time        `json:"created_at"`
	Attempts       int              `json:"attempts"`
	PassingScore   int              `json:"passing_score"`
	QuestionsCount int              `json:"questions_count"`
	AvgScore       float64          `json:"avg_score"`
	PassRate       float64          `json:"pass_rate"`
}

type QuestionSummary struct {
	ID         uint            `json:"id"`
	Type       QuestionType    `json:"type"`
	Text       string          `json:"text"`
	Points     int             `json:"points"`
	Difficulty DifficultyLevel `json:"difficulty"`
	Category   *string         `json:"category"`
	UsageCount int             `json:"usage_count"`
	AvgScore   float64         `json:"avg_score"`
	CreatedAt  time.Time       `json:"created_at"`
}

// ===== ERROR RESPONSES =====

type ErrorResponse struct {
	Error                  string                    `json:"error"`
	Message                string                    `json:"message"`
	Code                   string                    `json:"code"`
	Details                interface{}               `json:"details,omitempty"`
	Timestamp              time.Time                 `json:"timestamp"`
	Path                   string                    `json:"path"`
	ValidationErrors       []ValidationErrorResponse `json:"validation_errors,omitempty"`
	BusinessRuleViolations []BusinessRuleViolation   `json:"business_rule_violations,omitempty"`
}

type SuccessResponse struct {
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}
