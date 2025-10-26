package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/services"
	"github.com/SAP-F-2025/assessment-service/internal/utils"
	"github.com/gin-gonic/gin"
)

type DashboardHandler struct {
	BaseHandler
	service services.DashboardService
}

func NewDashboardHandler(service services.DashboardService, logger utils.Logger) *DashboardHandler {
	return &DashboardHandler{
		BaseHandler: NewBaseHandler(logger),
		service:     service,
	}
}

// ===== DASHBOARD ENDPOINTS =====

// GetDashboardStats returns overall dashboard statistics
// @Summary Get dashboard statistics
// @Description Get overview metrics, performance metrics, and trends for the dashboard. Teachers see only their data, admins see all data.
// @Tags dashboard
// @Accept json
// @Produce json
// @Param period query int false "Period in days for trend calculation (default: 30)"
// @Success 200 {object} services.DashboardStatsResponse
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /dashboard/stats [get]
func (h *DashboardHandler) GetDashboardStats(c *gin.Context) {
	h.LogRequest(c, "Getting dashboard stats")

	// Get period parameter (optional, defaults to 30 days)
	periodStr := c.DefaultQuery("period", "30")
	period, err := strconv.Atoi(periodStr)
	if err != nil || period < 1 {
		period = 30
	}

	// Get teacher ID filter based on user role
	teacherID := h.getTeacherIDFilter(c)

	// Call service
	stats, err := h.service.GetDashboardStats(c.Request.Context(), teacherID, period)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetActivityTrends returns activity trends over time
// @Summary Get activity trends
// @Description Get activity trends (attempts, users, scores) grouped by time period. Teachers see only their data, admins see all data.
// @Tags dashboard
// @Accept json
// @Produce json
// @Param period query string false "Time period: week, month, or year (default: month)"
// @Success 200 {array} services.ActivityTrendResponse
// @Failure 400 {object} ErrorResponse "Bad request - invalid period"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /dashboard/activity-trends [get]
func (h *DashboardHandler) GetActivityTrends(c *gin.Context) {
	h.LogRequest(c, "Getting activity trends")

	// Get period parameter (optional, defaults to month)
	period := c.DefaultQuery("period", "month")

	// Validate period
	if period != "week" && period != "month" && period != "year" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid period parameter",
			Details: "Period must be 'week', 'month', or 'year'",
		})
		return
	}

	// Get teacher ID filter based on user role
	teacherID := h.getTeacherIDFilter(c)

	// Call service
	trends, err := h.service.GetActivityTrends(c.Request.Context(), teacherID, period)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, trends)
}

// GetRecentActivities returns recent activities
// @Summary Get recent activities
// @Description Get a list of recent user activities (completed assessments, created questions, etc.). Teachers see only their data, admins see all data.
// @Tags dashboard
// @Accept json
// @Produce json
// @Param limit query int false "Number of activities to return (default: 10, max: 50)"
// @Success 200 {array} services.RecentActivityResponse
// @Failure 400 {object} ErrorResponse "Bad request - invalid limit"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /dashboard/recent-activities [get]
func (h *DashboardHandler) GetRecentActivities(c *gin.Context) {
	h.LogRequest(c, "Getting recent activities")

	// Get limit parameter (optional, defaults to 10)
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	// Get teacher ID filter based on user role
	teacherID := h.getTeacherIDFilter(c)

	// Call service
	activities, err := h.service.GetRecentActivities(c.Request.Context(), teacherID, limit)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, activities)
}

// GetQuestionDistribution returns question type distribution
// @Summary Get question type distribution
// @Description Get the distribution of questions by type with counts and percentages. Teachers see only their data, admins see all data.
// @Tags dashboard
// @Accept json
// @Produce json
// @Success 200 {array} services.QuestionDistributionResponse
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /dashboard/question-distribution [get]
func (h *DashboardHandler) GetQuestionDistribution(c *gin.Context) {
	h.LogRequest(c, "Getting question distribution")

	// Get teacher ID filter based on user role
	teacherID := h.getTeacherIDFilter(c)

	// Call service
	distribution, err := h.service.GetQuestionDistribution(c.Request.Context(), teacherID)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, distribution)
}

// GetPerformanceBySubject returns performance statistics by subject/category
// @Summary Get performance by subject
// @Description Get average scores and attempt counts grouped by subject/category. Teachers see only their data, admins see all data.
// @Tags dashboard
// @Accept json
// @Produce json
// @Param limit query int false "Number of subjects to return (default: 5, max: 20)"
// @Success 200 {array} services.SubjectPerformanceResponse
// @Failure 400 {object} ErrorResponse "Bad request - invalid limit"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /dashboard/performance-by-subject [get]
func (h *DashboardHandler) GetPerformanceBySubject(c *gin.Context) {
	h.LogRequest(c, "Getting performance by subject")

	// Get limit parameter (optional, defaults to 5)
	limitStr := c.DefaultQuery("limit", "5")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}

	// Get teacher ID filter based on user role
	teacherID := h.getTeacherIDFilter(c)

	// Call service
	performance, err := h.service.GetPerformanceBySubject(c.Request.Context(), teacherID, limit)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, performance)
}

// ===== HELPER METHODS =====

// getTeacherIDFilter returns the teacher ID filter based on user role
// Returns nil for admin (show all data), returns user_id for teacher (show only their data)
func (h *DashboardHandler) getTeacherIDFilter(c *gin.Context) *string {
	// Get user role from context
	userRole, exists := c.Get("user_role")
	if !exists {
		// If role not found, default to filtering by current user (safer)
		userID, _ := c.Get("user_id")
		if uid, ok := userID.(string); ok {
			return &uid
		}
		return nil
	}

	role, ok := userRole.(models.UserRole)
	if !ok {
		// If role type is invalid, default to filtering by current user
		userID, _ := c.Get("user_id")
		if uid, ok := userID.(string); ok {
			return &uid
		}
		return nil
	}

	// Admin sees all data (no filter)
	if role == models.RoleAdmin {
		return nil
	}

	// Teacher sees only their data (filter by user_id)
	userID, exists := c.Get("user_id")
	if !exists {
		return nil
	}

	uid, ok := userID.(string)
	if !ok {
		return nil
	}

	return &uid
}

// ===== ERROR HANDLING =====

func (h *DashboardHandler) handleServiceError(c *gin.Context, err error) {
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
	default:
		h.LogError(c, err, "Unexpected service error")
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Message: "Internal server error",
		})
	}
}
