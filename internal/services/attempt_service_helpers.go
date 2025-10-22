package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ===== TIME MANAGEMENT =====

func (s *attemptService) GetTimeRemaining(ctx context.Context, attemptID uint, studentID string) (int, error) {
	// Get attempt
	attempt, err := s.repo.Attempt().GetByID(ctx, nil, attemptID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return 0, ErrAttemptNotFound
		}
		return 0, fmt.Errorf("failed to get attempt: %w", err)
	}

	// Verify ownership
	if attempt.StudentID != studentID {
		return 0, NewPermissionError(studentID, attemptID, "attempt", "get_time_remaining", "not owned by student")
	}

	// Check if attempt is active
	if attempt.Status != models.AttemptInProgress {
		return 0, ErrAttemptNotActive
	}

	// Calculate time remaining
	if attempt.EndedAt == nil {
		return 0, nil // No time limit
	}

	remaining := int(time.Until(*attempt.EndedAt).Seconds())
	if remaining < 0 {
		return 0, nil // Time expired
	}

	return remaining, nil
}

func (s *attemptService) ExtendTime(ctx context.Context, attemptID uint, minutes int, userID string) error {
	s.logger.Info("Extending attempt time",
		"attempt_id", attemptID,
		"minutes", minutes,
		"user_id", userID)

	// Get user role
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return err
	}

	// Only teachers/admins can extend time
	if userRole != models.RoleTeacher && userRole != models.RoleAdmin {
		return NewPermissionError(userID, attemptID, "attempt", "extend_time", "insufficient permissions")
	}

	// Get attempt
	attempt, err := s.repo.Attempt().GetByID(ctx, nil, attemptID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return ErrAttemptNotFound
		}
		return fmt.Errorf("failed to get attempt: %w", err)
	}

	// Check if user can access the assessment
	assessmentService := NewAssessmentService(s.repo, s.db, s.logger, s.validator)
	canAccess, err := assessmentService.CanAccess(ctx, attempt.AssessmentID, userID)
	if err != nil {
		return err
	}
	if !canAccess {
		return NewPermissionError(userID, attempt.AssessmentID, "assessment", "extend_attempt_time", "not owner or insufficient permissions")
	}

	// Check if attempt is active
	if attempt.Status != models.AttemptInProgress {
		return ErrAttemptNotActive
	}

	// Extend time
	if attempt.EndedAt != nil {
		newEndTime := attempt.EndedAt.Add(time.Duration(minutes) * time.Minute)
		attempt.EndedAt = &newEndTime
	}

	if err := s.repo.Attempt().Update(ctx, nil, attempt); err != nil {
		return fmt.Errorf("failed to extend attempt time: %w", err)
	}

	s.logger.Info("Attempt time extended successfully",
		"attempt_id", attemptID,
		"new_end_time", attempt.EndedAt)

	return nil
}

func (s *attemptService) HandleTimeout(ctx context.Context, attemptID uint) error {
	s.logger.Info("Handling attempt timeout", "attempt_id", attemptID)

	// Get attempt
	attempt, err := s.repo.Attempt().GetByID(ctx, nil, attemptID)
	if err != nil {
		return fmt.Errorf("failed to get attempt: %w", err)
	}

	// Check if attempt is active
	if attempt.Status != models.AttemptInProgress {
		return nil // Already handled
	}

	// Update attempt status to timeout
	attempt.Status = models.AttemptTimeOut
	timeoutReason := models.AttemptEndReasonTimeout
	attempt.EndReason = &timeoutReason
	attempt.CompletedAt = timePtr(time.Now())

	if err := s.repo.Attempt().Update(ctx, nil, attempt); err != nil {
		return fmt.Errorf("failed to update attempt status: %w", err)
	}

	s.logger.Info("Attempt timeout handled successfully", "attempt_id", attemptID)

	// Auto-grade timed out attempt
	go func() {
		gradingService := NewGradingService(s.db, s.repo, s.logger, s.validator)
		if _, err := gradingService.AutoGradeAttempt(context.Background(), attemptID); err != nil {
			s.logger.Error("Failed to auto-grade timed out attempt", "attempt_id", attemptID, "error", err)
		}
	}()

	return nil
}

// ===== VALIDATION =====

func (s *attemptService) CanStart(ctx context.Context, assessmentID uint, studentID string) (bool, error) {
	// Check if assessment is available for taking
	assessmentService := NewAssessmentService(s.repo, s.db, s.logger, s.validator)
	canTake, err := assessmentService.CanTake(ctx, assessmentID, studentID)
	if err != nil {
		return false, err
	}
	if !canTake {
		return false, nil
	}

	// Get assessment to check attempt limits
	assessment, err := s.repo.Assessment().GetByID(ctx, s.db, assessmentID)
	if err != nil {
		return false, err
	}

	// Check attempt count
	attemptCount, err := s.GetAttemptCount(ctx, assessmentID, studentID)
	if err != nil {
		return false, err
	}

	if attemptCount >= assessment.MaxAttempts {
		return false, nil
	}

	// Check if student has an active attempt
	currentAttempt, err := s.GetCurrentAttempt(ctx, assessmentID, studentID)
	if err != nil && err != ErrAttemptNotFound {
		return false, err
	}

	// If there's an active attempt, can resume but not start new
	if currentAttempt != nil && currentAttempt.Status == models.AttemptInProgress {
		// Check if it has expired
		if currentAttempt.EndedAt != nil && time.Now().After(*currentAttempt.EndedAt) {
			// Auto-handle timeout
			if err := s.HandleTimeout(ctx, currentAttempt.ID); err != nil {
				s.logger.Error("Failed to handle expired attempt", "attempt_id", currentAttempt.ID, "error", err)
			}
			return true, nil // Can start new attempt after timeout
		}
		return false, nil // Has active attempt, should resume instead
	}

	return true, nil
}

func (s *attemptService) GetAttemptCount(ctx context.Context, assessmentID uint, studentID string) (int, error) {
	count, err := s.repo.Attempt().GetAttemptCount(ctx, nil, studentID, assessmentID)
	if err != nil {
		return 0, fmt.Errorf("failed to get attempt count: %w", err)
	}
	return count, nil
}

func (s *attemptService) IsAttemptActive(ctx context.Context, attemptID uint) (bool, error) {
	attempt, err := s.repo.Attempt().GetByID(ctx, nil, attemptID)
	if err != nil {
		return false, err
	}

	if attempt.Status != models.AttemptInProgress {
		return false, nil
	}

	// Check if time expired
	if attempt.EndedAt != nil && time.Now().After(*attempt.EndedAt) {
		return false, nil
	}

	return true, nil
}

// ===== STATISTICS =====

func (s *attemptService) GetStats(ctx context.Context, assessmentID uint, userID string) (*repositories.AttemptStats, error) {
	// Check access permission
	assessmentService := NewAssessmentService(s.repo, nil, s.logger, s.validator)
	canAccess, err := assessmentService.CanAccess(ctx, assessmentID, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, assessmentID, "assessment", "view_stats", "not owner or insufficient permissions")
	}

	stats, err := s.repo.Attempt().GetAssessmentAttemptStats(ctx, nil, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get attempt stats: %w", err)
	}

	return stats, nil
}

// ===== HELPER FUNCTIONS =====

func (s *attemptService) getUserRole(ctx context.Context, userID string) (models.UserRole, error) {
	user, err := s.repo.User().GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %w", err)
	}
	return user.Role, nil
}

func (s *attemptService) canAccessAttempt(ctx context.Context, attempt *models.AssessmentAttempt, userID string) (bool, error) {
	// Get user role
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return false, err
	}

	// Students can only access their own attempts
	if userRole == models.RoleStudent {
		return attempt.StudentID == userID, nil
	}

	if userRole == models.RoleAdmin {
		return true, nil
	}

	// Teachers/Admins can access attempts for their assessments
	if userRole == models.RoleTeacher {
		assessmentService := NewAssessmentService(s.repo, s.db, s.logger, s.validator)
		return assessmentService.CanAccess(ctx, attempt.AssessmentID, userID)
	}

	return false, nil
}

func (s *attemptService) buildAttemptResponse(ctx context.Context, attempt *models.AssessmentAttempt, userID string, includeQuestions bool) *AttemptResponse {
	response := &AttemptResponse{
		AssessmentAttempt: attempt,
	}

	// Determine permissions
	response.CanSubmit = attempt.Status == models.AttemptInProgress &&
		attempt.StudentID == userID &&
		(attempt.EndedAt == nil || time.Now().Before(*attempt.EndedAt))

	response.CanResume = response.CanSubmit

	// Include questions if requested and user is the student
	if includeQuestions && attempt.StudentID == userID {
		questions, err := s.getAttemptQuestions(ctx, attempt.AssessmentID)
		if err != nil {
			s.logger.Error("Failed to get attempt questions", "attempt_id", attempt.ID, "error", err)
		} else {
			// Check if we should show correct answers
			showCorrectAnswers, err := s.shouldShowCorrectAnswers(ctx, attempt)
			if err != nil {
				s.logger.Error("Failed to check shouldShowCorrectAnswers, defaulting to hide",
					"attempt_id", attempt.ID,
					"error", err)
				showCorrectAnswers = false
			}

			// Sanitize questions if we should not show correct answers
			if !showCorrectAnswers {
				questions = s.removeCorrectAnswersFromQuestions(questions)
				s.logger.Debug("Correct answers removed from questions",
					"attempt_id", attempt.ID,
					"status", attempt.Status,
					"student_id", userID)
			} else {
				s.logger.Debug("Showing correct answers",
					"attempt_id", attempt.ID,
					"status", attempt.Status,
					"student_id", userID)
			}

			response.Questions = questions
		}
	}

	return response
}

func (s *attemptService) getAttemptQuestions(ctx context.Context, assessmentId uint) ([]QuestionForAttempt, error) {
	// Get assessment questions with answers
	assessmentQuestions, err := s.repo.AssessmentQuestion().GetQuestionsForAssessment(ctx, nil, assessmentId)
	if err != nil {
		return nil, fmt.Errorf("failed to get assessment questions: %w", err)
	}

	questions := make([]QuestionForAttempt, len(assessmentQuestions))
	for i, aq := range assessmentQuestions {
		copyAq := *aq // Create a copy to avoid modifying the original
		questions[i] = QuestionForAttempt{
			Question: &copyAq,
			IsFirst:  i == 0,
			IsLast:   i == len(assessmentQuestions)-1,
		}
	}

	return questions, nil
}

func (s *attemptService) initializeAttemptAnswers(ctx context.Context, tx *gorm.DB, attempt *models.AssessmentAttempt, assessment *models.Assessment) error {
	// Get all questions for the assessment
	assessmentQuestions, err := s.repo.AssessmentQuestion().GetByAssessment(ctx, tx, assessment.ID)
	if err != nil {
		return fmt.Errorf("failed to get assessment questions: %w", err)
	}

	// Create empty answers for all questions
	answers := make([]*models.StudentAnswer, len(assessmentQuestions))
	for i, aq := range assessmentQuestions {
		answers[i] = &models.StudentAnswer{
			AttemptID:  attempt.ID,
			QuestionID: aq.QuestionID,
			Answer:     nil, // Empty initially
			Flagged:    false,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
	}

	// Batch create answers
	if err := s.repo.Answer().CreateBatch(ctx, tx, answers); err != nil {
		return fmt.Errorf("failed to create initial answers: %w", err)
	}

	return nil
}

func (s *attemptService) updateAttemptAnswer(ctx context.Context, tx *gorm.DB, attemptID uint, req SubmitAnswerRequest, studentID string) error {
	// Get existing answer
	answer, err := s.repo.Answer().GetByAttemptAndQuestion(ctx, tx, attemptID, req.QuestionID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			// Create new answer if doesn't exist
			answer = &models.StudentAnswer{
				AttemptID:  attemptID,
				QuestionID: req.QuestionID,
			}
		} else {
			return fmt.Errorf("failed to get existing answer: %w", err)
		}
	}

	// Convert answer data to JSON
	if req.AnswerData != nil {
		answerBytes, err := json.Marshal(req.AnswerData)
		if err != nil {
			return fmt.Errorf("failed to marshal answer data: %w", err)
		}
		answer.Answer = answerBytes
	}

	answer.UpdatedAt = time.Now()

	if req.TimeSpent != nil {
		answer.TimeSpent = *req.TimeSpent
	}

	// Upsert answer
	if answer.ID == 0 {
		if err := s.repo.Answer().Create(ctx, tx, answer); err != nil {
			return fmt.Errorf("failed to create answer: %w", err)
		}
	} else {
		if err := s.repo.Answer().Update(ctx, tx, answer); err != nil {
			return fmt.Errorf("failed to update answer: %w", err)
		}
	}

	return nil
}

// ===== ANSWER SANITIZATION HELPERS =====

// shouldShowCorrectAnswers determines if correct answers should be shown based on attempt status and settings
func (s *attemptService) shouldShowCorrectAnswers(ctx context.Context, attempt *models.AssessmentAttempt) (bool, error) {
	// NEVER show correct answers during in_progress
	if attempt.Status == models.AttemptInProgress {
		return false, nil
	}

	// For completed/timeout attempts, check settings
	if attempt.Status == models.AttemptCompleted || attempt.Status == models.AttemptTimeOut {
		settings, err := s.repo.Assessment().GetSettings(ctx, s.db, attempt.AssessmentID)
		if err != nil {
			// If settings not found, default to not showing
			s.logger.Warn("Failed to get assessment settings, defaulting to hide answers",
				"assessment_id", attempt.AssessmentID,
				"error", err)
			return false, nil
		}

		return settings.ShowCorrectAnswers, nil
	}

	// For other statuses (abandoned), don't show
	return false, nil
}

// removeCorrectAnswersFromQuestions removes correct answers from all questions
func (s *attemptService) removeCorrectAnswersFromQuestions(questions []QuestionForAttempt) []QuestionForAttempt {
	sanitized := make([]QuestionForAttempt, len(questions))
	for i, q := range questions {
		sanitized[i] = QuestionForAttempt{
			Question: s.removeCorrectAnswersFromQuestion(q.Question),
			IsFirst:  q.IsFirst,
			IsLast:   q.IsLast,
		}
	}
	return sanitized
}

// removeCorrectAnswersFromQuestion removes correct answer fields from a question
func (s *attemptService) removeCorrectAnswersFromQuestion(question *models.Question) *models.Question {
	if question == nil {
		return nil
	}

	// Create a copy to avoid modifying the original
	sanitized := *question

	// Clear the Answer field (this contains the correct answer)
	sanitized.Answer = nil

	// Sanitize Content based on question type
	if question.Content != nil {
		sanitized.Content = s.sanitizeQuestionContent(question.Type, question.Content)
	}

	return &sanitized
}

// sanitizeQuestionContent removes correct answer information from question content based on type
func (s *attemptService) sanitizeQuestionContent(questionType models.QuestionType, content datatypes.JSON) datatypes.JSON {
	switch questionType {
	case models.MultipleChoice:
		return s.sanitizeMultipleChoiceContent(content)
	case models.TrueFalse:
		return s.sanitizeTrueFalseContent(content)
	case models.Essay:
		return s.sanitizeEssayContent(content)
	case models.FillInBlank:
		return s.sanitizeFillBlankContent(content)
	case models.Matching:
		return s.sanitizeMatchingContent(content)
	case models.Ordering:
		return s.sanitizeOrderingContent(content)
	case models.ShortAnswer:
		return s.sanitizeShortAnswerContent(content)
	default:
		return content
	}
}

func (s *attemptService) sanitizeMultipleChoiceContent(content datatypes.JSON) datatypes.JSON {
	var mc models.MultipleChoiceContent
	if err := json.Unmarshal(content, &mc); err != nil {
		s.logger.Error("Failed to unmarshal multiple choice content", "error", err)
		return content
	}

	// Remove correct answers
	mc.CorrectAnswers = nil

	sanitized, err := json.Marshal(mc)
	if err != nil {
		s.logger.Error("Failed to marshal sanitized multiple choice content", "error", err)
		return content
	}

	return sanitized
}

func (s *attemptService) sanitizeTrueFalseContent(content datatypes.JSON) datatypes.JSON {
	var tf map[string]interface{}
	if err := json.Unmarshal(content, &tf); err != nil {
		s.logger.Error("Failed to unmarshal true/false content", "error", err)
		return content
	}

	// Remove correct answer
	delete(tf, "correct_answer")

	sanitized, err := json.Marshal(tf)
	if err != nil {
		s.logger.Error("Failed to marshal sanitized true/false content", "error", err)
		return content
	}

	return sanitized
}

func (s *attemptService) sanitizeEssayContent(content datatypes.JSON) datatypes.JSON {
	var essay map[string]interface{}
	if err := json.Unmarshal(content, &essay); err != nil {
		s.logger.Error("Failed to unmarshal essay content", "error", err)
		return content
	}

	// Remove sample answer and keywords used for auto-grading
	delete(essay, "sample_answer")
	delete(essay, "key_words")

	sanitized, err := json.Marshal(essay)
	if err != nil {
		s.logger.Error("Failed to marshal sanitized essay content", "error", err)
		return content
	}

	return sanitized
}

func (s *attemptService) sanitizeFillBlankContent(content datatypes.JSON) datatypes.JSON {
	var fb map[string]interface{}
	if err := json.Unmarshal(content, &fb); err != nil {
		s.logger.Error("Failed to unmarshal fill blank content", "error", err)
		return content
	}

	// Remove accepted answers from blanks
	if blanks, ok := fb["blanks"].(map[string]interface{}); ok {
		for key, blank := range blanks {
			if blankMap, ok := blank.(map[string]interface{}); ok {
				delete(blankMap, "accepted_answers")
				blanks[key] = blankMap
			}
		}
	}

	sanitized, err := json.Marshal(fb)
	if err != nil {
		s.logger.Error("Failed to marshal sanitized fill blank content", "error", err)
		return content
	}

	return sanitized
}

func (s *attemptService) sanitizeMatchingContent(content datatypes.JSON) datatypes.JSON {
	var matching map[string]interface{}
	if err := json.Unmarshal(content, &matching); err != nil {
		s.logger.Error("Failed to unmarshal matching content", "error", err)
		return content
	}

	// Remove correct pairs
	delete(matching, "correct_pairs")

	sanitized, err := json.Marshal(matching)
	if err != nil {
		s.logger.Error("Failed to marshal sanitized matching content", "error", err)
		return content
	}

	return sanitized
}

func (s *attemptService) sanitizeOrderingContent(content datatypes.JSON) datatypes.JSON {
	var ordering map[string]interface{}
	if err := json.Unmarshal(content, &ordering); err != nil {
		s.logger.Error("Failed to unmarshal ordering content", "error", err)
		return content
	}

	// Remove correct order
	delete(ordering, "correct_order")

	sanitized, err := json.Marshal(ordering)
	if err != nil {
		s.logger.Error("Failed to marshal sanitized ordering content", "error", err)
		return content
	}

	return sanitized
}

func (s *attemptService) sanitizeShortAnswerContent(content datatypes.JSON) datatypes.JSON {
	var sa map[string]interface{}
	if err := json.Unmarshal(content, &sa); err != nil {
		s.logger.Error("Failed to unmarshal short answer content", "error", err)
		return content
	}

	// Remove accepted answers
	delete(sa, "accepted_answers")

	sanitized, err := json.Marshal(sa)
	if err != nil {
		s.logger.Error("Failed to marshal sanitized short answer content", "error", err)
		return content
	}

	return sanitized
}
