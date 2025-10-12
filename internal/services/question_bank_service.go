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

type questionBankService struct {
	repo      repositories.Repository
	db        *gorm.DB
	logger    *slog.Logger
	validator *validator.Validator
}

func NewQuestionBankService(repo repositories.Repository, db *gorm.DB, logger *slog.Logger, validator *validator.Validator) QuestionBankService {
	return &questionBankService{
		repo:      repo,
		db:        db,
		logger:    logger,
		validator: validator,
	}
}

// ===== CORE CRUD OPERATIONS =====

func (s *questionBankService) Create(ctx context.Context, req *CreateQuestionBankRequest, creatorID string) (*QuestionBankResponse, error) {
	s.logger.Info("Creating question bank", "creator_id", creatorID, "name", req.Name)

	// Validate request
	if err := s.validator.Validate(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check if bank with same name exists for this user
	exists, err := s.repo.QuestionBank().ExistsByName(ctx, nil, req.Name, creatorID)
	if err != nil {
		return nil, fmt.Errorf("failed to check bank name uniqueness: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("question bank with name '%s' already exists", req.Name)
	}

	// Create question bank
	bank := &models.QuestionBank{
		Name:        req.Name,
		Description: req.Description,
		IsPublic:    req.IsPublic,
		IsShared:    req.IsShared,
		CreatedBy:   creatorID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err = s.repo.QuestionBank().Create(ctx, nil, bank); err != nil {
		return nil, fmt.Errorf("failed to create question bank: %w", err)
	}

	s.logger.Info("Question bank created successfully", "bank_id", bank.ID)

	return s.buildQuestionBankResponse(ctx, bank, creatorID), nil
}

func (s *questionBankService) GetByID(ctx context.Context, id uint, userID string) (*QuestionBankResponse, error) {
	// Check access permission
	canAccess, err := s.CanAccess(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, id, "question_bank", "read", "not owner, not public, or not shared")
	}

	// Get question bank
	bank, err := s.repo.QuestionBank().GetByID(ctx, nil, id)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrQuestionBankNotFound
		}
		return nil, fmt.Errorf("failed to get question bank: %w", err)
	}

	return s.buildQuestionBankResponse(ctx, bank, userID), nil
}

func (s *questionBankService) GetByIDWithDetails(ctx context.Context, id uint, userID string) (*QuestionBankResponse, error) {
	// Check access permission
	canAccess, err := s.CanAccess(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, id, "question_bank", "read", "not owner, not public, or not shared")
	}

	// Get question bank with details
	bank, err := s.repo.QuestionBank().GetByIDWithDetails(ctx, nil, id)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrQuestionBankNotFound
		}
		return nil, fmt.Errorf("failed to get question bank with details: %w", err)
	}

	return s.buildQuestionBankResponse(ctx, bank, userID), nil
}

func (s *questionBankService) Update(ctx context.Context, id uint, req *UpdateQuestionBankRequest, userID string) (*QuestionBankResponse, error) {
	s.logger.Info("Updating question bank", "bank_id", id, "user_id", userID)

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
		return nil, NewPermissionError(userID, id, "question_bank", "update", "not owner or insufficient permissions")
	}

	// Get current bank
	bank, err := s.repo.QuestionBank().GetByID(ctx, nil, id)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrQuestionBankNotFound
		}
		return nil, fmt.Errorf("failed to get question bank: %w", err)
	}

	// Check name uniqueness if name is being updated
	if req.Name != nil && *req.Name != bank.Name {
		exists, err := s.repo.QuestionBank().ExistsByName(ctx, nil, *req.Name, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to check bank name uniqueness: %w", err)
		}
		if exists {
			return nil, fmt.Errorf("question bank with name '%s' already exists", *req.Name)
		}
	}

	// Apply updates
	if req.Name != nil {
		bank.Name = *req.Name
	}
	if req.Description != nil {
		bank.Description = req.Description
	}
	if req.IsPublic != nil {
		bank.IsPublic = *req.IsPublic
	}
	if req.IsShared != nil {
		bank.IsShared = *req.IsShared
	}
	bank.UpdatedAt = time.Now()

	// Update bank
	if err = s.repo.QuestionBank().Update(ctx, nil, bank); err != nil {
		return nil, fmt.Errorf("failed to update question bank: %w", err)
	}

	s.logger.Info("Question bank updated successfully", "bank_id", id)

	return s.buildQuestionBankResponse(ctx, bank, userID), nil
}

func (s *questionBankService) Delete(ctx context.Context, id uint, userID string) error {
	s.logger.Info("Deleting question bank", "bank_id", id, "user_id", userID)

	// Check delete permission
	canDelete, err := s.CanDelete(ctx, id, userID)
	if err != nil {
		return err
	}
	if !canDelete {
		return NewPermissionError(userID, id, "question_bank", "delete", "not owner or insufficient permissions")
	}

	// Soft delete
	if err := s.repo.QuestionBank().Delete(ctx, nil, id); err != nil {
		return fmt.Errorf("failed to delete question bank: %w", err)
	}

	s.logger.Info("Question bank deleted successfully", "bank_id", id)
	return nil
}

// ===== LIST AND SEARCH OPERATIONS =====

func (s *questionBankService) List(ctx context.Context, filters repositories.QuestionBankFilters, userID string) (*QuestionBankListResponse, error) {
	// Get user role to determine access level
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return nil, err
	}

	// For non-admin users, get accessible banks (owned, public, or shared)
	if userRole != models.RoleAdmin {
		return s.getAccessibleBanks(ctx, filters, userID)
	}

	// Admin users can see all banks
	banks, total, err := s.repo.QuestionBank().List(ctx, nil, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list question banks: %w", err)
	}

	// Build response
	response := &QuestionBankListResponse{
		Banks: make([]*QuestionBankResponse, len(banks)),
		Total: total,
		Page:  (filters.Offset / max(filters.Limit, 1)) + 1,
		Size:  filters.Limit,
	}

	for i, bank := range banks {
		response.Banks[i] = s.buildQuestionBankResponse(ctx, bank, userID)
	}

	return response, nil
}

func (s *questionBankService) GetByCreator(ctx context.Context, creatorID string, filters repositories.QuestionBankFilters) (*QuestionBankListResponse, error) {
	filters.CreatedBy = &creatorID

	banks, total, err := s.repo.QuestionBank().GetByCreator(ctx, nil, creatorID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get question banks by creator: %w", err)
	}

	// Build response
	response := &QuestionBankListResponse{
		Banks: make([]*QuestionBankResponse, len(banks)),
		Total: total,
		Page:  (filters.Offset / max(filters.Limit, 1)) + 1,
		Size:  filters.Limit,
	}

	for i, bank := range banks {
		response.Banks[i] = s.buildQuestionBankResponse(ctx, bank, creatorID)
	}

	return response, nil
}

func (s *questionBankService) GetPublic(ctx context.Context, filters repositories.QuestionBankFilters) (*QuestionBankListResponse, error) {
	banks, total, err := s.repo.QuestionBank().GetPublicBanks(ctx, nil, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get public question banks: %w", err)
	}

	// Build response (use 0 as userID since these are public)
	response := &QuestionBankListResponse{
		Banks: make([]*QuestionBankResponse, len(banks)),
		Total: total,
		Page:  (filters.Offset / max(filters.Limit, 1)) + 1,
		Size:  filters.Limit,
	}

	for i, bank := range banks {
		response.Banks[i] = s.buildQuestionBankResponse(ctx, bank, "0")
	}

	return response, nil
}

func (s *questionBankService) GetSharedWithUser(ctx context.Context, userID string, filters repositories.QuestionBankFilters) (*QuestionBankListResponse, error) {
	banks, total, err := s.repo.QuestionBank().GetSharedWithUser(ctx, nil, userID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get shared question banks: %w", err)
	}

	// Build response
	response := &QuestionBankListResponse{
		Banks: make([]*QuestionBankResponse, len(banks)),
		Total: total,
		Page:  (filters.Offset / max(filters.Limit, 1)) + 1,
		Size:  filters.Limit,
	}

	for i, bank := range banks {
		response.Banks[i] = s.buildQuestionBankResponse(ctx, bank, userID)
	}

	return response, nil
}

func (s *questionBankService) Search(ctx context.Context, query string, filters repositories.QuestionBankFilters, userID string) (*QuestionBankListResponse, error) {
	// Get user role to determine access level
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return nil, err
	}

	// For non-admin users, limit search to accessible banks
	if userRole != models.RoleAdmin {
		// For now, search only in owned banks
		// TODO: Implement search across accessible banks (owned + public + shared)
		filters.CreatedBy = &userID
	}

	banks, total, err := s.repo.QuestionBank().Search(ctx, nil, query, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to search question banks: %w", err)
	}

	// Build response
	response := &QuestionBankListResponse{
		Banks: make([]*QuestionBankResponse, len(banks)),
		Total: total,
		Page:  (filters.Offset / max(filters.Limit, 1)) + 1,
		Size:  filters.Limit,
	}

	for i, bank := range banks {
		response.Banks[i] = s.buildQuestionBankResponse(ctx, bank, userID)
	}

	return response, nil
}

// ===== SHARING OPERATIONS =====

func (s *questionBankService) ShareBank(ctx context.Context, bankID uint, req *ShareQuestionBankRequest, sharerID string) error {
	s.logger.Info("Sharing question bank", "bank_id", bankID, "sharer_id", sharerID, "user_id", req.UserID)

	// Validate request
	if err := s.validator.Validate(req); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check if user is owner or has share permissions
	isOwner, err := s.IsOwner(ctx, bankID, sharerID)
	if err != nil {
		return err
	}
	if !isOwner {
		return NewPermissionError(sharerID, bankID, "question_bank", "share", "not owner")
	}

	// Check if target user exists
	_, err = s.repo.User().GetByID(ctx, req.UserID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return fmt.Errorf("target user not found")
		}
		return fmt.Errorf("failed to get target user: %w", err)
	}

	// Create share
	share := &models.QuestionBankShare{
		BankID:    bankID,
		UserID:    req.UserID,
		CanView:   true, // Always true when sharing
		CanEdit:   req.CanEdit,
		CanDelete: req.CanDelete,
		SharedAt:  time.Now(),
		SharedBy:  sharerID,
	}

	if err := s.repo.QuestionBank().ShareBank(ctx, nil, share); err != nil {
		return fmt.Errorf("failed to share question bank: %w", err)
	}

	s.logger.Info("Question bank shared successfully", "bank_id", bankID, "with_user", req.UserID)
	return nil
}

func (s *questionBankService) UnshareBank(ctx context.Context, bankID uint, userID string, sharerID string) error {
	s.logger.Info("Unsharing question bank", "bank_id", bankID, "sharer_id", sharerID, "user_id", userID)

	// Check if user is owner
	isOwner, err := s.IsOwner(ctx, bankID, sharerID)
	if err != nil {
		return err
	}
	if !isOwner {
		return NewPermissionError(sharerID, bankID, "question_bank", "unshare", "not owner")
	}

	if err := s.repo.QuestionBank().UnshareBank(ctx, nil, bankID, userID); err != nil {
		return fmt.Errorf("failed to unshare question bank: %w", err)
	}

	s.logger.Info("Question bank unshared successfully", "bank_id", bankID, "from_user", userID)
	return nil
}

func (s *questionBankService) UpdateSharePermissions(ctx context.Context, bankID uint, userID string, canEdit, canDelete bool, sharerID string) error {
	s.logger.Info("Updating share permissions", "bank_id", bankID, "sharer_id", sharerID, "user_id", userID)

	// Check if user is owner
	isOwner, err := s.IsOwner(ctx, bankID, sharerID)
	if err != nil {
		return err
	}
	if !isOwner {
		return NewPermissionError(sharerID, bankID, "question_bank", "update_share", "not owner")
	}

	if err := s.repo.QuestionBank().UpdateSharePermissions(ctx, nil, bankID, userID, canEdit, canDelete); err != nil {
		return fmt.Errorf("failed to update share permissions: %w", err)
	}

	s.logger.Info("Share permissions updated successfully", "bank_id", bankID, "user_id", userID)
	return nil
}

func (s *questionBankService) GetBankShares(ctx context.Context, bankID uint, userID string) ([]*QuestionBankShareResponse, error) {
	// Check if user is owner
	isOwner, err := s.IsOwner(ctx, bankID, userID)
	if err != nil {
		return nil, err
	}
	if !isOwner {
		return nil, NewPermissionError(userID, bankID, "question_bank", "view_shares", "not owner")
	}

	shares, err := s.repo.QuestionBank().GetBankShares(ctx, nil, bankID)
	if err != nil {
		return nil, fmt.Errorf("failed to get bank shares: %w", err)
	}

	response := make([]*QuestionBankShareResponse, len(shares))
	for i, share := range shares {
		response[i] = &QuestionBankShareResponse{
			QuestionBankShare: share,
			CanModify:         isOwner, // Only owner can modify shares
		}
	}

	return response, nil
}

func (s *questionBankService) GetUserShares(ctx context.Context, userID string, filters repositories.QuestionBankShareFilters) ([]*QuestionBankShareResponse, int64, error) {
	shares, total, err := s.repo.QuestionBank().GetUserShares(ctx, nil, userID, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get user shares: %w", err)
	}

	response := make([]*QuestionBankShareResponse, len(shares))
	for i, share := range shares {
		// User can modify their own share settings (leave/change permissions if allowed by owner)
		response[i] = &QuestionBankShareResponse{
			QuestionBankShare: share,
			CanModify:         false, // Users cannot modify their own share permissions
		}
	}

	return response, total, nil
}

// ===== QUESTION MANAGEMENT =====

func (s *questionBankService) AddQuestions(ctx context.Context, bankID uint, req *AddQuestionsTobankRequest, userID string) error {
	s.logger.Info("Adding questions to bank", "bank_id", bankID, "user_id", userID, "question_count", len(req.QuestionIDs))

	// Validate request
	if err := s.validator.Validate(req); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check edit permission
	canEdit, err := s.CanEdit(ctx, bankID, userID)
	if err != nil {
		return err
	}
	if !canEdit {
		return NewPermissionError(userID, bankID, "question_bank", "add_questions", "not owner or insufficient permissions")
	}

	// Verify all questions exist and user has access to them
	for _, questionID := range req.QuestionIDs {
		canAccess, err := s.repo.Question().GetByID(ctx, nil, questionID)
		if err != nil {
			if repositories.IsNotFoundError(err) {
				return fmt.Errorf("question %d not found", questionID)
			}
			return fmt.Errorf("failed to check question %d: %w", questionID, err)
		}

		// Check if user has access to this question
		if canAccess.CreatedBy != userID {
			// TODO: Add proper permission checking for questions
			userRole, roleErr := s.getUserRole(ctx, userID)
			if roleErr != nil || userRole != models.RoleAdmin {
				return fmt.Errorf("no access to question %d", questionID)
			}
		}
	}

	// Add questions to bank
	if err := s.repo.QuestionBank().AddQuestions(ctx, nil, bankID, req.QuestionIDs); err != nil {
		return fmt.Errorf("failed to add questions to bank: %w", err)
	}

	s.logger.Info("Questions added to bank successfully", "bank_id", bankID, "question_count", len(req.QuestionIDs))
	return nil
}

func (s *questionBankService) RemoveQuestions(ctx context.Context, bankID uint, questionIDs []uint, userID string) error {
	s.logger.Info("Removing questions from bank", "bank_id", bankID, "user_id", userID, "question_count", len(questionIDs))

	// Check edit permission
	canEdit, err := s.CanEdit(ctx, bankID, userID)
	if err != nil {
		return err
	}
	if !canEdit {
		return NewPermissionError(userID, bankID, "question_bank", "remove_questions", "not owner or insufficient permissions")
	}

	// Remove questions from bank
	if err := s.repo.QuestionBank().RemoveQuestions(ctx, nil, bankID, questionIDs); err != nil {
		return fmt.Errorf("failed to remove questions from bank: %w", err)
	}

	s.logger.Info("Questions removed from bank successfully", "bank_id", bankID, "question_count", len(questionIDs))
	return nil
}

func (s *questionBankService) GetBankQuestions(ctx context.Context, bankID uint, filters repositories.QuestionFilters, userID string) (*QuestionListResponse, error) {
	// Check access permission
	canAccess, err := s.CanAccess(ctx, bankID, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, bankID, "question_bank", "view_questions", "not owner, not public, or not shared")
	}

	questions, total, err := s.repo.QuestionBank().GetBankQuestions(ctx, nil, bankID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get bank questions: %w", err)
	}

	// Build response
	response := &QuestionListResponse{
		Questions: make([]*QuestionResponse, len(questions)),
		Total:     total,
		Page:      filters.Offset/max(filters.Limit, 1) + 1,
		Size:      filters.Limit,
	}

	for i, question := range questions {
		response.Questions[i] = s.buildQuestionResponse(ctx, question, userID)
	}

	return response, nil
}

// ===== STATISTICS =====

func (s *questionBankService) GetStats(ctx context.Context, bankID uint, userID string) (*repositories.QuestionBankStats, error) {
	// Check access permission
	canAccess, err := s.CanAccess(ctx, bankID, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, bankID, "question_bank", "view_stats", "not owner, not public, or not shared")
	}

	stats, err := s.repo.QuestionBank().GetBankStats(ctx, nil, bankID)
	if err != nil {
		return nil, fmt.Errorf("failed to get bank stats: %w", err)
	}

	return stats, nil
}

// ===== PERMISSION CHECKS =====

func (s *questionBankService) CanAccess(ctx context.Context, bankID uint, userID string) (bool, error) {
	return s.repo.QuestionBank().CanAccess(ctx, nil, bankID, userID)
}

func (s *questionBankService) CanEdit(ctx context.Context, bankID uint, userID string) (bool, error) {
	return s.repo.QuestionBank().CanEdit(ctx, nil, bankID, userID)
}

func (s *questionBankService) CanDelete(ctx context.Context, bankID uint, userID string) (bool, error) {
	return s.repo.QuestionBank().CanDelete(ctx, nil, bankID, userID)
}

func (s *questionBankService) IsOwner(ctx context.Context, bankID uint, userID string) (bool, error) {
	return s.repo.QuestionBank().IsOwner(ctx, nil, bankID, userID)
}

// ===== HELPER METHODS =====

func (s *questionBankService) buildQuestionBankResponse(ctx context.Context, bank *models.QuestionBank, userID string) *QuestionBankResponse {
	response := &QuestionBankResponse{
		QuestionBank: bank,
		IsOwner:      bank.CreatedBy == userID,
	}

	// Determine access level
	if bank.CreatedBy == userID {
		response.AccessLevel = "owner"
		response.CanEdit = true
		response.CanDelete = true
	} else if bank.IsPublic {
		response.AccessLevel = "public"
		response.CanEdit = false
		response.CanDelete = false
	} else {
		// Check if user has share permissions
		canEdit, _ := s.repo.QuestionBank().CanEdit(ctx, nil, bank.ID, userID)
		canDelete, _ := s.repo.QuestionBank().CanDelete(ctx, nil, bank.ID, userID)

		response.CanEdit = canEdit
		response.CanDelete = canDelete

		if canDelete {
			response.AccessLevel = "admin"
		} else if canEdit {
			response.AccessLevel = "editor"
		} else {
			response.AccessLevel = "viewer"
		}
	}

	// Get question count
	// If Questions relation is preloaded (not nil), use len()
	// Otherwise, query the count from database
	if bank.Questions != nil {
		response.QuestionCount = len(bank.Questions)
	} else {
		// Query count from database
		count, err := s.repo.QuestionBank().CountQuestionsInBank(ctx, nil, bank.ID)
		if err != nil {
			s.logger.Warn("Failed to count questions in bank", "bank_id", bank.ID, "error", err)
			response.QuestionCount = 0
		} else {
			response.QuestionCount = count
		}
	}

	// Get share count
	response.ShareCount = len(bank.SharedWith)

	return response
}

func (s *questionBankService) buildQuestionResponse(ctx context.Context, question *models.Question, userID string) *QuestionResponse {
	// This is a simplified version - you might want to reuse the question service's buildResponse method
	response := &QuestionResponse{
		Question:   question,
		CanEdit:    question.CreatedBy == userID,
		CanDelete:  question.CreatedBy == userID,
		UsageCount: question.UsageCount,
	}

	return response
}

func (s *questionBankService) getUserRole(ctx context.Context, userID string) (models.Role, error) {
	user, err := s.repo.User().GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %w", err)
	}
	return user.Role, nil
}

func (s *questionBankService) getAccessibleBanks(ctx context.Context, filters repositories.QuestionBankFilters, userID string) (*QuestionBankListResponse, error) {
	// This is a simplified implementation
	// In a real scenario, you'd want to combine owned, public, and shared banks efficiently

	// For now, just return owned banks
	filters.CreatedBy = &userID

	banks, total, err := s.repo.QuestionBank().List(ctx, nil, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get accessible banks: %w", err)
	}

	// Build response
	response := &QuestionBankListResponse{
		Banks: make([]*QuestionBankResponse, len(banks)),
		Total: total,
		Page:  (filters.Offset / max(filters.Limit, 1)) + 1,
		Size:  filters.Limit,
	}

	for i, bank := range banks {
		response.Banks[i] = s.buildQuestionBankResponse(ctx, bank, userID)
	}

	return response, nil
}
