package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"gorm.io/gorm"
)

// ===== RESPONSE DTOs =====

// Student Dashboard Stats Response
type StudentStatsResponse struct {
	Overview    StudentOverview        `json:"overview"`
	Performance StudentPerformance     `json:"performance"`
	Recent      []StudentRecentAttempt `json:"recent_attempts"`
	Upcoming    []StudentUpcomingExam  `json:"upcoming_assessments"`
}

type StudentOverview struct {
	TotalAssessmentsAvailable  int64 `json:"total_assessments_available"`
	TotalAssessmentsCompleted  int64 `json:"total_assessments_completed"`
	TotalAssessmentsInProgress int64 `json:"total_assessments_in_progress"`
	TotalAttempts              int64 `json:"total_attempts"`
}

type StudentPerformance struct {
	AverageScore float64 `json:"average_score"`
	PassRate     float64 `json:"pass_rate"`
	HighestScore float64 `json:"highest_score"`
	LowestScore  float64 `json:"lowest_score"`
}

type StudentRecentAttempt struct {
	ID              uint      `json:"id"`
	AssessmentID    uint      `json:"assessment_id"`
	AssessmentTitle string    `json:"assessment_title"`
	Score           float64   `json:"score"`
	Passed          bool      `json:"passed"`
	CompletedAt     time.Time `json:"completed_at"`
	TimeSpent       int       `json:"time_spent"`
}

type StudentUpcomingExam struct {
	ID            uint       `json:"id"`
	Title         string     `json:"title"`
	DueDate       *time.Time `json:"due_date"`
	DaysRemaining int        `json:"days_remaining"`
}

// Student Assessments Response
type StudentAssessmentsResponse struct {
	Assessments []StudentAssessmentItem `json:"assessments"`
	Total       int64                   `json:"total"`
	Page        int                     `json:"page"`
	Size        int                     `json:"size"`
	TotalPages  int                     `json:"total_pages"`
}

type StudentAssessmentItem struct {
	ID             uint                    `json:"id"`
	Title          string                  `json:"title"`
	Description    *string                 `json:"description"`
	Duration       int                     `json:"duration"`
	PassingScore   float64                 `json:"passing_score"`
	Status         models.AssessmentStatus `json:"status"`
	DueDate        *time.Time              `json:"due_date"`
	QuestionsCount int                     `json:"questions_count"`
	TotalPoints    int                     `json:"total_points"`

	// Student-specific fields
	AttemptsUsed     int        `json:"attempts_used"`
	MaxAttempts      int        `json:"max_attempts"`
	CanStart         bool       `json:"can_start"`
	HasActiveAttempt bool       `json:"has_active_attempt"`
	BestScore        *float64   `json:"best_score"`
	LastAttemptDate  *time.Time `json:"last_attempt_date"`
}

// Student Attempts Response
type StudentAttemptsResponse struct {
	Attempts   []StudentAttemptItem `json:"attempts"`
	Total      int64                `json:"total"`
	Page       int                  `json:"page"`
	Size       int                  `json:"size"`
	TotalPages int                  `json:"total_pages"`
}

type StudentAttemptItem struct {
	ID                uint                 `json:"id"`
	AssessmentID      uint                 `json:"assessment_id"`
	AssessmentTitle   string               `json:"assessment_title"`
	Status            models.AttemptStatus `json:"status"`
	Score             float64              `json:"score"`
	MaxScore          int                  `json:"max_score"`
	Passed            bool                 `json:"passed"`
	StartedAt         *time.Time           `json:"started_at"`
	CompletedAt       *time.Time           `json:"completed_at"`
	TimeSpent         int                  `json:"time_spent"`
	QuestionsAnswered int                  `json:"questions_answered"`
	TotalQuestions    int                  `json:"total_questions"`
}

// Student Assessment Detail Response
type StudentAssessmentDetailResponse struct {
	Assessment     *models.Assessment       `json:"assessment"`
	StudentContext StudentAssessmentContext `json:"student_context"`
}

type StudentAssessmentContext struct {
	AttemptsUsed     int                  `json:"attempts_used"`
	MaxAttempts      int                  `json:"max_attempts"`
	CanStart         bool                 `json:"can_start"`
	HasActiveAttempt bool                 `json:"has_active_attempt"`
	AttemptsHistory  []StudentAttemptItem `json:"attempts_history"`
	BestScore        *float64             `json:"best_score"`
	AverageScore     *float64             `json:"average_score"`
}

// ===== SERVICE INTERFACE =====

type StudentService interface {
	// Core student endpoints
	GetStudentStats(ctx context.Context, studentID string) (*StudentStatsResponse, error)
	GetStudentAssessments(ctx context.Context, studentID string, page, size int, status string, sortBy string) (*StudentAssessmentsResponse, error)
	GetStudentAttempts(ctx context.Context, studentID string, page, size int, assessmentID *uint, status string, fromDate, toDate *time.Time) (*StudentAttemptsResponse, error)
	GetStudentAssessmentDetail(ctx context.Context, studentID string, assessmentID uint) (*StudentAssessmentDetailResponse, error)
}

// ===== SERVICE IMPLEMENTATION =====

type studentService struct {
	repo   repositories.Repository
	db     *gorm.DB
	logger *slog.Logger
}

func NewStudentService(repo repositories.Repository, db *gorm.DB, logger *slog.Logger) StudentService {
	return &studentService{
		repo:   repo,
		db:     db,
		logger: logger,
	}
}

func (s *studentService) GetStudentStats(ctx context.Context, studentID string) (*StudentStatsResponse, error) {
	s.logger.Info("Getting student stats", "student_id", studentID)

	// Get student attempt stats
	attemptStats, err := s.repo.Attempt().GetStudentAttemptStats(ctx, s.db, studentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get student attempt stats: %w", err)
	}

	// Count active assessments (Active status and not expired)
	var activeAssessmentsCount int64
	err = s.db.WithContext(ctx).
		Model(&models.Assessment{}).
		Where("status = ? AND (due_date IS NULL OR due_date > ?)", models.StatusActive, time.Now()).
		Count(&activeAssessmentsCount).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count active assessments: %w", err)
	}

	// Get completed assessments (distinct assessments with completed attempts)
	var completedAssessmentsCount int64
	err = s.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("student_id = ? AND status = ?", studentID, models.AttemptCompleted).
		Distinct("assessment_id").
		Count(&completedAssessmentsCount).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count completed assessments: %w", err)
	}

	// Get in-progress assessments (assessments with active attempts)
	var inProgressCount int64
	err = s.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("student_id = ? AND status = ?", studentID, models.AttemptInProgress).
		Count(&inProgressCount).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count in-progress attempts: %w", err)
	}

	// Get recent 5 attempts
	var recentAttempts []StudentRecentAttempt
	err = s.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Select("assessment_attempts.id, assessment_attempts.assessment_id, assessments.title as assessment_title, assessment_attempts.score, assessment_attempts.passed, assessment_attempts.completed_at, assessment_attempts.time_spent").
		Joins("JOIN assessments ON assessments.id = assessment_attempts.assessment_id").
		Where("assessment_attempts.student_id = ? AND assessment_attempts.status = ?", studentID, models.AttemptCompleted).
		Order("assessment_attempts.completed_at DESC").
		Limit(5).
		Scan(&recentAttempts).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get recent attempts: %w", err)
	}

	// Get upcoming 5 assessments (ordered by due_date)
	var upcomingExams []StudentUpcomingExam
	var upcomingAssessments []*models.Assessment
	err = s.db.WithContext(ctx).
		Model(&models.Assessment{}).
		Where("status = ? AND due_date IS NOT NULL AND due_date > ?", models.StatusActive, time.Now()).
		Order("due_date ASC").
		Limit(5).
		Find(&upcomingAssessments).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get upcoming assessments: %w", err)
	}

	now := time.Now()
	for _, assess := range upcomingAssessments {
		daysRemaining := 0
		if assess.DueDate != nil {
			daysRemaining = int(assess.DueDate.Sub(now).Hours() / 24)
		}
		upcomingExams = append(upcomingExams, StudentUpcomingExam{
			ID:            assess.ID,
			Title:         assess.Title,
			DueDate:       assess.DueDate,
			DaysRemaining: daysRemaining,
		})
	}

	// Calculate pass rate and lowest score
	passRate := 0.0
	if attemptStats.CompletedAttempts > 0 {
		passRate = (float64(attemptStats.PassedCount) / float64(attemptStats.CompletedAttempts)) * 100
	}

	// For lowest score, use COALESCE to handle NULL when no completed attempts
	var lowestScore float64
	err = s.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("student_id = ? AND status = ?", studentID, models.AttemptCompleted).
		Select("COALESCE(MIN(score), 0) as lowest_score").
		Scan(&lowestScore).Error
	if err != nil {
		lowestScore = 0
	}

	response := &StudentStatsResponse{
		Overview: StudentOverview{
			TotalAssessmentsAvailable:  activeAssessmentsCount,
			TotalAssessmentsCompleted:  completedAssessmentsCount,
			TotalAssessmentsInProgress: inProgressCount,
			TotalAttempts:              int64(attemptStats.TotalAttempts),
		},
		Performance: StudentPerformance{
			AverageScore: attemptStats.AverageScore,
			PassRate:     passRate,
			HighestScore: attemptStats.BestScore,
			LowestScore:  lowestScore,
		},
		Recent:   recentAttempts,
		Upcoming: upcomingExams,
	}

	return response, nil
}

func (s *studentService) GetStudentAssessments(ctx context.Context, studentID string, page, size int, status string, sortBy string) (*StudentAssessmentsResponse, error) {
	s.logger.Info("Getting student assessments", "student_id", studentID, "page", page, "size", size)

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 10
	}

	offset := (page - 1) * size

	// Build query for active assessments
	query := s.db.WithContext(ctx).Model(&models.Assessment{}).
		Where("status = ?", models.StatusActive)

	// Filter by due date (only show non-expired)
	query = query.Where("due_date IS NULL OR due_date > ?", time.Now())

	// Count total
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count assessments: %w", err)
	}

	// Apply sorting
	switch sortBy {
	case "due_date":
		query = query.Order("due_date ASC")
	case "created_at":
		query = query.Order("created_at DESC")
	case "title":
		query = query.Order("title ASC")
	default:
		query = query.Order("created_at DESC")
	}

	// Get assessments
	var assessments []*models.Assessment
	if err := query.Offset(offset).Limit(size).Find(&assessments).Error; err != nil {
		return nil, fmt.Errorf("failed to get assessments: %w", err)
	}

	// Build response with student context
	items := make([]StudentAssessmentItem, 0, len(assessments))
	for _, assess := range assessments {
		// Get attempt count
		attemptCount, err := s.repo.Attempt().GetAttemptCount(ctx, s.db, studentID, assess.ID)
		if err != nil {
			s.logger.Error("Failed to get attempt count", "error", err, "assessment_id", assess.ID)
			attemptCount = 0
		}

		// Check if has active attempt
		hasActive, err := s.repo.Attempt().HasActiveAttempt(ctx, s.db, studentID, assess.ID)
		if err != nil {
			s.logger.Error("Failed to check active attempt", "error", err, "assessment_id", assess.ID)
			hasActive = false
		}

		// Get best score and last attempt date
		attempts, err := s.repo.Attempt().GetByStudentAndAssessment(ctx, s.db, studentID, assess.ID)
		var bestScore *float64
		var lastAttemptDate *time.Time
		if err == nil && len(attempts) > 0 {
			for _, att := range attempts {
				if att.Status == models.AttemptCompleted {
					if bestScore == nil || att.Score > *bestScore {
						score := att.Score
						bestScore = &score
					}
					if lastAttemptDate == nil || (att.CompletedAt != nil && att.CompletedAt.After(*lastAttemptDate)) {
						lastAttemptDate = att.CompletedAt
					}
				}
			}
		}

		// Check if can start
		validation, err := s.repo.Attempt().CanStartAttempt(ctx, s.db, studentID, assess.ID)
		canStart := err == nil && validation != nil && validation.CanStart

		// Get questions count and total points
		var questionsCount int64
		var totalPoints int
		s.db.WithContext(ctx).Model(&models.AssessmentQuestion{}).Where("assessment_id = ?", assess.ID).Count(&questionsCount)
		s.db.WithContext(ctx).Model(&models.AssessmentQuestion{}).Where("assessment_id = ?", assess.ID).Select("COALESCE(SUM(points), 0)").Scan(&totalPoints)

		items = append(items, StudentAssessmentItem{
			ID:               assess.ID,
			Title:            assess.Title,
			Description:      assess.Description,
			Duration:         assess.Duration,
			PassingScore:     float64(assess.PassingScore),
			Status:           assess.Status,
			DueDate:          assess.DueDate,
			QuestionsCount:   int(questionsCount),
			TotalPoints:      totalPoints,
			AttemptsUsed:     attemptCount,
			MaxAttempts:      assess.MaxAttempts,
			CanStart:         canStart,
			HasActiveAttempt: hasActive,
			BestScore:        bestScore,
			LastAttemptDate:  lastAttemptDate,
		})
	}

	totalPages := int((total + int64(size) - 1) / int64(size))

	return &StudentAssessmentsResponse{
		Assessments: items,
		Total:       total,
		Page:        page,
		Size:        size,
		TotalPages:  totalPages,
	}, nil
}

func (s *studentService) GetStudentAttempts(ctx context.Context, studentID string, page, size int, assessmentID *uint, status string, fromDate, toDate *time.Time) (*StudentAttemptsResponse, error) {
	s.logger.Info("Getting student attempts", "student_id", studentID, "page", page, "size", size)

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 10
	}

	offset := (page - 1) * size

	// Build filters
	filters := repositories.AttemptFilters{
		Limit:  size,
		Offset: offset,
	}

	if status != "" {
		attemptStatus := models.AttemptStatus(status)
		filters.Status = &attemptStatus
	}
	if fromDate != nil {
		filters.DateFrom = fromDate
	}
	if toDate != nil {
		filters.DateTo = toDate
	}

	// Get attempts with additional filter for assessmentID if provided
	var attempts []*models.AssessmentAttempt
	var total int64
	var err error

	if assessmentID != nil {
		attempts, total, err = s.repo.Attempt().GetByAssessment(ctx, s.db, *assessmentID, filters)
		if err != nil {
			return nil, fmt.Errorf("failed to get student attempts: %w", err)
		}
		// Filter by student
		filtered := make([]*models.AssessmentAttempt, 0)
		for _, att := range attempts {
			if att.StudentID == studentID {
				filtered = append(filtered, att)
			}
		}
		attempts = filtered
		total = int64(len(filtered))
	} else {
		attempts, total, err = s.repo.Attempt().GetByStudent(ctx, s.db, studentID, filters)
		if err != nil {
			return nil, fmt.Errorf("failed to get student attempts: %w", err)
		}
	}

	// Build response
	items := make([]StudentAttemptItem, 0, len(attempts))
	for _, att := range attempts {
		// Get assessment title
		var assessment models.Assessment
		if err := s.db.WithContext(ctx).First(&assessment, att.AssessmentID).Error; err != nil {
			s.logger.Error("Failed to get assessment", "error", err, "assessment_id", att.AssessmentID)
			continue
		}

		items = append(items, StudentAttemptItem{
			ID:                att.ID,
			AssessmentID:      att.AssessmentID,
			AssessmentTitle:   assessment.Title,
			Status:            att.Status,
			Score:             att.Score,
			MaxScore:          att.MaxScore,
			Passed:            att.Passed,
			StartedAt:         att.StartedAt,
			CompletedAt:       att.CompletedAt,
			TimeSpent:         att.TimeSpent,
			QuestionsAnswered: att.QuestionsAnswered,
			TotalQuestions:    att.TotalQuestions,
		})
	}

	totalPages := int((total + int64(size) - 1) / int64(size))

	return &StudentAttemptsResponse{
		Attempts:   items,
		Total:      total,
		Page:       page,
		Size:       size,
		TotalPages: totalPages,
	}, nil
}

func (s *studentService) GetStudentAssessmentDetail(ctx context.Context, studentID string, assessmentID uint) (*StudentAssessmentDetailResponse, error) {
	s.logger.Info("Getting student assessment detail", "student_id", studentID, "assessment_id", assessmentID)

	// Get assessment
	assessment, err := s.repo.Assessment().GetByID(ctx, s.db, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get assessment: %w", err)
	}

	// Get attempt count
	attemptCount, err := s.repo.Attempt().GetAttemptCount(ctx, s.db, studentID, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get attempt count: %w", err)
	}

	// Check if has active attempt
	hasActive, err := s.repo.Attempt().HasActiveAttempt(ctx, s.db, studentID, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to check active attempt: %w", err)
	}

	// Check if can start
	validation, err := s.repo.Attempt().CanStartAttempt(ctx, s.db, studentID, assessmentID)
	canStart := err == nil && validation != nil && validation.CanStart

	// Get attempts history
	attempts, err := s.repo.Attempt().GetByStudentAndAssessment(ctx, s.db, studentID, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get attempts: %w", err)
	}

	// Build attempts history and calculate stats
	history := make([]StudentAttemptItem, 0, len(attempts))
	var bestScore *float64
	var totalScore float64
	var completedCount int

	for _, att := range attempts {
		history = append(history, StudentAttemptItem{
			ID:                att.ID,
			AssessmentID:      att.AssessmentID,
			AssessmentTitle:   assessment.Title,
			Status:            att.Status,
			Score:             att.Score,
			MaxScore:          att.MaxScore,
			Passed:            att.Passed,
			StartedAt:         att.StartedAt,
			CompletedAt:       att.CompletedAt,
			TimeSpent:         att.TimeSpent,
			QuestionsAnswered: att.QuestionsAnswered,
			TotalQuestions:    att.TotalQuestions,
		})

		if att.Status == models.AttemptCompleted {
			completedCount++
			totalScore += att.Score
			if bestScore == nil || att.Score > *bestScore {
				score := att.Score
				bestScore = &score
			}
		}
	}

	var averageScore *float64
	if completedCount > 0 {
		avg := totalScore / float64(completedCount)
		averageScore = &avg
	}

	context := StudentAssessmentContext{
		AttemptsUsed:     attemptCount,
		MaxAttempts:      assessment.MaxAttempts,
		CanStart:         canStart,
		HasActiveAttempt: hasActive,
		AttemptsHistory:  history,
		BestScore:        bestScore,
		AverageScore:     averageScore,
	}

	return &StudentAssessmentDetailResponse{
		Assessment:     assessment,
		StudentContext: context,
	}, nil
}
