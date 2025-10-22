package utils

import (
	_ "context"
	"fmt"
	"strings"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/go-playground/validator/v10"
)

// BusinessValidator implements business rule validation based on docs.txt requirements
type BusinessValidator struct {
	validate *validator.Validate
}

// ValidationError represents a business validation error
type ValidationError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Value   interface{} `json:"value,omitempty"`
	Rule    string      `json:"rule,omitempty"`
}

type ValidationErrors []ValidationError

func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return "validation failed"
	}
	if len(ve) == 1 {
		return fmt.Sprintf("validation failed: %s %s", ve[0].Field, ve[0].Message)
	}
	return fmt.Sprintf("validation failed: %d field errors", len(ve))
}

// NewBusinessValidator creates a new business validator
func NewBusinessValidator() *BusinessValidator {
	validate := validator.New()
	// RegisterCustomValidators(validate)

	bv := &BusinessValidator{validate: validate}
	bv.registerBusinessRules()

	return bv
}

// Validate validates a struct against business rules
func (bv *BusinessValidator) Validate(s interface{}) ValidationErrors {
	var errors ValidationErrors

	err := bv.validate.Struct(s)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			errors = append(errors, ValidationError{
				Field:   err.Field(),
				Message: bv.getErrorMessage(err),
				Value:   err.Value(),
				Rule:    err.Tag(),
			})
		}
	}

	return errors
}

// ValidateAssessmentCreate validates assessment creation according to business rules
func (bv *BusinessValidator) ValidateAssessmentCreate(req interface{}) ValidationErrors {
	var errors ValidationErrors

	// Basic struct validation
	errors = append(errors, bv.Validate(req)...)

	// Business rule validations based on docs.txt
	if createReq, ok := req.(*AssessmentCreateRequest); ok {
		errors = append(errors, bv.validateAssessmentBusinessRules(createReq)...)
	}

	return errors
}

// ValidateAssessmentUpdate validates assessment update according to business rules
func (bv *BusinessValidator) ValidateAssessmentUpdate(req interface{}, existing *models.Assessment) ValidationErrors {
	var errors ValidationErrors

	// Basic struct validation
	errors = append(errors, bv.Validate(req)...)

	// Business rule validations for updates
	if updateReq, ok := req.(*AssessmentUpdateRequest); ok {
		errors = append(errors, bv.validateAssessmentUpdateRules(updateReq, existing)...)
	}

	return errors
}

// registerBusinessRules registers custom business rule validators
func (bv *BusinessValidator) registerBusinessRules() {
	// QT-007: Duration validation (5-180 minutes)
	bv.validate.RegisterValidation("assessment_duration", func(fl validator.FieldLevel) bool {
		duration := fl.Field().Int()
		return duration >= 5 && duration <= 180
	})

	// QT-008: Passing score validation (0-100)
	bv.validate.RegisterValidation("passing_score", func(fl validator.FieldLevel) bool {
		score := fl.Field().Int()
		return score >= 0 && score <= 100
	})

	// QT-005: Max attempts validation (1-10)
	bv.validate.RegisterValidation("max_attempts", func(fl validator.FieldLevel) bool {
		attempts := fl.Field().Int()
		return attempts >= 1 && attempts <= 10
	})

	// XT-001: Title validation (1-200 characters)
	bv.validate.RegisterValidation("assessment_title", func(fl validator.FieldLevel) bool {
		title := strings.TrimSpace(fl.Field().String())
		return len(title) >= 1 && len(title) <= 200
	})

	// XT-002: Description validation (max 1000 characters)
	bv.validate.RegisterValidation("assessment_description", func(fl validator.FieldLevel) bool {
		desc := fl.Field().String()
		return len(desc) <= 1000
	})

	// QT-009: Due date validation (must be in future)
	bv.validate.RegisterValidation("future_date", func(fl validator.FieldLevel) bool {
		if fl.Field().IsNil() {
			return true // Optional field
		}

		var dueDate time.Time
		dueDate = fl.Field().Interface().(time.Time)
		return dueDate.After(time.Now())
	})
}

// validateAssessmentBusinessRules validates business rules for assessment creation
func (bv *BusinessValidator) validateAssessmentBusinessRules(req *AssessmentCreateRequest) ValidationErrors {
	var errors ValidationErrors

	// QT-006: Title uniqueness (handled at service layer with repository check)
	// QT-009: Due date validation
	if req.DueDate != nil && req.DueDate.Before(time.Now()) {
		errors = append(errors, ValidationError{
			Field:   "due_date",
			Message: "must be in the future",
			Value:   req.DueDate,
			Rule:    "QT-009",
		})
	}

	// Additional business logic validations
	if req.MaxAttempts > 0 && req.Settings != nil && req.Settings.AllowRetake != nil && !*req.Settings.AllowRetake {
		if req.MaxAttempts > 1 {
			errors = append(errors, ValidationError{
				Field:   "max_attempts",
				Message: "cannot be greater than 1 when retakes are not allowed",
				Value:   req.MaxAttempts,
				Rule:    "business_logic",
			})
		}
	}

	return errors
}

// validateAssessmentUpdateRules validates business rules for assessment updates
func (bv *BusinessValidator) validateAssessmentUpdateRules(req *AssessmentUpdateRequest, existing *models.Assessment) ValidationErrors {
	var errors ValidationErrors

	// Business rule: Cannot change certain fields if assessment has attempts (handled at service layer)
	// QT-009: Due date validation
	if req.DueDate != nil && req.DueDate.Before(time.Now()) {
		errors = append(errors, ValidationError{
			Field:   "due_date",
			Message: "must be in the future",
			Value:   req.DueDate,
			Rule:    "QT-009",
		})
	}

	// Status-based validations
	if existing.Status == models.StatusActive {
		// Limited changes allowed for active assessments
		if req.PassingScore != nil && *req.PassingScore != existing.PassingScore {
			errors = append(errors, ValidationError{
				Field:   "passing_score",
				Message: "cannot be changed for active assessments",
				Value:   *req.PassingScore,
				Rule:    "business_logic",
			})
		}
	}

	return errors
}

// getErrorMessage returns user-friendly error messages
func (bv *BusinessValidator) getErrorMessage(err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return "is required"
	case "min":
		return fmt.Sprintf("must be at least %s", err.Param())
	case "max":
		return fmt.Sprintf("must be at most %s", err.Param())
	case "assessment_duration":
		return "must be between 5 and 180 minutes"
	case "passing_score":
		return "must be between 0 and 100"
	case "max_attempts":
		return "must be between 1 and 10"
	case "assessment_title":
		return "must be between 1 and 200 characters"
	case "assessment_description":
		return "must not exceed 1000 characters"
	case "future_date":
		return "must be in the future"
	case "question_type":
		return "must be a valid question type"
	case "difficulty_level":
		return "must be Easy, Medium, or Hard"
	case "user_role":
		return "must be a valid user role"
	default:
		return fmt.Sprintf("validation failed for rule '%s'", err.Tag())
	}
}

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
	QuestionsPerPage            *int  `json:"questions_per_page" validate:"omitempty,min=1,max=50"`
	ShowProgressBar             *bool `json:"show_progress_bar"`
	ShowResults                 *bool `json:"show_results"`
	ShowCorrectAnswers          *bool `json:"show_correct_answers"`
	ShowScoreBreakdown          *bool `json:"show_score_breakdown"`
	AllowRetake                 *bool `json:"allow_retake"`
	RetakeDelay                 *int  `json:"retake_delay" validate:"omitempty,min=0,max=1440"`
	TimeLimitEnforced           *bool `json:"time_limit_enforced"`
	AutoSubmitOnTimeout         *bool `json:"auto_submit_on_timeout"`
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

// ValidateQuestionCreate validates question creation
func (bv *BusinessValidator) ValidateQuestionCreate(req interface{}) ValidationErrors {
	var errors ValidationErrors

	// Basic struct validation
	errors = append(errors, bv.Validate(req)...)

	// Question-specific business rules
	if createReq, ok := req.(*QuestionCreateRequest); ok {
		errors = append(errors, bv.validateQuestionBusinessRules(createReq)...)
	}

	return errors
}

// QuestionCreateRequest represents the request structure for creating questions
type QuestionCreateRequest struct {
	Type        models.QuestionType    `json:"type" validate:"required,question_type"`
	Text        string                 `json:"text" validate:"required,min=1,max=2000"`
	Content     interface{}            `json:"content" validate:"required"`
	Points      int                    `json:"points" validate:"required,min=1,max=100"`
	TimeLimit   *int                   `json:"time_limit" validate:"omitempty,min=5,max=3600"`
	Difficulty  models.DifficultyLevel `json:"difficulty" validate:"required,difficulty_level"`
	CategoryID  *uint                  `json:"category_id"`
	Tags        []string               `json:"tags" validate:"omitempty,max=10,dive,max=50"`
	Explanation *string                `json:"explanation" validate:"omitempty,max=1000"`
}

// validateQuestionBusinessRules validates business rules for question creation
func (bv *BusinessValidator) validateQuestionBusinessRules(req *QuestionCreateRequest) ValidationErrors {
	var errors ValidationErrors

	// Validate tags
	if len(req.Tags) > 10 {
		errors = append(errors, ValidationError{
			Field:   "tags",
			Message: "cannot have more than 10 tags",
			Value:   len(req.Tags),
			Rule:    "business_logic",
		})
	}

	for i, tag := range req.Tags {
		if strings.TrimSpace(tag) == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("tags[%d]", i),
				Message: "tag cannot be empty",
				Value:   tag,
				Rule:    "business_logic",
			})
		}
	}

	return errors
}

// ValidateAttemptStart validates attempt start conditions
func (bv *BusinessValidator) ValidateAttemptStart(assessmentStatus models.AssessmentStatus, dueDate *time.Time, attemptCount int, maxAttempts int) ValidationErrors {
	var errors ValidationErrors

	// Assessment must be active
	if assessmentStatus != models.StatusActive {
		errors = append(errors, ValidationError{
			Field:   "assessment_status",
			Message: "assessment is not active",
			Value:   assessmentStatus,
			Rule:    "business_logic",
		})
	}

	// Check due date
	if dueDate != nil && time.Now().After(*dueDate) {
		errors = append(errors, ValidationError{
			Field:   "due_date",
			Message: "assessment has expired",
			Value:   dueDate,
			Rule:    "business_logic",
		})
	}

	// Check attempt limits
	if attemptCount >= maxAttempts {
		errors = append(errors, ValidationError{
			Field:   "attempts",
			Message: "maximum attempts exceeded",
			Value:   attemptCount,
			Rule:    "business_logic",
		})
	}

	return errors
}

// ValidateStatusTransition validates assessment status transitions
func (bv *BusinessValidator) ValidateStatusTransition(currentStatus, newStatus models.AssessmentStatus, hasAttempts bool, questionCount int) ValidationErrors {
	var errors ValidationErrors

	// Define allowed transitions based on docs.txt state diagram
	allowedTransitions := map[models.AssessmentStatus][]models.AssessmentStatus{
		models.StatusDraft:    {models.StatusActive, models.StatusArchived},
		models.StatusActive:   {models.StatusExpired, models.StatusArchived},
		models.StatusExpired:  {models.StatusActive, models.StatusArchived},
		models.StatusArchived: {}, // No transitions from archived
	}

	allowed := false
	for _, allowedStatus := range allowedTransitions[currentStatus] {
		if newStatus == allowedStatus {
			allowed = true
			break
		}
	}

	if !allowed {
		errors = append(errors, ValidationError{
			Field:   "status",
			Message: fmt.Sprintf("cannot transition from %s to %s", currentStatus, newStatus),
			Value:   newStatus,
			Rule:    "status_transition",
		})
	}

	// Additional validation for publishing (Draft -> Active)
	if newStatus == models.StatusActive && questionCount == 0 {
		errors = append(errors, ValidationError{
			Field:   "questions",
			Message: "assessment must have at least one question before publishing",
			Value:   questionCount,
			Rule:    "business_logic",
		})
	}

	return errors
}

// ValidateDeletePermission validates if an assessment can be deleted
func (bv *BusinessValidator) ValidateDeletePermission(hasAttempts bool, status models.AssessmentStatus) ValidationErrors {
	var errors ValidationErrors

	// QT-005: Can only delete assessment when no attempts exist
	if hasAttempts {
		errors = append(errors, ValidationError{
			Field:   "attempts",
			Message: "cannot delete assessment with existing attempts",
			Value:   hasAttempts,
			Rule:    "QT-005",
		})
	}

	// Cannot delete active assessments with students taking them
	if status == models.StatusActive {
		errors = append(errors, ValidationError{
			Field:   "status",
			Message: "cannot delete active assessment",
			Value:   status,
			Rule:    "business_logic",
		})
	}

	return errors
}

// AssessmentQuestionRequest represents adding questions to assessments
type AssessmentQuestionRequest struct {
	QuestionID uint `json:"question_id" validate:"required"`
	Order      int  `json:"order" validate:"required,min=1"`
	Points     *int `json:"points" validate:"omitempty,min=1,max=100"`
}
