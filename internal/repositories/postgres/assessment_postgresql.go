package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/cache"
	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type AssessmentPostgreSQL struct {
	db           *gorm.DB
	helpers      *SharedHelpers
	cacheManager *cache.CacheManager
}

func NewAssessmentPostgreSQL(db *gorm.DB, redisClient *redis.Client) repositories.AssessmentRepository {
	return &AssessmentPostgreSQL{
		db:           db,
		helpers:      NewSharedHelpers(db),
		cacheManager: cache.NewCacheManager(redisClient),
	}
}

// getDB returns the transaction DB if provided, otherwise returns the default DB
func (a *AssessmentPostgreSQL) getDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return a.db
}

// Create creates a new assessment with default settings and invalidates cache
func (a *AssessmentPostgreSQL) Create(ctx context.Context, tx *gorm.DB, assessment *models.Assessment) error {
	if err := tx.WithContext(ctx).Create(assessment).Error; err != nil {
		return fmt.Errorf("failed to create assessment: %w", err)
	}
	cache.SafeInvalidatePattern(ctx, a.cacheManager.Assessment, fmt.Sprintf("creator:%s:*", assessment.CreatedBy))
	cache.SafeInvalidatePattern(ctx, a.cacheManager.Assessment, "list:*")

	return nil
}

// GetByID retrieves an assessment by ID with caching
func (a *AssessmentPostgreSQL) GetByID(ctx context.Context, tx *gorm.DB, id uint) (*models.Assessment, error) {
	// Try cache first for fast performance (<200ms requirement)
	cacheKey := fmt.Sprintf("id:%d", id)
	var assessment models.Assessment

	err := a.cacheManager.Assessment.CacheOrExecute(ctx, cacheKey, &assessment, cache.AssessmentCacheConfig.TTL, func() (interface{}, error) {
		var dbAssessment models.Assessment
		err := tx.WithContext(ctx).
			Preload("Creator").
			Preload("Settings").
			First(&dbAssessment, id).Error
		if err != nil {
			return nil, fmt.Errorf("failed to get assessment: %w", err)
		}
		return &dbAssessment, nil
	})

	if err != nil {
		return nil, err
	}

	return &assessment, nil
}

// GetByIDWithDetails retrieves an assessment with full details (questions, settings)
func (a *AssessmentPostgreSQL) GetByIDWithDetails(ctx context.Context, tx *gorm.DB, id uint) (*models.Assessment, error) {
	// Cache the most expensive query with shorter TTL
	cacheKey := fmt.Sprintf("details:%d", id)
	var assessment models.Assessment

	err := a.cacheManager.Assessment.CacheOrExecute(ctx, cacheKey, &assessment, cache.AssessmentCacheConfig.TTL, func() (interface{}, error) {
		var dbAssessment models.Assessment
		err := tx.WithContext(ctx).
			Preload("Creator").
			Preload("Settings").
			Preload("Questions", func(db *gorm.DB) *gorm.DB {
				return db.Order("assessment_questions.order ASC")
			}).
			Preload("Questions.Question").
			First(&dbAssessment, id).Error
		if err != nil {
			return nil, fmt.Errorf("failed to get assessment details: %w", err)
		}

		// Calculate computed fields
		a.calculateComputedFields(&dbAssessment)
		return &dbAssessment, nil
	})

	return &assessment, err
}

// Update updates an assessment and invalidates cache
func (a *AssessmentPostgreSQL) Update(ctx context.Context, tx *gorm.DB, assessment *models.Assessment) error {
	// Get current assessment for validation
	//var currentAssessment models.Assessment
	//if err := tx.WithContext(ctx).First(&currentAssessment, assessment.ID).Error; err != nil {
	//	return fmt.Errorf("assessment not found: %w", err)
	//}
	//
	//// Check title uniqueness if title changed
	//if assessment.Title != currentAssessment.Title {
	//	exists, err := a.ExistsByTitle(ctx, tx, assessment.Title, assessment.CreatedBy, &assessment.ID)
	//	if err != nil {
	//		return fmt.Errorf("failed to check title uniqueness: %w", err)
	//	}
	//	if exists {
	//		return fmt.Errorf("assessment with title '%s' already exists for this creator", assessment.Title)
	//	}
	//}
	//
	//// Validate business rules for active assessments
	//if currentAssessment.Status == models.StatusActive {
	//	// Check if assessment has attempts
	//	hasAttempts, err := a.HasAttempts(ctx, tx, assessment.ID)
	//	if err != nil {
	//		return fmt.Errorf("failed to check attempts: %w", err)
	//	}
	//
	//	if hasAttempts {
	//		// Restrict modifications for assessments with attempts
	//		if assessment.Duration != currentAssessment.Duration {
	//			return fmt.Errorf("cannot change duration for active assessment with attempts")
	//		}
	//		if assessment.MaxAttempts < currentAssessment.MaxAttempts {
	//			return fmt.Errorf("cannot decrease max attempts for assessment with existing attempts")
	//		}
	//	}
	//}

	// Increment version
	//assessment.Version = currentAssessment.Version + 1
	//assessment.UpdatedAt = time.Now()

	// Update assessment
	if err := tx.WithContext(ctx).Model(&models.Assessment{}).Where("id = ?", assessment.ID).Updates(map[string]interface{}{
		"title":         assessment.Title,
		"description":   assessment.Description,
		"duration":      assessment.Duration,
		"max_attempts":  assessment.MaxAttempts,
		"passing_score": assessment.PassingScore,
		"time_warning":  assessment.TimeWarning,
		"due_date":      assessment.DueDate,
		"status":        assessment.Status,
		"version":       assessment.Version,
		"updated_at":    assessment.UpdatedAt,
	}).Error; err != nil {
		return fmt.Errorf("failed to update assessment: %w", err)
	}

	cache.InvalidateAssessmentCache(ctx, a.cacheManager, assessment.ID, assessment.CreatedBy)

	return nil
}

// Delete hard deletes an assessment
func (a *AssessmentPostgreSQL) Delete(ctx context.Context, tx *gorm.DB, id uint) error {
	// Get assessment info before deleting for cache invalidation
	var assessment models.Assessment
	if err := tx.WithContext(ctx).Select("id, created_by").First(&assessment, id).Error; err != nil {
		return fmt.Errorf("failed to get assessment before delete: %w", err)
	}

	// Check if assessment has attempts before deleting
	hasAttempts, err := a.HasAttempts(ctx, tx, id)
	if err != nil {
		return fmt.Errorf("failed to check attempts: %w", err)
	}
	if hasAttempts {
		return fmt.Errorf("cannot delete assessment with existing attempts")
	}

	if err := tx.WithContext(ctx).Unscoped().Delete(&models.Assessment{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete assessment: %w", err)
	}

	cache.InvalidateAssessmentCache(ctx, a.cacheManager, id, assessment.CreatedBy)
	cache.SafeDelete(ctx, a.cacheManager.Question, fmt.Sprintf("assessment:%d", id))

	return nil
}

// List retrieves assessments with filters and pagination
func (a *AssessmentPostgreSQL) List(ctx context.Context, tx *gorm.DB, filters repositories.AssessmentFilters) ([]*models.Assessment, int64, error) {
	query := tx.WithContext(ctx).Model(&models.Assessment{})

	// Apply filters
	query = a.applyFilters(query, filters)

	// Count total
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination and ordering
	query = a.applyPaginationAndSort(query, filters)

	// Execute query
	var assessments []*models.Assessment
	err := query.Preload("Creator").Find(&assessments).Error
	if err != nil {
		return nil, 0, err
	}

	// Calculate computed fields for each assessment
	for _, assessment := range assessments {
		a.calculateComputedFields(assessment)
	}

	return assessments, total, nil
}

// GetByCreator retrieves assessments created by a specific user
func (a *AssessmentPostgreSQL) GetByCreator(ctx context.Context, tx *gorm.DB, creatorID string, filters repositories.AssessmentFilters) ([]*models.Assessment, int64, error) {
	filters.CreatedBy = &creatorID
	return a.List(ctx, tx, filters)
}

// GetByStatus retrieves assessments by status with pagination
func (a *AssessmentPostgreSQL) GetByStatus(ctx context.Context, tx *gorm.DB, status models.AssessmentStatus, limit, offset int) ([]*models.Assessment, error) {
	db := a.getDB(tx)
	var assessments []*models.Assessment
	err := db.WithContext(ctx).
		Where("status = ?", status).
		Preload("Creator").
		Limit(limit).
		Offset(offset).
		Order("created_at DESC").
		Find(&assessments).Error

	if err != nil {
		return nil, err
	}

	return assessments, nil
}

// Search performs full-text search on assessments
func (a *AssessmentPostgreSQL) Search(ctx context.Context, tx *gorm.DB, query string, filters repositories.AssessmentFilters) ([]*models.Assessment, int64, error) {
	db := a.getDB(tx).WithContext(ctx).Model(&models.Assessment{})

	// Full-text search
	if query != "" {
		searchQuery := fmt.Sprintf("%%%s%%", query)
		db = db.Where("title ILIKE ? OR description ILIKE ?", searchQuery, searchQuery)
	}

	// Apply other filters
	db = a.applyFilters(db, filters)

	// Count total
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination and ordering
	db = a.applyPaginationAndSort(db, filters)

	// Execute query
	var assessments []*models.Assessment
	err := db.Preload("Creator").Find(&assessments).Error
	if err != nil {
		return nil, 0, err
	}

	return assessments, total, nil
}

// UpdateStatus updates the status of an assessment
func (a *AssessmentPostgreSQL) UpdateStatus(ctx context.Context, tx *gorm.DB, id uint, status models.AssessmentStatus) error {
	db := a.getDB(tx)

	// Get assessment info for cache invalidation
	var assessment models.Assessment
	if err := db.WithContext(ctx).Select("id, created_by").First(&assessment, id).Error; err != nil {
		return fmt.Errorf("failed to get assessment: %w", err)
	}

	if err := db.WithContext(ctx).
		Model(&models.Assessment{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		}).Error; err != nil {
		return err
	}

	cache.InvalidateAssessmentCache(ctx, a.cacheManager, id, assessment.CreatedBy)

	return nil
}

// GetExpiredAssessments retrieves assessments that have passed their due date
func (a *AssessmentPostgreSQL) GetExpiredAssessments(ctx context.Context, tx *gorm.DB) ([]*models.Assessment, error) {
	db := a.getDB(tx)
	var assessments []*models.Assessment
	err := db.WithContext(ctx).
		Where("status = ? AND due_date IS NOT NULL AND due_date < ?", models.StatusActive, time.Now()).
		Preload("Creator").
		Find(&assessments).Error

	return assessments, err
}

// AutoExpireAssessments automatically expires assessments past due date
func (a *AssessmentPostgreSQL) AutoExpireAssessments(ctx context.Context) (int, error) {
	result := a.db.WithContext(ctx).
		Model(&models.Assessment{}).
		Where("status = ? AND due_date IS NOT NULL AND due_date < ?", models.StatusActive, time.Now()).
		Updates(map[string]interface{}{
			"status":     models.StatusExpired,
			"updated_at": time.Now(),
		})

	return int(result.RowsAffected), result.Error
}

// GetAssessmentsNearExpiry gets assessments expiring within specified duration
func (a *AssessmentPostgreSQL) GetAssessmentsNearExpiry(ctx context.Context, withinDuration time.Duration) ([]*models.Assessment, error) {
	var assessments []*models.Assessment
	expiryTime := time.Now().Add(withinDuration)

	err := a.db.WithContext(ctx).
		Where("status = ? AND due_date IS NOT NULL AND due_date BETWEEN ? AND ?",
			models.StatusActive, time.Now(), expiryTime).
		Preload("Creator").
		Find(&assessments).Error

	return assessments, err
}

// BulkUpdateStatus updates the status of multiple assessments
func (a *AssessmentPostgreSQL) BulkUpdateStatus(ctx context.Context, tx *gorm.DB, ids []uint, status models.AssessmentStatus) error {
	return a.helpers.BulkUpdateAssessmentStatus(ctx, ids, status)
}

// IsOwner checks if a user is the owner of an assessment
func (a *AssessmentPostgreSQL) IsOwner(ctx context.Context, tx *gorm.DB, assessmentID uint, userID string) (bool, error) {
	db := a.getDB(tx)
	var count int64
	err := db.WithContext(ctx).
		Model(&models.Assessment{}).
		Where("id = ? AND created_by = ?", assessmentID, userID).
		Count(&count).Error

	return count > 0, err
}

// CanAccess checks if a user can access an assessment based on role
func (a *AssessmentPostgreSQL) CanAccess(ctx context.Context, tx *gorm.DB, assessmentID uint, userID string, role models.UserRole) (bool, error) {
	db := a.getDB(tx)
	// Admins can access everything
	if role == models.RoleAdmin {
		return true, nil
	}

	// Teachers can access their own assessments
	if role == models.RoleTeacher {
		return a.IsOwner(ctx, tx, assessmentID, userID)
	}

	// Students can only access active assessments they're enrolled in
	if role == models.RoleStudent {
		// Check if assessment is active
		var assessment models.Assessment
		err := db.WithContext(ctx).
			Select("status").
			First(&assessment, assessmentID).Error
		if err != nil {
			return false, err
		}

		return assessment.Status == models.StatusActive, nil
	}

	return false, nil
}

// GetAssessmentStats retrieves statistics for an assessment
func (a *AssessmentPostgreSQL) GetAssessmentStats(ctx context.Context, tx *gorm.DB, id uint) (*repositories.AssessmentStats, error) {
	db := a.getDB(tx)
	stats := &repositories.AssessmentStats{}

	// Use helper for total attempts
	totalAttempts, err := a.helpers.CountAttempts(ctx, id)
	if err != nil {
		return nil, err
	}

	// Use helper for completed attempts
	completedAttempts, err := a.helpers.CountAttemptsByStatus(ctx, id, models.AttemptCompleted)
	if err != nil {
		return nil, err
	}

	// Get assessment passing score
	assessment, err := a.helpers.GetAssessmentBasicInfo(ctx, id)
	if err != nil {
		return nil, err
	}

	// Aggregate stats in fewer queries
	var avgScore, avgTimeSpent float64
	var passedAttempts int64
	if completedAttempts > 0 {
		db.WithContext(ctx).
			Model(&models.AssessmentAttempt{}).
			Select("AVG(score), AVG(time_spent), SUM(CASE WHEN score >= ? THEN 1 ELSE 0 END)", assessment.PassingScore).
			Where("assessment_id = ? AND status = ?", id, models.AttemptCompleted).
			Row().
			Scan(&avgScore, &avgTimeSpent, &passedAttempts)
	}

	passRate := float64(0)
	if completedAttempts > 0 {
		passRate = float64(passedAttempts) / float64(completedAttempts) * 100
	}

	// Get question stats in single query
	var questionCount, totalPoints int64
	db.WithContext(ctx).
		Model(&models.AssessmentQuestion{}).
		Select("COUNT(*), COALESCE(SUM(points), 0)").
		Where("assessment_id = ?", id).
		Row().
		Scan(&questionCount, &totalPoints)

	stats.TotalAttempts = int(totalAttempts)
	stats.CompletedAttempts = int(completedAttempts)
	stats.AverageScore = avgScore
	stats.PassRate = passRate
	stats.AverageTimeSpent = int(avgTimeSpent)
	stats.QuestionCount = int(questionCount)
	stats.TotalPoints = int(totalPoints)

	return stats, nil
}

// GetCreatorStats retrieves statistics for a creator
func (a *AssessmentPostgreSQL) GetCreatorStats(ctx context.Context, tx *gorm.DB, creatorID string) (*repositories.CreatorStats, error) {
	db := a.getDB(tx)
	stats := &repositories.CreatorStats{}

	// Total assessments
	var totalAssessments int64
	db.WithContext(ctx).
		Model(&models.Assessment{}).
		Where("created_by = ?", creatorID).
		Count(&totalAssessments)

	// Active assessments
	var activeAssessments int64
	db.WithContext(ctx).
		Model(&models.Assessment{}).
		Where("created_by = ? AND status = ?", creatorID, models.StatusActive).
		Count(&activeAssessments)

	// Draft assessments
	var draftAssessments int64
	db.WithContext(ctx).
		Model(&models.Assessment{}).
		Where("created_by = ? AND status = ?", creatorID, models.StatusDraft).
		Count(&draftAssessments)

	// Total questions (from assessments created by this user)
	var totalQuestions int64
	db.WithContext(ctx).
		Table("assessment_questions aq").
		Joins("JOIN assessments a ON aq.assessment_id = a.id").
		Where("a.created_by = ?", creatorID).
		Count(&totalQuestions)

	// Total attempts on creator's assessments
	var totalAttempts int64
	db.WithContext(ctx).
		Table("assessment_attempts att").
		Joins("JOIN assessments a ON att.assessment_id = a.id").
		Where("a.created_by = ?", creatorID).
		Count(&totalAttempts)

	stats.TotalAssessments = int(totalAssessments)
	stats.ActiveAssessments = int(activeAssessments)
	stats.DraftAssessments = int(draftAssessments)
	stats.TotalQuestions = int(totalQuestions)
	stats.TotalAttempts = int(totalAttempts)

	return stats, nil
}

// GetPopularAssessments retrieves the most attempted assessments
func (a *AssessmentPostgreSQL) GetPopularAssessments(ctx context.Context, tx *gorm.DB, limit int) ([]*models.Assessment, error) {
	db := a.getDB(tx)
	var assessments []*models.Assessment

	err := db.WithContext(ctx).
		Table("assessments a").
		Select("a.*, COUNT(att.id) as attempt_count").
		Joins("LEFT JOIN assessment_attempts att ON a.id = att.assessment_id").
		Where("a.status = ?", models.StatusActive).
		Group("a.id").
		Order("attempt_count DESC").
		Limit(limit).
		Preload("Creator").
		Find(&assessments).Error

	return assessments, err
}

// ExistsByTitle checks if an assessment with the same title exists for a creator
func (a *AssessmentPostgreSQL) ExistsByTitle(ctx context.Context, tx *gorm.DB, title string, creatorID string, excludeID *uint) (bool, error) {
	query := tx.WithContext(ctx).
		Model(&models.Assessment{}).
		Where("title = ? AND created_by = ?", title, creatorID)

	if excludeID != nil {
		query = query.Where("id != ?", *excludeID)
	}

	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

// HasAttempts checks if an assessment has any attempts
func (a *AssessmentPostgreSQL) HasAttempts(ctx context.Context, tx *gorm.DB, id uint) (bool, error) {
	count, err := a.helpers.CountAttempts(ctx, id)
	return count > 0, err
}

// HasActiveAttempts checks if an assessment has any active/in-progress attempts
func (a *AssessmentPostgreSQL) HasActiveAttempts(ctx context.Context, tx *gorm.DB, id uint) (bool, error) {
	db := a.getDB(tx)
	var count int64
	err := db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("assessment_id = ? AND status IN ?", id, models.AttemptInProgress).
		Count(&count).Error

	return count > 0, err
}

// UpdateSettings updates assessment settings
func (a *AssessmentPostgreSQL) UpdateSettings(ctx context.Context, tx *gorm.DB, assessmentID uint, settings *models.AssessmentSettings) error {
	db := a.getDB(tx)
	settings.AssessmentID = assessmentID

	// Get assessment info for cache invalidation
	var assessment models.Assessment
	if err := db.WithContext(ctx).Select("id, created_by").First(&assessment, assessmentID).Error; err != nil {
		return fmt.Errorf("failed to get assessment: %w", err)
	}

	if err := db.WithContext(ctx).
		Model(&models.AssessmentSettings{}).
		Where("assessment_id = ?", assessmentID).
		Updates(settings).Error; err != nil {
		return err
	}

	cache.SafeDelete(ctx, a.cacheManager.Assessment,
		fmt.Sprintf("id:%d", assessmentID),
		fmt.Sprintf("details:%d", assessmentID))
	cache.SafeInvalidatePattern(ctx, a.cacheManager.Assessment, fmt.Sprintf("creator:%s:*", assessment.CreatedBy))

	return nil
}

// GetSettings retrieves assessment settings
func (a *AssessmentPostgreSQL) GetSettings(ctx context.Context, tx *gorm.DB, assessmentID uint) (*models.AssessmentSettings, error) {
	db := a.getDB(tx)
	var settings models.AssessmentSettings
	err := db.WithContext(ctx).
		Where("assessment_id = ?", assessmentID).
		First(&settings).Error

	if err != nil {
		return nil, err
	}

	return &settings, nil
}

// UpdateDuration updates assessment duration with business rules
func (a *AssessmentPostgreSQL) UpdateDuration(ctx context.Context, tx *gorm.DB, assessmentID uint, duration int) error {
	db := a.getDB(tx)
	// Validate duration range (5-300 minutes as per docs)
	if duration < 5 || duration > 300 {
		return fmt.Errorf("duration must be between 5 and 300 minutes")
	}

	// Check if assessment can be modified
	var assessment models.Assessment
	err := db.WithContext(ctx).
		Select("status, created_by").
		First(&assessment, assessmentID).Error
	if err != nil {
		return err
	}

	// Only allow duration change for Draft assessments
	if assessment.Status != models.StatusDraft {
		return fmt.Errorf("can only modify duration for draft assessments")
	}

	if err := db.WithContext(ctx).
		Model(&models.Assessment{}).
		Where("id = ?", assessmentID).
		Update("duration", duration).Error; err != nil {
		return err
	}

	cache.InvalidateAssessmentCache(ctx, a.cacheManager, assessmentID, assessment.CreatedBy)

	return nil
}

// UpdateMaxAttempts updates max attempts with business rules
func (a *AssessmentPostgreSQL) UpdateMaxAttempts(ctx context.Context, tx *gorm.DB, assessmentID uint, maxAttempts int) error {
	db := a.getDB(tx)
	// Validate max attempts range (1-10 as per docs)
	if maxAttempts < 1 || maxAttempts > 10 {
		return fmt.Errorf("max attempts must be between 1 and 10")
	}

	// Get current max attempts and creator
	var assessment models.Assessment
	err := db.WithContext(ctx).
		Model(&models.Assessment{}).
		Select("max_attempts, created_by").
		Where("id = ?", assessmentID).
		First(&assessment).Error
	if err != nil {
		return err
	}

	// Check if assessment has attempts
	hasAttempts, err := a.HasAttempts(ctx, tx, assessmentID)
	if err != nil {
		return err
	}

	// If has attempts, only allow increasing max attempts
	if hasAttempts && maxAttempts < assessment.MaxAttempts {
		return fmt.Errorf("cannot decrease max attempts when assessment has existing attempts")
	}

	if err := db.WithContext(ctx).
		Model(&models.Assessment{}).
		Where("id = ?", assessmentID).
		Update("max_attempts", maxAttempts).Error; err != nil {
		return err
	}

	cache.InvalidateAssessmentCache(ctx, a.cacheManager, assessmentID, assessment.CreatedBy)

	return nil
}

// Helper methods

// applyFilters applies common filters to a query
func (a *AssessmentPostgreSQL) applyFilters(query *gorm.DB, filters repositories.AssessmentFilters) *gorm.DB {
	return a.helpers.ApplyAssessmentFilters(query, filters)
}

// applyPaginationAndSort applies pagination and sorting to a query
func (a *AssessmentPostgreSQL) applyPaginationAndSort(query *gorm.DB, filters repositories.AssessmentFilters) *gorm.DB {
	return a.helpers.ApplyPaginationAndSort(query, filters.SortBy, filters.SortOrder, filters.Limit, filters.Offset)
}

// calculateComputedFields calculates computed fields for an assessment
func (a *AssessmentPostgreSQL) calculateComputedFields(assessment *models.Assessment) {
	// Calculate questions count
	assessment.QuestionsCount = len(assessment.Questions)

	// Calculate total points
	totalPoints := 0
	for _, aq := range assessment.Questions {
		if aq.Points != nil {
			totalPoints += *aq.Points
		}
	}
	assessment.TotalPoints = totalPoints

	// Calculate attempt count
	assessment.AttemptCount = len(assessment.Attempts)

	// Calculate average score
	if len(assessment.Attempts) > 0 {
		totalScore := 0.0
		completedCount := 0
		for _, attempt := range assessment.Attempts {
			if attempt.Status == models.AttemptCompleted {
				totalScore += attempt.Score
				completedCount++
			}
		}
		if completedCount > 0 {
			assessment.AvgScore = totalScore / float64(completedCount)
		}
	}
}
