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
	db        *gorm.DB
	repo      repositories.Repository
	logger    *slog.Logger
	validator *validator.Validator
}

func NewGradingService(db *gorm.DB, repo repositories.Repository, logger *slog.Logger, validator *validator.Validator) GradingService {
	return &gradingService{
		db:        db,
		repo:      repo,
		logger:    logger,
		validator: validator,
	}
}

// ===== MANUAL GRADING =====

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

func (s *gradingService) GradeAttempt(ctx context.Context, attemptID uint, graderID string) (*AttemptGradingResult, error) {
	s.logger.Info("Manually grading attempt",
		"attempt_id", attemptID,
		"grader_id", graderID)

	// Get attempt with details
	attempt, err := s.repo.Attempt().GetByIDWithDetails(ctx, nil, attemptID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, fmt.Errorf("attempt not found")
		}
		return nil, fmt.Errorf("failed to get attempt: %w", err)
	}

	// Check grading permissions
	assessmentService := NewAssessmentService(s.repo, s.db, s.logger, s.validator)
	canAccess, err := assessmentService.CanAccess(ctx, attempt.AssessmentID, graderID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(graderID, attempt.AssessmentID, "assessment", "grade", "not owner or insufficient permissions")
	}

	// Get all answers for attempt
	answers, err := s.repo.Answer().GetByAttempt(ctx, nil, attemptID)
	if err != nil {
		return nil, fmt.Errorf("failed to get attempt answers: %w", err)
	}

	// Grade all ungraded answers automatically first
	var questionResults []GradingResult
	totalScore := 0.0
	maxTotalScore := 0.0

	for _, answer := range answers {
		var result *GradingResult

		// If not already graded, try auto-grading
		if !answer.IsGraded {
			result, err = s.AutoGradeAnswer(ctx, answer.ID)
			if err != nil {
				s.logger.Warn("Failed to auto-grade answer", "answer_id", answer.ID, "error", err)
				// Create zero-score result for ungradeable answers
				result = &GradingResult{
					AnswerID:   answer.ID,
					QuestionID: answer.QuestionID,
					Score:      0,
					MaxScore:   float64(answer.Question.Points),
					IsCorrect:  false,
					GradedAt:   time.Now(),
					GradedBy:   &graderID,
				}
			}
		} else {
			// Use existing grade
			result = &GradingResult{
				AnswerID:      answer.ID,
				QuestionID:    answer.QuestionID,
				Score:         answer.Score,
				MaxScore:      float64(answer.Question.Points),
				IsCorrect:     answer.Score == float64(answer.Question.Points),
				PartialCredit: answer.Score > 0 && answer.Score < float64(answer.Question.Points),
				Feedback:      answer.Feedback,
				GradedAt:      *answer.GradedAt,
				GradedBy:      answer.GradedBy,
			}
		}

		questionResults = append(questionResults, *result)
		totalScore += result.Score
		maxTotalScore += result.MaxScore
	}

	// Calculate final grade
	percentage := 0.0
	if maxTotalScore > 0 {
		percentage = (totalScore / maxTotalScore) * 100
	}

	// Get assessment to check passing score
	assessment, err := s.repo.Assessment().GetByID(ctx, nil, attempt.AssessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get assessment: %w", err)
	}

	isPassing := percentage >= float64(assessment.PassingScore)
	grade := s.calculateLetterGrade(percentage)

	// Update attempt with final grade
	attempt.Score = totalScore
	attempt.Percentage = percentage
	attempt.Passed = isPassing

	if err := s.repo.Attempt().Update(ctx, nil, attempt); err != nil {
		return nil, fmt.Errorf("failed to update attempt grade: %w", err)
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

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to grade multiple answers: %w", err)
	}

	s.logger.Info("Multiple answers graded successfully", "count", len(grades))

	return results, nil
}

// ===== AUTO GRADING =====

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

func (s *gradingService) AutoGradeAttempt(ctx context.Context, attemptID uint) (*AttemptGradingResult, error) {
	s.logger.Info("Auto-grading attempt", "attempt_id", attemptID)

	// Get attempt with details
	attempt, err := s.repo.Attempt().GetByIDWithDetails(ctx, s.db, attemptID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, fmt.Errorf("attempt not found")
		}
		return nil, fmt.Errorf("failed to get attempt: %w", err)
	}

	// Get all answers for attempt
	answers, err := s.repo.Answer().GetByAttempt(ctx, s.db, attemptID)
	if err != nil {
		return nil, fmt.Errorf("failed to get attempt answers: %w", err)
	}

	// Auto-grade all gradeable answers
	var questionResults []GradingResult
	totalScore := 0.0
	maxTotalScore := 0.0
	hasManualGrading := false

	for _, answer := range answers {
		var result *GradingResult

		if !answer.IsGraded {
			// Try auto-grading
			if s.isAutoGradeable(answer.Question.Type) {
				result, err = s.AutoGradeAnswer(ctx, answer.ID)
				if err != nil {
					s.logger.Warn("Failed to auto-grade answer", "answer_id", answer.ID, "error", err)
					continue // Skip ungradeable answers
				}
			} else {
				// Requires manual grading
				hasManualGrading = true
				continue
			}
		} else {
			// Use existing grade
			result = &GradingResult{
				AnswerID:      answer.ID,
				QuestionID:    answer.QuestionID,
				Score:         answer.Score,
				MaxScore:      float64(answer.Question.Points),
				IsCorrect:     answer.Score == float64(answer.Question.Points),
				PartialCredit: answer.Score > 0 && answer.Score < float64(answer.Question.Points),
				Feedback:      answer.Feedback,
				GradedAt:      *answer.GradedAt,
				GradedBy:      answer.GradedBy,
			}
		}

		questionResults = append(questionResults, *result)
		totalScore += result.Score
		maxTotalScore += result.MaxScore
	}

	// Calculate final grade (only if no manual grading required)
	percentage := 0.0
	if maxTotalScore > 0 {
		percentage = (totalScore / maxTotalScore) * 100
	}

	// Get assessment to check passing score
	assessment, err := s.repo.Assessment().GetByID(ctx, s.db, attempt.AssessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get assessment: %w", err)
	}

	isPassing := percentage >= float64(assessment.PassingScore)
	grade := s.calculateLetterGrade(percentage)

	// Update attempt only if fully graded
	if !hasManualGrading {
		attempt.Score = totalScore
		attempt.Percentage = percentage
		attempt.Passed = isPassing
		// GradedBy is nil for auto-graded attempts

		if err := s.repo.Attempt().Update(ctx, s.db, attempt); err != nil {
			return nil, fmt.Errorf("failed to update attempt grade: %w", err)
		}
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
