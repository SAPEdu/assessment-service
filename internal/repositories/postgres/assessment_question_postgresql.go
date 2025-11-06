package postgres

import (
	"context"
	"errors"
	"fmt"
	"math/rand"

	"github.com/SAP-F-2025/assessment-service/internal/cache"
	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type AssessmentQuestionPostgreSQL struct {
	db           *gorm.DB
	helpers      *SharedHelpers
	cacheManager *cache.CacheManager
}

func NewAssessmentQuestionPostgreSQL(db *gorm.DB, redisClient *redis.Client) repositories.AssessmentQuestionRepository {
	return &AssessmentQuestionPostgreSQL{
		db:           db,
		helpers:      NewSharedHelpers(db),
		cacheManager: cache.NewCacheManager(redisClient),
	}
}

// ===== BASIC OPERATIONS =====

// getDB returns the transaction DB if provided, otherwise returns the default DB
func (aq *AssessmentQuestionPostgreSQL) getDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return aq.db
}

// Create creates a new assessment-question relationship
func (aq *AssessmentQuestionPostgreSQL) Create(ctx context.Context, tx *gorm.DB, assessmentQuestion *models.AssessmentQuestion) error {
	db := aq.getDB(tx)
	if err := db.WithContext(ctx).Create(assessmentQuestion).Error; err != nil {
		return fmt.Errorf("failed to create assessment question: %w", err)
	}

	// Invalidate caches
	aq.invalidateCachesForAssessmentQuestion(ctx, assessmentQuestion.AssessmentID)

	return nil
}

// GetByID retrieves an assessment-question relationship by ID
func (aq *AssessmentQuestionPostgreSQL) GetByID(ctx context.Context, tx *gorm.DB, id uint) (*models.AssessmentQuestion, error) {
	db := aq.getDB(tx)
	var assessmentQuestion models.AssessmentQuestion
	if err := db.WithContext(ctx).
		Preload("Assessment").
		Preload("Question").
		First(&assessmentQuestion, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("assessment question not found with ID %d", id)
		}
		return nil, fmt.Errorf("failed to get assessment question: %w", err)
	}
	return &assessmentQuestion, nil
}

// Update updates an assessment-question relationship
func (aq *AssessmentQuestionPostgreSQL) Update(ctx context.Context, tx *gorm.DB, assessmentQuestion *models.AssessmentQuestion) error {
	db := aq.getDB(tx)
	if err := db.WithContext(ctx).Save(assessmentQuestion).Error; err != nil {
		return fmt.Errorf("failed to update assessment question: %w", err)
	}

	// Invalidate caches
	aq.invalidateCachesForAssessmentQuestion(ctx, assessmentQuestion.AssessmentID)

	return nil
}

// Delete removes an assessment-question relationship
func (aq *AssessmentQuestionPostgreSQL) Delete(ctx context.Context, tx *gorm.DB, id uint) error {
	db := aq.getDB(tx)

	// Get assessment ID before deleting for cache invalidation
	var assessmentQuestion models.AssessmentQuestion
	if err := db.WithContext(ctx).Select("assessment_id").First(&assessmentQuestion, id).Error; err != nil {
		return fmt.Errorf("failed to get assessment question before delete: %w", err)
	}

	if err := db.WithContext(ctx).Delete(&models.AssessmentQuestion{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete assessment question: %w", err)
	}

	// Invalidate caches
	aq.invalidateCachesForAssessmentQuestion(ctx, assessmentQuestion.AssessmentID)

	return nil
}

// ===== RELATIONSHIP MANAGEMENT =====

// AddQuestion adds a question to an assessment with specified order and points
func (aq *AssessmentQuestionPostgreSQL) AddQuestion(ctx context.Context, tx *gorm.DB, assessmentID, questionID uint, order int, points *int) error {
	// Check if relationship already exists
	exists, err := aq.Exists(ctx, tx, assessmentID, questionID)
	if err != nil {
		return fmt.Errorf("failed to check if relationship exists: %w", err)
	}
	if exists {
		return fmt.Errorf("question %d is already added to assessment %d", questionID, assessmentID)
	}

	// If order is 0, get next order
	if order == 0 {
		order, err = aq.GetNextOrder(ctx, tx, assessmentID)
		if err != nil {
			return fmt.Errorf("failed to get next order: %w", err)
		}
	}

	assessmentQuestion := &models.AssessmentQuestion{
		AssessmentID: assessmentID,
		QuestionID:   questionID,
		Order:        order,
		Points:       points,
		Required:     true,
	}

	if err := aq.Create(ctx, tx, assessmentQuestion); err != nil {
		return err
	}

	// Invalidate caches (Create already invalidates, but being explicit)
	aq.invalidateCachesForAssessmentQuestion(ctx, assessmentID)

	return nil
}

// RemoveQuestion removes a question from an assessment
func (aq *AssessmentQuestionPostgreSQL) RemoveQuestion(ctx context.Context, tx *gorm.DB, assessmentID, questionID uint) error {
	db := aq.getDB(tx)
	result := db.WithContext(ctx).
		Where("assessment_id = ? AND question_id = ?", assessmentID, questionID).
		Delete(&models.AssessmentQuestion{})

	if result.Error != nil {
		return fmt.Errorf("failed to remove question from assessment: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("no relationship found between assessment %d and question %d", assessmentID, questionID)
	}

	// Invalidate caches
	aq.invalidateCachesForAssessmentQuestion(ctx, assessmentID)

	return nil
}

// AddQuestions adds multiple questions to an assessment
func (aq *AssessmentQuestionPostgreSQL) AddQuestions(ctx context.Context, tx *gorm.DB, assessmentID uint, questionIDs []uint) error {
	if len(questionIDs) == 0 {
		return nil
	}

	db := aq.getDB(tx)
	// Get next order
	nextOrder, err := aq.GetNextOrder(ctx, db, assessmentID)
	if err != nil {
		return fmt.Errorf("failed to get next order: %w", err)
	}

	// Check if relationship already exists
	var existingCount int64
	if err := db.Model(&models.AssessmentQuestion{}).
		Where("assessment_id = ? AND question_id IN ?", assessmentID, questionIDs).
		Count(&existingCount).Error; err != nil {
		return fmt.Errorf("failed to check existing relationships: %w", err)
	}

	if existingCount > 0 {
		return fmt.Errorf("some questions are already added to assessment %d", assessmentID)
	}

	// Create assessment questions
	assessmentQuestions := make([]*models.AssessmentQuestion, len(questionIDs))
	for i, questionID := range questionIDs {

		assessmentQuestions[i] = &models.AssessmentQuestion{
			AssessmentID: assessmentID,
			QuestionID:   questionID,
			Order:        nextOrder + i,
			Required:     true,
		}
	}

	if err := aq.CreateBatch(ctx, db, assessmentQuestions); err != nil {
		return err
	}

	// Invalidate caches
	aq.invalidateCachesForAssessmentQuestion(ctx, assessmentID)

	return nil
}

// RemoveQuestions removes multiple questions from an assessment
func (aq *AssessmentQuestionPostgreSQL) RemoveQuestions(ctx context.Context, tx *gorm.DB, assessmentID uint, questionIDs []uint) error {
	if len(questionIDs) == 0 {
		return nil
	}

	db := aq.getDB(tx)

	execFunc := func(execDB *gorm.DB) error {
		// Delete questions
		result := execDB.WithContext(ctx).
			Where("assessment_id = ? AND question_id IN ?", assessmentID, questionIDs).Unscoped().
			Delete(&models.AssessmentQuestion{})

		if result.Error != nil {
			return fmt.Errorf("failed to remove questions from assessment: %w", result.Error)
		}

		if result.RowsAffected == 0 {
			return fmt.Errorf("no questions found in assessment %d to remove", assessmentID)
		}

		// Reorder remaining questions
		var remaining []*models.AssessmentQuestion
		if err := execDB.WithContext(ctx).
			Where("assessment_id = ?", assessmentID).
			Order("\"order\" ASC").
			Find(&remaining).Error; err != nil {
			return fmt.Errorf("failed to get remaining questions: %w", err)
		}

		// Update order to be sequential
		for i, aq := range remaining {
			newOrder := i + 1
			if aq.Order != newOrder {
				if err := execDB.Model(aq).Update("order", newOrder).Error; err != nil {
					return fmt.Errorf("failed to reorder question %d: %w", aq.ID, err)
				}
			}
		}

		return nil
	}

	var err error
	if tx != nil {
		err = execFunc(db)
	} else {
		err = db.WithContext(ctx).Transaction(func(txInner *gorm.DB) error {
			return execFunc(txInner)
		})
	}

	if err != nil {
		return err
	}

	// Invalidate caches after successful removal
	aq.invalidateCachesForAssessmentQuestion(ctx, assessmentID)

	return nil
}

// ===== ORDER MANAGEMENT =====

// UpdateOrder updates the order of questions in an assessment
func (aq *AssessmentQuestionPostgreSQL) UpdateOrder(ctx context.Context, tx *gorm.DB, assessmentID uint, questionOrders []repositories.QuestionOrder) error {
	if len(questionOrders) == 0 {
		return nil
	}

	db := aq.getDB(tx)
	err := db.WithContext(ctx).Transaction(func(txInner *gorm.DB) error {
		for _, qo := range questionOrders {
			result := txInner.Model(&models.AssessmentQuestion{}).
				Where("assessment_id = ? AND question_id = ?", assessmentID, qo.QuestionID).
				Update("order", qo.Order)

			if result.Error != nil {
				return fmt.Errorf("failed to update order for question %d: %w", qo.QuestionID, result.Error)
			}

			if result.RowsAffected == 0 {
				return fmt.Errorf("no relationship found for assessment %d and question %d", assessmentID, qo.QuestionID)
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	// Invalidate caches (order changed affects assessment details)
	aq.invalidateCachesForAssessmentQuestion(ctx, assessmentID)

	return nil
}

// ReorderQuestions reorders questions based on provided order
func (aq *AssessmentQuestionPostgreSQL) ReorderQuestions(ctx context.Context, tx *gorm.DB, assessmentID uint, questions []repositories.QuestionOrder) error {
	questionOrders := make([]repositories.QuestionOrder, len(questions))
	for i, question := range questions {
		questionOrders[i] = repositories.QuestionOrder{
			QuestionID: question.QuestionID,
			Order:      i + 1,
		}
	}
	err := aq.UpdateOrder(ctx, tx, assessmentID, questionOrders)
	if err != nil {
		return err
	}

	// Invalidate caches (UpdateOrder already does this, but being explicit)
	aq.invalidateCachesForAssessmentQuestion(ctx, assessmentID)

	return nil
}

// GetMaxOrder gets the maximum order value for questions in an assessment
func (aq *AssessmentQuestionPostgreSQL) GetMaxOrder(ctx context.Context, tx *gorm.DB, assessmentID uint) (int, error) {
	var maxOrder int
	err := tx.WithContext(ctx).
		Model(&models.AssessmentQuestion{}).
		Where("assessment_id = ?", assessmentID).
		Select("COALESCE(MAX(\"order\"), 0)").
		Scan(&maxOrder).Error

	if err != nil {
		return 0, fmt.Errorf("failed to get max order: %w", err)
	}

	return maxOrder, nil
}

// GetNextOrder gets the next order value for adding a question
func (aq *AssessmentQuestionPostgreSQL) GetNextOrder(ctx context.Context, tx *gorm.DB, assessmentID uint) (int, error) {
	maxOrder, err := aq.GetMaxOrder(ctx, tx, assessmentID)
	if err != nil {
		return 0, err
	}
	return maxOrder + 1, nil
}

// ===== QUERY OPERATIONS =====

// GetByAssessment retrieves all assessment-question relationships for an assessment
func (aq *AssessmentQuestionPostgreSQL) GetByAssessment(ctx context.Context, tx *gorm.DB, assessmentID uint) ([]*models.AssessmentQuestion, error) {
	var assessmentQuestions []*models.AssessmentQuestion
	if err := tx.WithContext(ctx).
		Where("assessment_id = ?", assessmentID).
		Find(&assessmentQuestions).Error; err != nil {
		return nil, fmt.Errorf("failed to get assessment questions: %w", err)
	}
	return assessmentQuestions, nil
}

// GetByAssessmentOrdered retrieves assessment-question relationships ordered by order field
func (aq *AssessmentQuestionPostgreSQL) GetByAssessmentOrdered(ctx context.Context, tx *gorm.DB, assessmentID uint) ([]*models.AssessmentQuestion, error) {
	db := aq.getDB(tx)
	var assessmentQuestions []*models.AssessmentQuestion
	if err := db.WithContext(ctx).
		Where("assessment_id = ?", assessmentID).
		Order("\"order\" ASC").
		Find(&assessmentQuestions).Error; err != nil {
		return nil, fmt.Errorf("failed to get ordered assessment questions: %w", err)
	}
	return assessmentQuestions, nil
}

// GetByQuestion retrieves all assessment-question relationships for a question
func (aq *AssessmentQuestionPostgreSQL) GetByQuestion(ctx context.Context, tx *gorm.DB, questionID uint) ([]*models.AssessmentQuestion, error) {
	db := aq.getDB(tx)
	var assessmentQuestions []*models.AssessmentQuestion
	if err := db.WithContext(ctx).
		Where("question_id = ?", questionID).
		Find(&assessmentQuestions).Error; err != nil {
		return nil, fmt.Errorf("failed to get assessment questions by question: %w", err)
	}
	return assessmentQuestions, nil
}

// GetQuestionsForAssessment retrieves all questions for an assessment in order
func (aq *AssessmentQuestionPostgreSQL) GetQuestionsForAssessment(ctx context.Context, tx *gorm.DB, assessmentID uint) ([]*models.Question, error) {
	db := aq.getDB(tx)
	var questions []*models.Question
	if err := db.WithContext(ctx).
		Table("questions").
		Joins("JOIN assessment_questions aq ON aq.question_id = questions.id").
		Where("aq.assessment_id = ?", assessmentID).
		Order("aq.\"order\" ASC").
		Find(&questions).Error; err != nil {
		return nil, fmt.Errorf("failed to get questions for assessment: %w", err)
	}
	return questions, nil
}

// GetAssessmentsForQuestion retrieves all assessments that use a question
func (aq *AssessmentQuestionPostgreSQL) GetAssessmentsForQuestion(ctx context.Context, tx *gorm.DB, questionID uint) ([]*models.Assessment, error) {
	db := aq.getDB(tx)
	var assessments []*models.Assessment
	if err := db.WithContext(ctx).
		Table("assessments").
		Joins("JOIN assessment_questions aq ON aq.assessment_id = assessments.id").
		Where("aq.question_id = ?", questionID).
		Find(&assessments).Error; err != nil {
		return nil, fmt.Errorf("failed to get assessments for question: %w", err)
	}
	return assessments, nil
}

// ===== BULK OPERATIONS =====

// CreateBatch creates multiple assessment-question relationships
func (aq *AssessmentQuestionPostgreSQL) CreateBatch(ctx context.Context, tx *gorm.DB, assessmentQuestions []*models.AssessmentQuestion) error {
	if len(assessmentQuestions) == 0 {
		return nil
	}

	if err := tx.WithContext(ctx).CreateInBatches(assessmentQuestions, 100).Error; err != nil {
		return fmt.Errorf("failed to create assessment questions batch: %w", err)
	}
	// Invalidate details assessment for all affected assessments
	aq.cacheManager.Assessment.InvalidatePattern(ctx, fmt.Sprintf("details:%d", assessmentQuestions[0].AssessmentID))
	return nil
}

// UpdateBatch updates multiple assessment-question relationships
func (aq *AssessmentQuestionPostgreSQL) UpdateBatch(ctx context.Context, tx *gorm.DB, assessmentQuestions []*models.AssessmentQuestion) error {
	if len(assessmentQuestions) == 0 {
		return nil
	}

	db := aq.getDB(tx)
	return db.WithContext(ctx).Transaction(func(txInner *gorm.DB) error {
		for _, assessmentQuestion := range assessmentQuestions {
			if err := txInner.Save(assessmentQuestion).Error; err != nil {
				return fmt.Errorf("failed to update assessment question ID %d: %w", assessmentQuestion.ID, err)
			}
		}
		// Invalidate details assessment for all affected assessments
		aq.cacheManager.Assessment.InvalidatePattern(ctx, fmt.Sprintf("details:%d", assessmentQuestions[0].AssessmentID))

		return nil
	})

}

// DeleteByAssessment removes all questions from an assessment
func (aq *AssessmentQuestionPostgreSQL) DeleteByAssessment(ctx context.Context, tx *gorm.DB, assessmentID uint) error {
	db := aq.getDB(tx)
	if err := db.WithContext(ctx).
		Where("assessment_id = ?", assessmentID).
		Delete(&models.AssessmentQuestion{}).Error; err != nil {
		return fmt.Errorf("failed to delete assessment questions by assessment: %w", err)
	}

	// Invalidate caches (all questions removed from assessment)
	aq.invalidateCachesForAssessmentQuestion(ctx, assessmentID)

	return nil
}

// DeleteByQuestion removes a question from all assessments
func (aq *AssessmentQuestionPostgreSQL) DeleteByQuestion(ctx context.Context, tx *gorm.DB, questionID uint) error {
	db := aq.getDB(tx)

	// Get all affected assessment IDs before deleting for cache invalidation
	var assessmentIDs []uint
	if err := db.WithContext(ctx).
		Model(&models.AssessmentQuestion{}).
		Where("question_id = ?", questionID).
		Pluck("assessment_id", &assessmentIDs).Error; err != nil {
		return fmt.Errorf("failed to get assessment IDs before delete: %w", err)
	}

	if err := db.WithContext(ctx).
		Where("question_id = ?", questionID).
		Delete(&models.AssessmentQuestion{}).Error; err != nil {
		return fmt.Errorf("failed to delete assessment questions by question: %w", err)
	}

	// Invalidate caches for all affected assessments
	aq.invalidateCachesForAssessmentQuestions(ctx, assessmentIDs)

	return nil
}

// ===== VALIDATION AND CHECKS =====

// Exists checks if an assessment-question relationship exists
func (aq *AssessmentQuestionPostgreSQL) Exists(ctx context.Context, tx *gorm.DB, assessmentID, questionID uint) (bool, error) {
	var count int64
	err := tx.WithContext(ctx).
		Model(&models.AssessmentQuestion{}).
		Where("assessment_id = ? AND question_id = ?", assessmentID, questionID).
		Count(&count).Error

	if err != nil {
		return false, fmt.Errorf("failed to check assessment question existence: %w", err)
	}

	return count > 0, nil
}

// GetQuestionCount gets the number of questions in an assessment
func (aq *AssessmentQuestionPostgreSQL) GetQuestionCount(ctx context.Context, tx *gorm.DB, assessmentID uint) (int, error) {
	db := aq.getDB(tx)
	var count int64
	err := db.WithContext(ctx).
		Model(&models.AssessmentQuestion{}).
		Where("assessment_id = ?", assessmentID).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("failed to get question count: %w", err)
	}

	return int(count), nil
}

// GetAssessmentCount gets the number of assessments using a question
func (aq *AssessmentQuestionPostgreSQL) GetAssessmentCount(ctx context.Context, tx *gorm.DB, questionID uint) (int, error) {
	db := aq.getDB(tx)
	var count int64
	err := db.WithContext(ctx).
		Model(&models.AssessmentQuestion{}).
		Where("question_id = ?", questionID).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("failed to get assessment count: %w", err)
	}

	return int(count), nil
}

// ===== POINTS MANAGEMENT =====

// UpdatePoints updates the points for a specific question in an assessment
func (aq *AssessmentQuestionPostgreSQL) UpdatePoints(ctx context.Context, tx *gorm.DB, assessmentID, questionID uint, points int) error {
	db := aq.getDB(tx)
	result := db.WithContext(ctx).
		Model(&models.AssessmentQuestion{}).
		Where("assessment_id = ? AND question_id = ?", assessmentID, questionID).
		Update("points", points)

	if result.Error != nil {
		return fmt.Errorf("failed to update points: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("no relationship found between assessment %d and question %d", assessmentID, questionID)
	}

	// Invalidate caches (points changed affects total points in assessment)
	aq.invalidateCachesForAssessmentQuestion(ctx, assessmentID)

	return nil
}

// GetTotalPoints calculates the total points for all questions in an assessment
func (aq *AssessmentQuestionPostgreSQL) GetTotalPoints(ctx context.Context, tx *gorm.DB, assessmentID uint) (int, error) {
	db := aq.getDB(tx)
	var totalPoints int

	// Use COALESCE to handle NULL points and wrap SUM to handle empty result set
	err := db.WithContext(ctx).
		Table("assessment_questions aq").
		Joins("JOIN questions q ON q.id = aq.question_id").
		Where("aq.assessment_id = ?", assessmentID).
		Select("COALESCE(SUM(COALESCE(aq.points, 0)), 0)").
		Scan(&totalPoints).Error

	if err != nil {
		return 0, fmt.Errorf("failed to calculate total points: %w", err)
	}

	return totalPoints, nil
}

// GetPointsDistribution returns the distribution of points across questions
func (aq *AssessmentQuestionPostgreSQL) GetPointsDistribution(ctx context.Context, tx *gorm.DB, assessmentID uint) (map[uint]int, error) {
	db := aq.getDB(tx)
	var results []struct {
		QuestionID uint `json:"question_id"`
		Points     int  `json:"points"`
	}

	err := db.WithContext(ctx).
		Table("assessment_questions aq").
		Joins("JOIN questions q ON q.id = aq.question_id").
		Where("aq.assessment_id = ?", assessmentID).
		Select("aq.question_id, COALESCE(aq.points, q.points) as points").
		Find(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get points distribution: %w", err)
	}

	distribution := make(map[uint]int)
	for _, result := range results {
		distribution[result.QuestionID] = result.Points
	}

	return distribution, nil
}

// ===== ADVANCED QUERIES =====

// GetQuestionsByType retrieves questions of a specific type from an assessment
func (aq *AssessmentQuestionPostgreSQL) GetQuestionsByType(ctx context.Context, tx *gorm.DB, assessmentID uint, questionType models.QuestionType) ([]*models.Question, error) {
	db := aq.getDB(tx)
	var questions []*models.Question
	if err := db.WithContext(ctx).
		Table("questions").
		Joins("JOIN assessment_questions aq ON aq.question_id = questions.id").
		Where("aq.assessment_id = ? AND questions.type = ?", assessmentID, questionType).
		Order("aq.\"order\" ASC").
		Find(&questions).Error; err != nil {
		return nil, fmt.Errorf("failed to get questions by type: %w", err)
	}
	return questions, nil
}

// GetQuestionsByDifficulty retrieves questions of a specific difficulty from an assessment
func (aq *AssessmentQuestionPostgreSQL) GetQuestionsByDifficulty(ctx context.Context, tx *gorm.DB, assessmentID uint, difficulty models.DifficultyLevel) ([]*models.Question, error) {
	db := aq.getDB(tx)
	var questions []*models.Question
	if err := db.WithContext(ctx).
		Table("questions").
		Joins("JOIN assessment_questions aq ON aq.question_id = questions.id").
		Where("aq.assessment_id = ? AND questions.difficulty = ?", assessmentID, difficulty).
		Order("aq.\"order\" ASC").
		Find(&questions).Error; err != nil {
		return nil, fmt.Errorf("failed to get questions by difficulty: %w", err)
	}
	return questions, nil
}

// GetRandomizedQuestions retrieves questions in randomized order using provided seed
func (aq *AssessmentQuestionPostgreSQL) GetRandomizedQuestions(ctx context.Context, tx *gorm.DB, assessmentID uint, seed int64) ([]*models.Question, error) {
	questions, err := aq.GetQuestionsForAssessment(ctx, tx, assessmentID)
	if err != nil {
		return nil, err
	}

	// Randomize using provided seed
	r := rand.New(rand.NewSource(seed))
	r.Shuffle(len(questions), func(i, j int) {
		questions[i], questions[j] = questions[j], questions[i]
	})

	return questions, nil
}

// ===== STATISTICS =====

// GetAssessmentQuestionStats retrieves comprehensive statistics for an assessment
func (aq *AssessmentQuestionPostgreSQL) GetAssessmentQuestionStats(ctx context.Context, tx *gorm.DB, assessmentID uint) (*repositories.AssessmentQuestionStats, error) {
	db := aq.getDB(tx)
	stats := &repositories.AssessmentQuestionStats{
		AssessmentID:       assessmentID,
		QuestionsByType:    make(map[models.QuestionType]int),
		QuestionsByDiff:    make(map[models.DifficultyLevel]int),
		PointsDistribution: make(map[int]int),
	}

	// Get total questions and points
	questionCount, err := aq.GetQuestionCount(ctx, tx, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get question count: %w", err)
	}
	stats.TotalQuestions = questionCount

	totalPoints, err := aq.GetTotalPoints(ctx, tx, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get total points: %w", err)
	}
	stats.TotalPoints = totalPoints

	if questionCount > 0 {
		stats.AvgPointsPerQ = float64(totalPoints) / float64(questionCount)
	}

	// Get questions by type
	var typeResults []struct {
		Type  models.QuestionType `json:"type"`
		Count int                 `json:"count"`
	}
	err = db.WithContext(ctx).
		Table("questions q").
		Joins("JOIN assessment_questions aq ON aq.question_id = q.id").
		Where("aq.assessment_id = ?", assessmentID).
		Select("q.type, COUNT(*) as count").
		Group("q.type").
		Find(&typeResults).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get questions by type: %w", err)
	}
	for _, result := range typeResults {
		stats.QuestionsByType[result.Type] = result.Count
	}

	// Get questions by difficulty
	var diffResults []struct {
		Difficulty models.DifficultyLevel `json:"difficulty"`
		Count      int                    `json:"count"`
	}
	err = db.WithContext(ctx).
		Table("questions q").
		Joins("JOIN assessment_questions aq ON aq.question_id = q.id").
		Where("aq.assessment_id = ?", assessmentID).
		Select("q.difficulty, COUNT(*) as count").
		Group("q.difficulty").
		Find(&diffResults).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get questions by difficulty: %w", err)
	}
	for _, result := range diffResults {
		stats.QuestionsByDiff[result.Difficulty] = result.Count
	}

	// Get points distribution
	var pointsResults []struct {
		Points int `json:"points"`
		Count  int `json:"count"`
	}
	err = db.WithContext(ctx).
		Table("assessment_questions aq").
		Joins("JOIN questions q ON q.id = aq.question_id").
		Where("aq.assessment_id = ?", assessmentID).
		Select("COALESCE(aq.points, q.points) as points, COUNT(*) as count").
		Group("COALESCE(aq.points, q.points)").
		Find(&pointsResults).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get points distribution: %w", err)
	}
	for _, result := range pointsResults {
		stats.PointsDistribution[result.Points] = result.Count
	}

	return stats, nil
}

// GetQuestionUsageInAssessments retrieves usage statistics for a question across assessments
func (aq *AssessmentQuestionPostgreSQL) GetQuestionUsageInAssessments(ctx context.Context, tx *gorm.DB, questionID uint) (*repositories.QuestionAssessmentUsage, error) {
	db := aq.getDB(tx)
	usage := &repositories.QuestionAssessmentUsage{
		QuestionID: questionID,
	}

	// Get usage count
	usageCount, err := aq.GetAssessmentCount(ctx, tx, questionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage count: %w", err)
	}
	usage.UsedInCount = usageCount

	// Get assessment titles
	var titles []string
	err = db.WithContext(ctx).
		Table("assessments a").
		Joins("JOIN assessment_questions aq ON aq.assessment_id = a.id").
		Where("aq.question_id = ?", questionID).
		Pluck("a.title", &titles).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get assessment titles: %w", err)
	}
	usage.AssessmentTitles = titles

	// Note: TotalAttempts and AverageScore would require answers/attempts tables
	// which are not implemented in this basic version

	return usage, nil
}

func (aq *AssessmentQuestionPostgreSQL) GetQuestionAssessmentByAssessmentIdAndQuestionId(ctx context.Context, tx *gorm.DB, assessmentId, questionId uint) (*models.AssessmentQuestion, error) {
	db := aq.getDB(tx)

	var assessmentQuestion models.AssessmentQuestion
	err := db.WithContext(ctx).
		Model(&models.AssessmentQuestion{}).
		Where("assessment_id = ? AND question_id = ?", assessmentId, questionId).
		First(&assessmentQuestion).Error

	if err != nil {
		return nil, fmt.Errorf("failed to check assessment question existence: %w", err)
	}

	return &assessmentQuestion, nil
}

// ===== CACHE INVALIDATION HELPERS =====

// invalidateCachesForAssessmentQuestion invalidates all relevant caches when assessment-question relationship changes
func (aq *AssessmentQuestionPostgreSQL) invalidateCachesForAssessmentQuestion(ctx context.Context, assessmentID uint) {
	cache.SafeDelete(ctx, aq.cacheManager.Assessment,
		fmt.Sprintf("id:%d", assessmentID),
		fmt.Sprintf("details:%d", assessmentID))

	cache.SafeDelete(ctx, aq.cacheManager.Question, fmt.Sprintf("assessment:%d", assessmentID))
	cache.SafeInvalidatePattern(ctx, aq.cacheManager.Assessment, "list:*")
	cache.SafeInvalidatePattern(ctx, aq.cacheManager.Stats, fmt.Sprintf("assessment:%d:*", assessmentID))
}

// invalidateCachesForAssessmentQuestions invalidates caches for multiple assessments
func (aq *AssessmentQuestionPostgreSQL) invalidateCachesForAssessmentQuestions(ctx context.Context, assessmentIDs []uint) {
	for _, assessmentID := range assessmentIDs {
		aq.invalidateCachesForAssessmentQuestion(ctx, assessmentID)
	}
}
