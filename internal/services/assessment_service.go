package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/SAP-F-2025/assessment-service/internal/validator"
	"gorm.io/gorm"
)

type assessmentService struct {
	repo            repositories.Repository
	questionService QuestionService
	db              *gorm.DB
	logger          *slog.Logger
	validator       *validator.Validator
}

func NewAssessmentService(repo repositories.Repository, db *gorm.DB, logger *slog.Logger, validator *validator.Validator) AssessmentService {
	return &assessmentService{
		repo:            repo,
		db:              db,
		logger:          logger,
		validator:       validator,
		questionService: NewQuestionService(repo, db, logger, validator),
	}
}

// ===== CORE CRUD OPERATIONS =====

func (s *assessmentService) Create(ctx context.Context, req *CreateAssessmentRequest, creatorID string) (*AssessmentResponse, error) {
	s.logger.Info("Creating assessment", "creator_id", creatorID, "title", req.Title)

	// Validate request with business rules
	if errors := s.validator.GetBusinessValidator().ValidateAssessmentCreate(req); len(errors) > 0 {
		return nil, errors
	}

	// Check user permissions
	canCreate, err := s.canCreateAssessment(ctx, creatorID)
	if err != nil {
		return nil, fmt.Errorf("permission check failed: %w", err)
	}
	if !canCreate {
		return nil, NewPermissionError(creatorID, 0, "assessment", "create", "insufficient role permissions")
	}

	// Validate business rules
	if err := s.validateCreateRequest(ctx, req, creatorID); err != nil {
		return nil, err
	}

	// Use transaction for complex operation
	var assessment *models.Assessment
	err = s.withTx(ctx, func(tx *gorm.DB) error {
		// Create assessment
		assessment = &models.Assessment{
			Title:        req.Title,
			Description:  req.Description,
			Duration:     req.Duration,
			Status:       models.StatusDraft,
			PassingScore: req.PassingScore,
			MaxAttempts:  req.MaxAttempts,
			TimeWarning:  300, // Default 5 minutes
			DueDate:      req.DueDate,
			CreatedBy:    creatorID,
			Version:      1,
		}

		if req.TimeWarning != nil {
			assessment.TimeWarning = *req.TimeWarning
		}

		if err := s.repo.Assessment().Create(ctx, tx, assessment); err != nil {
			return fmt.Errorf("failed to create assessment: %w", err)
		}

		// Create settings
		settings := s.buildAssessmentSettings(assessment.ID, req.Settings)
		if err := s.repo.AssessmentSettings().Create(ctx, tx, settings); err != nil {
			return fmt.Errorf("failed to create assessment settings: %w", err)
		}

		// Add questions if provided
		if len(req.Questions) > 0 {
			if err := s.addQuestionsToAssessment(ctx, tx, assessment.ID, req.Questions, creatorID); err != nil {
				return fmt.Errorf("failed to add questions: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Info("Assessment created successfully", "assessment_id", assessment.ID)

	// Return response
	return s.GetByIDWithDetails(ctx, assessment.ID, creatorID)
}

func (s *assessmentService) GetByID(ctx context.Context, id uint, userID string) (*AssessmentResponse, error) {
	// Check access permission
	canAccess, err := s.CanAccess(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, id, "assessment", "read", "not owner or insufficient permissions")
	}

	// Get assessment using wrapper
	assessment, err := s.getAssessmentByID(ctx, id)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrAssessmentNotFound
		}
		return nil, fmt.Errorf("failed to get assessment: %w", err)
	}

	return s.buildAssessmentResponse(ctx, assessment, userID), nil
}

func (s *assessmentService) GetByIDWithDetails(ctx context.Context, id uint, userID string) (*AssessmentResponse, error) {
	// Check access permission
	canAccess, err := s.CanAccess(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, id, "assessment", "read", "not owner or insufficient permissions")
	}

	// Get assessment with details using wrapper
	assessment, err := s.getAssessmentWithDetails(ctx, id)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrAssessmentNotFound
		}
		return nil, fmt.Errorf("failed to get assessment with details: %w", err)
	}

	return s.buildAssessmentResponse(ctx, assessment, userID), nil
}

func (s *assessmentService) Update(ctx context.Context, id uint, req *UpdateAssessmentRequest, userID string) (*AssessmentResponse, error) {
	s.logger.Info("Updating assessment", "assessment_id", id, "user_id", userID)

	// Get current assessment for validation
	assessment, err := s.repo.Assessment().GetByID(ctx, s.db, id)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrAssessmentNotFound
		}
		return nil, fmt.Errorf("failed to get assessment: %w", err)
	}

	// Validate request with business rules
	if errors := s.validator.GetBusinessValidator().ValidateAssessmentUpdate(req, assessment); len(errors) > 0 {
		return nil, errors
	}

	// Check edit permission
	canEdit, err := s.CanEdit(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if !canEdit {
		return nil, NewPermissionError(userID, id, "assessment", "update", "not owner or assessment not editable")
	}

	// Validate business rules for update
	if err := s.validateUpdateRequest(ctx, req, assessment, userID); err != nil {
		return nil, err
	}

	// Begin transaction at service layer
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Apply updates
		s.applyAssessmentUpdates(assessment, req)

		// Update assessment
		if err := s.repo.Assessment().Update(ctx, tx, assessment); err != nil {
			return fmt.Errorf("failed to update assessment: %w", err)
		}

		// Update settings if provided
		if req.Settings != nil {
			settings, err := s.repo.AssessmentSettings().GetByAssessmentID(ctx, tx, id)
			if err != nil {
				return fmt.Errorf("failed to get assessment settings: %w", err)
			}

			s.applySettingsUpdates(settings, req.Settings)

			if err := s.repo.AssessmentSettings().Update(ctx, tx, settings); err != nil {
				return fmt.Errorf("failed to update assessment settings: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Info("Assessment updated successfully", "assessment_id", id)

	// Return updated assessment
	return s.GetByIDWithDetails(ctx, id, userID)
}

func (s *assessmentService) Delete(ctx context.Context, id uint, userID string) error {
	s.logger.Info("Deleting assessment", "assessment_id", id, "user_id", userID)

	// Check delete permission
	canDelete, err := s.CanDelete(ctx, id, userID)
	if err != nil {
		return err
	}
	if !canDelete {
		return NewPermissionError(userID, id, "assessment", "delete", "not owner or assessment has attempts")
	}

	// Soft delete
	if err := s.repo.Assessment().Delete(ctx, s.db, id); err != nil {
		return fmt.Errorf("failed to delete assessment: %w", err)
	}

	s.logger.Info("Assessment deleted successfully", "assessment_id", id)
	return nil
}

// ===== LIST AND SEARCH OPERATIONS =====

func (s *assessmentService) List(ctx context.Context, filters repositories.AssessmentFilters, userID string) (*AssessmentListResponse, error) {
	// Filter based on user role
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Apply role-based filtering
	switch userRole {
	case models.RoleStudent:
		// Students: only Active assessments that haven't expired
		activeStatus := models.StatusActive
		filters.Status = &activeStatus

	case models.RoleTeacher:
		// Teachers: only their own assessments
		filters.CreatedBy = &userID

	case models.RoleAdmin:
		// Admins: no additional filtering (can see all)

	default:
		// Unknown role: no access
		return &AssessmentListResponse{
			Assessments: []*AssessmentResponse{},
			Total:       0,
			Page:        1,
			Size:        filters.Limit,
		}, nil
	}

	assessments, total, err := s.repo.Assessment().List(ctx, s.db, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list assessments: %w", err)
	}

	// For students, filter out expired assessments (where due_date has passed)
	if userRole == models.RoleStudent {
		now := time.Now()
		filteredAssessments := make([]*models.Assessment, 0, len(assessments))
		for _, assessment := range assessments {
			// Include if no due_date or due_date is in the future
			if assessment.DueDate == nil || assessment.DueDate.After(now) {
				filteredAssessments = append(filteredAssessments, assessment)
			}
		}
		assessments = filteredAssessments
		total = int64(len(filteredAssessments))
	}

	// Build response
	response := &AssessmentListResponse{
		Assessments: make([]*AssessmentResponse, len(assessments)),
		Total:       total,
		Page:        (filters.Offset / max(filters.Limit, 1)) + 1,
		Size:        filters.Limit,
	}

	for i, assessment := range assessments {
		response.Assessments[i] = s.buildAssessmentResponse(ctx, assessment, userID)
	}

	return response, nil
}

func (s *assessmentService) GetByCreator(ctx context.Context, creatorID string, filters repositories.AssessmentFilters) (*AssessmentListResponse, error) {
	// Set creator filter
	filters.CreatedBy = &creatorID

	assessments, total, err := s.repo.Assessment().GetByCreator(ctx, s.db, creatorID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get assessments by creator: %w", err)
	}

	// Build response
	response := &AssessmentListResponse{
		Assessments: make([]*AssessmentResponse, len(assessments)),
		Total:       total,
		Page:        (filters.Offset / max(filters.Limit, 1)) + 1,
		Size:        filters.Limit,
	}

	for i, assessment := range assessments {
		response.Assessments[i] = s.buildAssessmentResponse(ctx, assessment, creatorID)
	}

	return response, nil
}

func (s *assessmentService) Search(ctx context.Context, query string, filters repositories.AssessmentFilters, userID string) (*AssessmentListResponse, error) {
	// Filter based on user role
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Apply role-based filtering (same as List)
	switch userRole {
	case models.RoleStudent:
		// Students: only Active assessments that haven't expired
		activeStatus := models.StatusActive
		filters.Status = &activeStatus

	case models.RoleTeacher:
		// Teachers: only their own assessments
		filters.CreatedBy = &userID

	case models.RoleAdmin:
		// Admins: no additional filtering (can see all)

	default:
		// Unknown role: no access
		return &AssessmentListResponse{
			Assessments: []*AssessmentResponse{},
			Total:       0,
			Page:        1,
			Size:        filters.Limit,
		}, nil
	}

	assessments, total, err := s.repo.Assessment().Search(ctx, nil, query, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to search assessments: %w", err)
	}

	// For students, filter out expired assessments (where due_date has passed)
	if userRole == models.RoleStudent {
		now := time.Now()
		filteredAssessments := make([]*models.Assessment, 0, len(assessments))
		for _, assessment := range assessments {
			// Include if no due_date or due_date is in the future
			if assessment.DueDate == nil || assessment.DueDate.After(now) {
				filteredAssessments = append(filteredAssessments, assessment)
			}
		}
		assessments = filteredAssessments
		total = int64(len(filteredAssessments))
	}

	// Build response
	response := &AssessmentListResponse{
		Assessments: make([]*AssessmentResponse, len(assessments)),
		Total:       total,
		Page:        (filters.Offset / max(filters.Limit, 1)) + 1,
		Size:        filters.Limit,
	}

	for i, assessment := range assessments {
		response.Assessments[i] = s.buildAssessmentResponse(ctx, assessment, userID)
	}

	return response, nil
}

// ===== STATUS MANAGEMENT =====

func (s *assessmentService) UpdateStatus(ctx context.Context, id uint, req *UpdateStatusRequest, userID string) error {
	s.logger.Info("Updating assessment status", "assessment_id", id, "new_status", req.Status, "user_id", userID)

	// Validate request
	if err := s.validator.Validate(req); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check edit permission
	canEdit, err := s.CanEdit(ctx, id, userID)
	if err != nil {
		return err
	}
	if !canEdit {
		return NewPermissionError(userID, id, "assessment", "update_status", "not owner or insufficient permissions")
	}

	// Get current assessment
	assessment, err := s.repo.Assessment().GetByID(ctx, s.db, id)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return ErrAssessmentNotFound
		}
		return fmt.Errorf("failed to get assessment: %w", err)
	}

	// Validate status transition
	if err := s.validateStatusTransition(ctx, assessment, req.Status); err != nil {
		return err
	}

	// Update status
	assessment.Status = req.Status
	assessment.UpdatedAt = time.Now()

	if err := s.repo.Assessment().Update(ctx, s.db, assessment); err != nil {
		return fmt.Errorf("failed to update assessment status: %w", err)
	}

	s.logger.Info("Assessment status updated successfully",
		"assessment_id", id,
		"new_status", req.Status,
		"reason", req.Reason)

	return nil
}

func (s *assessmentService) Publish(ctx context.Context, id uint, userID string) error {
	return s.UpdateStatus(ctx, id, &UpdateStatusRequest{
		Status: models.StatusActive,
		Reason: stringPtr("Published by user"),
	}, userID)
}

func stringPtr(s string) *string {
	return &s
}

func (s *assessmentService) Archive(ctx context.Context, id uint, userID string) error {
	return s.UpdateStatus(ctx, id, &UpdateStatusRequest{
		Status: models.StatusArchived,
		Reason: stringPtr("Archived by user"),
	}, userID)
}

// ===== QUESTION MANAGEMENT =====

func (s *assessmentService) AddQuestion(ctx context.Context, assessmentID, questionID uint, order int, points int, userID string) error {
	s.logger.Info("Adding question to assessment",
		"assessment_id", assessmentID,
		"question_id", questionID,
		"order", order,
		"points", points,
		"user_id", userID)

	// Check edit permission
	canEdit, err := s.CanEdit(ctx, assessmentID, userID)
	if err != nil {
		return err
	}
	if !canEdit {
		return NewPermissionError(userID, assessmentID, "assessment", "add_question", "not owner or assessment not editable")
	}

	// Validate assessment questions are editable (not locked by attempts)
	if err := s.validateAssessmentQuestionsEditable(ctx, nil, assessmentID); err != nil {
		return err
	}

	// Verify question exists and user has access
	canAccessQuestion, err := s.questionService.CanAccess(ctx, questionID, userID)
	if err != nil {
		return err
	}
	if !canAccessQuestion {
		return NewPermissionError(userID, questionID, "question", "access", "question not found or access denied")
	}

	// Validate total points would not exceed 100
	if err := s.validateTotalPoints(ctx, nil, assessmentID, points, 0); err != nil {
		return fmt.Errorf("points validation failed: %w", err)
	}

	// Add question to assessment (points is now required, not optional)
	pointsPtr := &points
	if err := s.repo.AssessmentQuestion().AddQuestion(ctx, s.db, assessmentID, questionID, order, pointsPtr); err != nil {
		return fmt.Errorf("failed to add question to assessment: %w", err)
	}

	s.logger.Info("Question added to assessment successfully",
		"assessment_id", assessmentID,
		"question_id", questionID)

	return nil
}

func (s *assessmentService) AddQuestions(ctx context.Context, assessmentID uint, questionsId []uint, userID string) error {
	s.logger.Info("Adding multiple questions to assessment",
		"assessment_id", assessmentID,
		"question_count", len(questionsId),
		"user_id", userID)

	// Check edit permission
	canEdit, err := s.CanEdit(ctx, assessmentID, userID)
	if err != nil {
		return err
	}

	if !canEdit {
		return NewPermissionError(userID, assessmentID, "assessment", "add_questions", "not owner or assessment not editable")
	}

	// Add questions to assessment
	if err := s.repo.AssessmentQuestion().AddQuestions(ctx, s.db, assessmentID, questionsId); err != nil {
		return fmt.Errorf("failed to add questions to assessment: %w", err)
	}

	s.logger.Info("Questions added to assessment successfully",
		"assessment_id", assessmentID,
		"question_count", len(questionsId))

	return nil
}

// AddQuestionsBatch adds multiple questions to assessment in a single transaction with points validation
func (s *assessmentService) AddQuestionsBatch(ctx context.Context, assessmentID uint, questions []AssessmentQuestionRequest, userID string) error {
	s.logger.Info("Adding multiple questions to assessment (batch)",
		"assessment_id", assessmentID,
		"question_count", len(questions),
		"user_id", userID)

	if len(questions) == 0 {
		return fmt.Errorf("no questions provided")
	}

	// Check edit permission
	canEdit, err := s.CanEdit(ctx, assessmentID, userID)
	if err != nil {
		return err
	}
	if !canEdit {
		return NewPermissionError(userID, assessmentID, "assessment", "add_questions_batch", "not owner or assessment not editable")
	}

	// Begin transaction for atomic operation
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// 0. Validate assessment questions are editable (not locked by attempts)
		if err := s.validateAssessmentQuestionsEditable(ctx, tx, assessmentID); err != nil {
			return err
		}

		// 1. Get current total points
		currentTotal, err := s.repo.AssessmentQuestion().GetTotalPoints(ctx, tx, assessmentID)
		if err != nil {
			return fmt.Errorf("failed to get current total points: %w", err)
		}

		// 2. Calculate new total points
		newPointsTotal := 0
		questionIDs := make([]uint, len(questions))
		for i, q := range questions {
			// Validate points range
			if q.Points < 1 || q.Points > 100 {
				return fmt.Errorf("question %d has invalid points %d (must be 1-100)", q.QuestionID, q.Points)
			}
			newPointsTotal += q.Points
			questionIDs[i] = q.QuestionID

			// Verify question exists
			_, err := s.repo.Question().GetByID(ctx, tx, q.QuestionID)
			if err != nil {
				if repositories.IsNotFoundError(err) {
					return fmt.Errorf("question %d not found", q.QuestionID)
				}
				return fmt.Errorf("failed to verify question %d: %w", q.QuestionID, err)
			}
		}

		// 3. Validate total points would not exceed 100
		finalTotal := currentTotal + newPointsTotal
		if finalTotal > 100 {
			return fmt.Errorf("total points (%d) would exceed maximum allowed (100). Current: %d, adding: %d",
				finalTotal, currentTotal, newPointsTotal)
		}

		// 4. Check for existing relationships
		var existingCount int64
		if err := tx.Model(&models.AssessmentQuestion{}).
			Where("assessment_id = ? AND question_id IN ?", assessmentID, questionIDs).
			Count(&existingCount).Error; err != nil {
			return fmt.Errorf("failed to check existing questions: %w", err)
		}
		if existingCount > 0 {
			return fmt.Errorf("some questions are already in this assessment")
		}

		// 5. Batch create AssessmentQuestions
		assessmentQuestions := make([]*models.AssessmentQuestion, len(questions))
		for i, q := range questions {
			assessmentQuestions[i] = &models.AssessmentQuestion{
				AssessmentID: assessmentID,
				QuestionID:   q.QuestionID,
				Order:        q.Order,
				Points:       &q.Points,
				Required:     true,
			}
		}

		// 6. Batch insert
		if err := s.repo.AssessmentQuestion().CreateBatch(ctx, tx, assessmentQuestions); err != nil {
			return fmt.Errorf("failed to batch create assessment questions: %w", err)
		}

		s.logger.Info("Batch insert successful",
			"assessment_id", assessmentID,
			"questions_added", len(questions),
			"new_total_points", finalTotal)

		return nil
	})

	if err != nil {
		return err
	}

	s.logger.Info("Questions added to assessment successfully (batch)",
		"assessment_id", assessmentID,
		"question_count", len(questions))

	return nil
}

// AutoAssignQuestions adds multiple questions to assessment with automatic point rebalancing
// Points are calculated automatically: 100 / total_questions (existing + new)
// This REBALANCES all existing questions to equal points
// Protected by lock policy: only works when assessment has no attempts
func (s *assessmentService) AutoAssignQuestions(ctx context.Context, assessmentID uint, questionIDs []uint, userID string) error {
	s.logger.Info("Auto-assigning questions to assessment (with rebalance)",
		"assessment_id", assessmentID,
		"new_question_count", len(questionIDs),
		"user_id", userID)

	if len(questionIDs) == 0 {
		return fmt.Errorf("no questions provided")
	}

	// Check edit permission
	canEdit, err := s.CanEdit(ctx, assessmentID, userID)
	if err != nil {
		return err
	}
	if !canEdit {
		return NewPermissionError(userID, assessmentID, "assessment", "auto_assign_questions", "not owner or assessment not editable")
	}

	// Begin transaction for atomic operation
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// 0. Validate assessment questions are editable (not locked by attempts)
		// This prevents rebalancing when students have started
		if err := s.validateAssessmentQuestionsEditable(ctx, tx, assessmentID); err != nil {
			return err
		}

		// 1. Calculate auto-assigned points for ALL questions (existing + new)
		basePoints, remainder, err := s.calculateAutoAssignPoints(ctx, tx, assessmentID, len(questionIDs))
		if err != nil {
			return err // Error already contains user-friendly message
		}

		// 2. Verify all new questions exist and user has access
		for _, qid := range questionIDs {
			// Verify question exists
			_, err := s.repo.Question().GetByID(ctx, tx, qid)
			if err != nil {
				if repositories.IsNotFoundError(err) {
					return fmt.Errorf("question %d not found", qid)
				}
				return fmt.Errorf("failed to verify question %d: %w", qid, err)
			}

			// Check access
			canAccessQuestion, err := s.questionService.CanAccess(ctx, qid, userID)
			if err != nil {
				return fmt.Errorf("failed to check access for question %d: %w", qid, err)
			}
			if !canAccessQuestion {
				return fmt.Errorf("no access to question %d", qid)
			}
		}

		// 3. Check for existing relationships (prevent duplicates)
		var existingCount int64
		if err := tx.Model(&models.AssessmentQuestion{}).
			Where("assessment_id = ? AND question_id IN ?", assessmentID, questionIDs).
			Count(&existingCount).Error; err != nil {
			return fmt.Errorf("failed to check existing questions: %w", err)
		}
		if existingCount > 0 {
			return fmt.Errorf("some questions are already in this assessment")
		}

		// 4. Get ALL existing AssessmentQuestions
		var existingQuestions []*models.AssessmentQuestion
		if err := tx.Where("assessment_id = ?", assessmentID).
			Order(`"order" ASC`).
			Find(&existingQuestions).Error; err != nil {
			return fmt.Errorf("failed to get existing questions: %w", err)
		}

		// 5. Rebalance ALL existing questions with new points
		// First N questions (where N = remainder) get basePoints + 1, rest get basePoints
		totalQuestions := len(existingQuestions) + len(questionIDs)

		for i, aq := range existingQuestions {
			newPoints := basePoints
			if i < remainder {
				newPoints++ // First N questions get extra point
			}
			aq.Points = &newPoints

			if err := s.repo.AssessmentQuestion().Update(ctx, tx, aq); err != nil {
				return fmt.Errorf("failed to update question %d points: %w", aq.QuestionID, err)
			}
		}

		// 6. Get next order number for new questions
		var maxOrder int
		if err := tx.Model(&models.AssessmentQuestion{}).
			Where("assessment_id = ?", assessmentID).
			Select(`COALESCE(MAX("order"), 0)`).
			Scan(&maxOrder).Error; err != nil {
			return fmt.Errorf("failed to get max order: %w", err)
		}

		// 7. Add new questions with calculated points
		// Continue remainder distribution from where existing questions left off
		assessmentQuestions := make([]*models.AssessmentQuestion, len(questionIDs))
		for i, qid := range questionIDs {
			// Index in total questions (for remainder calculation)
			totalIndex := len(existingQuestions) + i

			newPoints := basePoints
			if totalIndex < remainder {
				newPoints++ // Continue remainder distribution
			}

			assessmentQuestions[i] = &models.AssessmentQuestion{
				AssessmentID: assessmentID,
				QuestionID:   qid,
				Order:        maxOrder + i + 1,
				Points:       &newPoints,
				Required:     true,
			}
		}

		// 8. Batch insert new questions
		if err := s.repo.AssessmentQuestion().CreateBatch(ctx, tx, assessmentQuestions); err != nil {
			return fmt.Errorf("failed to batch create assessment questions: %w", err)
		}

		s.logger.Info("Auto-assign with rebalance successful",
			"assessment_id", assessmentID,
			"existing_questions_rebalanced", len(existingQuestions),
			"new_questions_added", len(questionIDs),
			"total_questions", totalQuestions,
			"points_per_question", basePoints,
			"remainder", remainder)

		return nil
	})

	if err != nil {
		return err
	}

	s.logger.Info("Questions auto-assigned to assessment successfully (with rebalance)",
		"assessment_id", assessmentID,
		"new_question_count", len(questionIDs))

	return nil
}

func (s *assessmentService) UpdateAssessmentQuestion(ctx context.Context, assessmentID, questionID uint, req *UpdateAssessmentQuestionRequest, userID string) error {
	s.logger.Info("Updating assessment question",
		"assessment_id", assessmentID,
		"question_id", questionID,
		"new_points", req.Points,
		"user_id", userID)

	// Check edit permission
	canEdit, err := s.CanEdit(ctx, assessmentID, userID)
	if err != nil {
		return err
	}
	if !canEdit {
		return NewPermissionError(userID, assessmentID, "assessment", "update_assessment_question", "not owner or assessment not editable")
	}

	// Validate assessment questions are editable (not locked by attempts)
	if err := s.validateAssessmentQuestionsEditable(ctx, nil, assessmentID); err != nil {
		return err
	}

	// Check if question is part of the assessment
	assessmentQuestion, err := s.repo.AssessmentQuestion().GetQuestionAssessmentByAssessmentIdAndQuestionId(ctx, s.db, assessmentID, questionID)
	if err != nil {
		return fmt.Errorf("failed to check if question exists in assessment: %w", err)
	}

	// Validate total points would not exceed 100 (excluding current question's points)
	if err := s.validateTotalPoints(ctx, nil, assessmentID, req.Points, questionID); err != nil {
		return fmt.Errorf("points validation failed: %w", err)
	}

	// Update assessment question points (now required, not optional)
	assessmentQuestion.Points = &req.Points

	// NOTE: TimeLimit is stored but NOT used in attempt timing logic. Assessment.Duration is used instead.
	if req.TimeLimit != nil {
		assessmentQuestion.TimeLimit = req.TimeLimit
	}

	if err := s.repo.AssessmentQuestion().Update(ctx, s.db, assessmentQuestion); err != nil {
		return fmt.Errorf("failed to update assessment question: %w", err)
	}

	s.logger.Info("Assessment question updated successfully",
		"assessment_id", assessmentID,
		"question_id", questionID,
		"updated_points", req.Points)
	return nil
}

func (s *assessmentService) RemoveQuestions(ctx context.Context, assessmentID uint, questionsId []uint, userID string) error {
	s.logger.Info("Removing multiple questions from assessment",
		"assessment_id", assessmentID,
		"question_count", len(questionsId),
		"user_id", userID)

	// Check edit permission
	canEdit, err := s.CanEdit(ctx, assessmentID, userID)
	if err != nil {
		return err
	}
	if !canEdit {
		return NewPermissionError(userID, assessmentID, "assessment", "remove_questions", "not owner or assessment not editable")
	}

	// Validate assessment questions are editable (not locked by attempts)
	if err := s.validateAssessmentQuestionsEditable(ctx, nil, assessmentID); err != nil {
		return err
	}

	// Remove questions from assessment
	if err := s.repo.AssessmentQuestion().RemoveQuestions(ctx, s.db, assessmentID, questionsId); err != nil {
		return fmt.Errorf("failed to remove questions from assessment: %w", err)
	}

	s.logger.Info("Questions removed from assessment successfully",
		"assessment_id", assessmentID,
		"question_count", len(questionsId))
	return nil
}

func (s *assessmentService) UpdateAssessmentQuestionBatch(ctx context.Context, assessmentID uint, reqs []UpdateAssessmentQuestionRequest, userID string) error {
	s.logger.Info("Updating multiple assessment questions",
		"assessment_id", assessmentID,
		"question_count", len(reqs),
		"user_id", userID)

	// Check edit permission
	canEdit, err := s.CanEdit(ctx, assessmentID, userID)
	if err != nil {
		return err
	}
	if !canEdit {
		return NewPermissionError(userID, assessmentID, "assessment", "update_assessment_questions", "not owner or assessment not editable")
	}

	// Update assessment questions in batch
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// 0. Validate assessment questions are editable (not locked by attempts)
		if err := s.validateAssessmentQuestionsEditable(ctx, tx, assessmentID); err != nil {
			return err
		}

		// First, get current total points and build a map of question IDs being updated
		currentTotal, err := s.repo.AssessmentQuestion().GetTotalPoints(ctx, tx, assessmentID)
		if err != nil {
			return fmt.Errorf("failed to get current total points: %w", err)
		}

		// Calculate new total by subtracting old points and adding new points for each question
		updatedQuestionPoints := make(map[uint]int)
		for _, req := range reqs {
			// Get current points for this question
			assessmentQuestion, err := s.repo.AssessmentQuestion().GetQuestionAssessmentByAssessmentIdAndQuestionId(ctx, tx, assessmentID, req.QuestionId)
			if err != nil {
				return fmt.Errorf("failed to get assessment question (question_id: %d): %w", req.QuestionId, err)
			}

			// Track old and new points
			oldPoints := 0
			if assessmentQuestion.Points != nil {
				oldPoints = *assessmentQuestion.Points
			}
			currentTotal -= oldPoints
			currentTotal += req.Points

			updatedQuestionPoints[req.QuestionId] = req.Points
		}

		// Validate new total
		if currentTotal > 100 {
			return fmt.Errorf("total points (%d) would exceed maximum allowed (100) after batch update", currentTotal)
		}

		// Now perform the updates
		for _, req := range reqs {
			// Get assessment question
			assessmentQuestion, err := s.repo.AssessmentQuestion().GetQuestionAssessmentByAssessmentIdAndQuestionId(ctx, tx, assessmentID, req.QuestionId)
			if err != nil {
				return fmt.Errorf("failed to get assessment question (question_id: %d): %w", req.QuestionId, err)
			}

			// Update fields (Points is now required, not optional)
			assessmentQuestion.Points = &req.Points

			// NOTE: TimeLimit is stored but NOT used in attempt timing logic. Assessment.Duration is used instead.
			if req.TimeLimit != nil {
				assessmentQuestion.TimeLimit = req.TimeLimit
			}
			// Save
			if err := s.repo.AssessmentQuestion().Update(ctx, tx, assessmentQuestion); err != nil {
				return fmt.Errorf("failed to update assessment question (question_id: %d): %w", req.QuestionId, err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.logger.Info("Assessment questions updated successfully",
		"assessment_id", assessmentID,
		"question_count", len(reqs))
	return nil
}

func (s *assessmentService) RemoveQuestion(ctx context.Context, assessmentID, questionID uint, userID string) error {
	s.logger.Info("Removing question from assessment",
		"assessment_id", assessmentID,
		"question_id", questionID,
		"user_id", userID)

	// Check edit permission
	canEdit, err := s.CanEdit(ctx, assessmentID, userID)
	if err != nil {
		return err
	}
	if !canEdit {
		return NewPermissionError(userID, assessmentID, "assessment", "remove_question", "not owner or assessment not editable")
	}

	// Validate assessment questions are editable (not locked by attempts)
	if err := s.validateAssessmentQuestionsEditable(ctx, nil, assessmentID); err != nil {
		return err
	}

	// Remove question from assessment
	if err := s.repo.AssessmentQuestion().RemoveQuestion(ctx, s.db, assessmentID, questionID); err != nil {
		return fmt.Errorf("failed to remove question from assessment: %w", err)
	}

	s.logger.Info("Question removed from assessment successfully",
		"assessment_id", assessmentID,
		"question_id", questionID)

	return nil
}

func (s *assessmentService) ReorderQuestions(ctx context.Context, assessmentID uint, orders []repositories.QuestionOrder, userID string) error {
	s.logger.Info("Reordering assessment questions",
		"assessment_id", assessmentID,
		"question_count", len(orders),
		"user_id", userID)

	// Check edit permission
	canEdit, err := s.CanEdit(ctx, assessmentID, userID)
	if err != nil {
		return err
	}
	if !canEdit {
		return NewPermissionError(userID, assessmentID, "assessment", "reorder_questions", "not owner or assessment not editable")
	}

	// Validate assessment questions are editable (not locked by attempts)
	if err := s.validateAssessmentQuestionsEditable(ctx, nil, assessmentID); err != nil {
		return err
	}

	// Reorder questions
	if err := s.repo.AssessmentQuestion().ReorderQuestions(ctx, s.db, assessmentID, orders); err != nil {
		return fmt.Errorf("failed to reorder questions: %w", err)
	}

	s.logger.Info("Assessment questions reordered successfully", "assessment_id", assessmentID)

	return nil
}

// ===== STATISTICS AND ANALYTICS =====

func (s *assessmentService) GetStats(ctx context.Context, id uint, userID string) (*repositories.AssessmentStats, error) {
	// Check access permission
	canAccess, err := s.CanAccess(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, id, "assessment", "view_stats", "not owner or insufficient permissions")
	}

	stats, err := s.repo.Assessment().GetAssessmentStats(ctx, nil, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get assessment stats: %w", err)
	}

	return stats, nil
}

func (s *assessmentService) GetCreatorStats(ctx context.Context, creatorID string) (*repositories.CreatorStats, error) {
	stats, err := s.repo.Assessment().GetCreatorStats(ctx, nil, creatorID)
	if err != nil {
		return nil, fmt.Errorf("failed to get creator stats: %w", err)
	}

	return stats, nil
}
