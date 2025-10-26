package services

import (
	"context"
	"fmt"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"gorm.io/gorm"
)

// ===== PERMISSION CHECKS =====

func (s *assessmentService) CanAccess(ctx context.Context, assessmentID uint, userID string) (bool, error) {
	// Get user role
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return false, err
	}

	// Admin can access all assessments
	if userRole == models.RoleAdmin {
		return true, nil
	}

	// Get assessment to check ownership
	assessment, err := s.repo.Assessment().GetByID(ctx, s.db, assessmentID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return false, nil
		}
		return false, err
	}

	// Teachers can access their own assessments
	if userRole == models.RoleTeacher && assessment.CreatedBy == userID {
		return true, nil
	}

	// Students can access active assessments they're enrolled in
	if userRole == models.RoleStudent && assessment.Status == models.StatusActive {
		// TODO: Check if student is enrolled in assessment/course
		// For now, allow all students to access active assessments
		return true, nil
	}

	return false, nil
}

func (s *assessmentService) CanEdit(ctx context.Context, assessmentID uint, userID string) (bool, error) {
	// Get user role
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return false, err
	}

	// Get assessment
	assessment, err := s.repo.Assessment().GetByID(ctx, s.db, assessmentID)
	if err != nil {
		return false, err
	}

	// Admin can edit all assessments
	if userRole == models.RoleAdmin {
		return true, nil
	}

	// Only owners can edit their assessments
	if assessment.CreatedBy != userID {
		return false, nil
	}

	// Teachers can edit their own assessments in Draft status
	if userRole == models.RoleTeacher && assessment.Status == models.StatusDraft {
		return true, nil
	}

	// Limited editing allowed for Active assessments (e.g., extend due date)
	if userRole == models.RoleTeacher && assessment.Status == models.StatusActive {
		return true, nil // Allow limited edits
	}

	return false, nil
}

func (s *assessmentService) CanDelete(ctx context.Context, assessmentID uint, userID string) (bool, error) {
	// Get user role
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return false, err
	}

	// Get assessment
	assessment, err := s.repo.Assessment().GetByID(ctx, s.db, assessmentID)
	if err != nil {
		return false, err
	}

	// Only owners or admins can delete
	if userRole != models.RoleAdmin && assessment.CreatedBy != userID {
		return false, nil
	}

	// Check if assessment has attempts
	hasAttempts, err := s.repo.Assessment().HasAttempts(ctx, s.db, assessmentID)
	if err != nil {
		return false, err
	}

	// Cannot delete if has attempts (except admin override)
	if hasAttempts && userRole != models.RoleAdmin {
		return false, nil
	}

	return true, nil
}

func (s *assessmentService) CanTake(ctx context.Context, assessmentID uint, userID string) (bool, error) {
	// Get user role
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return false, err
	}

	// Only students can take assessments
	if userRole != models.RoleStudent {
		return false, nil
	}

	// Get assessment
	assessment, err := s.repo.Assessment().GetByIDWithDetails(ctx, s.db, assessmentID)
	if err != nil {
		return false, err
	}

	// Assessment must be active
	if assessment.Status != models.StatusActive {
		return false, nil
	}

	// Check if not expired
	if assessment.DueDate != nil && time.Now().After(*assessment.DueDate) {
		return false, nil
	}

	// Check attempt limits
	attemptCount, err := s.repo.Attempt().GetAttemptCount(ctx, s.db, userID, assessmentID)
	if err != nil {
		return false, err
	}

	if attemptCount >= assessment.MaxAttempts {
		return false, nil
	}

	// TODO: Check enrollment/assignment status
	// For now, allow all students to take active assessments

	return true, nil
}

// ===== HELPER FUNCTIONS =====

func (s *assessmentService) getUserRole(ctx context.Context, userID string) (models.UserRole, error) {
	user, err := s.repo.User().GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %w", err)
	}
	return user.Role, nil
}

func (s *assessmentService) canCreateAssessment(ctx context.Context, userID string) (bool, error) {
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return false, err
	}

	return userRole == models.RoleTeacher || userRole == models.RoleAdmin, nil
}

func (s *assessmentService) buildAssessmentResponse(ctx context.Context, assessment *models.Assessment, userID string) *AssessmentResponse {
	response := &AssessmentResponse{
		Assessment: assessment,
	}

	// Determine permissions
	canEdit, _ := s.CanEdit(ctx, assessment.ID, userID)
	canDelete, _ := s.CanDelete(ctx, assessment.ID, userID)
	canTake, _ := s.CanTake(ctx, assessment.ID, userID)

	response.CanEdit = canEdit
	response.CanDelete = canDelete
	response.CanTake = canTake

	return response
}

func (s *assessmentService) buildAssessmentSettings(assessmentID uint, req *AssessmentSettingsRequest) *models.AssessmentSettings {
	settings := &models.AssessmentSettings{
		AssessmentID: assessmentID,
		// Set defaults
		RandomizeQuestions:          false,
		RandomizeOptions:            false,
		ShowProgressBar:             true,
		RequireWebcam:               false,
		PreventTabSwitching:         false,
		PreventRightClick:           false,
		PreventCopyPaste:            false,
		RequireIdentityVerification: false,
		RequireFullScreen:           false,
		AllowScreenReader:           false,
		FontSizeAdjustment:          0,
		HighContrastMode:            false,
	}

	// Apply provided settings
	if req != nil {
		s.applySettingsUpdates(settings, req)
	}

	return settings
}

func (s *assessmentService) applyAssessmentUpdates(assessment *models.Assessment, req *UpdateAssessmentRequest) {
	if req.Title != nil {
		assessment.Title = *req.Title
	}
	if req.Description != nil {
		assessment.Description = req.Description
	}
	if req.Duration != nil {
		assessment.Duration = *req.Duration
	}
	if req.PassingScore != nil {
		assessment.PassingScore = *req.PassingScore
	}
	if req.MaxAttempts != nil {
		assessment.MaxAttempts = *req.MaxAttempts
	}
	if req.TimeWarning != nil {
		assessment.TimeWarning = *req.TimeWarning
	}
	if req.DueDate != nil {
		assessment.DueDate = req.DueDate
	}

	assessment.Version += 1
	assessment.UpdatedAt = time.Now()
}

func (s *assessmentService) applySettingsUpdates(settings *models.AssessmentSettings, req *AssessmentSettingsRequest) {
	if req.RandomizeQuestions != nil {
		settings.RandomizeQuestions = *req.RandomizeQuestions
	}
	if req.RandomizeOptions != nil {
		settings.RandomizeOptions = *req.RandomizeOptions
	}
	if req.ShowProgressBar != nil {
		settings.ShowProgressBar = *req.ShowProgressBar
	}
	if req.RequireWebcam != nil {
		settings.RequireWebcam = *req.RequireWebcam
	}
	if req.PreventTabSwitching != nil {
		settings.PreventTabSwitching = *req.PreventTabSwitching
	}
	if req.PreventRightClick != nil {
		settings.PreventRightClick = *req.PreventRightClick
	}
	if req.PreventCopyPaste != nil {
		settings.PreventCopyPaste = *req.PreventCopyPaste
	}
	if req.RequireIdentityVerification != nil {
		settings.RequireIdentityVerification = *req.RequireIdentityVerification
	}
	if req.RequireFullScreen != nil {
		settings.RequireFullScreen = *req.RequireFullScreen
	}
	if req.AllowScreenReader != nil {
		settings.AllowScreenReader = *req.AllowScreenReader
	}
	if req.FontSizeAdjustment != nil {
		settings.FontSizeAdjustment = *req.FontSizeAdjustment
	}
	if req.HighContrastMode != nil {
		settings.HighContrastMode = *req.HighContrastMode
	}
}

func (s *assessmentService) addQuestionsToAssessment(ctx context.Context, tx *gorm.DB, assessmentID uint, questions []AssessmentQuestionRequest, userID string) error {
	for _, qReq := range questions {
		// Add question to assessment
		if err := s.repo.AssessmentQuestion().AddQuestion(ctx, tx, assessmentID, qReq.QuestionID, qReq.Order, &qReq.Points); err != nil {
			return fmt.Errorf("failed to add question %d to assessment: %w", qReq.QuestionID, err)
		}
	}

	return nil
}

// ===== VALIDATION FUNCTIONS =====

func (s *assessmentService) validateCreateRequest(ctx context.Context, req *CreateAssessmentRequest, creatorID string) error {
	var errors ValidationErrors

	// Check title uniqueness
	existing, err := s.repo.Assessment().ExistsByTitle(ctx, s.db, req.Title, creatorID, nil)
	if err != nil && !repositories.IsNotFoundError(err) {
		return fmt.Errorf("failed to check title uniqueness: %w", err)
	}
	if existing {
		errors = append(errors, *NewValidationError("title", "already exists", req.Title))
	}

	// Validate due date
	if req.DueDate != nil && req.DueDate.Before(time.Now()) {
		errors = append(errors, *NewValidationError("due_date", "must be in the future", req.DueDate))
	}

	// Validate questions if provided
	if len(req.Questions) > 0 {
		orderMap := make(map[int]bool)
		totalPoints := 0

		for i, q := range req.Questions {
			// Check for duplicate orders
			if orderMap[q.Order] {
				errors = append(errors, *NewValidationError(fmt.Sprintf("questions[%d].order", i), "duplicate order", q.Order))
			}
			orderMap[q.Order] = true

			// Validate question exists
			_, err := s.repo.Question().GetByID(ctx, nil, q.QuestionID)
			if err != nil {
				if repositories.IsNotFoundError(err) {
					errors = append(errors, *NewValidationError(fmt.Sprintf("questions[%d].question_id", i), "question not found", q.QuestionID))
				} else {
					return fmt.Errorf("failed to validate question %d: %w", q.QuestionID, err)
				}
			}

			// Accumulate points for total validation
			totalPoints += q.Points
		}

		// Validate total points does not exceed 100
		if totalPoints > 100 {
			errors = append(errors, *NewValidationError("questions",
				fmt.Sprintf("total points (%d) exceeds maximum allowed (100)", totalPoints), totalPoints))
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

func (s *assessmentService) validateUpdateRequest(ctx context.Context, req *UpdateAssessmentRequest, assessment *models.Assessment, userID string) error {
	var errors ValidationErrors

	// Check title uniqueness if title is being updated
	if req.Title != nil && *req.Title != assessment.Title {
		existing, err := s.repo.Assessment().ExistsByTitle(ctx, s.db, *req.Title, assessment.CreatedBy, &assessment.ID)
		if err != nil && !repositories.IsNotFoundError(err) {
			return fmt.Errorf("failed to check title uniqueness: %w", err)
		}
		if existing {
			errors = append(errors, *NewValidationError("title", "already exists", *req.Title))
		}
	}

	// Validate due date
	if req.DueDate != nil && req.DueDate.Before(time.Now()) {
		errors = append(errors, *NewValidationError("due_date", "must be in the future", req.DueDate))
	}

	// Business rule: Cannot change certain fields if assessment has attempts
	if assessment.Status != models.StatusDraft {
		hasAttempts, err := s.repo.Assessment().HasAttempts(ctx, s.db, assessment.ID)
		if err != nil {
			return fmt.Errorf("failed to check attempts: %w", err)
		}

		if hasAttempts {
			if req.Duration != nil && *req.Duration != assessment.Duration {
				errors = append(errors, *NewValidationError("duration", "cannot change duration after students have started", *req.Duration))
			}
			if req.PassingScore != nil && *req.PassingScore != assessment.PassingScore {
				errors = append(errors, *NewValidationError("passing_score", "cannot change passing score after students have started", *req.PassingScore))
			}
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

func (s *assessmentService) validateStatusTransition(ctx context.Context, assessment *models.Assessment, newStatus models.AssessmentStatus) error {
	currentStatus := assessment.Status

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
		return NewBusinessRuleError(
			"QT-INVALID-STATUS-TRANSITION",
			fmt.Sprintf("Cannot transition from %s to %s", currentStatus, newStatus),
			map[string]interface{}{
				"current_status": currentStatus,
				"new_status":     newStatus,
				"assessment_id":  assessment.ID,
			},
		)
	}

	// Additional validation for specific transitions
	if newStatus == models.StatusActive {
		// Validate assessment is ready to be published
		if err := s.validateAssessmentReadyForPublish(ctx, assessment); err != nil {
			return err
		}
	}

	return nil
}

func (s *assessmentService) validateAssessmentReadyForPublish(ctx context.Context, assessment *models.Assessment) error {
	// Must have at least one question
	questionCount, err := s.repo.AssessmentQuestion().GetQuestionCount(ctx, nil, assessment.ID)
	if err != nil {
		return fmt.Errorf("failed to get question count: %w", err)
	}

	if questionCount == 0 {
		return NewBusinessRuleError(
			"QT-ASSESSMENT-NO-QUESTIONS",
			"Assessment must have at least one question before publishing",
			map[string]interface{}{
				"assessment_id": assessment.ID,
			},
		)
	}

	// Validate due date
	if assessment.DueDate != nil && assessment.DueDate.Before(time.Now()) {
		return NewBusinessRuleError(
			"QT-ASSESSMENT-EXPIRED-DUE-DATE",
			"Cannot publish assessment with due date in the past",
			map[string]interface{}{
				"assessment_id": assessment.ID,
				"due_date":      assessment.DueDate,
			},
		)
	}

	return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// withDB executes a function with the service's database instance
// Use this for non-transactional operations
func (s *assessmentService) withDB(fn func(tx *gorm.DB) error) error {
	return fn(s.db)
}

// withTx executes a function within a transaction
// Use this for operations that require transaction management
func (s *assessmentService) withTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return s.db.WithContext(ctx).Transaction(fn)
}

// ===== REPOSITORY WRAPPERS FOR NON-TRANSACTIONAL OPERATIONS =====

// getAssessmentByID is a wrapper for simple assessment retrieval
func (s *assessmentService) getAssessmentByID(ctx context.Context, id uint) (*models.Assessment, error) {
	return s.repo.Assessment().GetByID(ctx, s.db, id)
}

// getAssessmentWithDetails is a wrapper for detailed assessment retrieval
func (s *assessmentService) getAssessmentWithDetails(ctx context.Context, id uint) (*models.Assessment, error) {
	return s.repo.Assessment().GetByIDWithDetails(ctx, s.db, id)
}

// listAssessments is a wrapper for assessment listing
func (s *assessmentService) listAssessments(ctx context.Context, filters repositories.AssessmentFilters) ([]*models.Assessment, int64, error) {
	return s.repo.Assessment().List(ctx, s.db, filters)
}

// ===== POINTS VALIDATION =====

// validateTotalPoints validates that total points for assessment does not exceed 100
// pointsToAdd: points for the new question being added (use 0 if not adding)
// excludeQuestionID: question ID to exclude from total (use when updating existing question's points)
func (s *assessmentService) validateTotalPoints(ctx context.Context, tx *gorm.DB, assessmentID uint, pointsToAdd int, excludeQuestionID uint) error {
	db := s.db
	if tx != nil {
		db = tx
	}

	// Get current total points
	currentTotal, err := s.repo.AssessmentQuestion().GetTotalPoints(ctx, db, assessmentID)
	if err != nil {
		return fmt.Errorf("failed to get total points: %w", err)
	}

	// If excluding a question (when updating), subtract its current points
	if excludeQuestionID != 0 {
		assessmentQuestion, err := s.repo.AssessmentQuestion().GetQuestionAssessmentByAssessmentIdAndQuestionId(ctx, db, assessmentID, excludeQuestionID)
		if err == nil && assessmentQuestion.Points != nil {
			currentTotal -= *assessmentQuestion.Points
		}
	}

	// Calculate new total
	newTotal := currentTotal + pointsToAdd

	// Validate
	if newTotal > 100 {
		return fmt.Errorf("total points (%d) would exceed maximum allowed (100). Current total: %d, adding: %d",
			newTotal, currentTotal, pointsToAdd)
	}

	return nil
}

// calculateAutoAssignPoints calculates points distribution for auto-assigning questions
// Formula: Divide 100 points evenly among ALL questions (existing + new)
// This rebalances ALL questions in the assessment to have equal points
// Returns: points per question (all questions get same value)
func (s *assessmentService) calculateAutoAssignPoints(ctx context.Context, tx *gorm.DB, assessmentID uint, newQuestionCount int) (int, int, error) {
	db := s.db
	if tx != nil {
		db = tx
	}

	// Get current question count
	var currentQuestionCount int64
	if err := db.Model(&models.AssessmentQuestion{}).
		Where("assessment_id = ?", assessmentID).
		Count(&currentQuestionCount).Error; err != nil {
		return 0, 0, fmt.Errorf("failed to count current questions: %w", err)
	}

	// Calculate total questions (existing + new)
	totalQuestions := int(currentQuestionCount) + newQuestionCount

	// Validate we can have at least 1 point per question
	if totalQuestions > 100 {
		return 0, 0, fmt.Errorf(
			"cannot add %d questions: total would be %d questions, exceeding maximum of 100 (need minimum 1 point per question)",
			newQuestionCount, totalQuestions,
		)
	}

	if totalQuestions == 0 {
		return 0, 0, fmt.Errorf("cannot calculate points for 0 questions")
	}

	// Calculate base points and remainder for ALL questions
	// Formula: 100 / total_questions
	basePoints := 100 / totalQuestions
	remainder := 100 % totalQuestions

	return basePoints, remainder, nil
}

// validateAssessmentQuestionsEditable checks if an assessment's questions can be modified
// This prevents editing questions when assessment is Active/Expired and has attempts
func (s *assessmentService) validateAssessmentQuestionsEditable(ctx context.Context, tx *gorm.DB, assessmentID uint) error {
	db := s.db
	if tx != nil {
		db = tx
	}

	// Get assessment
	assessment, err := s.repo.Assessment().GetByID(ctx, db, assessmentID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return ErrAssessmentNotFound
		}
		return fmt.Errorf("failed to get assessment: %w", err)
	}

	// Draft assessments can always be edited
	if assessment.Status == models.StatusDraft {
		return nil
	}

	// Archived assessments cannot be edited
	if assessment.Status == models.StatusArchived {
		return NewBusinessRuleError(
			"assessment_questions_locked",
			"cannot modify questions: assessment is archived",
			map[string]interface{}{
				"assessment_id": assessmentID,
				"status":        assessment.Status,
			},
		)
	}

	// For Active/Expired assessments, check if there are any attempts
	if assessment.Status == models.StatusActive || assessment.Status == models.StatusExpired {
		hasAttempts, err := s.repo.Assessment().HasAttempts(ctx, db, assessmentID)
		if err != nil {
			return fmt.Errorf("failed to check attempts: %w", err)
		}

		if hasAttempts {
			return NewBusinessRuleError(
				"assessment_questions_locked",
				"cannot modify questions: assessment has active student attempts. Modifying questions would invalidate existing attempt scores.",
				map[string]interface{}{
					"assessment_id": assessmentID,
					"status":        assessment.Status,
					"has_attempts":  true,
				},
			)
		}
	}

	return nil
}
