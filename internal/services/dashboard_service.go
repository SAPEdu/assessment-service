package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"gorm.io/gorm"
)

// ===== RESPONSE DTOs =====

type DashboardStatsResponse struct {
	Overview DashboardOverview `json:"overview"`
	Metrics  DashboardMetrics  `json:"metrics"`
	Trends   DashboardTrends   `json:"trends"`
}

type DashboardOverview struct {
	TotalAssessments   int64 `json:"total_assessments"`
	TotalQuestions     int64 `json:"total_questions"`
	TotalQuestionBanks int64 `json:"total_question_banks"`
	TotalAttempts      int64 `json:"total_attempts"`
	ActiveUsers        int64 `json:"active_users"`
}

type DashboardMetrics struct {
	CompletionRate float64 `json:"completion_rate"`
	AverageScore   float64 `json:"average_score"`
	PassRate       float64 `json:"pass_rate"`
}

type DashboardTrends struct {
	AssessmentsChange float64 `json:"assessments_change"`
	AttemptsChange    float64 `json:"attempts_change"`
	ScoreChange       float64 `json:"score_change"`
}

type ActivityTrendResponse struct {
	Period       string  `json:"period"`
	Attempts     int64   `json:"attempts"`
	Users        int64   `json:"users"`
	AverageScore float64 `json:"average_score"`
}

type RecentActivityResponse struct {
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
	TimeAgo          string    `json:"time_ago"`
}

type QuestionDistributionResponse struct {
	Type       string  `json:"type"`
	Name       string  `json:"name"`
	Count      int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}

type SubjectPerformanceResponse struct {
	SubjectID     *uint   `json:"subject_id"`
	SubjectName   string  `json:"subject_name"`
	AverageScore  float64 `json:"average_score"`
	TotalAttempts int64   `json:"total_attempts"`
}

// ===== SERVICE INTERFACE =====

type DashboardService interface {
	// Core dashboard endpoints
	GetDashboardStats(ctx context.Context, period int) (*DashboardStatsResponse, error)
	GetActivityTrends(ctx context.Context, period string) ([]ActivityTrendResponse, error)
	GetRecentActivities(ctx context.Context, limit int) ([]RecentActivityResponse, error)
	GetQuestionDistribution(ctx context.Context) ([]QuestionDistributionResponse, error)
	GetPerformanceBySubject(ctx context.Context, limit int) ([]SubjectPerformanceResponse, error)
}

// ===== SERVICE IMPLEMENTATION =====

type dashboardService struct {
	repo   repositories.Repository
	db     *gorm.DB
	logger *slog.Logger
}

func NewDashboardService(repo repositories.Repository, db *gorm.DB, logger *slog.Logger) DashboardService {
	return &dashboardService{
		repo:   repo,
		db:     db,
		logger: logger,
	}
}

func (s *dashboardService) GetDashboardStats(ctx context.Context, period int) (*DashboardStatsResponse, error) {
	s.logger.Info("Getting dashboard stats", "period", period)

	// Use default period if not specified
	if period <= 0 {
		period = 30 // Default: 30 days for trends
	}

	// Get overview data
	totalAssessments, err := s.repo.Dashboard().GetTotalAssessments(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get total assessments: %w", err)
	}

	totalQuestions, err := s.repo.Dashboard().GetTotalQuestions(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get total questions: %w", err)
	}

	totalQuestionBanks, err := s.repo.Dashboard().GetTotalQuestionBanks(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get total question banks: %w", err)
	}

	totalAttempts, err := s.repo.Dashboard().GetTotalAttempts(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get total attempts: %w", err)
	}

	activeUsers, err := s.repo.Dashboard().GetActiveUsers(ctx, nil, 30)
	if err != nil {
		return nil, fmt.Errorf("failed to get active users: %w", err)
	}

	// Get metrics
	completionRate, err := s.repo.Dashboard().GetCompletionRate(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get completion rate: %w", err)
	}

	averageScore, err := s.repo.Dashboard().GetAverageScore(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get average score: %w", err)
	}

	passRate, err := s.repo.Dashboard().GetPassRate(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get pass rate: %w", err)
	}

	// Get trends
	assessmentsChange, err := s.repo.Dashboard().GetTrendChange(ctx, nil, "assessments", period)
	if err != nil {
		s.logger.Warn("Failed to get assessments trend", "error", err)
		assessmentsChange = 0
	}

	attemptsChange, err := s.repo.Dashboard().GetTrendChange(ctx, nil, "attempts", period)
	if err != nil {
		s.logger.Warn("Failed to get attempts trend", "error", err)
		attemptsChange = 0
	}

	// Calculate score change (simplified - comparing current avg to previous period)
	scoreChange := 0.0 // TODO: Implement proper score trend calculation

	response := &DashboardStatsResponse{
		Overview: DashboardOverview{
			TotalAssessments:   totalAssessments,
			TotalQuestions:     totalQuestions,
			TotalQuestionBanks: totalQuestionBanks,
			TotalAttempts:      totalAttempts,
			ActiveUsers:        activeUsers,
		},
		Metrics: DashboardMetrics{
			CompletionRate: roundFloat(completionRate, 1),
			AverageScore:   roundFloat(averageScore, 1),
			PassRate:       roundFloat(passRate, 1),
		},
		Trends: DashboardTrends{
			AssessmentsChange: roundFloat(assessmentsChange, 1),
			AttemptsChange:    roundFloat(attemptsChange, 1),
			ScoreChange:       roundFloat(scoreChange, 1),
		},
	}

	return response, nil
}

func (s *dashboardService) GetActivityTrends(ctx context.Context, period string) ([]ActivityTrendResponse, error) {
	s.logger.Info("Getting activity trends", "period", period)

	// Validate period
	if period == "" {
		period = "month"
	}

	if period != "week" && period != "month" && period != "year" {
		return nil, fmt.Errorf("invalid period: must be 'week', 'month', or 'year'")
	}

	// Get trends from repository
	trends, err := s.repo.Dashboard().GetActivityTrends(ctx, nil, period)
	if err != nil {
		return nil, fmt.Errorf("failed to get activity trends: %w", err)
	}

	// Convert to response format
	response := make([]ActivityTrendResponse, len(trends))
	for i, trend := range trends {
		response[i] = ActivityTrendResponse{
			Period:       trend.Period,
			Attempts:     trend.Attempts,
			Users:        trend.Users,
			AverageScore: roundFloat(trend.AverageScore, 1),
		}
	}

	return response, nil
}

func (s *dashboardService) GetRecentActivities(ctx context.Context, limit int) ([]RecentActivityResponse, error) {
	s.logger.Info("Getting recent activities", "limit", limit)

	// Validate limit
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	// Get activities from repository
	activities, err := s.repo.Dashboard().GetRecentActivities(ctx, nil, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent activities: %w", err)
	}

	// Convert to response format with time ago
	response := make([]RecentActivityResponse, len(activities))
	for i, activity := range activities {
		response[i] = RecentActivityResponse{
			ID:               activity.ID,
			UserID:           activity.UserID,
			UserName:         activity.UserName,
			Action:           activity.Action,
			AssessmentID:     activity.AssessmentID,
			AssessmentTitle:  activity.AssessmentTitle,
			QuestionID:       activity.QuestionID,
			QuestionBankName: activity.QuestionBankName,
			Score:            activity.Score,
			CreatedAt:        activity.CreatedAt,
			TimeAgo:          formatTimeAgo(activity.CreatedAt),
		}
	}

	return response, nil
}

func (s *dashboardService) GetQuestionDistribution(ctx context.Context) ([]QuestionDistributionResponse, error) {
	s.logger.Info("Getting question distribution")

	// Get distribution from repository
	distribution, err := s.repo.Dashboard().GetQuestionDistribution(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get question distribution: %w", err)
	}

	// Convert to response format with Vietnamese names
	response := make([]QuestionDistributionResponse, len(distribution))
	for i, dist := range distribution {
		response[i] = QuestionDistributionResponse{
			Type:       dist.Type,
			Name:       getQuestionTypeName(dist.Type),
			Count:      dist.Count,
			Percentage: roundFloat(dist.Percentage, 1),
		}
	}

	return response, nil
}

func (s *dashboardService) GetPerformanceBySubject(ctx context.Context, limit int) ([]SubjectPerformanceResponse, error) {
	s.logger.Info("Getting performance by subject", "limit", limit)

	// Validate limit
	if limit <= 0 || limit > 20 {
		limit = 5
	}

	// Get performance from repository
	performance, err := s.repo.Dashboard().GetPerformanceByCategory(ctx, nil, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get performance by subject: %w", err)
	}

	// Convert to response format
	response := make([]SubjectPerformanceResponse, len(performance))
	for i, perf := range performance {
		response[i] = SubjectPerformanceResponse{
			SubjectID:     perf.CategoryID,
			SubjectName:   perf.CategoryName,
			AverageScore:  roundFloat(perf.AverageScore, 1),
			TotalAttempts: perf.TotalAttempts,
		}
	}

	return response, nil
}

// ===== HELPER FUNCTIONS =====

func roundFloat(val float64, precision int) float64 {
	ratio := 1.0
	for i := 0; i < precision; i++ {
		ratio *= 10
	}
	return float64(int(val*ratio+0.5)) / ratio
}

func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return fmt.Sprintf("%d giây trước", int(duration.Seconds()))
	} else if duration < time.Hour {
		return fmt.Sprintf("%d phút trước", int(duration.Minutes()))
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%d giờ trước", int(duration.Hours()))
	} else if duration < 7*24*time.Hour {
		return fmt.Sprintf("%d ngày trước", int(duration.Hours()/24))
	} else if duration < 30*24*time.Hour {
		return fmt.Sprintf("%d tuần trước", int(duration.Hours()/(24*7)))
	} else if duration < 365*24*time.Hour {
		return fmt.Sprintf("%d tháng trước", int(duration.Hours()/(24*30)))
	} else {
		return fmt.Sprintf("%d năm trước", int(duration.Hours()/(24*365)))
	}
}

func getQuestionTypeName(questionType string) string {
	typeNames := map[string]string{
		"multiple_choice": "Trắc nghiệm",
		"true_false":      "Đúng/Sai",
		"essay":           "Tự luận",
		"fill_blank":      "Điền vào chỗ trống",
		"matching":        "Ghép đôi",
		"ordering":        "Sắp xếp",
		"short_answer":    "Trả lời ngắn",
	}

	if name, ok := typeNames[questionType]; ok {
		return name
	}
	return questionType
}
