package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"gorm.io/gorm"
)

// ===== GRADING UTILITIES =====

func (s *gradingService) CalculateScore(ctx context.Context, questionType models.QuestionType, questionContent json.RawMessage, studentAnswer json.RawMessage) (float64, bool, error) {
	if studentAnswer == nil {
		return 0.0, false, nil // No answer provided
	}

	switch questionType {
	case models.MultipleChoice:
		return s.gradeMultipleChoice(questionContent, studentAnswer)
	case models.TrueFalse:
		return s.gradeTrueFalse(questionContent, studentAnswer)
	case models.FillInBlank:
		return s.gradeFillBlank(questionContent, studentAnswer)
	case models.ShortAnswer:
		return s.gradeShortAnswer(questionContent, studentAnswer)
	case models.Matching:
		return s.gradeMatching(questionContent, studentAnswer)
	case models.Ordering:
		return s.gradeOrdering(questionContent, studentAnswer)
	case models.Essay:
		// Essays require manual grading
		return 0.0, false, ErrGradingNotAllowed
	default:
		return 0.0, false, fmt.Errorf("unsupported question type: %s", questionType)
	}
}

func (s *gradingService) GenerateFeedback(ctx context.Context, questionType models.QuestionType, questionContent json.RawMessage, studentAnswer json.RawMessage, isCorrect bool) (*string, error) {
	var feedback string

	switch questionType {
	case models.MultipleChoice:
		feedback = s.generateMultipleChoiceFeedback(questionContent, studentAnswer, isCorrect)
	case models.TrueFalse:
		feedback = s.generateTrueFalseFeedback(questionContent, studentAnswer, isCorrect)
	case models.FillInBlank:
		feedback = s.generateFillBlankFeedback(questionContent, studentAnswer, isCorrect)
	case models.ShortAnswer:
		feedback = s.generateShortAnswerFeedback(questionContent, studentAnswer, isCorrect)
	case models.Matching:
		feedback = s.generateMatchingFeedback(questionContent, studentAnswer, isCorrect)
	case models.Ordering:
		feedback = s.generateOrderingFeedback(questionContent, studentAnswer, isCorrect)
	case models.Essay:
		feedback = "Essay questions require manual grading."
	default:
		if isCorrect {
			feedback = "Correct answer!"
		} else {
			feedback = "Incorrect answer."
		}
	}

	return &feedback, nil
}

// ===== BULK OPERATIONS =====

func (s *gradingService) ReGradeQuestion(ctx context.Context, questionID uint, userID string) ([]GradingResult, error) {
	s.logger.Info("Re-grading all answers for question", "question_id", questionID, "user_id", userID)

	// Check permission to regrade (must be able to access question)
	questionService := NewQuestionService(s.repo, s.db, s.logger, s.validator)
	canAccess, err := questionService.CanAccess(ctx, questionID, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, questionID, "question", "regrade", "not owner or insufficient permissions")
	}

	// Get all answers for this question
	answers, err := s.repo.Answer().GetByQuestion(ctx, nil, questionID, repositories.AnswerFilters{})
	if err != nil {
		return nil, fmt.Errorf("failed to get answers for question: %w", err)
	}

	var results []GradingResult

	// Re-grade each answer
	for _, answer := range answers {
		result, err := s.AutoGradeAnswer(ctx, answer.ID)
		if err != nil {
			s.logger.Error("Failed to re-grade answer", "answer_id", answer.ID, "error", err)
			continue
		}
		results = append(results, *result)
	}

	s.logger.Info("Question re-grading completed",
		"question_id", questionID,
		"answers_processed", len(results))

	return results, nil
}

func (s *gradingService) ReGradeAssessment(ctx context.Context, assessmentID uint, userID string) (map[uint]*AttemptGradingResult, error) {
	s.logger.Info("Re-grading all attempts for assessment", "assessment_id", assessmentID, "user_id", userID)

	// Check permission
	assessmentService := NewAssessmentService(s.repo, s.db, s.logger, s.validator)
	canAccess, err := assessmentService.CanAccess(ctx, assessmentID, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, assessmentID, "assessment", "regrade", "not owner or insufficient permissions")
	}

	// Get all attempts for assessment
	attempts, _, err := s.repo.Attempt().GetByAssessment(ctx, nil, assessmentID, repositories.AttemptFilters{})
	if err != nil {
		return nil, fmt.Errorf("failed to get assessment attempts: %w", err)
	}

	results := make(map[uint]*AttemptGradingResult)

	// Re-grade each attempt
	for _, attempt := range attempts {
		if attempt.Status == models.AttemptCompleted || attempt.Status == models.AttemptTimeOut {
			result, err := s.AutoGradeAttempt(ctx, attempt.ID)
			if err != nil {
				s.logger.Error("Failed to re-grade attempt", "attempt_id", attempt.ID, "error", err)
				continue
			}
			results[attempt.ID] = result
		}
	}

	s.logger.Info("Assessment re-grading completed",
		"assessment_id", assessmentID,
		"attempts_processed", len(results))

	return results, nil
}

// ===== STATISTICS =====

func (s *gradingService) GetGradingOverview(ctx context.Context, assessmentID uint, userID string) (*repositories.GradingStats, error) {
	// Check permission
	assessmentService := NewAssessmentService(s.repo, s.db, s.logger, s.validator)
	canAccess, err := assessmentService.CanAccess(ctx, assessmentID, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, assessmentID, "assessment", "view_grading_overview", "not owner or insufficient permissions")
	}

	// Get grading stats
	stats, err := s.repo.Answer().GetGradingStats(ctx, nil, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get grading stats: %w", err)
	}

	return stats, nil
}

// ===== QUESTION TYPE SPECIFIC GRADING =====

func (s *gradingService) gradeMultipleChoice(questionContent json.RawMessage, studentAnswer json.RawMessage) (float64, bool, error) {
	var content models.MultipleChoiceContent
	if err := json.Unmarshal(questionContent, &content); err != nil {
		return 0.0, false, fmt.Errorf("failed to unmarshal question content: %w", err)
	}

	var answer []string
	if err := json.Unmarshal(studentAnswer, &answer); err != nil {
		var singleAnswer string
		if err = json.Unmarshal(studentAnswer, &singleAnswer); err != nil {
			return 0.0, false, fmt.Errorf("failed to unmarshal student answer: %w", err)
		} else {
			answer = []string{singleAnswer}
		}
	}

	correctAnswers := content.CorrectAnswers

	// Perfect match scoring
	if reflect.DeepEqual(sortStrings(answer), sortStrings(correctAnswers)) {
		return 1.0, true, nil
	}

	// Partial credit scoring for multiple correct answers
	if len(correctAnswers) > 1 {
		correct := 0
		incorrect := 0

		answerSet := make(map[string]bool)
		for _, a := range answer {
			answerSet[a] = true
		}

		correctSet := make(map[string]bool)
		for _, c := range correctAnswers {
			correctSet[c] = true
		}

		// Count correct selections
		for _, a := range answer {
			if correctSet[a] {
				correct++
			} else {
				incorrect++
			}
		}

		// Count missed correct answers
		for _, c := range correctAnswers {
			if !answerSet[c] {
				incorrect++
			}
		}

		// Calculate partial credit (at least 0)
		totalCorrect := len(correctAnswers)
		score := float64(correct-incorrect) / float64(totalCorrect)
		return math.Max(0.0, score), false, nil
	}

	return 0.0, false, nil
}

func (s *gradingService) gradeTrueFalse(questionContent json.RawMessage, studentAnswer json.RawMessage) (float64, bool, error) {
	var content models.TrueFalseContent
	if err := json.Unmarshal(questionContent, &content); err != nil {
		return 0.0, false, fmt.Errorf("failed to unmarshal question content: %w", err)
	}

	var answer bool
	if err := json.Unmarshal(studentAnswer, &answer); err != nil {
		return 0.0, false, fmt.Errorf("failed to unmarshal student answer: %w", err)
	}

	if answer == content.CorrectAnswer {
		return 1.0, true, nil
	}

	return 0.0, false, nil
}

func (s *gradingService) gradeFillBlank(questionContent json.RawMessage, studentAnswer json.RawMessage) (float64, bool, error) {
	var content models.FillBlankContent
	if err := json.Unmarshal(questionContent, &content); err != nil {
		return 0.0, false, fmt.Errorf("failed to unmarshal question content: %w", err)
	}

	var answers map[string]string
	if err := json.Unmarshal(studentAnswer, &answers); err != nil {
		return 0.0, false, fmt.Errorf("failed to unmarshal student answer: %w", err)
	}

	totalPoints := 0
	earnedPoints := 0
	allCorrect := true

	for blankID, blankDef := range content.Blanks {
		totalPoints += blankDef.Points

		studentAns, exists := answers[blankID]
		if !exists {
			allCorrect = false
			continue
		}

		// Check against accepted answers
		correct := false
		for _, accepted := range blankDef.AcceptedAnswers {
			if s.compareStrings(studentAns, accepted, content.CaseSensitive) {
				correct = true
				break
			}
		}

		if correct {
			earnedPoints += blankDef.Points
		} else {
			allCorrect = false
		}
	}

	if totalPoints == 0 {
		return 0.0, false, nil
	}

	score := float64(earnedPoints) / float64(totalPoints)
	return score, allCorrect, nil
}

func (s *gradingService) gradeShortAnswer(questionContent json.RawMessage, studentAnswer json.RawMessage) (float64, bool, error) {
	var content models.ShortAnswerContent
	if err := json.Unmarshal(questionContent, &content); err != nil {
		return 0.0, false, fmt.Errorf("failed to unmarshal question content: %w", err)
	}

	var answer string
	if err := json.Unmarshal(studentAnswer, &answer); err != nil {
		return 0.0, false, fmt.Errorf("failed to unmarshal student answer: %w", err)
	}

	// Check against accepted answers
	for _, accepted := range content.AcceptedAnswers {
		if s.compareStrings(answer, accepted, content.CaseSensitive) {
			return 1.0, true, nil
		}
	}

	// Fuzzy matching for partial credit
	if content.FuzzyMatching {
		bestMatch := 0.0
		for _, accepted := range content.AcceptedAnswers {
			similarity := s.calculateStringSimilarity(answer, accepted)
			if similarity > bestMatch {
				bestMatch = similarity
			}
		}

		// Award partial credit if similarity is above threshold
		if bestMatch >= 0.8 {
			return bestMatch, false, nil
		}
	}

	return 0.0, false, nil
}

func (s *gradingService) gradeMatching(questionContent json.RawMessage, studentAnswer json.RawMessage) (float64, bool, error) {
	var content models.MatchingContent
	if err := json.Unmarshal(questionContent, &content); err != nil {
		return 0.0, false, fmt.Errorf("failed to unmarshal question content: %w", err)
	}

	var answers map[string]string // left -> right mappings
	if err := json.Unmarshal(studentAnswer, &answers); err != nil {
		return 0.0, false, fmt.Errorf("failed to unmarshal student answer: %w", err)
	}

	// Build correct mappings
	correctMappings := make(map[string]string)
	for _, pair := range content.CorrectPairs {
		correctMappings[pair.LeftID] = pair.RightID
	}

	correct := 0
	total := len(content.CorrectPairs)

	for left, expectedRight := range correctMappings {
		if studentRight, exists := answers[left]; exists && studentRight == expectedRight {
			correct++
		}
	}

	if total == 0 {
		return 0.0, false, nil
	}

	score := float64(correct) / float64(total)
	return score, correct == total, nil
}

func (s *gradingService) gradeOrdering(questionContent json.RawMessage, studentAnswer json.RawMessage) (float64, bool, error) {
	var content models.OrderingContent
	if err := json.Unmarshal(questionContent, &content); err != nil {
		return 0.0, false, fmt.Errorf("failed to unmarshal question content: %w", err)
	}

	var answer []string // ordered list of item IDs
	if err := json.Unmarshal(studentAnswer, &answer); err != nil {
		return 0.0, false, fmt.Errorf("failed to unmarshal student answer: %w", err)
	}

	expectedOrder := content.CorrectOrder

	// Perfect match
	if reflect.DeepEqual(answer, expectedOrder) {
		return 1.0, true, nil
	}

	// Partial credit based on position accuracy
	correct := 0
	for i, itemID := range answer {
		if i < len(expectedOrder) && itemID == expectedOrder[i] {
			correct++
		}
	}

	score := float64(correct) / float64(len(content.Items))
	return score, false, nil
}

// ===== FEEDBACK GENERATION =====

func (s *gradingService) generateMultipleChoiceFeedback(questionContent json.RawMessage, studentAnswer json.RawMessage, isCorrect bool) string {
	if isCorrect {
		return "Correct! Well done."
	}

	var content models.MultipleChoiceContent
	if err := json.Unmarshal(questionContent, &content); err != nil {
		return "Incorrect answer."
	}

	// Generate feedback showing correct options
	correctOptions := make([]string, 0)
	for correctIndex := range content.CorrectAnswers {
		if correctIndex < len(content.Options) {
			correctOptions = append(correctOptions, content.Options[correctIndex].Text)
		}
	}

	if len(correctOptions) == 1 {
		return fmt.Sprintf("Incorrect. The correct answer is: %s", correctOptions[0])
	} else {
		return fmt.Sprintf("Incorrect. The correct answers are: %s", strings.Join(correctOptions, ", "))
	}
}

func (s *gradingService) generateTrueFalseFeedback(questionContent json.RawMessage, studentAnswer json.RawMessage, isCorrect bool) string {
	if isCorrect {
		return "Correct!"
	}

	var content models.TrueFalseContent
	if err := json.Unmarshal(questionContent, &content); err != nil {
		return "Incorrect answer."
	}

	correctText := "True"
	if !content.CorrectAnswer {
		correctText = "False"
	}
	return fmt.Sprintf("Incorrect. The correct answer is: %s", correctText)
}

func (s *gradingService) generateFillBlankFeedback(questionContent json.RawMessage, studentAnswer json.RawMessage, isCorrect bool) string {
	if isCorrect {
		return "All blanks filled correctly!"
	}
	return "Some answers are incorrect. Please review your responses."
}

func (s *gradingService) generateShortAnswerFeedback(questionContent json.RawMessage, studentAnswer json.RawMessage, isCorrect bool) string {
	if isCorrect {
		return "Correct answer!"
	}
	return "Your answer doesn't match the expected response. Please review the question."
}

func (s *gradingService) generateMatchingFeedback(questionContent json.RawMessage, studentAnswer json.RawMessage, isCorrect bool) string {
	if isCorrect {
		return "All items matched correctly!"
	}
	return "Some matches are incorrect. Please review your pairings."
}

func (s *gradingService) generateOrderingFeedback(questionContent json.RawMessage, studentAnswer json.RawMessage, isCorrect bool) string {
	if isCorrect {
		return "Perfect sequence!"
	}
	return "The order is not completely correct. Please review the sequence."
}

// ===== HELPER FUNCTIONS =====

func (s *gradingService) checkGradingPermission(ctx context.Context, answer *models.StudentAnswer, graderID string) error {
	// Get user role
	userRole, err := s.getUserRole(ctx, graderID)
	if err != nil {
		return err
	}

	// Only teachers and admins can grade
	if userRole != models.RoleTeacher && userRole != models.RoleAdmin {
		return NewPermissionError(graderID, answer.ID, "answer", "grade", "insufficient role permissions")
	}

	// Check if grader has access to the assessment
	assessmentService := NewAssessmentService(s.repo, s.db, s.logger, s.validator)
	canAccess, err := assessmentService.CanAccess(ctx, answer.Attempt.AssessmentID, graderID)
	if err != nil {
		return err
	}
	if !canAccess {
		return NewPermissionError(graderID, answer.Attempt.AssessmentID, "assessment", "grade", "not owner or insufficient permissions")
	}

	return nil
}

func (s *gradingService) getUserRole(ctx context.Context, userID string) (models.UserRole, error) {
	user, err := s.repo.User().GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %w", err)
	}
	return user.Role, nil
}

func (s *gradingService) isAutoGradeable(questionType models.QuestionType) bool {
	autoGradeableTypes := map[models.QuestionType]bool{
		models.MultipleChoice: true,
		models.TrueFalse:      true,
		models.FillInBlank:    true,
		models.ShortAnswer:    true,
		models.Matching:       true,
		models.Ordering:       true,
		models.Essay:          false, // Requires manual grading
	}

	return autoGradeableTypes[questionType]
}

func (s *gradingService) calculateLetterGrade(percentage float64) string {
	if percentage >= 97 {
		return "A+"
	} else if percentage >= 93 {
		return "A"
	} else if percentage >= 90 {
		return "A-"
	} else if percentage >= 87 {
		return "B+"
	} else if percentage >= 83 {
		return "B"
	} else if percentage >= 80 {
		return "B-"
	} else if percentage >= 77 {
		return "C+"
	} else if percentage >= 73 {
		return "C"
	} else if percentage >= 70 {
		return "C-"
	} else if percentage >= 67 {
		return "D+"
	} else if percentage >= 63 {
		return "D"
	} else if percentage >= 60 {
		return "D-"
	} else {
		return "F"
	}
}

func (s *gradingService) gradeAnswerInTransaction(ctx context.Context, tx *gorm.DB, answerID uint, score float64, feedback *string, graderID string) (*GradingResult, error) {
	// Get answer
	answer, err := s.repo.Answer().GetByIDWithDetails(ctx, tx, answerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get answer: %w", err)
	}

	// Update with grade
	maxScore := float64(answer.Question.Points)
	answer.Score = score
	answer.Feedback = feedback
	answer.GradedBy = &graderID
	answer.GradedAt = timePtr(time.Now())
	answer.IsGraded = true

	if err := s.repo.Answer().Update(ctx, tx, answer); err != nil {
		return nil, fmt.Errorf("failed to update answer: %w", err)
	}

	return &GradingResult{
		AnswerID:      answerID,
		QuestionID:    answer.QuestionID,
		Score:         score,
		MaxScore:      maxScore,
		IsCorrect:     score == maxScore,
		PartialCredit: score > 0 && score < maxScore,
		Feedback:      feedback,
		GradedAt:      time.Now(),
		GradedBy:      &graderID,
	}, nil
}

func (s *gradingService) updateAttemptGradeIfComplete(attemptID uint) {
	ctx := context.Background()

	// Check if all answers are graded
	allGraded, err := s.repo.Answer().AreAllAnswersGraded(ctx, nil, attemptID)
	if err != nil {
		s.logger.Error("Failed to check if all answers graded", "attempt_id", attemptID, "error", err)
		return
	}

	if allGraded {
		if _, err := s.AutoGradeAttempt(ctx, attemptID); err != nil {
			s.logger.Error("Failed to update attempt grade", "attempt_id", attemptID, "error", err)
		}
	}
}

func (s *gradingService) compareStrings(s1, s2 string, caseSensitive bool) bool {
	if !caseSensitive {
		s1 = strings.ToLower(strings.TrimSpace(s1))
		s2 = strings.ToLower(strings.TrimSpace(s2))
	} else {
		s1 = strings.TrimSpace(s1)
		s2 = strings.TrimSpace(s2)
	}
	return s1 == s2
}

func (s *gradingService) calculateStringSimilarity(s1, s2 string) float64 {
	// Simple similarity calculation using Levenshtein distance
	s1 = strings.ToLower(strings.TrimSpace(s1))
	s2 = strings.ToLower(strings.TrimSpace(s2))

	if s1 == s2 {
		return 1.0
	}

	maxLen := math.Max(float64(len(s1)), float64(len(s2)))
	if maxLen == 0 {
		return 1.0
	}

	distance := float64(levenshteinDistance(s1, s2))
	return 1.0 - (distance / maxLen)
}

func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

func sortStrings(arr []string) []string {
	sorted := make([]string, len(arr))
	copy(sorted, arr)
	sort.Strings(sorted)
	return sorted
}

func min(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
}
