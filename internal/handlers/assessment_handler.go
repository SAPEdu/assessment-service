package handlers

import (
	"errors"
	"fmt"
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

type AssessmentHandler struct {
	BaseHandler
	assessmentService services.AssessmentService
	validator         *validator.Validator
}

func NewAssessmentHandler(
	assessmentService services.AssessmentService,
	validator *validator.Validator,
	logger utils.Logger,
) *AssessmentHandler {
	return &AssessmentHandler{
		BaseHandler:       NewBaseHandler(logger),
		assessmentService: assessmentService,
		validator:         validator,
	}
}

// CreateAssessment creates a new assessment
// @Summary Create assessment
// @Description Creates a new assessment with the provided details
// @Tags assessments
// @Accept json
// @Produce json
// @Param assessment body services.CreateAssessmentRequest true "Assessment data"
// @Success 201 {object} SuccessResponse{data=services.AssessmentResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /assessments [post]
func (h *AssessmentHandler) CreateAssessment(c *gin.Context) {
	var req services.CreateAssessmentRequest
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

	assessment, err := h.assessmentService.Create(c.Request.Context(), &req, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, assessment)
}

// GetAssessment retrieves an assessment by ID
// @Summary Get assessment
// @Description Retrieves an assessment by its ID
// @Tags assessments
// @Accept json
// @Produce json
// @Param id path uint true "Assessment ID"
// @Success 200 {object} SuccessResponse{data=services.AssessmentResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /assessments/{id} [get]
func (h *AssessmentHandler) GetAssessment(c *gin.Context) {
	id := h.parseIDParam(c, "id")
	if id == 0 {
		return
	}

	h.LogRequest(c, "Getting assessment", "assessment_id", id)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	assessment, err := h.assessmentService.GetByID(c.Request.Context(), id, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, assessment)
}

// GetAssessmentWithDetails retrieves an assessment with full details
// @Summary Get assessment with details
// @Description Retrieves an assessment with full details including questions
// @Tags assessments
// @Accept json
// @Produce json
// @Param id path uint true "Assessment ID"
// @Success 200 {object} SuccessResponse{data=services.AssessmentResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /assessments/{id}/details [get]
func (h *AssessmentHandler) GetAssessmentWithDetails(c *gin.Context) {
	id := h.parseIDParam(c, "id")
	if id == 0 {
		return
	}

	h.LogRequest(c, "Getting assessment with details", "assessment_id", id)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	assessment, err := h.assessmentService.GetByIDWithDetails(c.Request.Context(), id, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, assessment)
}

// UpdateAssessment updates an existing assessment
// @Summary Update assessment
// @Description Updates an existing assessment with the provided details
// @Tags assessments
// @Accept json
// @Produce json
// @Param id path uint true "Assessment ID"
// @Param assessment body services.UpdateAssessmentRequest true "Assessment update data"
// @Success 200 {object} SuccessResponse{data=services.AssessmentResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /assessments/{id} [put]
func (h *AssessmentHandler) UpdateAssessment(c *gin.Context) {
	id := h.parseIDParam(c, "id")
	if id == 0 {
		return
	}

	h.LogRequest(c, "Updating assessment", "assessment_id", id)

	var req services.UpdateAssessmentRequest
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

	assessment, err := h.assessmentService.Update(c.Request.Context(), id, &req, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, assessment)
}

// DeleteAssessment deletes an assessment
// @Summary Delete assessment
// @Description Deletes an assessment by ID
// @Tags assessments
// @Accept json
// @Produce json
// @Param id path uint true "Assessment ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /assessments/{id} [delete]
func (h *AssessmentHandler) DeleteAssessment(c *gin.Context) {
	id := h.parseIDParam(c, "id")
	if id == 0 {
		return
	}

	h.LogRequest(c, "Deleting assessment", "assessment_id", id)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	err := h.assessmentService.Delete(c.Request.Context(), id, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ListAssessments lists assessments with filters
// @Summary List assessments
// @Description Lists assessments with optional filtering
// @Tags assessments
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param size query int false "Page size" default(10)
// @Param status query string false "Assessment status"
// @Param creator_id query uint false "Creator ID"
// @Success 200 {object} SuccessResponse{data=services.AssessmentListResponse}
// @Failure 500 {object} ErrorResponse
// @Router /assessments [get]
func (h *AssessmentHandler) ListAssessments(c *gin.Context) {
	h.LogRequest(c, "Listing assessments")

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	filters := h.parseAssessmentFilters(c)
	assessments, err := h.assessmentService.List(c.Request.Context(), filters, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, assessments)
}

// GetAssessmentsByCreator lists assessments by creator
// @Summary Get assessments by creator
// @Description Lists assessments created by a specific user
// @Tags assessments
// @Accept json
// @Produce json
// @Param creator_id path uint true "Creator ID"
// @Param page query int false "Page number" default(1)
// @Param size query int false "Page size" default(10)
// @Success 200 {object} SuccessResponse{data=services.AssessmentListResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /assessments/creator/{creator_id} [get]
func (h *AssessmentHandler) GetAssessmentsByCreator(c *gin.Context) {
	creatorID := ParseStringIDParam(c, "creator_id")
	if creatorID == "" {
		return
	}

	h.LogRequest(c, "Getting assessments by creator", "creator_id", creatorID)

	filters := h.parseAssessmentFilters(c)
	assessments, err := h.assessmentService.GetByCreator(c.Request.Context(), creatorID, filters)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, assessments)
}

// SearchAssessments searches assessments
// @Summary Search assessments
// @Description Searches assessments by query string
// @Tags assessments
// @Accept json
// @Produce json
// @Param q query string true "Search query"
// @Param page query int false "Page number" default(1)
// @Param size query int false "Page size" default(10)
// @Success 200 {object} SuccessResponse{data=services.AssessmentListResponse}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /assessments/search [get]
func (h *AssessmentHandler) SearchAssessments(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Search query parameter 'q' is required",
		})
		return
	}

	h.LogRequest(c, "Searching assessments", "query", query)

	filters := h.parseAssessmentFilters(c)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	assessments, err := h.assessmentService.Search(c.Request.Context(), query, filters, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, assessments)
}

// UpdateAssessmentStatus updates assessment status
// @Summary Update assessment status
// @Description Updates the status of an assessment
// @Tags assessments
// @Accept json
// @Produce json
// @Param id path uint true "Assessment ID"
// @Param status body services.UpdateStatusRequest true "Status update data"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /assessments/{id}/status [put]
func (h *AssessmentHandler) UpdateAssessmentStatus(c *gin.Context) {
	id := h.parseIDParam(c, "id")
	if id == 0 {
		return
	}

	h.LogRequest(c, "Updating assessment status", "assessment_id", id)

	var req services.UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.RespondWithError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if err := h.validator.Validate(&req); err != nil {
		h.RespondWithError(c, http.StatusBadRequest, "Validation failed", err)
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	err := h.assessmentService.UpdateStatus(c.Request.Context(), id, &req, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Assessment status updated successfully",
	})
}

// PublishAssessment publishes an assessment
// @Summary Publish assessment
// @Description Publishes an assessment making it active
// @Tags assessments
// @Accept json
// @Produce json
// @Param id path uint true "Assessment ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /assessments/{id}/publish [post]
func (h *AssessmentHandler) PublishAssessment(c *gin.Context) {
	id := h.parseIDParam(c, "id")
	if id == 0 {
		return
	}

	h.LogRequest(c, "Publishing assessment", "assessment_id", id)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	err := h.assessmentService.Publish(c.Request.Context(), id, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Assessment published successfully",
	})
}

// ArchiveAssessment archives an assessment
// @Summary Archive assessment
// @Description Archives an assessment
// @Tags assessments
// @Accept json
// @Produce json
// @Param id path uint true "Assessment ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /assessments/{id}/archive [post]
func (h *AssessmentHandler) ArchiveAssessment(c *gin.Context) {
	id := h.parseIDParam(c, "id")
	if id == 0 {
		return
	}

	h.LogRequest(c, "Archiving assessment", "assessment_id", id)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	err := h.assessmentService.Archive(c.Request.Context(), id, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Assessment archived successfully",
	})
}

// AddQuestionToAssessment adds a question to an assessment
// @Summary Add question to assessment
// @Description Adds a question to an assessment with specified order and points
// @Tags assessments
// @Accept json
// @Produce json
// @Param id path uint true "Assessment ID"
// @Param question_id path uint true "Question ID"
// @Param order query int false "Question order"
// @Param points query int false "Question points"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /assessments/{id}/questions/{question_id} [post]
func (h *AssessmentHandler) AddQuestionToAssessment(c *gin.Context) {
	assessmentID := h.parseIDParam(c, "id")
	if assessmentID == 0 {
		return
	}

	questionID := h.parseIDParam(c, "question_id")
	if questionID == 0 {
		return
	}

	h.LogRequest(c, "Adding question to assessment", "assessment_id", assessmentID, "question_id", questionID)

	order := h.parseIntQuery(c, "order", 0)
	points := h.parseIntQueryPtr(c, "points")
	if points == nil {
		points = new(int)
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	err := h.assessmentService.AddQuestion(c.Request.Context(), assessmentID, questionID, order, *points, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Question added to assessment successfully",
	})
}

// RemoveQuestionFromAssessment removes a question from an assessment
// @Summary Remove question from assessment
// @Description Removes a question from an assessment
// @Tags assessments
// @Accept json
// @Produce json
// @Param id path uint true "Assessment ID"
// @Param question_id path uint true "Question ID"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /assessments/{id}/questions/{question_id} [delete]
func (h *AssessmentHandler) RemoveQuestionFromAssessment(c *gin.Context) {
	assessmentID := h.parseIDParam(c, "id")
	if assessmentID == 0 {
		return
	}

	questionID := h.parseIDParam(c, "question_id")
	if questionID == 0 {
		return
	}

	h.LogRequest(c, "Removing question from assessment", "assessment_id", assessmentID, "question_id", questionID)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	err := h.assessmentService.RemoveQuestion(c.Request.Context(), assessmentID, questionID, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Question removed from assessment successfully",
	})
}

// ReorderAssessmentQuestions reorders questions in an assessment
// @Summary Reorder assessment questions
// @Description Reorders questions within an assessment
// @Tags assessments
// @Accept json
// @Produce json
// @Param id path uint true "Assessment ID"
// @Param orders body []repositories.QuestionOrder true "Question order data"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /assessments/{id}/questions/reorder [put]
func (h *AssessmentHandler) ReorderAssessmentQuestions(c *gin.Context) {
	id := h.parseIDParam(c, "id")
	if id == 0 {
		return
	}

	h.LogRequest(c, "Reordering assessment questions", "assessment_id", id)

	var ordersRequest services.ReorderQuestionsRequest
	if err := c.ShouldBindJSON(&ordersRequest); err != nil {
		h.RespondWithError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if len(ordersRequest.QuestionOrders) == 0 {
		h.RespondWithError(c, http.StatusBadRequest, "No question orders provided", errors.New("empty order list"))
		return
	}

	var orders []repositories.QuestionOrder
	for _, o := range ordersRequest.QuestionOrders {
		orders = append(orders, repositories.QuestionOrder{
			QuestionID: o.QuestionID,
			Order:      o.Order,
		})
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	err := h.assessmentService.ReorderQuestions(c.Request.Context(), id, orders, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Questions reordered successfully",
	})
}

// GetAssessmentStats retrieves assessment statistics
// @Summary Get assessment statistics
// @Description Retrieves statistics for an assessment
// @Tags assessments
// @Accept json
// @Produce json
// @Param id path uint true "Assessment ID"
// @Success 200 {object} SuccessResponse{data=repositories.AssessmentStats}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /assessments/{id}/stats [get]
func (h *AssessmentHandler) GetAssessmentStats(c *gin.Context) {
	id := h.parseIDParam(c, "id")
	if id == 0 {
		return
	}

	h.LogRequest(c, "Getting assessment stats", "assessment_id", id)

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}
	stats, err := h.assessmentService.GetStats(c.Request.Context(), id, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetCreatorStats retrieves creator statistics
// @Summary Get creator statistics
// @Description Retrieves statistics for a creator
// @Tags assessments
// @Accept json
// @Produce json
// @Param creator_id path uint true "Creator ID"
// @Success 200 {object} SuccessResponse{data=repositories.CreatorStats}
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /assessments/creator/{creator_id}/stats [get]
func (h *AssessmentHandler) GetCreatorStats(c *gin.Context) {
	creatorID := ParseStringIDParam(c, "creator_id")
	if creatorID == "" {
		return
	}

	h.LogRequest(c, "Getting creator stats", "creator_id", creatorID)

	stats, err := h.assessmentService.GetCreatorStats(c.Request.Context(), creatorID)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, stats)
}

// Helper methods

func (h *AssessmentHandler) getUserID(c *gin.Context) string {
	userID, exists := c.Get("user_id")
	if !exists {
		return ""
	}
	if id, ok := userID.(string); ok {
		return id
	}
	return ""
}

func (h *AssessmentHandler) parseIDParam(c *gin.Context, param string) uint {
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

func (h *AssessmentHandler) parseStringIDParam(c *gin.Context, param string) string {
	idStr := c.Param(param)
	idStr = strings.TrimSpace(idStr)
	if idStr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid " + param,
			Details: "ID cannot be empty",
		})
		return ""
	}
	return idStr
}

func (h *AssessmentHandler) parseIntQuery(c *gin.Context, param string, defaultValue int) int {
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

func (h *AssessmentHandler) parseIntQueryPtr(c *gin.Context, param string) *int {
	valueStr := c.Query(param)
	if valueStr == "" {
		return nil
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return nil
	}
	return &value
}

func (h *AssessmentHandler) parseAssessmentFilters(c *gin.Context) repositories.AssessmentFilters {
	page := h.parseIntQuery(c, "page", 1)
	size := h.parseIntQuery(c, "size", 10)

	filters := repositories.AssessmentFilters{
		Limit:  size,
		Offset: (page - 1) * size,
	}

	if status := c.Query("status"); status != "" {
		assessmentStatus := models.AssessmentStatus(status)
		filters.Status = &assessmentStatus
	}

	if creatorIDStr := c.Query("creator_id"); creatorIDStr != "" {
		filters.CreatedBy = &creatorIDStr
	}

	return filters
}

// AddQuestionsToAssessment adds multiple questions to an assessment
// @Summary Add multiple questions to assessment (batch)
// @Description Adds multiple questions to an assessment in a single transaction. Points are required and total must not exceed 100.
// @Tags assessments
// @Accept json
// @Produce json
// @Param id path uint true "Assessment ID"
// @Param request body object{questions=[]object{question_id=uint,order=int,points=int}} true "Questions with points"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /assessments/{id}/questions/batch [post]
func (h *AssessmentHandler) AddQuestionsToAssessment(c *gin.Context) {
	assessmentID := h.parseIDParam(c, "id")
	if assessmentID == 0 {
		return
	}

	h.LogRequest(c, "Adding multiple questions to assessment (batch)", "assessment_id", assessmentID)

	var req struct {
		Questions []struct {
			QuestionID uint `json:"question_id" binding:"required"`
			Order      int  `json:"order" binding:"required,min=1"`
			Points     int  `json:"points" binding:"required,min=1,max=100"`
		} `json:"questions" binding:"required,min=1,dive"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.RespondWithError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	// Convert to service DTO
	questions := make([]services.AssessmentQuestionRequest, len(req.Questions))
	for i, q := range req.Questions {
		questions[i] = services.AssessmentQuestionRequest{
			QuestionID: q.QuestionID,
			Order:      q.Order,
			Points:     q.Points,
		}
	}

	// Use optimized batch method (single transaction)
	err := h.assessmentService.AddQuestionsBatch(c.Request.Context(), assessmentID, questions, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: fmt.Sprintf("Successfully added %d questions to assessment", len(questions)),
	})
}

// AutoAssignQuestionsToAssessment adds multiple questions with automatic point distribution
// @Summary Auto-assign questions to assessment (with rebalancing)
// @Description Adds multiple questions to an assessment with automatic point distribution. IMPORTANT: This rebalances ALL questions (existing + new) to equal points using formula: 100 / total_questions. Points are calculated as: remaining_points / number_of_questions. No manual point input required. Can only be used when assessment has no student attempts.
// @Tags assessments
// @Accept json
// @Produce json
// @Param id path uint true "Assessment ID"
// @Param request body object{question_ids=[]uint} true "Question IDs to add (points will be auto-assigned and ALL questions rebalanced)"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 422 {object} ErrorResponse "Assessment locked: has attempts, cannot rebalance"
// @Failure 500 {object} ErrorResponse
// @Router /assessments/{id}/questions/auto-assign [post]
func (h *AssessmentHandler) AutoAssignQuestionsToAssessment(c *gin.Context) {
	assessmentID := h.parseIDParam(c, "id")
	if assessmentID == 0 {
		return
	}

	h.LogRequest(c, "Auto-assigning questions to assessment", "assessment_id", assessmentID)

	var req struct {
		QuestionIDs []uint `json:"question_ids" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.RespondWithError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	// Use auto-assign service method
	err := h.assessmentService.AutoAssignQuestions(c.Request.Context(), assessmentID, req.QuestionIDs, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: fmt.Sprintf("Successfully auto-assigned %d questions to assessment", len(req.QuestionIDs)),
	})
}

// RemoveQuestionsFromAssessment removes multiple questions from an assessment
// @Summary Remove multiple questions from assessment
// @Description Removes multiple questions from an assessment in batch
// @Tags assessments
// @Accept json
// @Produce json
// @Param id path uint true "Assessment ID"
// @Param question_ids body object{question_ids=[]uint} true "Question IDs"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /assessments/{id}/questions/batch [delete]
func (h *AssessmentHandler) RemoveQuestionsFromAssessment(c *gin.Context) {
	assessmentID := h.parseIDParam(c, "id")
	if assessmentID == 0 {
		return
	}

	h.LogRequest(c, "Removing multiple questions from assessment", "assessment_id", assessmentID)

	var req struct {
		QuestionIDs []uint `json:"question_ids" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.RespondWithError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	err := h.assessmentService.RemoveQuestions(c.Request.Context(), assessmentID, req.QuestionIDs, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Questions removed from assessment successfully",
	})
}

// UpdateAssessmentQuestion updates a question's settings in an assessment
// @Summary Update assessment question
// @Description Updates points and time limit for a question in an assessment
// @Tags assessments
// @Accept json
// @Produce json
// @Param id path uint true "Assessment ID"
// @Param question_id path uint true "Question ID"
// @Param update body services.UpdateAssessmentQuestionRequest true "Update data"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /assessments/{id}/questions/{question_id} [put]
func (h *AssessmentHandler) UpdateAssessmentQuestion(c *gin.Context) {
	assessmentID := h.parseIDParam(c, "id")
	if assessmentID == 0 {
		return
	}

	questionID := h.parseIDParam(c, "question_id")
	if questionID == 0 {
		return
	}

	h.LogRequest(c, "Updating assessment question", "assessment_id", assessmentID, "question_id", questionID)

	var req services.UpdateAssessmentQuestionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.RespondWithError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Set questionId from path param
	req.QuestionId = questionID

	if err := h.validator.Validate(&req); err != nil {
		h.RespondWithError(c, http.StatusBadRequest, "Validation failed", err)
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	err := h.assessmentService.UpdateAssessmentQuestion(c.Request.Context(), assessmentID, questionID, &req, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Assessment question updated successfully",
	})
}

// UpdateAssessmentQuestionsBatch updates multiple questions' settings in an assessment
// @Summary Update multiple assessment questions
// @Description Updates points and time limits for multiple questions in an assessment
// @Tags assessments
// @Accept json
// @Produce json
// @Param id path uint true "Assessment ID"
// @Param updates body []services.UpdateAssessmentQuestionRequest true "Update data"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /assessments/{id}/questions/batch [put]
func (h *AssessmentHandler) UpdateAssessmentQuestionsBatch(c *gin.Context) {
	assessmentID := h.parseIDParam(c, "id")
	if assessmentID == 0 {
		return
	}

	h.LogRequest(c, "Updating multiple assessment questions", "assessment_id", assessmentID)

	var reqs []services.UpdateAssessmentQuestionRequest
	if err := c.ShouldBindJSON(&reqs); err != nil {
		h.RespondWithError(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if len(reqs) == 0 {
		h.RespondWithError(c, http.StatusBadRequest, "No updates provided", nil)
		return
	}

	// Validate each request
	for i, req := range reqs {
		if err := h.validator.Validate(&req); err != nil {
			h.RespondWithError(c, http.StatusBadRequest, "Validation failed for request "+strconv.Itoa(i), err)
			return
		}
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	err := h.assessmentService.UpdateAssessmentQuestionBatch(c.Request.Context(), assessmentID, reqs, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Assessment questions updated successfully",
	})
}

func (h *AssessmentHandler) handleServiceError(c *gin.Context, err error) {
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

	// Handle specific assessment errors
	switch {
	case errors.Is(err, services.ErrAssessmentNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Message: "Assessment not found",
		})
	case errors.Is(err, services.ErrAssessmentAccessDenied):
		c.JSON(http.StatusForbidden, ErrorResponse{
			Message: "Access denied to assessment",
		})
	case errors.Is(err, services.ErrAssessmentNotEditable):
		c.JSON(http.StatusConflict, ErrorResponse{
			Message: "Assessment cannot be edited in current status",
		})
	case errors.Is(err, services.ErrAssessmentNotDeletable):
		c.JSON(http.StatusConflict, ErrorResponse{
			Message: "Assessment cannot be deleted - has existing attempts",
		})
	case errors.Is(err, services.ErrAssessmentInvalidStatus):
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid assessment status transition",
		})
	case errors.Is(err, services.ErrAssessmentDuplicateTitle):
		c.JSON(http.StatusConflict, ErrorResponse{
			Message: "Assessment title already exists for this user",
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
			Details: err.Error(),
		})
	}
}
