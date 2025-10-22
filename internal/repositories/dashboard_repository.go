package repositories

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// DashboardRepository interface for dashboard analytics operations
type DashboardRepository interface {
	// Dashboard stats
	GetTotalAssessments(ctx context.Context, tx *gorm.DB) (int64, error)
	GetTotalQuestions(ctx context.Context, tx *gorm.DB) (int64, error)
	GetTotalQuestionBanks(ctx context.Context, tx *gorm.DB) (int64, error)
	GetTotalAttempts(ctx context.Context, tx *gorm.DB) (int64, error)
	GetActiveUsers(ctx context.Context, tx *gorm.DB, days int) (int64, error)

	// Metrics
	GetCompletionRate(ctx context.Context, tx *gorm.DB) (float64, error)
	GetAverageScore(ctx context.Context, tx *gorm.DB) (float64, error)
	GetPassRate(ctx context.Context, tx *gorm.DB) (float64, error)

	// Trends
	GetTrendChange(ctx context.Context, tx *gorm.DB, entity string, days int) (float64, error)

	// Activity trends
	GetActivityTrends(ctx context.Context, tx *gorm.DB, period string) ([]ActivityTrendData, error)

	// Recent activities
	GetRecentActivities(ctx context.Context, tx *gorm.DB, limit int) ([]RecentActivityData, error)

	// Question distribution
	GetQuestionDistribution(ctx context.Context, tx *gorm.DB) ([]QuestionDistributionData, error)

	// Performance by category
	GetPerformanceByCategory(ctx context.Context, tx *gorm.DB, limit int) ([]CategoryPerformanceData, error)
}

// Data structures for dashboard responses

type ActivityTrendData struct {
	Period       string  `json:"period"`
	Attempts     int64   `json:"attempts"`
	Users        int64   `json:"users"`
	AverageScore float64 `json:"average_score"`
	Date         time.Time
}

type RecentActivityData struct {
	ID               uint      `json:"id"`
	UserID           string    `json:"user_id"`
	UserName         string    `json:"user_name"`
	Action           string    `json:"action"`
	AssessmentID     *uint     `json:"assessment_id,omitempty"`
	AssessmentTitle  *string   `json:"assessment_title,omitempty"`
	QuestionID       *uint     `json:"question_id,omitempty"`
	QuestionBankName *string   `json:"question_bank_name,omitempty"`
	Score            *float64  `json:"score,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type QuestionDistributionData struct {
	Type       string  `json:"type"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}

type CategoryPerformanceData struct {
	CategoryID    *uint   `json:"category_id"`
	CategoryName  string  `json:"category_name"`
	AverageScore  float64 `json:"average_score"`
	TotalAttempts int64   `json:"total_attempts"`
}
