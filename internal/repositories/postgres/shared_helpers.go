package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"gorm.io/gorm"
)

// SharedHelpers contains common database operations
type SharedHelpers struct {
	db *gorm.DB
}

func NewSharedHelpers(db *gorm.DB) *SharedHelpers {
	return &SharedHelpers{db: db}
}

// CountAttempts counts attempts for an assessment
func (h *SharedHelpers) CountAttempts(ctx context.Context, assessmentID uint) (int64, error) {
	var count int64
	err := h.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("assessment_id = ?", assessmentID).
		Count(&count).Error
	return count, err
}

// CountAttemptsByStudent counts attempts by student for an assessment
func (h *SharedHelpers) CountAttemptsByStudent(ctx context.Context, assessmentID uint, studentID string) (int64, error) {
	var count int64
	err := h.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("assessment_id = ? AND student_id = ?", assessmentID, studentID).
		Count(&count).Error
	return count, err
}

// CountAttemptsByStatus counts attempts by status
func (h *SharedHelpers) CountAttemptsByStatus(ctx context.Context, assessmentID uint, status models.AttemptStatus) (int64, error) {
	var count int64
	err := h.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("assessment_id = ? AND status = ?", assessmentID, status).
		Count(&count).Error
	return count, err
}

// GetAssessmentBasicInfo gets basic assessment info
func (h *SharedHelpers) GetAssessmentBasicInfo(ctx context.Context, assessmentID uint) (*models.Assessment, error) {
	var assessment models.Assessment
	err := h.db.WithContext(ctx).
		Select("id, status, max_attempts, due_date, passing_score").
		First(&assessment, assessmentID).Error
	return &assessment, err
}

// ApplyFilters applies common filters to assessment queries
func (h *SharedHelpers) ApplyAssessmentFilters(query *gorm.DB, filters repositories.AssessmentFilters) *gorm.DB {
	if filters.Status != nil {
		query = query.Where("status = ?", *filters.Status)
	}
	if filters.CreatedBy != nil {
		query = query.Where("created_by = ?", *filters.CreatedBy)
	}
	if filters.DateFrom != nil {
		query = query.Where("created_at >= ?", *filters.DateFrom)
	}
	if filters.DateTo != nil {
		query = query.Where("created_at <= ?", *filters.DateTo)
	}
	return query
}

// ApplyAttemptFilters applies common filters to attempt queries
func (h *SharedHelpers) ApplyAttemptFilters(query *gorm.DB, filters repositories.AttemptFilters) *gorm.DB {
	if filters.Status != nil {
		query = query.Where("status = ?", *filters.Status)
	}
	if filters.StudentID != nil {
		query = query.Where("student_id = ?", *filters.StudentID)
	}
	if filters.DateFrom != nil {
		query = query.Where("created_at >= ?", *filters.DateFrom)
	}
	if filters.DateTo != nil {
		query = query.Where("created_at <= ?", *filters.DateTo)
	}
	return query
}

// ApplyPaginationAndSort applies pagination and sorting with SQL injection protection
func (h *SharedHelpers) ApplyPaginationAndSort(query *gorm.DB, sortBy, sortOrder string, limit, offset int) *gorm.DB {
	// Whitelist allowed sort columns
	allowedSortColumns := map[string]bool{
		"created_at": true,
		"updated_at": true,
		"id":         true,
		"title":      true,
		"status":     true,
		"difficulty": true,
		"type":       true,
		"score":      true,
	}

	// Validate and set sort column
	if sortBy == "" || !allowedSortColumns[sortBy] {
		sortBy = "created_at"
	}

	// Validate and set sort order
	if sortOrder != "asc" && sortOrder != "ASC" {
		sortOrder = "DESC"
	} else {
		sortOrder = "ASC"
	}

	query = query.Order(sortBy + " " + sortOrder)

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	return query
}

// BulkUpdateStatus updates status for multiple records
func (h *SharedHelpers) BulkUpdateAssessmentStatus(ctx context.Context, ids []uint, status models.AssessmentStatus) error {
	if len(ids) == 0 {
		return nil
	}
	return h.db.WithContext(ctx).
		Model(&models.Assessment{}).
		Where("id IN ?", ids).
		Update("status", status).Error
}

// BulkUpdateAttemptStatus updates status for multiple attempts
func (h *SharedHelpers) BulkUpdateAttemptStatus(ctx context.Context, ids []uint, status models.AttemptStatus) error {
	if len(ids) == 0 {
		return fmt.Errorf("no IDs provided for bulk update")
	}
	return h.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("id IN ?", ids).
		Update("status", status).Error
}

// ValidateAttemptEligibility checks if student can start new attempt
func (h *SharedHelpers) ValidateAttemptEligibility(ctx context.Context, assessmentID uint, studentID string) (*repositories.AttemptValidation, error) {
	validation := &repositories.AttemptValidation{CanStart: true}

	// Get assessment info
	assessment, err := h.GetAssessmentBasicInfo(ctx, assessmentID)
	if err != nil {
		return nil, err
	}

	// Check assessment status
	if assessment.Status != models.StatusActive {
		validation.CanStart = false
		validation.Reason = "Assessment is not active"
		return validation, nil
	}

	// Check due date
	if assessment.DueDate != nil && time.Now().After(*assessment.DueDate) {
		validation.CanStart = false
		validation.Reason = "Assessment due date has passed"
		return validation, nil
	}

	// Check max attempts
	if assessment.MaxAttempts > 0 {
		attemptCount, err := h.CountAttemptsByStudent(ctx, assessmentID, studentID)
		if err != nil {
			return nil, err
		}
		if attemptCount >= int64(assessment.MaxAttempts) {
			validation.CanStart = false
			validation.Reason = "Maximum attempts reached"
			return validation, nil
		}
	}

	// Check for active attempts
	var activeCount int64
	err = h.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("student_id = ? AND status = ?", studentID, models.AttemptInProgress).
		Count(&activeCount).Error
	if err != nil {
		return nil, err
	}

	if activeCount > 0 {
		validation.CanStart = false
		validation.Reason = "An attempt is already in progress"
		return validation, nil
	}

	return validation, nil
}

// GetRemainingAttempts calculates remaining attempts for a student
func (h *SharedHelpers) GetRemainingAttempts(ctx context.Context, assessmentID uint, studentID string) (int, error) {
	assessment, err := h.GetAssessmentBasicInfo(ctx, assessmentID)
	if err != nil {
		return 0, err
	}

	attemptCount, err := h.CountAttemptsByStudent(ctx, assessmentID, studentID)
	if err != nil {
		return 0, err
	}

	remaining := assessment.MaxAttempts - int(attemptCount)
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}
