package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/SAP-F-2025/assessment-service/internal/validator"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type questionService struct {
	repo      repositories.Repository
	db        *gorm.DB
	logger    *slog.Logger
	validator *validator.Validator
}

func NewQuestionService(repo repositories.Repository, db *gorm.DB, logger *slog.Logger, validator *validator.Validator) QuestionService {
	return &questionService{
		repo:      repo,
		db:        db,
		logger:    logger,
		validator: validator,
	}
}

// ===== CORE CRUD OPERATIONS =====

func (s *questionService) Create(ctx context.Context, req *CreateQuestionRequest, creatorID string) (*QuestionResponse, error) {
	s.logger.Info("Creating question", "creator_id", creatorID, "type", req.Type)

	// Validate request with business rules
	if errors := s.validator.GetBusinessValidator().ValidateQuestionCreate(req); len(errors) > 0 {
		return nil, errors
	}

	// Check user permissions
	canCreate, err := s.canCreateQuestion(ctx, creatorID)
	if err != nil {
		return nil, fmt.Errorf("permission check failed: %w", err)
	}
	if !canCreate {
		return nil, NewPermissionError(creatorID, 0, "question", "create", "insufficient role permissions")
	}

	// Validate question content for type
	if err := s.validateQuestionContent(req.Type, req.Content); err != nil {
		return nil, fmt.Errorf("content validation failed: %w", err)
	}

	// Validate category exists if provided
	if req.CategoryID != nil {
		// TODO: Enable category access validation when categories are implemented
		//if err := s.validateCategoryAccess(ctx, *req.CategoryID, creatorID); err != nil {
		//	return nil, err
		//}
	}

	// Convert content to JSON
	contentBytes, err := json.Marshal(req.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal content: %w", err)
	}

	// Convert tag strings to JSON
	if req.Tags == nil {
		req.Tags = []string{}
	}

	tagsBytes, err := json.Marshal(req.Tags)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tags: %w", err)
	}

	// Create question
	question := &models.Question{
		Type:        req.Type,
		Text:        req.Text,
		Content:     contentBytes,
		Points:      req.Points,
		TimeLimit:   req.TimeLimit,
		Difficulty:  req.Difficulty,
		CategoryID:  req.CategoryID,
		Tags:        datatypes.JSON(tagsBytes),
		Explanation: req.Explanation,
		CreatedBy:   creatorID,
	}

	if err = s.repo.Question().Create(ctx, nil, question); err != nil {
		return nil, fmt.Errorf("failed to create question: %w", err)
	}

	s.logger.Info("Question created successfully", "question_id", question.ID)

	// Return response
	return s.buildQuestionResponse(ctx, question, creatorID), nil
}

func (s *questionService) GetByID(ctx context.Context, id uint, userID string) (*QuestionResponse, error) {
	// Check access permission
	canAccess, err := s.CanAccess(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, id, "question", "read", "not owner or insufficient permissions")
	}

	// Get question
	question, err := s.repo.Question().GetByID(ctx, nil, id)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrQuestionNotFound
		}
		return nil, fmt.Errorf("failed to get question: %w", err)
	}

	return s.buildQuestionResponse(ctx, question, userID), nil
}

func (s *questionService) GetByIDWithDetails(ctx context.Context, id uint, userID string) (*QuestionResponse, error) {
	// Check access permission
	canAccess, err := s.CanAccess(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, id, "question", "read", "not owner or insufficient permissions")
	}

	// Get question with details
	question, err := s.repo.Question().GetByIDWithDetails(ctx, nil, id)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrQuestionNotFound
		}
		return nil, fmt.Errorf("failed to get question with details: %w", err)
	}

	return s.buildQuestionResponse(ctx, question, userID), nil
}

func (s *questionService) Update(ctx context.Context, id uint, req *UpdateQuestionRequest, userID string) (*QuestionResponse, error) {
	s.logger.Info("Updating question", "question_id", id, "user_id", userID)

	// Validate request
	if err := s.validator.Validate(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check edit permission
	canEdit, err := s.CanEdit(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if !canEdit {
		return nil, NewPermissionError(userID, id, "question", "update", "not owner or question not editable")
	}

	// Get current question
	question, err := s.repo.Question().GetByID(ctx, nil, id)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrQuestionNotFound
		}
		return nil, fmt.Errorf("failed to get question: %w", err)
	}

	// Validate content if being updated
	if req.Content != nil {
		questionType := question.Type
		if err := s.validateQuestionContent(questionType, req.Content); err != nil {
			return nil, fmt.Errorf("content validation failed: %w", err)
		}
	}

	// Validate category if being updated
	if req.CategoryID != nil {
		if err := s.validateCategoryAccess(ctx, *req.CategoryID, userID); err != nil {
			return nil, err
		}
	}

	// Apply updates
	if err := s.applyQuestionUpdates(question, req); err != nil {
		return nil, err
	}

	// Update question
	if err = s.repo.Question().Update(ctx, nil, question); err != nil {
		return nil, fmt.Errorf("failed to update question: %w", err)
	}

	s.logger.Info("Question updated successfully", "question_id", id)

	// Return updated question
	return s.buildQuestionResponse(ctx, question, userID), nil
}

func (s *questionService) Delete(ctx context.Context, id uint, userID string) error {
	s.logger.Info("Deleting question", "question_id", id, "user_id", userID)

	// Check delete permission
	canDelete, err := s.CanDelete(ctx, id, userID)
	if err != nil {
		return err
	}
	if !canDelete {
		return NewPermissionError(userID, id, "question", "delete", "not owner or question in use")
	}

	// Soft delete
	if err := s.repo.Question().Delete(ctx, nil, id); err != nil {
		return fmt.Errorf("failed to delete question: %w", err)
	}

	s.logger.Info("Question deleted successfully", "question_id", id)
	return nil
}

// ===== LIST AND SEARCH OPERATIONS =====

func (s *questionService) List(ctx context.Context, filters repositories.QuestionFilters, userID string) (*QuestionListResponse, error) {
	// For non-admin users, limit to their own questions
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return nil, err
	}

	if userRole != models.RoleAdmin {
		filters.CreatedBy = &userID
	}

	questions, total, err := s.repo.Question().List(ctx, nil, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list questions: %w", err)
	}

	// Build response
	response := &QuestionListResponse{
		Questions: make([]*QuestionResponse, len(questions)),
		Total:     total,
		Page:      (filters.Offset / max(filters.Limit, 1)) + 1,
		Size:      filters.Limit,
	}

	for i, question := range questions {
		response.Questions[i] = s.buildQuestionResponse(ctx, question, userID)
	}

	return response, nil
}

func (s *questionService) GetByCreator(ctx context.Context, creatorID string, filters repositories.QuestionFilters) (*QuestionListResponse, error) {
	// Set creator filter
	filters.CreatedBy = &creatorID

	questions, total, err := s.repo.Question().GetByCreator(ctx, nil, creatorID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get questions by creator: %w", err)
	}

	// Build response
	response := &QuestionListResponse{
		Questions: make([]*QuestionResponse, len(questions)),
		Total:     total,
		Page:      (filters.Offset / max(filters.Limit, 1)) + 1,
		Size:      filters.Limit,
	}

	for i, question := range questions {
		response.Questions[i] = s.buildQuestionResponse(ctx, question, creatorID)
	}

	return response, nil
}

func (s *questionService) Search(ctx context.Context, query string, filters repositories.QuestionFilters, userID string) (*QuestionListResponse, error) {
	// For non-admin users, limit to their own questions
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return nil, err
	}

	if userRole != models.RoleAdmin {
		filters.CreatedBy = &userID
	}

	questions, total, err := s.repo.Question().Search(ctx, nil, query, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to search questions: %w", err)
	}

	// Build response
	response := &QuestionListResponse{
		Questions: make([]*QuestionResponse, len(questions)),
		Total:     total,
		Page:      (filters.Offset / max(filters.Limit, 1)) + 1,
		Size:      filters.Limit,
	}

	for i, question := range questions {
		response.Questions[i] = s.buildQuestionResponse(ctx, question, userID)
	}

	return response, nil
}

func (s *questionService) GetRandomQuestions(ctx context.Context, filters repositories.RandomQuestionFilters, userID string) ([]*models.Question, error) {
	// For non-admin users, add permission filter
	_, err := s.getUserRole(ctx, userID)
	if err != nil {
		return nil, err
	}

	// TODO: Add permission filtering for random questions based on user role

	questions, err := s.repo.Question().GetRandomQuestions(ctx, nil, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get random questions: %w", err)
	}

	return questions, nil
}

// ===== BULK OPERATIONS =====

func (s *questionService) CreateBatch(ctx context.Context, questions []*CreateQuestionRequest, creatorID string) ([]*QuestionResponse, []error) {
	s.logger.Info("Creating batch questions", "creator_id", creatorID, "count", len(questions))

	results := make([]*QuestionResponse, len(questions))
	errors := make([]error, len(questions))

	// Check user permissions once
	canCreate, err := s.canCreateQuestion(ctx, creatorID)
	if err != nil {
		for i := range errors {
			errors[i] = fmt.Errorf("permission check failed: %w", err)
		}
		return results, errors
	}
	if !canCreate {
		permErr := NewPermissionError(creatorID, 0, "question", "create", "insufficient role permissions")
		for i := range errors {
			errors[i] = permErr
		}
		return results, errors
	}

	// Process each question
	for i, req := range questions {
		result, err := s.Create(ctx, req, creatorID)
		results[i] = result
		errors[i] = err
	}

	return results, errors
}

func (s *questionService) UpdateBatch(ctx context.Context, updates map[uint]*UpdateQuestionRequest, userID string) (map[uint]*QuestionResponse, map[uint]error) {
	s.logger.Info("Updating batch questions", "user_id", userID, "count", len(updates))

	results := make(map[uint]*QuestionResponse)
	errors := make(map[uint]error)

	// Process each update
	for questionID, req := range updates {
		result, err := s.Update(ctx, questionID, req, userID)
		results[questionID] = result
		errors[questionID] = err
	}

	return results, errors
}

// ===== QUESTION BANKING =====

func (s *questionService) GetByBank(ctx context.Context, bankID uint, filters repositories.QuestionFilters, userID string) (*QuestionListResponse, error) {
	// Check access to question bank
	canAccess, err := s.canAccessQuestionBank(ctx, bankID, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, bankID, "question_bank", "access", "not owner or insufficient permissions")
	}

	questions, total, err := s.repo.Question().GetByBank(ctx, bankID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get questions by bank: %w", err)
	}

	// Build response
	response := &QuestionListResponse{
		Questions: make([]*QuestionResponse, len(questions)),
		Total:     total,
		Page:      (filters.Offset / max(filters.Limit, 1)) + 1,
		Size:      filters.Limit,
	}

	for i, question := range questions {
		response.Questions[i] = s.buildQuestionResponse(ctx, question, userID)
	}

	return response, nil
}

func (s *questionService) AddToBank(ctx context.Context, questionID, bankID uint, userID string) error {
	s.logger.Info("Adding question to bank", "question_id", questionID, "bank_id", bankID, "user_id", userID)

	// Check question access
	canAccess, err := s.CanAccess(ctx, questionID, userID)
	if err != nil {
		return err
	}
	if !canAccess {
		return NewPermissionError(userID, questionID, "question", "access", "question not found or access denied")
	}

	// Check bank access
	canAccessBank, err := s.canAccessQuestionBank(ctx, bankID, userID)
	if err != nil {
		return err
	}
	if !canAccessBank {
		return NewPermissionError(userID, bankID, "question_bank", "edit", "bank not found or access denied")
	}

	// Add question to bank
	if err := s.repo.Question().AddToBank(ctx, questionID, bankID); err != nil {
		return fmt.Errorf("failed to add question to bank: %w", err)
	}

	s.logger.Info("Question added to bank successfully", "question_id", questionID, "bank_id", bankID)
	return nil
}

func (s *questionService) RemoveFromBank(ctx context.Context, questionID, bankID uint, userID string) error {
	s.logger.Info("Removing question from bank", "question_id", questionID, "bank_id", bankID, "user_id", userID)

	// Check bank edit permission
	canEdit, err := s.canEditQuestionBank(ctx, bankID, userID)
	if err != nil {
		return err
	}
	if !canEdit {
		return NewPermissionError(userID, bankID, "question_bank", "edit", "not owner or insufficient permissions")
	}

	// Remove question from bank
	if err := s.repo.Question().RemoveFromBank(ctx, questionID, bankID); err != nil {
		return fmt.Errorf("failed to remove question from bank: %w", err)
	}

	s.logger.Info("Question removed from bank successfully", "question_id", questionID, "bank_id", bankID)
	return nil
}
