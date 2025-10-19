package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
)

// ===== STATISTICS =====

func (s *questionService) GetStats(ctx context.Context, questionID uint, userID string) (*repositories.QuestionStats, error) {
	// Check access permission
	canAccess, err := s.CanAccess(ctx, questionID, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, questionID, "question", "view_stats", "not owner or insufficient permissions")
	}

	stats, err := s.repo.Question().GetQuestionStats(ctx, nil, questionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get question stats: %w", err)
	}

	return stats, nil
}

func (s *questionService) GetUsageStats(ctx context.Context, creatorID string) (*repositories.QuestionUsageStats, error) {
	stats, err := s.repo.Question().GetUsageStats(ctx, nil, creatorID)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage stats: %w", err)
	}

	return stats, nil
}

// ===== PERMISSION CHECKS =====

func (s *questionService) CanAccess(ctx context.Context, questionID uint, userID string) (bool, error) {
	// Get user role
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return false, err
	}

	// Admin can access all questions
	if userRole == models.RoleAdmin {
		return true, nil
	}

	// Get question to check ownership
	question, err := s.repo.Question().GetByID(ctx, nil, questionID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return false, nil
		}
		return false, err
	}

	// Teachers can access their own questions
	if userRole == models.RoleTeacher && question.CreatedBy == userID {
		return true, nil
	}

	// Teachers can access public questions or questions shared with them
	if userRole == models.RoleTeacher {
		// TODO: Check if question is public or shared
		// For now, allow access to all questions for teachers
		return true, nil
	}

	// Students can access questions that are part of active assessments they can take
	if userRole == models.RoleStudent {
		// TODO: Check if question is part of an accessible assessment
		// For now, deny access to students for individual questions
		return false, nil
	}

	return false, nil
}

func (s *questionService) CanEdit(ctx context.Context, questionID uint, userID string) (bool, error) {
	// Get user role
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return false, err
	}

	// Get question
	question, err := s.repo.Question().GetByID(ctx, nil, questionID)
	if err != nil {
		return false, err
	}

	// Admin can edit all questions
	if userRole == models.RoleAdmin {
		return true, nil
	}

	// Only owners can edit their questions
	if question.CreatedBy != userID {
		return false, nil
	}

	// Teachers can edit their own questions
	if userRole == models.RoleTeacher {
		return true, nil
	}

	return false, nil
}

func (s *questionService) CanDelete(ctx context.Context, questionID uint, userID string) (bool, error) {
	// Get user role
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return false, err
	}

	// Get question
	question, err := s.repo.Question().GetByID(ctx, nil, questionID)
	if err != nil {
		return false, err
	}

	// Only owners or admins can delete
	if userRole != models.RoleAdmin && question.CreatedBy != userID {
		return false, nil
	}

	// Check if question is in use by assessments
	inUse, err := s.repo.Question().IsUsedInAssessments(ctx, nil, questionID)
	if err != nil {
		return false, err
	}

	// Cannot delete if in use (except admin override)
	if inUse && userRole != models.RoleAdmin {
		return false, nil
	}

	return true, nil
}

// ===== HELPER FUNCTIONS =====

func (s *questionService) getUserRole(ctx context.Context, userID string) (models.UserRole, error) {
	user, err := s.repo.User().GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %w", err)
	}
	return user.Role, nil
}

func (s *questionService) canCreateQuestion(ctx context.Context, userID string) (bool, error) {
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return false, err
	}

	return userRole == models.RoleTeacher || userRole == models.RoleAdmin, nil
}

func (s *questionService) canAccessQuestionBank(ctx context.Context, bankID uint, userID string) (bool, error) {
	// Get user role
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return false, err
	}

	// Admin can access all banks
	if userRole == models.RoleAdmin {
		return true, nil
	}

	// Get bank to check ownership/access
	bank, err := s.repo.QuestionBank().GetByID(ctx, nil, bankID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return false, nil
		}
		return false, err
	}

	// Owners can access their banks
	if bank.CreatedBy == userID {
		return true, nil
	}

	// Check if bank is public
	if bank.IsPublic {
		return true, nil
	}

	// TODO: Check if user has been granted access to private bank

	return false, nil
}

func (s *questionService) canEditQuestionBank(ctx context.Context, bankID uint, userID string) (bool, error) {
	// Get user role
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return false, err
	}

	// Admin can edit all banks
	if userRole == models.RoleAdmin {
		return true, nil
	}

	// Get bank to check ownership
	bank, err := s.repo.QuestionBank().GetByID(ctx, nil, bankID)
	if err != nil {
		return false, err
	}

	// Only owners can edit their banks
	return bank.CreatedBy == userID, nil
}

func (s *questionService) buildQuestionResponse(ctx context.Context, question *models.Question, userID string) *QuestionResponse {
	response := &QuestionResponse{
		Question: question,
	}

	// Determine permissions
	canEdit, _ := s.CanEdit(ctx, question.ID, userID)
	canDelete, _ := s.CanDelete(ctx, question.ID, userID)

	response.CanEdit = canEdit
	response.CanDelete = canDelete

	// Get usage count
	stats, err := s.repo.Question().GetQuestionStats(ctx, nil, question.ID)
	if err == nil {
		response.UsageCount = stats.UsageCount
	}

	return response
}

func (s *questionService) applyQuestionUpdates(question *models.Question, req *UpdateQuestionRequest) error {
	if req.Text != nil {
		question.Text = *req.Text
	}

	if req.Content != nil {
		contentBytes, err := json.Marshal(req.Content)
		if err != nil {
			return fmt.Errorf("failed to marshal content: %w", err)
		}
		question.Content = contentBytes
		// Note: Version tracking would need to be added to Question model if needed
	}

	if req.Points != nil {
		question.Points = *req.Points
	}

	// NOTE: TimeLimit is stored but NOT used in attempt timing logic. Assessment.Duration is used instead.
	if req.TimeLimit != nil {
		question.TimeLimit = req.TimeLimit
	}

	if req.Difficulty != nil {
		question.Difficulty = *req.Difficulty
	}

	if req.CategoryID != nil {
		question.CategoryID = req.CategoryID
	}

	if req.Tags != nil {
		tagsJSon, err := json.Marshal(req.Tags)
		if err != nil {
			return fmt.Errorf("failed to marshal tags: %w", err)
		}
		question.Tags = tagsJSon
	}

	if req.Explanation != nil {
		question.Explanation = req.Explanation
	}

	return nil
}

func (s *questionService) validateCategoryAccess(ctx context.Context, categoryID uint, userID string) error {
	// Get category
	category, err := s.repo.QuestionCategory().GetByID(ctx, nil, categoryID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return NewValidationError("category_id", "category not found", categoryID)
		}
		return fmt.Errorf("failed to get category: %w", err)
	}

	// Check if user can access category
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return err
	}

	if userRole != models.RoleAdmin && category.CreatedBy != userID {
		return NewValidationError("category_id", "access denied to category", categoryID)
	}

	return nil
}

// ===== CONTENT VALIDATION =====

func (s *questionService) validateQuestionContent(questionType models.QuestionType, content interface{}) error {
	switch questionType {
	case models.MultipleChoice:
		return s.validateMultipleChoiceContent(content)
	case models.TrueFalse:
		return s.validateTrueFalseContent(content)
	case models.Essay:
		return s.validateEssayContent(content)
	case models.FillInBlank:
		return s.validateFillBlankContent(content)
	case models.Matching:
		return s.validateMatchingContent(content)
	case models.Ordering:
		return s.validateOrderingContent(content)
	case models.ShortAnswer:
		return s.validateShortAnswerContent(content)
	default:
		return NewValidationError("type", "unsupported question type", questionType)
	}
}

func (s *questionService) validateMultipleChoiceContent(content interface{}) error {
	var mcContent models.MultipleChoiceContent

	// Convert content to struct
	if err := s.convertContent(content, &mcContent); err != nil {
		return err
	}

	var errors ValidationErrors

	// Validate options
	if len(mcContent.Options) < 2 {
		errors = append(errors, *NewValidationError("content.options", "must have at least 2 options", len(mcContent.Options)))
	}

	if len(mcContent.Options) > 10 {
		errors = append(errors, *NewValidationError("content.options", "cannot have more than 10 options", len(mcContent.Options)))
	}

	// Validate correct answers
	if len(mcContent.CorrectAnswers) == 0 {
		errors = append(errors, *NewValidationError("content.correct_answers", "must specify at least one correct answer", nil))
	}

	// Validate correct answer option IDs
	optionIDs := make(map[string]bool)
	for _, option := range mcContent.Options {
		optionIDs[option.ID] = true
	}

	for i, correctID := range mcContent.CorrectAnswers {
		if !optionIDs[correctID] {
			errors = append(errors, *NewValidationError(fmt.Sprintf("content.correct_answers[%d]", i), "invalid option ID", correctID))
		}
	}

	// Validate option texts
	for i, option := range mcContent.Options {
		if option.Text == "" {
			errors = append(errors, *NewValidationError(fmt.Sprintf("content.options[%d].text", i), "option text cannot be empty", nil))
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

func (s *questionService) validateTrueFalseContent(content interface{}) error {
	var tfContent models.TrueFalseContent

	if err := s.convertContent(content, &tfContent); err != nil {
		return err
	}

	// Note: CorrectAnswer is a bool, not a pointer, so validation is implicit

	return nil
}

func (s *questionService) validateEssayContent(content interface{}) error {
	var essayContent models.EssayContent

	if err := s.convertContent(content, &essayContent); err != nil {
		return err
	}

	var errors ValidationErrors

	// Validate word limits
	if essayContent.MinWords != nil && essayContent.MaxWords != nil {
		if *essayContent.MinWords > *essayContent.MaxWords {
			errors = append(errors, *NewValidationError("content.min_words", "min_words cannot be greater than max_words", *essayContent.MinWords))
		}
	}

	if essayContent.MinWords != nil && *essayContent.MinWords < 0 {
		errors = append(errors, *NewValidationError("content.min_words", "min_words cannot be negative", *essayContent.MinWords))
	}

	if essayContent.MaxWords != nil && *essayContent.MaxWords <= 0 {
		errors = append(errors, *NewValidationError("content.max_words", "max_words must be positive", *essayContent.MaxWords))
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

func (s *questionService) validateFillBlankContent(content interface{}) error {
	var fbContent models.FillBlankContent

	if err := s.convertContent(content, &fbContent); err != nil {
		return err
	}

	var errors ValidationErrors

	// Validate template has blanks
	if fbContent.Template == "" {
		errors = append(errors, *NewValidationError("content.template", "template cannot be empty", nil))
	}

	// Validate blanks exist
	if len(fbContent.Blanks) == 0 {
		errors = append(errors, *NewValidationError("content.blanks", "must have at least one blank", nil))
	}

	// Validate each blank
	for blankID, blank := range fbContent.Blanks {
		if len(blank.AcceptedAnswers) == 0 {
			errors = append(errors, *NewValidationError(fmt.Sprintf("content.blanks[%s].accepted_answers", blankID), "must have at least one accepted answer", nil))
		}

		if blank.Points <= 0 {
			errors = append(errors, *NewValidationError(fmt.Sprintf("content.blanks[%s].points", blankID), "points must be positive", blank.Points))
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

func (s *questionService) validateMatchingContent(content interface{}) error {
	var matchContent models.MatchingContent

	if err := s.convertContent(content, &matchContent); err != nil {
		return err
	}

	var errors ValidationErrors

	// Validate left and right items
	if len(matchContent.LeftItems) < 2 {
		errors = append(errors, *NewValidationError("content.left_items", "must have at least 2 left items", len(matchContent.LeftItems)))
	}
	if len(matchContent.RightItems) < 2 {
		errors = append(errors, *NewValidationError("content.right_items", "must have at least 2 right items", len(matchContent.RightItems)))
	}

	// Validate left items
	for i, item := range matchContent.LeftItems {
		if item.Text == "" {
			errors = append(errors, *NewValidationError(fmt.Sprintf("content.left_items[%d].text", i), "left item text cannot be empty", nil))
		}
	}

	// Validate right items
	for i, item := range matchContent.RightItems {
		if item.Text == "" {
			errors = append(errors, *NewValidationError(fmt.Sprintf("content.right_items[%d].text", i), "right item text cannot be empty", nil))
		}
	}

	// Validate correct pairs
	if len(matchContent.CorrectPairs) == 0 {
		errors = append(errors, *NewValidationError("content.correct_pairs", "must have at least one correct pair", nil))
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

func (s *questionService) validateOrderingContent(content interface{}) error {
	var orderContent models.OrderingContent

	if err := s.convertContent(content, &orderContent); err != nil {
		return err
	}

	var errors ValidationErrors

	// Validate items
	if len(orderContent.Items) < 2 {
		errors = append(errors, *NewValidationError("content.items", "must have at least 2 items", len(orderContent.Items)))
	}

	// Validate each item
	for i, item := range orderContent.Items {
		if item.Text == "" {
			errors = append(errors, *NewValidationError(fmt.Sprintf("content.items[%d].text", i), "item text cannot be empty", nil))
		}
	}

	// Validate correct order length
	if len(orderContent.CorrectOrder) != len(orderContent.Items) {
		errors = append(errors, *NewValidationError("content.correct_order", "correct order must match number of items", len(orderContent.CorrectOrder)))
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

func (s *questionService) validateShortAnswerContent(content interface{}) error {
	var saContent models.ShortAnswerContent

	if err := s.convertContent(content, &saContent); err != nil {
		return err
	}

	var errors ValidationErrors

	// Validate accepted answers
	if len(saContent.AcceptedAnswers) == 0 {
		errors = append(errors, *NewValidationError("content.accepted_answers", "must have at least one accepted answer", nil))
	}

	// Validate each accepted answer
	for i, answer := range saContent.AcceptedAnswers {
		if answer == "" {
			errors = append(errors, *NewValidationError(fmt.Sprintf("content.accepted_answers[%d]", i), "accepted answer cannot be empty", nil))
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// convertContent converts interface{} content to specific struct type
func (s *questionService) convertContent(content interface{}, target interface{}) error {
	// Convert to JSON and back to ensure proper type conversion
	jsonBytes, err := json.Marshal(content)
	if err != nil {
		return NewValidationError("content", "invalid content format", content)
	}

	if err := json.Unmarshal(jsonBytes, target); err != nil {
		return NewValidationError("content", "content does not match question type", content)
	}

	return nil
}
