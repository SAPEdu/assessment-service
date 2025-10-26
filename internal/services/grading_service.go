package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/SAP-F-2025/assessment-service/internal/validator"
	"gorm.io/gorm"
)

type gradingService struct {
	db             *gorm.DB
	repo           repositories.Repository
	logger         *slog.Logger
	validator      *validator.Validator
	attemptService AttemptService
}

func NewGradingService(db *gorm.DB, repo repositories.Repository, logger *slog.Logger, validator *validator.Validator) GradingService {
	return &gradingService{
		db:             db,
		repo:           repo,
		logger:         logger,
		validator:      validator,
		attemptService: NewAttemptService(repo, db, logger, validator, nil),
	}
}

// ===== MANUAL GRADING =====

// GradeAnswer grades a single answer manually
func (s *gradingService) GradeAnswer(ctx context.Context, answerID uint, score float64, feedback *string, graderID string) (*GradingResult, error) {
	s.logger.Info("Manually grading answer",
		"answer_id", answerID,
		"score", score,
		"grader_id", graderID)

	// Get answer with question details
	answer, err := s.repo.Answer().GetByIDWithDetails(ctx, nil, answerID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, fmt.Errorf("answer not found")
		}
		return nil, fmt.Errorf("failed to get answer: %w", err)
	}

	// Check grading permissions
	if err := s.checkGradingPermission(ctx, answer, graderID); err != nil {
		return nil, err
	}

	// Validate score
	maxScore := float64(answer.Question.Points)
	if score < 0 || score > maxScore {
		return nil, NewValidationError("score", "score must be between 0 and max points", score)
	}

	// Update answer with grade
	answer.Score = score
	answer.Feedback = feedback
	answer.GradedBy = &graderID
	answer.GradedAt = timePtr(time.Now())
	answer.IsGraded = true

	if err := s.repo.Answer().Update(ctx, nil, answer); err != nil {
		return nil, fmt.Errorf("failed to update answer grade: %w", err)
	}

	result := &GradingResult{
		AnswerID:      answerID,
		QuestionID:    answer.QuestionID,
		Score:         score,
		MaxScore:      maxScore,
		IsCorrect:     score == maxScore,
		PartialCredit: score > 0 && score < maxScore,
		Feedback:      feedback,
		GradedAt:      time.Now(),
		GradedBy:      &graderID,
	}

	s.logger.Info("Answer graded successfully",
		"answer_id", answerID,
		"score", score,
		"max_score", maxScore)

	// Update attempt grade if all questions are graded
	go s.updateAttemptGradeIfComplete(answer.AttemptID)

	return result, nil
}

// GradeAttempt
// DEPRECATED: This method is deprecated. Use AutoGradeAttempt instead to grade all answers in a batch
func (s *gradingService) GradeAttempt(ctx context.Context, attemptID uint, graderID string) (*AttemptGradingResult, error) {
	s.logger.Info("Manually grading attempt",
		"attempt_id", attemptID,
		"grader_id", graderID)

	// Begin transaction to ensure atomicity
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Ensure rollback on error
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			s.logger.Error("Panic during grading, rolled back", "attempt_id", attemptID, "panic", r)
		}
	}()

	// Get attempt with details
	attempt, err := s.repo.Attempt().GetByIDWithDetails(ctx, tx, attemptID)
	if err != nil {
		tx.Rollback()
		if repositories.IsNotFoundError(err) {
			return nil, fmt.Errorf("attempt not found")
		}
		return nil, fmt.Errorf("failed to get attempt: %w", err)
	}

	// Check grading permissions
	assessmentService := NewAssessmentService(s.repo, tx, s.logger, s.validator)
	canAccess, err := assessmentService.CanAccess(ctx, attempt.AssessmentID, graderID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if !canAccess {
		tx.Rollback()
		return nil, NewPermissionError(graderID, attempt.AssessmentID, "assessment", "grade", "not owner or insufficient permissions")
	}

	// Get all answers for attempt
	answers, err := s.repo.Answer().GetByAttempt(ctx, tx, attemptID)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to get attempt answers: %w", err)
	}

	// Use batch auto-grading for all ungraded answers
	questionResults, err := s.autoGradeAnswers(ctx, tx, answers, attempt.AssessmentID)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to auto-grade answers: %w", err)
	}

	// Calculate final grade
	totalScore := 0.0
	maxTotalScore := 0.0
	for _, result := range questionResults {
		totalScore += result.Score
		maxTotalScore += result.MaxScore
	}

	percentage := 0.0
	if maxTotalScore > 0 {
		percentage = (totalScore / maxTotalScore) * 100
	}

	// Get assessment to check passing score
	assessment, err := s.repo.Assessment().GetByID(ctx, tx, attempt.AssessmentID)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to get assessment: %w", err)
	}

	isPassing := percentage >= float64(assessment.PassingScore)
	grade := s.calculateLetterGrade(percentage)

	// Update attempt with final grade
	attempt.Score = totalScore
	attempt.Percentage = percentage
	attempt.Passed = isPassing

	if err := s.repo.Attempt().Update(ctx, tx, attempt); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update attempt grade: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	result := &AttemptGradingResult{
		AttemptID:  attemptID,
		TotalScore: totalScore,
		MaxScore:   maxTotalScore,
		Percentage: percentage,
		IsPassing:  isPassing,
		Grade:      &grade,
		Questions:  questionResults,
		GradedAt:   time.Now(),
		GradedBy:   graderID,
	}

	s.logger.Info("Attempt graded successfully",
		"attempt_id", attemptID,
		"total_score", totalScore,
		"percentage", percentage,
		"is_passing", isPassing)

	return result, nil
}

// DEPRECATED: This method is deprecated. Use GradeAnswer in a loop instead to grade multiple answers individually.
func (s *gradingService) GradeMultipleAnswers(ctx context.Context, grades []repositories.AnswerGrade, graderID string) ([]GradingResult, error) {
	s.logger.Info("Grading multiple answers",
		"count", len(grades),
		"grader_id", graderID)

	results := make([]GradingResult, len(grades))

	// Process each grade in transaction
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for i, grade := range grades {
			result, gradeErr := s.gradeAnswerInTransaction(ctx, tx, grade.ID, grade.Score, grade.Feedback, graderID)
			if gradeErr != nil {
				return fmt.Errorf("failed to grade answer %d: %w", grade.ID, gradeErr)
			}
			results[i] = *result
		}
		var attemptId uint
		if len(grades) > 0 {
			// Get attempt ID from first answer
			err := tx.Model(&models.StudentAnswer{}).
				Select("attempt_id").
				Where("id = ?", grades[0].ID).
				First(&attemptId).Error
			if err != nil {
				return fmt.Errorf("failed to get attempt ID for updating attempt grade: %w", err)
			}

			isPendingGrade, err := s.attemptService.HasPendingManualGrading(ctx, tx, attemptId)
			if err != nil {
				return fmt.Errorf("failed to check pending manual grading for attempt: %w", err)
			}

			if !isPendingGrade {
				// Update attempt grade if all questions are graded
				err = tx.Model(&models.AssessmentAttempt{}).
					Where("id = ?", attemptId).
					Update("is_graded", true).Error
				if err != nil {
					return fmt.Errorf("failed to update attempt grade: %w", err)
				}
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to grade multiple answers: %w", err)
	}

	s.logger.Info("Multiple answers graded successfully", "count", len(grades))

	return results, nil
}

// ===== AUTO GRADING =====

// AutoGradeAnswer auto-grades a single answer
// DEPRECATED: This method is deprecated. Use AutoGradeAttempt instead to auto-grade all answers in a batch
// for better performance and consistency. This method will be removed in a future version.
func (s *gradingService) AutoGradeAnswer(ctx context.Context, answerID uint) (*GradingResult, error) {
	s.logger.Debug("Auto-grading answer", "answer_id", answerID)

	// Get answer with question details
	answer, err := s.repo.Answer().GetByIDWithDetails(ctx, s.db, answerID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, fmt.Errorf("answer not found")
		}
		return nil, fmt.Errorf("failed to get answer: %w", err)
	}

	// Skip if already graded
	if answer.IsGraded {
		return &GradingResult{
			AnswerID:      answerID,
			QuestionID:    answer.QuestionID,
			Score:         answer.Score,
			MaxScore:      float64(answer.Question.Points),
			IsCorrect:     answer.Score == float64(answer.Question.Points),
			PartialCredit: answer.Score > 0 && answer.Score < float64(answer.Question.Points),
			Feedback:      answer.Feedback,
			GradedAt:      *answer.GradedAt,
			GradedBy:      answer.GradedBy,
		}, nil
	}

	// Calculate score based on question type
	score, isCorrect, err := s.CalculateScore(ctx, answer.Question.Type, json.RawMessage(answer.Question.Content), json.RawMessage(answer.Answer))
	if err != nil {
		return nil, fmt.Errorf("failed to calculate score: %w", err)
	}

	// Generate feedback
	feedback, err := s.GenerateFeedback(ctx, answer.Question.Type, json.RawMessage(answer.Question.Content), json.RawMessage(answer.Answer), isCorrect)
	if err != nil {
		s.logger.Warn("Failed to generate feedback", "answer_id", answerID, "error", err)
	}

	// Update answer with auto-grade
	finalScore := score * float64(answer.Question.Points)
	answer.Score = finalScore
	answer.Feedback = feedback
	answer.GradedAt = timePtr(time.Now())
	answer.IsGraded = true
	answer.IsCorrect = &isCorrect
	answer.MaxScore = answer.Question.Points
	// Note: GradedBy is nil for auto-graded answers

	if err := s.repo.Answer().Update(ctx, s.db, answer); err != nil {
		return nil, fmt.Errorf("failed to update answer with auto-grade: %w", err)
	}

	result := &GradingResult{
		AnswerID:      answerID,
		QuestionID:    answer.QuestionID,
		Score:         finalScore,
		MaxScore:      float64(answer.Question.Points),
		IsCorrect:     isCorrect,
		PartialCredit: score > 0 && score < 1.0,
		Feedback:      feedback,
		GradedAt:      time.Now(),
		GradedBy:      nil, // Auto-graded
	}

	s.logger.Debug("Answer auto-graded successfully",
		"answer_id", answerID,
		"score", finalScore,
		"is_correct", isCorrect)

	return result, nil
}

// autoGradeAnswers performs batch auto-grading for multiple answers
// This method handles transaction management internally for consistency
func (s *gradingService) autoGradeAnswers(ctx context.Context, tx *gorm.DB, answers []*models.StudentAnswer, assessmentId uint) ([]GradingResult, error) {
	if len(answers) == 0 {
		return []GradingResult{}, nil
	}

	var result []GradingResult
	var answersToUpdate []*models.StudentAnswer

	// Get assessment questions for points mapping
	assessmentQuestions, err := s.repo.AssessmentQuestion().GetByAssessment(ctx, tx, assessmentId)
	if err != nil {
		return nil, fmt.Errorf("failed to get assessment questions: %w", err)
	}

	mapAssessmentQuestions := make(map[uint]models.AssessmentQuestion)
	for _, aq := range assessmentQuestions {
		mapAssessmentQuestions[aq.QuestionID] = *aq
	}

	// Process each answer
	for _, answer := range answers {
		assessmentQuestion, exists := mapAssessmentQuestions[answer.QuestionID]
		if !exists {
			s.logger.Warn("Question not found in assessment",
				"question_id", answer.QuestionID,
				"assessment_id", assessmentId)
			continue
		}

		// If already graded, include in results but don't update
		//if answer.IsGraded {
		//	result = append(result, GradingResult{
		//		AnswerID:      answer.ID,
		//		QuestionID:    answer.QuestionID,
		//		Score:         answer.Score,
		//		MaxScore:      float64(*assessmentQuestion.Points),
		//		IsCorrect:     answer.Score == float64(*assessmentQuestion.Points),
		//		PartialCredit: answer.Score > 0 && answer.Score < float64(*assessmentQuestion.Points),
		//		Feedback:      answer.Feedback,
		//		GradedAt:      *answer.GradedAt,
		//		GradedBy:      answer.GradedBy,
		//	})
		//	continue
		//}

		// Skip if no answer provided
		if answer.Answer == nil || len(answer.Answer) == 0 {
			// Mark as graded with 0 score
			answer.Score = 0.0
			answer.IsGraded = true
			answer.GradedAt = timePtr(time.Now())
			isCorrect := false
			answer.IsCorrect = &isCorrect
			answersToUpdate = append(answersToUpdate, answer)

			result = append(result, GradingResult{
				AnswerID:   answer.ID,
				QuestionID: answer.QuestionID,
				Score:      0.0,
				MaxScore:   float64(*assessmentQuestion.Points),
				IsCorrect:  false,
				GradedAt:   time.Now(),
				GradedBy:   nil,
			})
			continue
		}

		// Calculate score based on question type
		score, isCorrect, err := s.CalculateScore(ctx, answer.Question.Type,
			json.RawMessage(answer.Question.Content),
			json.RawMessage(answer.Answer))
		if err != nil {
			// If grading fails (e.g., essay type), mark with 0 score
			s.logger.Warn("Failed to calculate score, marking as 0",
				"answer_id", answer.ID,
				"question_type", answer.Question.Type,
				"error", err)
			score = 0.0
			isCorrect = false
		}

		// Generate feedback
		feedback, err := s.GenerateFeedback(ctx, answer.Question.Type,
			json.RawMessage(answer.Question.Content),
			json.RawMessage(answer.Answer),
			isCorrect)
		if err != nil {
			s.logger.Warn("Failed to generate feedback", "answer_id", answer.ID, "error", err)
		}

		// Update answer with auto-grade
		finalScore := score * float64(*assessmentQuestion.Points)
		answer.Score = finalScore
		answer.Feedback = feedback
		answer.GradedAt = timePtr(time.Now())
		answer.IsGraded = true
		answer.IsCorrect = &isCorrect
		answer.UpdatedAt = time.Now()
		answer.MaxScore = *assessmentQuestion.Points
		// Note: GradedBy is nil for auto-graded answers
		if !s.isAutoGradeable(answer.Question.Type) {
			answer.IsGraded = false
		}

		answersToUpdate = append(answersToUpdate, answer)

		result = append(result, GradingResult{
			AnswerID:      answer.ID,
			QuestionID:    answer.QuestionID,
			Score:         finalScore,
			MaxScore:      float64(*assessmentQuestion.Points),
			IsCorrect:     isCorrect,
			PartialCredit: score > 0 && score < 1.0,
			Feedback:      feedback,
			GradedAt:      time.Now(),
			GradedBy:      nil, // Auto-graded
		})
	}

	// Batch update all answers within the provided transaction
	if len(answersToUpdate) > 0 {
		if err := s.repo.Answer().UpdateBatch(ctx, tx, answersToUpdate); err != nil {
			return nil, fmt.Errorf("failed to batch update graded answers: %w", err)
		}

		s.logger.Info("Batch auto-graded answers successfully",
			"count", len(answersToUpdate),
			"total_answers", len(answers))
	}

	return result, nil
}

func (s *gradingService) AutoGradeAttempt(ctx context.Context, attemptID uint) (*AttemptGradingResult, error) {
	s.logger.Info("Auto-grading attempt", "attempt_id", attemptID)

	// Begin transaction to ensure atomicity
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Ensure rollback on error
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			s.logger.Error("Panic during auto-grading, rolled back", "attempt_id", attemptID, "panic", r)
		}
	}()

	// Get attempt with details
	attempt, err := s.repo.Attempt().GetByIDWithDetails(ctx, tx, attemptID)
	if err != nil {
		tx.Rollback()
		if repositories.IsNotFoundError(err) {
			return nil, fmt.Errorf("attempt not found")
		}
		return nil, fmt.Errorf("failed to get attempt: %w", err)
	}

	// Get all answers for attempt
	answers, err := s.repo.Answer().GetByAttempt(ctx, tx, attemptID)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to get attempt answers: %w", err)
	}

	// Auto-grade all gradeable answers (within transaction)
	var questionResults []GradingResult
	totalScore := 0.0
	maxTotalScore := 0.0
	hasManualGrading := false

	questionResults, err = s.autoGradeAnswers(ctx, tx, answers, attempt.AssessmentID)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to auto-grade answers: %w", err)
	}

	for _, answer := range answers {
		if !s.isAutoGradeable(answer.Question.Type) {
			hasManualGrading = true
		}
	}

	for _, result := range questionResults {
		totalScore += result.Score
		maxTotalScore += result.MaxScore
	}

	// Calculate final grade (only if no manual grading required)
	percentage := 0.0
	if maxTotalScore > 0 {
		percentage = (totalScore / maxTotalScore) * 100
	}

	// Get assessment to check passing score
	assessment, err := s.repo.Assessment().GetByID(ctx, tx, attempt.AssessmentID)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to get assessment: %w", err)
	}

	isPassing := percentage >= float64(assessment.PassingScore)
	grade := s.calculateLetterGrade(percentage)

	attempt.Score = totalScore
	attempt.Percentage = percentage
	attempt.Passed = isPassing
	attempt.IsGraded = true
	attempt.MaxScore = int(maxTotalScore)

	if hasManualGrading {
		attempt.IsGraded = false
	}

	if err := s.repo.Attempt().Update(ctx, tx, attempt); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update attempt grade: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	result := &AttemptGradingResult{
		AttemptID:  attemptID,
		TotalScore: totalScore,
		MaxScore:   maxTotalScore,
		Percentage: percentage,
		IsPassing:  isPassing,
		Grade:      &grade,
		Questions:  questionResults,
		GradedAt:   time.Now(),
		GradedBy:   "", // Auto-graded
	}

	s.logger.Info("Attempt auto-graded successfully",
		"attempt_id", attemptID,
		"total_score", totalScore,
		"has_manual_grading", hasManualGrading)

	return result, nil
}

func (s *gradingService) AutoGradeAssessment(ctx context.Context, assessmentID uint) (map[uint]*AttemptGradingResult, error) {
	s.logger.Info("Auto-grading all attempts for assessment", "assessment_id", assessmentID)

	// Get all submitted attempts for assessment
	status := models.AttemptCompleted
	filters := repositories.AttemptFilters{
		Status: &status,
	}

	attempts, _, err := s.repo.Attempt().GetByAssessment(ctx, nil, assessmentID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get assessment attempts: %w", err)
	}

	results := make(map[uint]*AttemptGradingResult)

	// Auto-grade each attempt
	for _, attempt := range attempts {
		result, err := s.AutoGradeAttempt(ctx, attempt.ID)
		if err != nil {
			s.logger.Error("Failed to auto-grade attempt", "attempt_id", attempt.ID, "error", err)
			continue
		}
		results[attempt.ID] = result
	}

	s.logger.Info("Assessment auto-grading completed",
		"assessment_id", assessmentID,
		"attempts_processed", len(results))

	return results, nil
}
