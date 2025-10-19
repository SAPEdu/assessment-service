package validator

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/go-playground/validator/v10"
)

// BusinessValidator handles business rule validation
type BusinessValidator struct {
	validate *validator.Validate
}

// NewBusinessValidator creates a new business validator
func NewBusinessValidator() *BusinessValidator {
	validate := validator.New()

	bv := &BusinessValidator{validate: validate}
	bv.registerBusinessRules()

	return bv
}

// Validate validates business rules for any struct
func (bv *BusinessValidator) Validate(s interface{}) ValidationErrors {
	err := bv.validate.Struct(s)
	if err != nil {
		return ToValidationErrors(err)
	}
	return nil
}

// ValidateAssessmentCreate validates assessment creation business rules
func (bv *BusinessValidator) ValidateAssessmentCreate(req *AssessmentCreateRequest) ValidationErrors {
	var errors ValidationErrors

	// Basic struct validation
	errors = append(errors, bv.Validate(req)...)

	// Additional business validations
	errors = append(errors, bv.validateAssessmentBusinessRules(req)...)

	return errors
}

// ValidateAssessmentUpdate validates assessment update business rules
func (bv *BusinessValidator) ValidateAssessmentUpdate(req *AssessmentUpdateRequest, existing *models.Assessment) ValidationErrors {
	var errors ValidationErrors

	// Basic struct validation
	errors = append(errors, bv.Validate(req)...)

	// Update-specific business validations
	errors = append(errors, bv.validateAssessmentUpdateRules(req, existing)...)

	return errors
}

// ValidateQuestionCreate validates question creation business rules
func (bv *BusinessValidator) ValidateQuestionCreate(req *QuestionCreateRequest) ValidationErrors {
	var errors ValidationErrors

	// Basic struct validation
	errors = append(errors, bv.Validate(req)...)

	// Question-specific business validations
	errors = append(errors, bv.validateQuestionBusinessRules(req)...)

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

	// Define allowed transitions
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

	// Cannot delete if has attempts
	if hasAttempts {
		errors = append(errors, ValidationError{
			Field:   "attempts",
			Message: "cannot delete assessment with existing attempts",
			Value:   hasAttempts,
			Rule:    "QT-005",
		})
	}

	// Cannot delete active assessments
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

// registerBusinessRules registers custom business rule validators
func (bv *BusinessValidator) registerBusinessRules() {
	// Assessment duration validation (5-300 minutes)
	bv.validate.RegisterValidation("assessment_duration", func(fl validator.FieldLevel) bool {
		duration := fl.Field().Int()
		return duration >= 5 && duration <= 300
	})

	// Passing score validation (0-100)
	bv.validate.RegisterValidation("passing_score", func(fl validator.FieldLevel) bool {
		score := fl.Field().Int()
		return score >= 0 && score <= 100
	})

	// Max attempts validation (1-10)
	bv.validate.RegisterValidation("max_attempts", func(fl validator.FieldLevel) bool {
		attempts := fl.Field().Int()
		return attempts >= 1 && attempts <= 10
	})

	// Title validation (1-200 characters)
	bv.validate.RegisterValidation("assessment_title", func(fl validator.FieldLevel) bool {
		title := strings.TrimSpace(fl.Field().String())
		return len(title) >= 1 && len(title) <= 200
	})

	// Description validation (max 1000 characters)
	bv.validate.RegisterValidation("assessment_description", func(fl validator.FieldLevel) bool {
		desc := fl.Field().String()
		return len(desc) <= 1000
	})

	// Due date validation (must be in future)
	bv.validate.RegisterValidation("future_date", func(fl validator.FieldLevel) bool {
		field := fl.Field()

		// Check if field can be nil and is nil (for pointer types)
		if field.Kind() == reflect.Ptr && field.IsNil() {
			return true // Optional field
		}

		// Handle both *time.Time and time.Time
		var dueDate time.Time
		if field.Kind() == reflect.Ptr {
			// Dereference pointer to get the actual time.Time value
			dueDate = field.Elem().Interface().(time.Time)
		} else {
			// Direct time.Time value
			dueDate = field.Interface().(time.Time)
		}

		return dueDate.After(time.Now())
	})

	// Points range validation
	bv.validate.RegisterValidation("points_range", func(fl validator.FieldLevel) bool {
		points := fl.Field().Int()
		return points >= 1 && points <= 100
	})

	// Time limit validation - DEPRECATED: TimeLimit field not used in timing logic, kept for backward compatibility
	bv.validate.RegisterValidation("time_limit", func(fl validator.FieldLevel) bool {
		timeLimit := fl.Field().Int()
		return timeLimit >= 5 && timeLimit <= 3600
	})

	// question type validation
	bv.validate.RegisterValidation("question_type", func(fl validator.FieldLevel) bool {
		qType := fl.Field().String()
		validTypes := []models.QuestionType{models.TrueFalse, models.MultipleChoice, models.Essay, models.Matching, models.Ordering, models.ShortAnswer}
		for _, vt := range validTypes {
			if models.QuestionType(qType) == vt {
				return true
			}
		}
		return false
	})

	// difficulty level validation
	bv.validate.RegisterValidation("difficulty_level", func(fl validator.FieldLevel) bool {
		level := fl.Field().String()
		validLevels := []models.DifficultyLevel{models.DifficultyEasy, models.DifficultyMedium, models.DifficultyHard}
		for _, vl := range validLevels {
			if models.DifficultyLevel(level) == vl {
				return true
			}
		}
		return false
	})
}

// validateAssessmentBusinessRules validates business rules for assessment creation
func (bv *BusinessValidator) validateAssessmentBusinessRules(req *AssessmentCreateRequest) ValidationErrors {
	var errors ValidationErrors

	// Due date validation
	if req.DueDate != nil && req.DueDate.Before(time.Now()) {
		errors = append(errors, ValidationError{
			Field:   "due_date",
			Message: "must be in the future",
			Value:   req.DueDate,
			Rule:    "QT-009",
		})
	}

	// Max attempts vs retake consistency
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

	// Due date validation
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
