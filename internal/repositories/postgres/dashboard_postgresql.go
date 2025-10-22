package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"gorm.io/gorm"
)

type dashboardRepository struct {
	db *gorm.DB
}

func NewDashboardRepository(db *gorm.DB) repositories.DashboardRepository {
	return &dashboardRepository{db: db}
}

func (r *dashboardRepository) getDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return r.db
}

// ===== DASHBOARD STATS =====

func (r *dashboardRepository) GetTotalAssessments(ctx context.Context, tx *gorm.DB) (int64, error) {
	db := r.getDB(tx)
	var count int64

	if err := db.WithContext(ctx).
		Model(&models.Assessment{}).
		Where("deleted_at IS NULL").
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to get total assessments: %w", err)
	}

	return count, nil
}

func (r *dashboardRepository) GetTotalQuestions(ctx context.Context, tx *gorm.DB) (int64, error) {
	db := r.getDB(tx)
	var count int64

	if err := db.WithContext(ctx).
		Model(&models.Question{}).
		Where("deleted_at IS NULL").
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to get total questions: %w", err)
	}

	return count, nil
}

func (r *dashboardRepository) GetTotalQuestionBanks(ctx context.Context, tx *gorm.DB) (int64, error) {
	db := r.getDB(tx)
	var count int64

	if err := db.WithContext(ctx).
		Model(&models.QuestionBank{}).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to get total question banks: %w", err)
	}

	return count, nil
}

func (r *dashboardRepository) GetTotalAttempts(ctx context.Context, tx *gorm.DB) (int64, error) {
	db := r.getDB(tx)
	var count int64

	if err := db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to get total attempts: %w", err)
	}

	return count, nil
}

func (r *dashboardRepository) GetActiveUsers(ctx context.Context, tx *gorm.DB, days int) (int64, error) {
	db := r.getDB(tx)
	var count int64

	startDate := time.Now().AddDate(0, 0, -days)

	if err := db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("created_at >= ?", startDate).
		Distinct("student_id").
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to get active users: %w", err)
	}

	return count, nil
}

// ===== METRICS =====

func (r *dashboardRepository) GetCompletionRate(ctx context.Context, tx *gorm.DB) (float64, error) {
	db := r.getDB(tx)

	var total int64
	var completed int64

	if err := db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Count(&total).Error; err != nil {
		return 0, fmt.Errorf("failed to get total attempts: %w", err)
	}

	if total == 0 {
		return 0, nil
	}

	if err := db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("status = ?", models.AttemptCompleted).
		Count(&completed).Error; err != nil {
		return 0, fmt.Errorf("failed to get completed attempts: %w", err)
	}

	return float64(completed) / float64(total) * 100, nil
}

func (r *dashboardRepository) GetAverageScore(ctx context.Context, tx *gorm.DB) (float64, error) {
	db := r.getDB(tx)

	var result struct {
		AvgScore float64
	}

	if err := db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("status = ?", models.AttemptCompleted).
		Select("AVG(score) as avg_score").
		Scan(&result).Error; err != nil {
		return 0, fmt.Errorf("failed to get average score: %w", err)
	}

	return result.AvgScore, nil
}

func (r *dashboardRepository) GetPassRate(ctx context.Context, tx *gorm.DB) (float64, error) {
	db := r.getDB(tx)

	var totalCompleted int64
	var passed int64

	if err := db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("status = ?", models.AttemptCompleted).
		Count(&totalCompleted).Error; err != nil {
		return 0, fmt.Errorf("failed to get total completed attempts: %w", err)
	}

	if totalCompleted == 0 {
		return 0, nil
	}

	if err := db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("status = ? AND passed = ?", models.AttemptCompleted, true).
		Count(&passed).Error; err != nil {
		return 0, fmt.Errorf("failed to get passed attempts: %w", err)
	}

	return float64(passed) / float64(totalCompleted) * 100, nil
}

// ===== TRENDS =====

func (r *dashboardRepository) GetTrendChange(ctx context.Context, tx *gorm.DB, entity string, days int) (float64, error) {
	db := r.getDB(tx)

	currentPeriodStart := time.Now().AddDate(0, 0, -days)
	previousPeriodStart := time.Now().AddDate(0, 0, -days*2)
	previousPeriodEnd := currentPeriodStart

	var currentCount int64
	var previousCount int64

	var model interface{}
	switch entity {
	case "assessments":
		model = &models.Assessment{}
	case "attempts":
		model = &models.AssessmentAttempt{}
	case "questions":
		model = &models.Question{}
	default:
		return 0, fmt.Errorf("unsupported entity: %s", entity)
	}

	// Current period count
	query := db.WithContext(ctx).Model(model).Where("created_at >= ?", currentPeriodStart)
	if entity == "assessments" || entity == "questions" {
		query = query.Where("deleted_at IS NULL")
	}
	if err := query.Count(&currentCount).Error; err != nil {
		return 0, fmt.Errorf("failed to get current count: %w", err)
	}

	// Previous period count
	query = db.WithContext(ctx).Model(model).
		Where("created_at >= ? AND created_at < ?", previousPeriodStart, previousPeriodEnd)
	if entity == "assessments" || entity == "questions" {
		query = query.Where("deleted_at IS NULL")
	}
	if err := query.Count(&previousCount).Error; err != nil {
		return 0, fmt.Errorf("failed to get previous count: %w", err)
	}

	if previousCount == 0 {
		if currentCount > 0 {
			return 100, nil
		}
		return 0, nil
	}

	change := float64(currentCount-previousCount) / float64(previousCount) * 100
	return change, nil
}

// ===== ACTIVITY TRENDS =====

func (r *dashboardRepository) GetActivityTrends(ctx context.Context, tx *gorm.DB, period string) ([]repositories.ActivityTrendData, error) {
	db := r.getDB(tx)

	var results []repositories.ActivityTrendData

	switch period {
	case "week":
		// Last 7 days
		for i := 6; i >= 0; i-- {
			date := time.Now().AddDate(0, 0, -i)
			startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
			endOfDay := startOfDay.Add(24 * time.Hour)

			var attempts int64
			var users int64
			var avgScore float64

			db.WithContext(ctx).
				Model(&models.AssessmentAttempt{}).
				Where("created_at >= ? AND created_at < ?", startOfDay, endOfDay).
				Count(&attempts)

			db.WithContext(ctx).
				Model(&models.AssessmentAttempt{}).
				Where("created_at >= ? AND created_at < ?", startOfDay, endOfDay).
				Distinct("student_id").
				Count(&users)

			var scoreResult struct {
				AvgScore float64
			}
			db.WithContext(ctx).
				Model(&models.AssessmentAttempt{}).
				Where("created_at >= ? AND created_at < ? AND status = ?", startOfDay, endOfDay, models.AttemptCompleted).
				Select("COALESCE(AVG(score), 0) as avg_score").
				Scan(&scoreResult)
			avgScore = scoreResult.AvgScore

			results = append(results, repositories.ActivityTrendData{
				Period:       date.Format("Mon"),
				Attempts:     attempts,
				Users:        users,
				AverageScore: avgScore,
				Date:         date,
			})
		}

	case "month":
		// Last 30 days, grouped by week
		for i := 3; i >= 0; i-- {
			endDate := time.Now().AddDate(0, 0, -i*7)
			startDate := endDate.AddDate(0, 0, -7)

			var attempts int64
			var users int64
			var avgScore float64

			db.WithContext(ctx).
				Model(&models.AssessmentAttempt{}).
				Where("created_at >= ? AND created_at < ?", startDate, endDate).
				Count(&attempts)

			db.WithContext(ctx).
				Model(&models.AssessmentAttempt{}).
				Where("created_at >= ? AND created_at < ?", startDate, endDate).
				Distinct("student_id").
				Count(&users)

			var scoreResult struct {
				AvgScore float64
			}
			db.WithContext(ctx).
				Model(&models.AssessmentAttempt{}).
				Where("created_at >= ? AND created_at < ? AND status = ?", startDate, endDate, models.AttemptCompleted).
				Select("COALESCE(AVG(score), 0) as avg_score").
				Scan(&scoreResult)
			avgScore = scoreResult.AvgScore

			weekNum := 4 - i
			results = append(results, repositories.ActivityTrendData{
				Period:       fmt.Sprintf("T%d", weekNum),
				Attempts:     attempts,
				Users:        users,
				AverageScore: avgScore,
				Date:         startDate,
			})
		}

	case "year":
		// Last 12 months
		for i := 11; i >= 0; i-- {
			date := time.Now().AddDate(0, -i, 0)
			startOfMonth := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
			endOfMonth := startOfMonth.AddDate(0, 1, 0)

			var attempts int64
			var users int64
			var avgScore float64

			db.WithContext(ctx).
				Model(&models.AssessmentAttempt{}).
				Where("created_at >= ? AND created_at < ?", startOfMonth, endOfMonth).
				Count(&attempts)

			db.WithContext(ctx).
				Model(&models.AssessmentAttempt{}).
				Where("created_at >= ? AND created_at < ?", startOfMonth, endOfMonth).
				Distinct("student_id").
				Count(&users)

			var scoreResult struct {
				AvgScore float64
			}
			db.WithContext(ctx).
				Model(&models.AssessmentAttempt{}).
				Where("created_at >= ? AND created_at < ? AND status = ?", startOfMonth, endOfMonth, models.AttemptCompleted).
				Select("COALESCE(AVG(score), 0) as avg_score").
				Scan(&scoreResult)
			avgScore = scoreResult.AvgScore

			results = append(results, repositories.ActivityTrendData{
				Period:       fmt.Sprintf("T%d", 12-i),
				Attempts:     attempts,
				Users:        users,
				AverageScore: avgScore,
				Date:         startOfMonth,
			})
		}
	}

	return results, nil
}

// ===== RECENT ACTIVITIES =====

func (r *dashboardRepository) GetRecentActivities(ctx context.Context, tx *gorm.DB, limit int) ([]repositories.RecentActivityData, error) {
	db := r.getDB(tx)

	var activities []repositories.RecentActivityData

	// Get recent completed assessments
	var attempts []struct {
		ID              uint
		StudentID       string
		AssessmentID    uint
		AssessmentTitle string
		Score           float64
		CreatedAt       time.Time
		UserName        string
	}

	if err := db.WithContext(ctx).
		Table("assessment_attempts").
		Select("assessment_attempts.id, assessment_attempts.student_id, assessment_attempts.assessment_id, "+
			"assessments.title as assessment_title, assessment_attempts.score, assessment_attempts.completed_at as created_at, "+
			"users.full_name as user_name").
		Joins("LEFT JOIN assessments ON assessment_attempts.assessment_id = assessments.id").
		Joins("LEFT JOIN users ON assessment_attempts.student_id = users.id").
		Where("assessment_attempts.status = ?", models.AttemptCompleted).
		Order("assessment_attempts.completed_at DESC").
		Limit(limit).
		Scan(&attempts).Error; err != nil {
		return nil, fmt.Errorf("failed to get recent activities: %w", err)
	}

	for _, attempt := range attempts {
		activities = append(activities, repositories.RecentActivityData{
			ID:              attempt.ID,
			UserID:          attempt.StudentID,
			UserName:        attempt.UserName,
			Action:          "completed_assessment",
			AssessmentID:    &attempt.AssessmentID,
			AssessmentTitle: &attempt.AssessmentTitle,
			Score:           &attempt.Score,
			CreatedAt:       attempt.CreatedAt,
		})
	}

	return activities, nil
}

// ===== QUESTION DISTRIBUTION =====

func (r *dashboardRepository) GetQuestionDistribution(ctx context.Context, tx *gorm.DB) ([]repositories.QuestionDistributionData, error) {
	db := r.getDB(tx)

	var results []struct {
		Type  string
		Count int64
	}

	if err := db.WithContext(ctx).
		Model(&models.Question{}).
		Select("type, COUNT(*) as count").
		Where("deleted_at IS NULL").
		Group("type").
		Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get question distribution: %w", err)
	}

	// Calculate total for percentages
	var total int64
	for _, r := range results {
		total += r.Count
	}

	var distribution []repositories.QuestionDistributionData
	for _, r := range results {
		percentage := float64(0)
		if total > 0 {
			percentage = float64(r.Count) / float64(total) * 100
		}

		distribution = append(distribution, repositories.QuestionDistributionData{
			Type:       r.Type,
			Count:      r.Count,
			Percentage: percentage,
		})
	}

	return distribution, nil
}

// ===== PERFORMANCE BY CATEGORY =====

func (r *dashboardRepository) GetPerformanceByCategory(ctx context.Context, tx *gorm.DB, limit int) ([]repositories.CategoryPerformanceData, error) {
	db := r.getDB(tx)

	var results []struct {
		CategoryID    *uint
		CategoryName  string
		AverageScore  float64
		TotalAttempts int64
	}

	// Query to get performance by category through questions in assessments
	if err := db.WithContext(ctx).
		Table("assessment_attempts").
		Select("question_categories.id as category_id, "+
			"COALESCE(question_categories.name, 'Uncategorized') as category_name, "+
			"AVG(assessment_attempts.score) as average_score, "+
			"COUNT(DISTINCT assessment_attempts.id) as total_attempts").
		Joins("JOIN assessments ON assessment_attempts.assessment_id = assessments.id").
		Joins("JOIN assessment_questions ON assessments.id = assessment_questions.assessment_id").
		Joins("JOIN questions ON assessment_questions.question_id = questions.id").
		Joins("LEFT JOIN question_categories ON questions.category_id = question_categories.id").
		Where("assessment_attempts.status = ?", models.AttemptCompleted).
		Group("question_categories.id, question_categories.name").
		Order("total_attempts DESC").
		Limit(limit).
		Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get performance by category: %w", err)
	}

	var performance []repositories.CategoryPerformanceData
	for _, r := range results {
		performance = append(performance, repositories.CategoryPerformanceData{
			CategoryID:    r.CategoryID,
			CategoryName:  r.CategoryName,
			AverageScore:  r.AverageScore,
			TotalAttempts: r.TotalAttempts,
		})
	}

	return performance, nil
}
