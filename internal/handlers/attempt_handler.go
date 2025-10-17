package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/SAP-F-2025/assessment-service/internal/services"
	"github.com/SAP-F-2025/assessment-service/internal/utils"
	"github.com/SAP-F-2025/assessment-service/internal/validator"
	"github.com/gin-gonic/gin"
)

type AttemptHandler struct {
	BaseHandler
	attemptService services.AttemptService
	validator      *validator.Validator
}

func NewAttemptHandler(
	attemptService services.AttemptService,
	validator *validator.Validator,
	logger utils.Logger,
) *AttemptHandler {
	return &AttemptHandler{
		BaseHandler:    NewBaseHandler(logger),
		attemptService: attemptService,
		validator:      validator,
	}
}

// StartAttempt starts a new assessment attempt
// @Summary Start assessment attempt
// @Description Starts a new attempt for an assessment
// @Tags attempts
// @Accept json
// @Produce json
// @Param attempt body services.StartAttemptRequest true "Start attempt data"
// @Success 201 {object} SuccessResponse{data=services.AttemptResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /attempts/start [post]
func (h *AttemptHandler) StartAttempt(c *gin.Context) {
	h.LogRequest(c, "Starting assessment attempt")

	var req services.StartAttemptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid request payload",
			Details: err.Error(),
		})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	attempt, err := h.attemptService.Start(c.Request.Context(), &req, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, attempt)
}

// ResumeAttempt resumes an existing attempt
// @Summary Resume assessment attempt
// @Description Resumes an existing assessment attempt
// @Tags attempts
// @Accept json
// @Produce json
// @Param id path uint true "Attempt ID"
// @Success 200 {object} SuccessResponse{data=services.AttemptResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /attempts/{id}/resume [post]
func (h *AttemptHandler) ResumeAttempt(c *gin.Context) {
	id := h.parseIDParam(c, "id")
	if id == 0 {
		return
	}

	h.LogRequest(c, "Resuming assessment attempt", "attempt_id", id)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	attempt, err := h.attemptService.Resume(c.Request.Context(), id, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, attempt)
}

// SubmitAttempt submits an assessment attempt
// @Summary Submit assessment attempt
// @Description Submits an assessment attempt with all answers
// @Tags attempts
// @Accept json
// @Produce json
// @Param attempt body services.SubmitAttemptRequest true "Submit attempt data"
// @Success 200 {object} SuccessResponse{data=services.AttemptResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /attempts/submit [post]
func (h *AttemptHandler) SubmitAttempt(c *gin.Context) {
	h.LogRequest(c, "Submitting assessment attempt")

	var req services.SubmitAttemptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid request payload",
			Details: err.Error(),
		})
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Validation failed",
			Details: err.Error(),
		})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	attempt, err := h.attemptService.Submit(c.Request.Context(), &req, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, attempt)
}

// SubmitAnswer submits an answer for a specific question
// @Summary Submit answer
// @Description Submits an answer for a specific question in an attempt
// @Tags attempts
// @Accept json
// @Produce json
// @Param id path uint true "Attempt ID"
// @Param answer body services.SubmitAnswerRequest true "Answer data"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /attempts/{id}/answer [post]
func (h *AttemptHandler) SubmitAnswer(c *gin.Context) {
	attemptID := h.parseIDParam(c, "id")
	if attemptID == 0 {
		return
	}

	h.LogRequest(c, "Submitting answer", "attempt_id", attemptID)

	var req services.SubmitAnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid request payload",
			Details: err.Error(),
		})
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Validation failed",
			Details: err.Error(),
		})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	err := h.attemptService.SubmitAnswer(c.Request.Context(), attemptID, &req, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Answer submitted successfully",
	})
}

// GetAttempt retrieves an attempt by ID
// @Summary Get attempt
// @Description Retrieves an attempt by its ID
// @Tags attempts
// @Accept json
// @Produce json
// @Param id path uint true "Attempt ID"
// @Success 200 {object} SuccessResponse{data=services.AttemptResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /attempts/{id} [get]
func (h *AttemptHandler) GetAttempt(c *gin.Context) {
	id := h.parseIDParam(c, "id")
	if id == 0 {
		return
	}

	h.LogRequest(c, "Getting attempt", "attempt_id", id)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	attempt, err := h.attemptService.GetByID(c.Request.Context(), id, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, attempt)
}

// GetAttemptWithDetails retrieves an attempt with full details
// @Summary Get attempt with details
// @Description Retrieves an attempt with full details including questions and answers
// @Tags attempts
// @Accept json
// @Produce json
// @Param id path uint true "Attempt ID"
// @Success 200 {object} SuccessResponse{data=services.AttemptResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /attempts/{id}/details [get]
func (h *AttemptHandler) GetAttemptWithDetails(c *gin.Context) {
	id := h.parseIDParam(c, "id")
	if id == 0 {
		return
	}

	h.LogRequest(c, "Getting attempt with details", "attempt_id", id)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	attempt, err := h.attemptService.GetByIDWithDetails(c.Request.Context(), id, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, attempt)
}

// GetCurrentAttempt retrieves the current active attempt for an assessment
// @Summary Get current attempt
// @Description Retrieves the current active attempt for a specific assessment
// @Tags attempts
// @Accept json
// @Produce json
// @Param assessment_id path uint true "Assessment ID"
// @Success 200 {object} SuccessResponse{data=services.AttemptResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /attempts/current/{assessment_id} [get]
func (h *AttemptHandler) GetCurrentAttempt(c *gin.Context) {
	assessmentID := h.parseIDParam(c, "assessment_id")
	if assessmentID == 0 {
		return
	}

	h.LogRequest(c, "Getting current attempt", "assessment_id", assessmentID)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	attempt, err := h.attemptService.GetCurrentAttempt(c.Request.Context(), assessmentID, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, attempt)
}

// ListAttempts lists attempts with filters
// @Summary List attempts
// @Description Lists attempts with optional filtering
// @Tags attempts
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param size query int false "Page size" default(10)
// @Param status query string false "Attempt status"
// @Param assessment_id query uint false "Assessment ID"
// @Success 200 {object} SuccessResponse{data=[]services.AttemptResponse}
// @Failure 500 {object} ErrorResponse
// @Router /attempts [get]
func (h *AttemptHandler) ListAttempts(c *gin.Context) {
	h.LogRequest(c, "Listing attempts")

	filters := h.parseAttemptFilters(c)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	attempts, total, err := h.attemptService.List(c.Request.Context(), filters, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	page := (filters.Offset / max(filters.Limit, 1)) + 1
	totalPage := (int(total) + filters.Limit - 1) / max(filters.Limit, 1)
	response := map[string]interface{}{
		"data":        attempts,
		"total":       total,
		"page":        page,
		"size":        filters.Limit,
		"total_pages": totalPage,
	}

	c.JSON(http.StatusOK, response)
}

// GetAttemptsByStudent lists attempts by student
// @Summary Get attempts by student
// @Description Lists attempts made by a specific student
// @Tags attempts
// @Accept json
// @Produce json
// @Param student_id path uint true "Student ID"
// @Param page query int false "Page number" default(1)
// @Param size query int false "Page size" default(10)
// @Success 200 {object} SuccessResponse{data=[]services.AttemptResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /attempts/student/{student_id} [get]
func (h *AttemptHandler) GetAttemptsByStudent(c *gin.Context) {
	studentID := ParseStringIDParam(c, "student_id")
	if studentID == "" {
		return
	}

	h.LogRequest(c, "Getting attempts by student", "student_id", studentID)

	filters := h.parseAttemptFilters(c)
	attempts, total, err := h.attemptService.GetByStudent(c.Request.Context(), studentID, filters)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	page := (filters.Offset / max(filters.Limit, 1)) + 1
	totalPage := (int(total) + filters.Limit - 1) / max(filters.Limit, 1)
	response := map[string]interface{}{
		"data":        attempts,
		"total":       total,
		"page":        page,
		"size":        filters.Limit,
		"total_pages": totalPage,
	}

	c.JSON(http.StatusOK, response)
}

// GetAttemptsByAssessment lists attempts by assessment
// @Summary Get attempts by assessment
// @Description Lists attempts for a specific assessment
// @Tags attempts
// @Accept json
// @Produce json
// @Param assessment_id path uint true "Assessment ID"
// @Param page query int false "Page number" default(1)
// @Param size query int false "Page size" default(10)
// @Success 200 {object} SuccessResponse{data=[]services.AttemptResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /attempts/assessment/{assessment_id} [get]
func (h *AttemptHandler) GetAttemptsByAssessment(c *gin.Context) {
	assessmentID := h.parseIDParam(c, "assessment_id")
	if assessmentID == 0 {
		return
	}

	h.LogRequest(c, "Getting attempts by assessment", "assessment_id", assessmentID)

	filters := h.parseAttemptFilters(c)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	attempts, total, err := h.attemptService.GetByAssessment(c.Request.Context(), assessmentID, filters, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	page := (filters.Offset / max(filters.Limit, 1)) + 1
	totalPage := (int(total) + filters.Limit - 1) / max(filters.Limit, 1)
	response := map[string]interface{}{
		"data":        attempts,
		"total":       total,
		"page":        page,
		"size":        filters.Limit,
		"total_pages": totalPage,
	}

	c.JSON(http.StatusOK, response)
}

// GetTimeRemaining gets the remaining time for an attempt
// @Summary Get time remaining
// @Description Gets the remaining time for an active attempt
// @Tags attempts
// @Accept json
// @Produce json
// @Param id path uint true "Attempt ID"
// @Success 200 {object} SuccessResponse{data=int}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /attempts/{id}/time-remaining [get]
func (h *AttemptHandler) GetTimeRemaining(c *gin.Context) {
	id := h.parseIDParam(c, "id")
	if id == 0 {
		return
	}

	h.LogRequest(c, "Getting time remaining", "attempt_id", id)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	timeRemaining, err := h.attemptService.GetTimeRemaining(c.Request.Context(), id, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Time remaining retrieved successfully",
		Data:    timeRemaining,
	})
}

// ExtendTime extends time for an attempt
// @Summary Extend attempt time
// @Description Extends the time limit for an active attempt
// @Tags attempts
// @Accept json
// @Produce json
// @Param id path uint true "Attempt ID"
// @Param minutes query int true "Minutes to extend"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /attempts/{id}/extend [post]
func (h *AttemptHandler) ExtendTime(c *gin.Context) {
	id := h.parseIDParam(c, "id")
	if id == 0 {
		return
	}

	minutesStr := c.Query("minutes")
	if minutesStr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Minutes parameter is required",
		})
		return
	}

	minutes, err := strconv.Atoi(minutesStr)
	if err != nil || minutes <= 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid minutes value",
			Details: err.Error(),
		})
		return
	}

	h.LogRequest(c, "Extending attempt time", "attempt_id", id, "minutes", minutes)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	err = h.attemptService.ExtendTime(c.Request.Context(), id, minutes, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Time extended successfully",
	})
}

// HandleTimeout handles attempt timeout
// @Summary Handle attempt timeout
// @Description Handles timeout for an attempt (system endpoint)
// @Tags attempts
// @Accept json
// @Produce json
// @Param id path uint true "Attempt ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /attempts/{id}/timeout [post]
func (h *AttemptHandler) HandleTimeout(c *gin.Context) {
	id := h.parseIDParam(c, "id")
	if id == 0 {
		return
	}

	h.LogRequest(c, "Handling attempt timeout", "attempt_id", id)

	err := h.attemptService.HandleTimeout(c.Request.Context(), id)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Timeout handled successfully",
	})
}

// CanStartAttempt checks if user can start an attempt
// @Summary Check if can start attempt
// @Description Checks if a user can start a new attempt for an assessment
// @Tags attempts
// @Accept json
// @Produce json
// @Param assessment_id path uint true "Assessment ID"
// @Success 200 {object} SuccessResponse{data=bool}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /attempts/can-start/{assessment_id} [get]
func (h *AttemptHandler) CanStartAttempt(c *gin.Context) {
	assessmentID := h.parseIDParam(c, "assessment_id")
	if assessmentID == 0 {
		return
	}

	h.LogRequest(c, "Checking if can start attempt", "assessment_id", assessmentID)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	canStart, err := h.attemptService.CanStart(c.Request.Context(), assessmentID, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Can start check completed",
		Data:    canStart,
	})
}

// GetAttemptCount gets attempt count for user and assessment
// @Summary Get attempt count
// @Description Gets the number of attempts a user has made for an assessment
// @Tags attempts
// @Accept json
// @Produce json
// @Param assessment_id path uint true "Assessment ID"
// @Success 200 {object} SuccessResponse{data=int}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /attempts/count/{assessment_id} [get]
func (h *AttemptHandler) GetAttemptCount(c *gin.Context) {
	assessmentID := h.parseIDParam(c, "assessment_id")
	if assessmentID == 0 {
		return
	}

	h.LogRequest(c, "Getting attempt count", "assessment_id", assessmentID)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	count, err := h.attemptService.GetAttemptCount(c.Request.Context(), assessmentID, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Attempt count retrieved successfully",
		Data:    count,
	})
}

// IsAttemptActive checks if an attempt is active
// @Summary Check if attempt is active
// @Description Checks if an attempt is currently active
// @Tags attempts
// @Accept json
// @Produce json
// @Param id path uint true "Attempt ID"
// @Success 200 {object} SuccessResponse{data=bool}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /attempts/{id}/is-active [get]
func (h *AttemptHandler) IsAttemptActive(c *gin.Context) {
	id := h.parseIDParam(c, "id")
	if id == 0 {
		return
	}

	h.LogRequest(c, "Checking if attempt is active", "attempt_id", id)

	isActive, err := h.attemptService.IsAttemptActive(c.Request.Context(), id)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Active check completed",
		Data:    isActive,
	})
}

// GetAttemptStats retrieves attempt statistics
// @Summary Get attempt statistics
// @Description Retrieves statistics for attempts of an assessment
// @Tags attempts
// @Accept json
// @Produce json
// @Param assessment_id path uint true "Assessment ID"
// @Success 200 {object} SuccessResponse{data=repositories.AttemptStats}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /attempts/stats/{assessment_id} [get]
func (h *AttemptHandler) GetAttemptStats(c *gin.Context) {
	assessmentID := h.parseIDParam(c, "assessment_id")
	if assessmentID == 0 {
		return
	}

	h.LogRequest(c, "Getting attempt stats", "assessment_id", assessmentID)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	stats, err := h.attemptService.GetStats(c.Request.Context(), assessmentID, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Attempt stats retrieved successfully",
		Data:    stats,
	})
}

// Helper methods

func (h *AttemptHandler) getUserID(c *gin.Context) string {
	userID, exists := c.Get("user_id")
	if !exists {
		return ""
	}
	if id, ok := userID.(string); ok {
		return id
	}
	return ""
}

func (h *AttemptHandler) parseIDParam(c *gin.Context, param string) uint {
	idStr := c.Param(param)
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid " + param,
			Details: err.Error(),
		})
		return 0
	}
	return uint(id)
}

func (h *AttemptHandler) parseIntQuery(c *gin.Context, param string, defaultValue int) int {
	valueStr := c.Query(param)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func (h *AttemptHandler) parseAttemptFilters(c *gin.Context) repositories.AttemptFilters {
	page := h.parseIntQuery(c, "page", 1)
	size := h.parseIntQuery(c, "size", 10)

	filters := repositories.AttemptFilters{
		Limit:  size,
		Offset: (page - 1) * size,
	}

	if status := c.Query("status"); status != "" {
		attemptStatus := models.AttemptStatus(status)
		filters.Status = &attemptStatus
	}

	if studentIDStr := c.Query("student_id"); strings.TrimSpace(studentIDStr) != "" {
		studentIDStr = strings.TrimSpace(studentIDStr)
		filters.StudentID = &studentIDStr
	}

	return filters
}

func (h *AttemptHandler) handleServiceError(c *gin.Context, err error) {
	// Handle custom error types first
	var validationErrors services.ValidationErrors
	if errors.As(err, &validationErrors) {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Validation failed",
			Details: validationErrors,
		})
		return
	}

	var businessRuleError *services.BusinessRuleError
	if errors.As(err, &businessRuleError) {
		c.JSON(http.StatusUnprocessableEntity, ErrorResponse{
			Message: businessRuleError.Message,
			Details: map[string]interface{}{
				"rule":    businessRuleError.Rule,
				"context": businessRuleError.Context,
			},
		})
		return
	}

	var permissionError *services.PermissionError
	if errors.As(err, &permissionError) {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Message: "Access denied",
			Details: map[string]interface{}{
				"resource": permissionError.Resource,
				"action":   permissionError.Action,
				"reason":   permissionError.Reason,
			},
		})
		return
	}

	// Handle specific attempt errors
	switch {
	case errors.Is(err, services.ErrAttemptNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Message: "Attempt not found",
		})
	case errors.Is(err, services.ErrAttemptAccessDenied):
		c.JSON(http.StatusForbidden, ErrorResponse{
			Message: "Access denied to attempt",
		})
	case errors.Is(err, services.ErrAttemptNotActive):
		c.JSON(http.StatusConflict, ErrorResponse{
			Message: "Attempt is not active",
		})
	case errors.Is(err, services.ErrAttemptAlreadySubmitted):
		c.JSON(http.StatusConflict, ErrorResponse{
			Message: "Attempt already submitted",
		})
	case errors.Is(err, services.ErrAttemptLimitExceeded):
		c.JSON(http.StatusConflict, ErrorResponse{
			Message: "Maximum attempts exceeded",
		})
	case errors.Is(err, services.ErrAttemptTimeExpired):
		c.JSON(http.StatusGone, ErrorResponse{
			Message: "Attempt time has expired",
		})
	case errors.Is(err, services.ErrAttemptNotStarted):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Attempt not started",
		})
	case errors.Is(err, services.ErrAttemptCannotStart):
		c.JSON(http.StatusConflict, ErrorResponse{
			Message: "Cannot start new attempt",
		})
	// Assessment related errors
	case errors.Is(err, services.ErrAssessmentNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Message: "Assessment not found",
		})
	case errors.Is(err, services.ErrAssessmentExpired):
		c.JSON(http.StatusGone, ErrorResponse{
			Message: "Assessment has expired",
		})
	case errors.Is(err, services.ErrAssessmentNotPublished):
		c.JSON(http.StatusForbidden, ErrorResponse{
			Message: "Assessment is not published",
		})
	// Generic errors
	case errors.Is(err, services.ErrValidationFailed):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Validation failed",
			Details: err.Error(),
		})
	case errors.Is(err, services.ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "Unauthorized access",
		})
	case errors.Is(err, services.ErrForbidden), errors.Is(err, services.ErrInsufficientPermissions):
		c.JSON(http.StatusForbidden, ErrorResponse{
			Message: "Forbidden - insufficient permissions",
		})
	case errors.Is(err, services.ErrBadRequest):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Bad request",
		})
	case errors.Is(err, services.ErrConflict):
		c.JSON(http.StatusConflict, ErrorResponse{
			Message: "Resource conflict",
		})
	case errors.Is(err, services.ErrUserNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Message: "User not found",
		})
	default:
		h.LogError(c, err, "Unexpected service error")
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Message: "Internal server error",
		})
	}
}
