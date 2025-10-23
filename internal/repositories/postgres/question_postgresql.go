package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/SAP-F-2025/assessment-service/internal/cache"
	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type QuestionPostgreSQL struct {
	db           *gorm.DB
	helpers      *SharedHelpers
	cacheManager *cache.CacheManager
}

func NewQuestionPostgreSQL(db *gorm.DB, redisClient *redis.Client) repositories.QuestionRepository {
	return &QuestionPostgreSQL{
		db:           db,
		helpers:      NewSharedHelpers(db),
		cacheManager: cache.NewCacheManager(redisClient),
	}
}

// ===== BASIC CRUD OPERATIONS =====

// Create creates a new question and invalidates cache
func (q *QuestionPostgreSQL) Create(ctx context.Context, tx *gorm.DB, question *models.Question) error {
	db := q.getDB(tx)
	if err := db.WithContext(ctx).Create(question).Error; err != nil {
		return fmt.Errorf("failed to create question: %w", err)
	}

	cache.SafeInvalidatePattern(ctx, q.cacheManager.Question, fmt.Sprintf("creator:%s:*", question.CreatedBy))
	cache.SafeInvalidatePattern(ctx, q.cacheManager.Question, "list:*")

	return nil
}

// GetByID retrieves a question by ID with caching
func (q *QuestionPostgreSQL) GetByID(ctx context.Context, tx *gorm.DB, id uint) (*models.Question, error) {
	db := q.getDB(tx)
	// Try cache first for performance
	cacheKey := fmt.Sprintf("id:%d", id)
	var question models.Question

	err := q.cacheManager.Question.CacheOrExecute(ctx, cacheKey, &question, cache.QuestionCacheConfig.TTL, func() (interface{}, error) {
		var dbQuestion models.Question
		if err := db.WithContext(ctx).First(&dbQuestion, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("question not found with ID %d", id)
			}
			return nil, fmt.Errorf("failed to get question: %w", err)
		}
		return &dbQuestion, nil
	})

	if err != nil {
		return nil, err
	}

	return &question, nil
}

// GetByIDWithDetails retrieves a question with all related data
func (q *QuestionPostgreSQL) GetByIDWithDetails(ctx context.Context, tx *gorm.DB, id uint) (*models.Question, error) {
	db := q.getDB(tx)
	var question models.Question
	if err := db.WithContext(ctx).
		Preload("Category").
		Preload("Attachments").
		Preload("Creator", func(db *gorm.DB) *gorm.DB {
			return db.Select("id, username, email")
		}).
		First(&question, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("question not found with ID %d", id)
		}
		return nil, fmt.Errorf("failed to get question with details: %w", err)
	}
	return &question, nil
}

// Update updates a question
func (q *QuestionPostgreSQL) Update(ctx context.Context, tx *gorm.DB, question *models.Question) error {
	db := q.getDB(tx)
	if err := db.WithContext(ctx).Save(question).Error; err != nil {
		return fmt.Errorf("failed to update question: %w", err)
	}

	cache.InvalidateQuestionCache(ctx, q.cacheManager, question.ID, question.CreatedBy)
	q.invalidateAssessmentCachesForQuestion(ctx, db, question.ID)

	return nil
}

// Delete soft deletes a question
func (q *QuestionPostgreSQL) Delete(ctx context.Context, tx *gorm.DB, id uint) error {
	db := q.getDB(tx)

	// Get question info before deleting for cache invalidation
	var question models.Question
	if err := db.WithContext(ctx).Select("id, created_by").First(&question, id).Error; err != nil {
		return fmt.Errorf("failed to get question before delete: %w", err)
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		// delete question in assessment_questions first due to foreign key constraint
		if err := tx.WithContext(ctx).Where("question_id = ?", id).Delete(&models.AssessmentQuestion{}).Error; err != nil {
			return fmt.Errorf("failed to delete question from assessment_questions: %w", err)
		}

		queryDeleteQuestionBank := `DELETE FROM question_bank_questions WHERE question_id = ?`
		if err := tx.WithContext(ctx).Exec(queryDeleteQuestionBank, id).Error; err != nil {
			return fmt.Errorf("failed to delete question from question_bank_questions: %w", err)
		}
		// delete the question
		if err := tx.WithContext(ctx).Delete(&models.Question{}, id).Error; err != nil {
			return fmt.Errorf("failed to delete question: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	cache.InvalidateQuestionCache(ctx, q.cacheManager, id, question.CreatedBy)
	q.invalidateAssessmentCachesForQuestion(ctx, db, id)

	return nil
}

// ===== BULK OPERATIONS =====

// CreateBatch creates multiple questions in a batch
func (q *QuestionPostgreSQL) CreateBatch(ctx context.Context, tx *gorm.DB, questions []*models.Question) error {
	if len(questions) == 0 {
		return nil
	}

	db := q.getDB(tx)
	if err := db.WithContext(ctx).CreateInBatches(questions, 100).Error; err != nil {
		return fmt.Errorf("failed to create questions batch: %w", err)
	}
	return nil
}

// UpdateBatch updates multiple questions in a batch
func (q *QuestionPostgreSQL) UpdateBatch(ctx context.Context, tx *gorm.DB, questions []*models.Question) error {
	if len(questions) == 0 {
		return nil
	}

	db := q.getDB(tx)
	if err := db.WithContext(ctx).Save(questions).Error; err != nil {
		return fmt.Errorf("failed to update questions: %w", err)
	}
	return nil
}

// GetByIDs retrieves multiple questions by their IDs
func (q *QuestionPostgreSQL) GetByIDs(ctx context.Context, tx *gorm.DB, ids []uint) ([]*models.Question, error) {
	if len(ids) == 0 {
		return []*models.Question{}, nil
	}

	db := q.getDB(tx)
	var questions []*models.Question
	if err := db.WithContext(ctx).
		Where("id IN ?", ids).
		Find(&questions).Error; err != nil {
		return nil, fmt.Errorf("failed to get questions by IDs: %w", err)
	}

	return questions, nil
}

// DeleteBatch soft deletes multiple questions
func (q *QuestionPostgreSQL) DeleteBatch(ctx context.Context, tx *gorm.DB, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}

	db := q.getDB(tx)
	if err := db.WithContext(ctx).Delete(&models.Question{}, ids).Error; err != nil {
		return fmt.Errorf("failed to delete questions batch: %w", err)
	}

	return nil
}

// ===== QUERY OPERATIONS =====

// List retrieves questions with filtering and pagination
func (q *QuestionPostgreSQL) List(ctx context.Context, tx *gorm.DB, filters repositories.QuestionFilters) ([]*models.Question, int64, error) {
	db := q.getDB(tx)
	query := db.WithContext(ctx).Model(&models.Question{})

	// Apply filters
	query = q.applyQuestionFilters(query, filters)

	// Count total records
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count questions: %w", err)
	}

	// Apply pagination and sorting
	query = q.helpers.ApplyPaginationAndSort(query, filters.SortBy, filters.SortOrder, filters.Limit, filters.Offset)

	var questions []*models.Question
	if err := query.Find(&questions).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list questions: %w", err)
	}

	return questions, total, nil
}

// GetByCreator retrieves questions created by a specific user
func (q *QuestionPostgreSQL) GetByCreator(ctx context.Context, tx *gorm.DB, creatorID string, filters repositories.QuestionFilters) ([]*models.Question, int64, error) {
	filters.CreatedBy = &creatorID
	return q.List(ctx, tx, filters)
}

// GetByCategory retrieves questions by category
func (q *QuestionPostgreSQL) GetByCategory(ctx context.Context, tx *gorm.DB, categoryID uint, filters repositories.QuestionFilters) ([]*models.Question, error) {
	filters.CategoryID = &categoryID
	questions, _, err := q.List(ctx, tx, filters)
	return questions, err
}

// GetByType retrieves questions by type
func (q *QuestionPostgreSQL) GetByType(ctx context.Context, tx *gorm.DB, questionType models.QuestionType, filters repositories.QuestionFilters) ([]*models.Question, error) {
	filters.Type = &questionType
	questions, _, err := q.List(ctx, tx, filters)
	return questions, err
}

// GetByDifficulty retrieves questions by difficulty level
func (q *QuestionPostgreSQL) GetByDifficulty(ctx context.Context, tx *gorm.DB, difficulty models.DifficultyLevel, limit, offset int) ([]*models.Question, error) {
	db := q.getDB(tx)
	var questions []*models.Question
	query := db.WithContext(ctx).Where("difficulty = ?", difficulty)

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&questions).Error; err != nil {
		return nil, fmt.Errorf("failed to get questions by difficulty: %w", err)
	}

	return questions, nil
}

// Search performs full-text search on questions
func (q *QuestionPostgreSQL) Search(ctx context.Context, tx *gorm.DB, query string, filters repositories.QuestionFilters) ([]*models.Question, int64, error) {
	db := q.getDB(tx)
	dbQuery := db.WithContext(ctx).Model(&models.Question{})

	// Apply text search
	if query != "" {
		searchTerm := "%" + strings.ToLower(query) + "%"
		dbQuery = dbQuery.Where("LOWER(text) LIKE ? OR LOWER(explanation) LIKE ?", searchTerm, searchTerm)
	}

	// Apply additional filters
	dbQuery = q.applyQuestionFilters(dbQuery, filters)

	// Count total records
	var total int64
	if err := dbQuery.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	// Apply pagination and sorting
	dbQuery = q.helpers.ApplyPaginationAndSort(dbQuery, filters.SortBy, filters.SortOrder, filters.Limit, filters.Offset)

	var questions []*models.Question
	if err := dbQuery.Find(&questions).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to search questions: %w", err)
	}

	return questions, total, nil
}

// ===== ASSESSMENT-SPECIFIC QUERIES =====

// GetByAssessment retrieves questions for a specific assessment with caching
func (q *QuestionPostgreSQL) GetByAssessment(ctx context.Context, tx *gorm.DB, assessmentID uint) ([]*models.Question, error) {
	db := q.getDB(tx)
	// Cache frequently accessed assessment questions
	cacheKey := fmt.Sprintf("assessment:%d", assessmentID)
	var questions []*models.Question

	err := q.cacheManager.Question.CacheOrExecute(ctx, cacheKey, &questions, cache.QuestionCacheConfig.TTL, func() (interface{}, error) {
		var dbQuestions []*models.Question
		if err := db.WithContext(ctx).
			Joins("JOIN assessment_questions aq ON aq.question_id = questions.id").
			Where("aq.assessment_id = ?", assessmentID).
			Order("aq.order ASC").
			Find(&dbQuestions).Error; err != nil {
			return nil, fmt.Errorf("failed to get questions by assessment: %w", err)
		}
		return dbQuestions, nil
	})

	return questions, err
}

// GetRandomQuestions retrieves random questions based on filters
func (q *QuestionPostgreSQL) GetRandomQuestions(ctx context.Context, tx *gorm.DB, filters repositories.RandomQuestionFilters) ([]*models.Question, error) {
	db := q.getDB(tx)
	query := db.WithContext(ctx).Model(&models.Question{})

	// Apply filters
	if filters.CategoryID != nil {
		query = query.Where("category_id = ?", *filters.CategoryID)
	}
	if filters.Difficulty != nil {
		query = query.Where("difficulty = ?", *filters.Difficulty)
	}
	if filters.Type != nil {
		query = query.Where("type = ?", *filters.Type)
	}
	if len(filters.ExcludeIDs) > 0 {
		query = query.Where("id NOT IN ?", filters.ExcludeIDs)
	}

	// Apply random ordering and limit
	query = query.Order("RANDOM()").Limit(filters.Count)

	var questions []*models.Question
	if err := query.Find(&questions).Error; err != nil {
		return nil, fmt.Errorf("failed to get random questions: %w", err)
	}

	return questions, nil
}

// GetQuestionBank retrieves questions for question bank
func (q *QuestionPostgreSQL) GetQuestionBank(ctx context.Context, tx *gorm.DB, creatorID string, filters repositories.QuestionBankFilters) ([]*models.Question, int64, error) {
	db := q.getDB(tx)
	query := db.WithContext(ctx).Model(&models.Question{}).Where("created_by = ?", creatorID)

	// Apply bank-specific filters
	if filters.CategoryID != nil {
		query = query.Where("category_id = ?", *filters.CategoryID)
	}
	if filters.Type != nil {
		query = query.Where("type = ?", *filters.Type)
	}
	if filters.Difficulty != nil {
		query = query.Where("difficulty = ?", *filters.Difficulty)
	}
	if len(filters.Tags) > 0 {
		for _, tag := range filters.Tags {
			query = query.Where("tags::text LIKE ?", "%\""+tag+"\"%")
		}
	}

	// Count total records
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count question bank: %w", err)
	}

	// Apply pagination and sorting
	query = q.helpers.ApplyPaginationAndSort(query, filters.SortBy, filters.SortOrder, filters.Limit, filters.Offset)

	var questions []*models.Question
	if err := query.Find(&questions).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get question bank: %w", err)
	}

	return questions, total, nil
}

// ===== ADVANCED FILTERING =====

// GetByTags retrieves questions by tags
func (q *QuestionPostgreSQL) GetByTags(ctx context.Context, tx *gorm.DB, tags []string, filters repositories.QuestionFilters) ([]*models.Question, error) {
	db := q.getDB(tx)
	query := db.WithContext(ctx).Model(&models.Question{})

	// Apply tag filters using JSONB operations
	for _, tag := range tags {
		query = query.Where("tags::text LIKE ?", "%\""+tag+"\"%")
	}

	// Apply additional filters
	query = q.applyQuestionFilters(query, filters)

	// Apply pagination and sorting
	query = q.helpers.ApplyPaginationAndSort(query, filters.SortBy, filters.SortOrder, filters.Limit, filters.Offset)

	var questions []*models.Question
	if err := query.Find(&questions).Error; err != nil {
		return nil, fmt.Errorf("failed to get questions by tags: %w", err)
	}

	return questions, nil
}

// GetSimilarQuestions finds similar questions based on text similarity
func (q *QuestionPostgreSQL) GetSimilarQuestions(ctx context.Context, tx *gorm.DB, questionID uint, limit int) ([]*models.Question, error) {
	db := q.getDB(tx)
	// Get the base question first
	baseQuestion, err := q.GetByID(ctx, tx, questionID)
	if err != nil {
		return nil, err
	}

	// Find similar questions using simple text matching
	words := strings.Fields(strings.ToLower(baseQuestion.Text))
	if len(words) < 2 {
		return []*models.Question{}, nil
	}

	// Use first few significant words for similarity
	searchWords := words[:min(len(words), 5)]
	likeConditions := make([]string, len(searchWords))
	args := make([]interface{}, len(searchWords)+1)

	for i, word := range searchWords {
		likeConditions[i] = "LOWER(text) LIKE ?"
		args[i] = "%" + word + "%"
	}
	args[len(searchWords)] = questionID

	whereClause := fmt.Sprintf("(%s) AND id != ?", strings.Join(likeConditions, " OR "))

	var questions []*models.Question
	if err := db.WithContext(ctx).
		Where(whereClause, args...).
		Where("type = ? AND created_by = ?", baseQuestion.Type, baseQuestion.CreatedBy).
		Limit(limit).
		Find(&questions).Error; err != nil {
		return nil, fmt.Errorf("failed to get similar questions: %w", err)
	}

	return questions, nil
}

// ===== STATISTICS AND ANALYTICS =====

// GetQuestionStats retrieves basic statistics for a question
func (q *QuestionPostgreSQL) GetQuestionStats(ctx context.Context, tx *gorm.DB, id uint) (*repositories.QuestionStats, error) {
	db := q.getDB(tx)
	stats := &repositories.QuestionStats{}

	// Get usage count from assessment_questions table
	var usageCount int64
	if err := db.WithContext(ctx).
		Table("assessment_questions").
		Where("question_id = ?", id).
		Count(&usageCount).Error; err != nil {
		return nil, fmt.Errorf("failed to get question usage count: %w", err)
	}
	stats.UsageCount = int(usageCount)

	// Get performance statistics from answers table if exists
	var correctAnswers, totalAnswers int64
	err := db.WithContext(ctx).
		Table("student_answers").
		Where("question_id = ?", id).
		Count(&totalAnswers).Error
	if err == nil && totalAnswers > 0 {
		db.WithContext(ctx).
			Table("student_answers").
			Where("question_id = ? AND score > 0", id).
			Count(&correctAnswers)

		stats.CorrectRate = float64(correctAnswers) / float64(totalAnswers)
	}

	return stats, nil
}

// GetUsageStats retrieves usage statistics for a creator
func (q *QuestionPostgreSQL) GetUsageStats(ctx context.Context, tx *gorm.DB, creatorID string) (*repositories.QuestionUsageStats, error) {
	db := q.getDB(tx)
	stats := &repositories.QuestionUsageStats{
		QuestionsByType: make(map[models.QuestionType]int),
		QuestionsByDiff: make(map[models.DifficultyLevel]int),
	}

	// Get total questions count
	var totalCount int64
	if err := db.WithContext(ctx).
		Model(&models.Question{}).
		Where("created_by = ?", creatorID).
		Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count total questions: %w", err)
	}
	stats.TotalQuestions = int(totalCount)

	// Get questions by type
	var typeResults []struct {
		Type  models.QuestionType
		Count int
	}
	if err := db.WithContext(ctx).
		Model(&models.Question{}).
		Select("type, COUNT(*) as count").
		Where("created_by = ?", creatorID).
		Group("type").
		Find(&typeResults).Error; err != nil {
		return nil, fmt.Errorf("failed to get questions by type: %w", err)
	}
	for _, result := range typeResults {
		stats.QuestionsByType[result.Type] = result.Count
	}

	// Get questions by difficulty
	var diffResults []struct {
		Difficulty models.DifficultyLevel
		Count      int
	}
	if err := db.WithContext(ctx).
		Model(&models.Question{}).
		Select("difficulty, COUNT(*) as count").
		Where("created_by = ?", creatorID).
		Group("difficulty").
		Find(&diffResults).Error; err != nil {
		return nil, fmt.Errorf("failed to get questions by difficulty: %w", err)
	}
	for _, result := range diffResults {
		stats.QuestionsByDiff[result.Difficulty] = result.Count
	}

	return stats, nil
}

// GetPerformanceStats retrieves detailed performance statistics for a question
func (q *QuestionPostgreSQL) GetPerformanceStats(ctx context.Context, tx *gorm.DB, questionID uint) (*repositories.QuestionPerformanceStats, error) {
	stats := &repositories.QuestionPerformanceStats{
		AnswerDistribution: make(map[string]int),
	}

	// This would require answers table - implementing basic version
	// In a real implementation, you would join with answers/attempts tables

	return stats, nil
}

// ===== VALIDATION AND CHECKS =====

// ExistsByText checks if a question with the same text exists for the creator
func (q *QuestionPostgreSQL) ExistsByText(ctx context.Context, tx *gorm.DB, text string, creatorID string, excludeID *uint) (bool, error) {
	db := q.getDB(tx)
	query := db.WithContext(ctx).
		Model(&models.Question{}).
		Where("text = ? AND created_by = ?", text, creatorID)

	if excludeID != nil {
		query = query.Where("id != ?", *excludeID)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check question text existence: %w", err)
	}

	return count > 0, nil
}

// IsUsedInAssessments checks if a question is used in any assessments
func (q *QuestionPostgreSQL) IsUsedInAssessments(ctx context.Context, tx *gorm.DB, id uint) (bool, error) {
	db := q.getDB(tx)
	var count int64
	if err := db.WithContext(ctx).
		Table("assessment_questions").
		Where("question_id = ?", id).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check question usage in assessments: %w", err)
	}

	return count > 0, nil
}

// GetUsageCount returns how many times a question has been used
func (q *QuestionPostgreSQL) GetUsageCount(ctx context.Context, tx *gorm.DB, id uint) (int, error) {
	db := q.getDB(tx)
	var count int64
	if err := db.WithContext(ctx).
		Table("assessment_questions").
		Where("question_id = ?", id).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to get question usage count: %w", err)
	}

	return int(count), nil
}

// ===== CONTENT MANAGEMENT =====

// UpdateContent updates only the content field of a question
func (q *QuestionPostgreSQL) UpdateContent(ctx context.Context, tx *gorm.DB, id uint, content interface{}) error {
	db := q.getDB(tx)

	// Get question info before updating for cache invalidation
	var question models.Question
	if err := db.WithContext(ctx).Select("id, created_by").First(&question, id).Error; err != nil {
		return fmt.Errorf("failed to get question before update content: %w", err)
	}

	// Validate content structure
	contentBytes, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("failed to marshal content: %w", err)
	}

	if err := db.WithContext(ctx).
		Model(&models.Question{}).
		Where("id = ?", id).
		Update("content", contentBytes).Error; err != nil {
		return fmt.Errorf("failed to update question content: %w", err)
	}

	cache.InvalidateQuestionCache(ctx, q.cacheManager, id, question.CreatedBy)
	q.invalidateAssessmentCachesForQuestion(ctx, db, id)

	return nil
}

// ===== QUESTION BANK OPERATIONS =====

// GetByBank retrieves questions from a specific question bank with filters
func (q *QuestionPostgreSQL) GetByBank(ctx context.Context, bankID uint, filters repositories.QuestionFilters) ([]*models.Question, int64, error) {
	db := q.db
	var questions []*models.Question
	var total int64

	// Build the query to get questions from the bank
	query := db.WithContext(ctx).
		Table("questions q").
		Select("q.*").
		Joins("INNER JOIN question_bank_questions qbq ON q.id = qbq.question_id").
		Where("qbq.question_bank_id = ?", bankID).
		Preload("Category").
		Preload("Creator").
		Preload("Attachments")

	// Apply question filters
	query = q.applyQuestionFilters(query, filters)

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count questions in bank: %w", err)
	}

	// Apply pagination and sorting with safe column names
	allowedSortColumns := map[string]bool{
		"created_at": true,
		"updated_at": true,
		"difficulty": true,
		"type":       true,
	}

	sortBy := "created_at"
	if filters.SortBy != "" && allowedSortColumns[filters.SortBy] {
		sortBy = fmt.Sprintf("q.%s", filters.SortBy)
	}

	sortOrder := "DESC"
	if filters.SortOrder == "asc" || filters.SortOrder == "ASC" {
		sortOrder = "ASC"
	}

	query = query.Order(sortBy + " " + sortOrder)

	if filters.Limit > 0 {
		query = query.Limit(filters.Limit)
	}
	if filters.Offset > 0 {
		query = query.Offset(filters.Offset)
	}

	if err := query.Find(&questions).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get questions from bank: %w", err)
	}

	return questions, total, nil
}

// AddToBank adds a question to a question bank
func (q *QuestionPostgreSQL) AddToBank(ctx context.Context, questionID, bankID uint) error {
	db := q.db

	// Check if the relationship already exists
	var count int64
	if err := db.WithContext(ctx).
		Table("question_bank_questions").
		Where("question_id = ? AND question_bank_id = ?", questionID, bankID).
		Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check question-bank relationship: %w", err)
	}

	if count > 0 {
		return nil // Already exists, no error
	}

	// Insert the relationship
	if err := db.WithContext(ctx).
		Exec("INSERT INTO question_bank_questions (question_id, question_bank_id, created_at) VALUES (?, ?, NOW())",
			questionID, bankID).Error; err != nil {
		return fmt.Errorf("failed to add question to bank: %w", err)
	}

	cache.SafeInvalidatePattern(ctx, q.cacheManager.Question, fmt.Sprintf("bank:%d:*", bankID))
	return nil
}

// RemoveFromBank removes a question from a question bank
func (q *QuestionPostgreSQL) RemoveFromBank(ctx context.Context, questionID, bankID uint) error {
	db := q.db

	if err := db.WithContext(ctx).
		Exec("DELETE FROM question_bank_questions WHERE question_id = ? AND question_bank_id = ?",
			questionID, bankID).Error; err != nil {
		return fmt.Errorf("failed to remove question from bank: %w", err)
	}

	cache.SafeInvalidatePattern(ctx, q.cacheManager.Question, fmt.Sprintf("bank:%d:*", bankID))
	return nil
}

// ===== HELPER METHODS =====

// applyQuestionFilters applies common question filters to a query
func (q *QuestionPostgreSQL) applyQuestionFilters(query *gorm.DB, filters repositories.QuestionFilters) *gorm.DB {
	if filters.Type != nil {
		query = query.Where("type = ?", *filters.Type)
	}
	if filters.Difficulty != nil {
		query = query.Where("difficulty = ?", *filters.Difficulty)
	}
	if filters.CategoryID != nil {
		query = query.Where("category_id = ?", *filters.CategoryID)
	}
	if filters.CreatedBy != nil {
		query = query.Where("created_by = ?", *filters.CreatedBy)
	}
	if len(filters.Tags) > 0 {
		for _, tag := range filters.Tags {
			query = query.Where("tags::text LIKE ?", "%\""+tag+"\"%")
		}
	}

	return query
}

// getDB returns the transaction DB if provided, otherwise returns the default DB
func (q *QuestionPostgreSQL) getDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return q.db
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// invalidateAssessmentCachesForQuestion invalidates all assessment caches that use this question
func (q *QuestionPostgreSQL) invalidateAssessmentCachesForQuestion(ctx context.Context, db *gorm.DB, questionID uint) {
	// Get all assessment IDs that use this question
	var assessmentIDs []uint
	if err := db.WithContext(ctx).
		Table("assessment_questions").
		Where("question_id = ?", questionID).
		Pluck("assessment_id", &assessmentIDs).Error; err != nil {
		// Log error but don't fail the operation
		return
	}

	for _, assessmentID := range assessmentIDs {
		cache.SafeDelete(ctx, q.cacheManager.Assessment,
			fmt.Sprintf("id:%d", assessmentID),
			fmt.Sprintf("details:%d", assessmentID))
		cache.SafeDelete(ctx, q.cacheManager.Question, fmt.Sprintf("assessment:%d", assessmentID))
		cache.SafeInvalidatePattern(ctx, q.cacheManager.Stats, fmt.Sprintf("assessment:%d:*", assessmentID))
	}
}
