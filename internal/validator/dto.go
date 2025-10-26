package validator

import (
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
)

// AssessmentCreateRequest represents the request structure for creating assessments
type AssessmentCreateRequest struct {
	Title        string                      `json:"title" validate:"required,assessment_title"`
	Description  *string                     `json:"description" validate:"omitempty,assessment_description"`
	Duration     int                         `json:"duration" validate:"required,assessment_duration"`
	PassingScore int                         `json:"passing_score" validate:"required,passing_score"`
	MaxAttempts  int                         `json:"max_attempts" validate:"required,max_attempts"`
	TimeWarning  *int                        `json:"time_warning" validate:"omitempty,min=60,max=1800"`
	DueDate      *time.Time                  `json:"due_date" validate:"omitempty,future_date"`
	Settings     *AssessmentSettingsRequest  `json:"settings"`
	Questions    []AssessmentQuestionRequest `json:"questions"`
}

// AssessmentUpdateRequest represents the request structure for updating assessments
type AssessmentUpdateRequest struct {
	Title        *string                    `json:"title" validate:"omitempty,assessment_title"`
	Description  *string                    `json:"description" validate:"omitempty,assessment_description"`
	Duration     *int                       `json:"duration" validate:"omitempty,assessment_duration"`
	PassingScore *int                       `json:"passing_score" validate:"omitempty,passing_score"`
	MaxAttempts  *int                       `json:"max_attempts" validate:"omitempty,max_attempts"`
	TimeWarning  *int                       `json:"time_warning" validate:"omitempty,min=60,max=1800"`
	DueDate      *time.Time                 `json:"due_date" validate:"omitempty,future_date"`
	Settings     *AssessmentSettingsRequest `json:"settings"`
}

// AssessmentSettingsRequest represents assessment settings
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

// AssessmentQuestionRequest represents adding questions to assessments
type AssessmentQuestionRequest struct {
	QuestionID uint `json:"question_id" validate:"required"`
	Order      int  `json:"order" validate:"required,min=1"`
	Points     int  `json:"points" validate:"required,points_range"` // Required: User must specify points when adding to assessment
}

// QuestionCreateRequest represents the request structure for creating questions
type QuestionCreateRequest struct {
	Type        models.QuestionType    `json:"type" validate:"required,question_type"`
	Text        string                 `json:"text" validate:"required,min=1,max=2000"`
	Content     interface{}            `json:"content" validate:"required"`
	Points      int                    `json:"points" validate:"required,points_range"`
	TimeLimit   *int                   `json:"time_limit" validate:"omitempty,time_limit"` // DEPRECATED: Not used in timing logic
	Difficulty  models.DifficultyLevel `json:"difficulty" validate:"required,difficulty_level"`
	CategoryID  *uint                  `json:"category_id"`
	Tags        []string               `json:"tags" validate:"omitempty,max=10,dive,max=50"`
	Explanation *string                `json:"explanation" validate:"omitempty,max=1000"`
}

// QuestionUpdateRequest represents the request structure for updating questions
type QuestionUpdateRequest struct {
	Type        *models.QuestionType    `json:"type" validate:"omitempty,question_type"`
	Text        *string                 `json:"text" validate:"omitempty,min=1,max=2000"`
	Content     interface{}             `json:"content"`
	Points      *int                    `json:"points" validate:"omitempty,points_range"`
	TimeLimit   *int                    `json:"time_limit" validate:"omitempty,time_limit"` // DEPRECATED: Not used in timing logic
	Difficulty  *models.DifficultyLevel `json:"difficulty" validate:"omitempty,difficulty_level"`
	CategoryID  *uint                   `json:"category_id"`
	Tags        []string                `json:"tags" validate:"omitempty,max=10,dive,max=50"`
	Explanation *string                 `json:"explanation" validate:"omitempty,max=1000"`
}
