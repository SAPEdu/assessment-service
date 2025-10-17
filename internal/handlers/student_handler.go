package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/services"
	"github.com/SAP-F-2025/assessment-service/internal/utils"
	"github.com/gin-gonic/gin"
)

type StudentHandler struct {
	BaseHandler
	service services.StudentService
}

func NewStudentHandler(service services.StudentService, logger utils.Logger) *StudentHandler {
	return &StudentHandler{
		BaseHandler: NewBaseHandler(logger),
		service:     service,
	}
}

// ===== STUDENT ENDPOINTS =====

// GetStudentStats returns statistics for the current student
// @Summary Get student statistics
// @Description Get overview metrics, performance metrics, recent attempts, and upcoming assessments for the current student
// @Tags students
// @Accept json
// @Produce json
// @Success 200 {object} services.StudentStatsResponse
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /students/me/stats [get]
func (h *StudentHandler) GetStudentStats(c *gin.Context) {
	h.LogRequest(c, "Getting student stats")

	// Get student ID from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	studentID := userID.(string)

	// Call service
	stats, err := h.service.GetStudentStats(c.Request.Context(), studentID)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetStudentAssessments returns available assessments for the current student
// @Summary Get student assessments
// @Description Get list of available assessments for the current student with student-specific context
// @Tags students
// @Accept json
// @Produce json
// @Param page query int false "Page number (default: 1)"
// @Param size query int false "Page size (default: 10, max: 100)"
// @Param status query string false "Filter by status: available, in_progress, completed, expired"
// @Param sort_by query string false "Sort by: due_date, created_at, title (default: created_at)"
// @Success 200 {object} services.StudentAssessmentsResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /students/me/assessments [get]
func (h *StudentHandler) GetStudentAssessments(c *gin.Context) {
	h.LogRequest(c, "Getting student assessments")

	// Get student ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	studentID := userID.(string)

	// Parse query parameters
	pageStr := c.DefaultQuery("page", "1")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	sizeStr := c.DefaultQuery("size", "10")
	size, err := strconv.Atoi(sizeStr)
	if err != nil || size < 1 {
		size = 10
	}
	if size > 100 {
		size = 100
	}

	status := c.Query("status")
	sortBy := c.DefaultQuery("sort_by", "created_at")

	// Call service
	assessments, err := h.service.GetStudentAssessments(c.Request.Context(), studentID, page, size, status, sortBy)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, assessments)
}

// GetStudentAttempts returns attempt history for the current student
// @Summary Get student attempts
// @Description Get attempt history for the current student with filtering options
// @Tags students
// @Accept json
// @Produce json
// @Param page query int false "Page number (default: 1)"
// @Param size query int false "Page size (default: 10, max: 100)"
// @Param assessment_id query int false "Filter by assessment ID"
// @Param status query string false "Filter by status: in_progress, completed, abandoned, timeout"
// @Param from_date query string false "Filter from date (RFC3339 format)"
// @Param to_date query string false "Filter to date (RFC3339 format)"
// @Success 200 {object} services.StudentAttemptsResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /students/me/attempts [get]
func (h *StudentHandler) GetStudentAttempts(c *gin.Context) {
	h.LogRequest(c, "Getting student attempts")

	// Get student ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	studentID := userID.(string)

	// Parse query parameters
	pageStr := c.DefaultQuery("page", "1")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	sizeStr := c.DefaultQuery("size", "10")
	size, err := strconv.Atoi(sizeStr)
	if err != nil || size < 1 {
		size = 10
	}
	if size > 100 {
		size = 100
	}

	var assessmentID *uint
	if assessmentIDStr := c.Query("assessment_id"); assessmentIDStr != "" {
		id, err := strconv.ParseUint(assessmentIDStr, 10, 32)
		if err == nil {
			aid := uint(id)
			assessmentID = &aid
		}
	}

	status := c.Query("status")

	var fromDate, toDate *time.Time
	if fromDateStr := c.Query("from_date"); fromDateStr != "" {
		if parsed, err := time.Parse(time.RFC3339, fromDateStr); err == nil {
			fromDate = &parsed
		}
	}

	if toDateStr := c.Query("to_date"); toDateStr != "" {
		if parsed, err := time.Parse(time.RFC3339, toDateStr); err == nil {
			toDate = &parsed
		}
	}

	// Call service
	attempts, err := h.service.GetStudentAttempts(c.Request.Context(), studentID, page, size, assessmentID, status, fromDate, toDate)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, attempts)
}

// GetStudentAssessmentDetail returns assessment details with student context
// @Summary Get student assessment detail
// @Description Get detailed information about an assessment with student-specific context (attempts, scores, etc.)
// @Tags students
// @Accept json
// @Produce json
// @Param id path uint true "Assessment ID"
// @Success 200 {object} services.StudentAssessmentDetailResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 404 {object} ErrorResponse "Assessment not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /students/me/assessments/{id} [get]
func (h *StudentHandler) GetStudentAssessmentDetail(c *gin.Context) {
	h.LogRequest(c, "Getting student assessment detail")

	// Get student ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	studentID := userID.(string)

	// Parse assessment ID
	id := h.parseIDParam(c, "id")
	if id == 0 {
		return
	}

	// Call service
	detail, err := h.service.GetStudentAssessmentDetail(c.Request.Context(), studentID, id)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, detail)
}

// ===== HELPER METHODS =====

func (h *StudentHandler) parseIDParam(c *gin.Context, param string) uint {
	idStr := c.Param(param)
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid " + param,
			Details: "ID must be a valid number",
		})
		return 0
	}
	return uint(id)
}

// ===== ERROR HANDLING =====

func (h *StudentHandler) handleServiceError(c *gin.Context, err error) {
	// Map service errors to HTTP status codes
	switch {
	case errors.Is(err, services.ErrValidationFailed):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Validation failed",
			Details: err.Error(),
		})
	case errors.Is(err, services.ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "Unauthorized",
		})
	case errors.Is(err, services.ErrForbidden):
		c.JSON(http.StatusForbidden, ErrorResponse{
			Message: "Forbidden",
		})
	case errors.Is(err, services.ErrNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Message: "Resource not found",
		})
	default:
		h.LogError(c, err, "Unexpected service error")
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Message: "Internal server error",
		})
	}
}
