package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/SAP-F-2025/assessment-service/internal/services"
	"github.com/SAP-F-2025/assessment-service/internal/utils"
	"github.com/gin-gonic/gin"
)

type QuestionBankHandler struct {
	BaseHandler
	service services.QuestionBankService
}

func NewQuestionBankHandler(service services.QuestionBankService, logger utils.Logger) *QuestionBankHandler {
	return &QuestionBankHandler{
		BaseHandler: NewBaseHandler(logger),
		service:     service,
	}
}

// ===== CORE CRUD ENDPOINTS =====

// CreateQuestionBank creates a new question bank
// @Summary Create a new question bank
// @Description Create a new question bank with the provided details
// @Tags question-banks
// @Accept json
// @Produce json
// @Param request body services.CreateQuestionBankRequest true "Question Bank creation request"
// @Success 201 {object} services.QuestionBankResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 409 {object} ErrorResponse "Conflict - bank name already exists"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /question-banks [post]
func (h *QuestionBankHandler) CreateQuestionBank(c *gin.Context) {
	var req services.CreateQuestionBankRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid request payload",
			Details: err.Error(),
		})
		return
	}

	// Get user ID from JWT token (middleware should set this)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	response, err := h.service.Create(c.Request.Context(), &req, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, response)
}

// GetQuestionBank retrieves a question bank by ID
// @Summary Get a question bank by ID
// @Description Retrieve a question bank with its basic information
// @Tags question-banks
// @Accept json
// @Produce json
// @Param id path int true "Question Bank ID"
// @Success 200 {object} services.QuestionBankResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - no access to bank"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /question-banks/{id} [get]
func (h *QuestionBankHandler) GetQuestionBank(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid question bank ID",
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

	response, err := h.service.GetByID(c.Request.Context(), uint(id), userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetQuestionBankWithDetails retrieves a question bank by ID with full details
// @Summary Get a question bank by ID with details
// @Description Retrieve a question bank with questions and sharing information
// @Tags question-banks
// @Accept json
// @Produce json
// @Param id path int true "Question Bank ID"
// @Success 200 {object} services.QuestionBankResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - no access to bank"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /question-banks/{id}/details [get]
func (h *QuestionBankHandler) GetQuestionBankWithDetails(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid question bank ID",
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

	response, err := h.service.GetByIDWithDetails(c.Request.Context(), uint(id), userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// UpdateQuestionBank updates a question bank
// @Summary Update a question bank
// @Description Update question bank details (only owner or editors can update)
// @Tags question-banks
// @Accept json
// @Produce json
// @Param id path int true "Question Bank ID"
// @Param request body services.UpdateQuestionBankRequest true "Question Bank update request"
// @Success 200 {object} services.QuestionBankResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - no edit access"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 409 {object} ErrorResponse "Conflict - bank name already exists"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /question-banks/{id} [put]
func (h *QuestionBankHandler) UpdateQuestionBank(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid question bank ID",
		})
		return
	}

	var req services.UpdateQuestionBankRequest
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

	response, err := h.service.Update(c.Request.Context(), uint(id), &req, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// DeleteQuestionBank deletes a question bank
// @Summary Delete a question bank
// @Description Delete a question bank (only owner can delete)
// @Tags question-banks
// @Accept json
// @Produce json
// @Param id path int true "Question Bank ID"
// @Success 204 "No content"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - no delete access"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /question-banks/{id} [delete]
func (h *QuestionBankHandler) DeleteQuestionBank(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid question bank ID",
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

	err = h.service.Delete(c.Request.Context(), uint(id), userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ===== LIST AND SEARCH ENDPOINTS =====

// ListQuestionBanks lists question banks with filtering
// @Summary List question banks
// @Description Get a paginated list of accessible question banks
// @Tags question-banks
// @Accept json
// @Produce json
// @Param is_public query bool false "Filter by public banks"
// @Param is_shared query bool false "Filter by shared banks"
// @Param created_by query int false "Filter by creator ID"
// @Param name query string false "Filter by bank name (partial match)"
// @Param page query int false "Page number (default: 1)"
// @Param size query int false "Page size (default: 10, max: 100)"
// @Param sort_by query string false "Sort field (name, created_at) (default: created_at)"
// @Param sort_order query string false "Sort order (asc, desc) (default: desc)"
// @Success 200 {object} services.QuestionBankListResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /question-banks [get]
func (h *QuestionBankHandler) ListQuestionBanks(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	filters := h.parseQuestionBankFilters(c)
	response, err := h.service.List(c.Request.Context(), filters, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// ListPublicQuestionBanks lists public question banks
// @Summary List public question banks
// @Description Get a paginated list of public question banks
// @Tags question-banks
// @Accept json
// @Produce json
// @Param name query string false "Filter by bank name (partial match)"
// @Param page query int false "Page number (default: 1)"
// @Param size query int false "Page size (default: 10, max: 100)"
// @Param sort_by query string false "Sort field (name, created_at) (default: created_at)"
// @Param sort_order query string false "Sort order (asc, desc) (default: desc)"
// @Success 200 {object} services.QuestionBankListResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /question-banks/public [get]
func (h *QuestionBankHandler) GetPublicQuestionBanksOLD(c *gin.Context) {
	filters := h.parseQuestionBankFilters(c)
	response, err := h.service.GetPublic(c.Request.Context(), filters)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// ListSharedQuestionBanks lists question banks shared with the user
// @Summary List shared question banks
// @Description Get a paginated list of question banks shared with the current user
// @Tags question-banks
// @Accept json
// @Produce json
// @Param name query string false "Filter by bank name (partial match)"
// @Param page query int false "Page number (default: 1)"
// @Param size query int false "Page size (default: 10, max: 100)"
// @Param sort_by query string false "Sort field (name, created_at) (default: created_at)"
// @Param sort_order query string false "Sort order (asc, desc) (default: desc)"
// @Success 200 {object} services.QuestionBankListResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /question-banks/shared [get]
func (h *QuestionBankHandler) GetSharedQuestionBanksOLD(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	filters := h.parseQuestionBankFilters(c)
	response, err := h.service.GetSharedWithUser(c.Request.Context(), userID.(string), filters)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// SearchQuestionBanks searches question banks
// @Summary Search question banks
// @Description Search accessible question banks by name and description
// @Tags question-banks
// @Accept json
// @Produce json
// @Param q query string true "Search query"
// @Param is_public query bool false "Filter by public banks"
// @Param is_shared query bool false "Filter by shared banks"
// @Param created_by query int false "Filter by creator ID"
// @Param page query int false "Page number (default: 1)"
// @Param size query int false "Page size (default: 10, max: 100)"
// @Param sort_by query string false "Sort field (name, created_at) (default: created_at)"
// @Param sort_order query string false "Sort order (asc, desc) (default: desc)"
// @Success 200 {object} services.QuestionBankListResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /question-banks/search [get]
func (h *QuestionBankHandler) SearchQuestionBanks(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Search query parameter 'q' is required",
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

	filters := h.parseQuestionBankFilters(c)
	response, err := h.service.Search(c.Request.Context(), query, filters, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// ===== SHARING ENDPOINTS =====

// ShareQuestionBank shares a question bank with another user
// @Summary Share a question bank
// @Description Share a question bank with another user with specified permissions
// @Tags question-banks
// @Accept json
// @Produce json
// @Param id path int true "Question Bank ID"
// @Param request body services.ShareQuestionBankRequest true "Share request"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - not owner"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 409 {object} ErrorResponse "Conflict - already shared with user"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /question-banks/{id}/share [post]
func (h *QuestionBankHandler) ShareQuestionBank(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid question bank ID",
		})
		return
	}

	var req services.ShareQuestionBankRequest
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

	err = h.service.ShareBank(c.Request.Context(), uint(id), &req, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Question bank shared successfully",
	})
}

// UnshareQuestionBank removes sharing of a question bank with a user
// @Summary Unshare a question bank
// @Description Remove sharing access of a question bank from a specific user
// @Tags question-banks
// @Accept json
// @Produce json
// @Param id path int true "Question Bank ID"
// @Param user_id path int true "User ID to unshare from"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - not owner"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /question-banks/{id}/unshare/{user_id} [delete]
func (h *QuestionBankHandler) UnshareQuestionBank(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid question bank ID",
		})
		return
	}

	targetUserId := ParseStringIDParam(c, "user_id")
	if targetUserId == "" {
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	err = h.service.UnshareBank(c.Request.Context(), uint(id), targetUserId, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Question bank unshared successfully",
	})
}

// UpdateSharePermissions updates sharing permissions for a user
// @Summary Update share permissions
// @Description Update permissions for a user who has access to a shared question bank
// @Tags question-banks
// @Accept json
// @Produce json
// @Param id path int true "Question Bank ID"
// @Param user_id path int true "User ID"
// @Param request body UpdateSharePermissionsRequest true "Permission update request"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - not owner"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /question-banks/{id}/share/{user_id} [put]
func (h *QuestionBankHandler) UpdateSharePermissions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid question bank ID",
		})
		return
	}

	targetUserID := ParseStringIDParam(c, "user_id")
	if targetUserID == "" {
		return
	}

	var req UpdateSharePermissionsRequest
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

	err = h.service.UpdateSharePermissions(c.Request.Context(), uint(id), targetUserID, req.CanEdit, req.CanDelete, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Share permissions updated successfully",
	})
}

// GetBankShares lists all users who have access to a question bank
// @Summary Get bank shares
// @Description Get list of users who have access to a question bank (only owner can view)
// @Tags question-banks
// @Accept json
// @Produce json
// @Param id path int true "Question Bank ID"
// @Success 200 {array} services.QuestionBankShareResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - not owner"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /question-banks/{id}/shares [get]
func (h *QuestionBankHandler) GetQuestionBankSharesOLD(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid question bank ID",
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

	shares, err := h.service.GetBankShares(c.Request.Context(), uint(id), userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, shares)
}

// ===== QUESTION MANAGEMENT ENDPOINTS =====

// AddQuestionsToBank adds questions to a question bank
// @Summary Add questions to bank
// @Description Add multiple questions to a question bank
// @Tags question-banks
// @Accept json
// @Produce json
// @Param id path int true "Question Bank ID"
// @Param request body services.AddQuestionsTobankRequest true "Questions to add"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - no edit access"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /question-banks/{id}/questions [post]
func (h *QuestionBankHandler) AddQuestionsToBank(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid question bank ID",
		})
		return
	}

	var req services.AddQuestionsTobankRequest
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

	err = h.service.AddQuestions(c.Request.Context(), uint(id), &req, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Questions added to bank successfully",
	})
}

// RemoveQuestionsFromBank removes questions from a question bank
// @Summary Remove questions from bank
// @Description Remove multiple questions from a question bank
// @Tags question-banks
// @Accept json
// @Produce json
// @Param id path int true "Question Bank ID"
// @Param request body RemoveQuestionsRequest true "Questions to remove"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - no edit access"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /question-banks/{id}/questions [delete]
func (h *QuestionBankHandler) RemoveQuestionsFromBank(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid question bank ID",
		})
		return
	}

	var req RemoveQuestionsRequest
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

	err = h.service.RemoveQuestions(c.Request.Context(), uint(id), req.QuestionIDs, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Message: "Questions removed from bank successfully",
	})
}

// GetBankQuestions lists questions in a question bank
// @Summary Get questions in bank
// @Description Get a paginated list of questions in a question bank
// @Tags question-banks
// @Accept json
// @Produce json
// @Param id path int true "Question Bank ID"
// @Param type query string false "Filter by question type"
// @Param difficulty query string false "Filter by difficulty level"
// @Param category_id query int false "Filter by category ID"
// @Param page query int false "Page number (default: 1)"
// @Param size query int false "Page size (default: 10, max: 100)"
// @Param sort_by query string false "Sort field (created_at, text) (default: created_at)"
// @Param sort_order query string false "Sort order (asc, desc) (default: desc)"
// @Success 200 {object} services.QuestionListResponse
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - no access to bank"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /question-banks/{id}/questions [get]
func (h *QuestionBankHandler) GetBankQuestions(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid question bank ID",
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

	filters := h.parseQuestionFilters(c)
	response, err := h.service.GetBankQuestions(c.Request.Context(), uint(id), filters, userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetPublicQuestionBanks gets public question banks
// @Summary Get public question banks
// @Description Get all publicly available question banks
// @Tags question-banks
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param size query int false "Page size" default(10)
// @Param sort query string false "Sort field" default("created_at")
// @Param order query string false "Sort order" default("desc")
// @Success 200 {object} map[string]interface{} "Public question banks list"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /question-banks/public [get]
func (h *QuestionBankHandler) GetPublicQuestionBanks(c *gin.Context) {
	filters := h.parseQuestionBankFilters(c)
	isPublic := true
	filters.IsPublic = &isPublic

	response, err := h.service.GetPublic(c.Request.Context(), filters)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetSharedQuestionBanks gets question banks shared with the current user
// @Summary Get shared question banks
// @Description Get all question banks that have been shared with the current user
// @Tags question-banks
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param size query int false "Page size" default(10)
// @Param sort query string false "Sort field" default("created_at")
// @Param order query string false "Sort order" default("desc")
// @Success 200 {object} map[string]interface{} "Shared question banks list"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /question-banks/shared [get]
func (h *QuestionBankHandler) GetSharedQuestionBanks(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	filters := h.parseQuestionBankFilters(c)

	response, err := h.service.GetSharedWithUser(c.Request.Context(), userID.(string), filters)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetQuestionBankStats gets statistics for a question bank
// @Summary Get question bank statistics
// @Description Get detailed statistics about a question bank including usage metrics
// @Tags question-banks
// @Accept json
// @Produce json
// @Param id path uint true "Question Bank ID"
// @Success 200 {object} repositories.QuestionBankStats "Question bank statistics"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - no access to bank"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /question-banks/{id}/stats [get]
func (h *QuestionBankHandler) GetQuestionBankStats(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid question bank ID",
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

	stats, err := h.service.GetStats(c.Request.Context(), uint(id), userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetQuestionBankShares gets all shares for a question bank
// @Summary Get question bank shares
// @Description Get all users that a question bank has been shared with
// @Tags question-banks
// @Accept json
// @Produce json
// @Param id path uint true "Question Bank ID"
// @Success 200 {object} []repositories.QuestionBankShare "List of bank shares"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - not owner of bank"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /question-banks/{id}/shares [get]
func (h *QuestionBankHandler) GetQuestionBankShares(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Invalid question bank ID",
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

	shares, err := h.service.GetBankShares(c.Request.Context(), uint(id), userID.(string))
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, shares)
}

// GetUserShares gets all question banks shared with a specific user
// @Summary Get user shares
// @Description Get all question banks that have been shared with a specific user
// @Tags question-banks
// @Accept json
// @Produce json
// @Param user_id path uint true "User ID"
// @Param page query int false "Page number" default(1)
// @Param size query int false "Page size" default(10)
// @Success 200 {object} map[string]interface{} "List of question banks shared with user"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden - insufficient permissions"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /question-banks/user/{user_id}/shares [get]
func (h *QuestionBankHandler) GetUserShares(c *gin.Context) {
	targetUserID := ParseStringIDParam(c, "user_id")
	if targetUserID == "" {
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	// Only allow users to see their own shares, or admin access
	if userID.(string) != (targetUserID) {
		// You might want to add admin role check here
		c.JSON(http.StatusForbidden, ErrorResponse{
			Message: "Cannot view other user's shares",
		})
		return
	}

	filters := h.parseQuestionBankFilters(c)

	response, err := h.service.GetSharedWithUser(c.Request.Context(), targetUserID, filters)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetQuestionBanksByCreator gets all question banks created by a specific user
// @Summary Get question banks by creator
// @Description Get all question banks created by a specific user
// @Tags question-banks
// @Accept json
// @Produce json
// @Param creator_id path uint true "Creator User ID"
// @Param page query int false "Page number" default(1)
// @Param size query int false "Page size" default(10)
// @Param sort query string false "Sort field" default("created_at")
// @Param order query string false "Sort order" default("desc")
// @Success 200 {object} map[string]interface{} "List of question banks by creator"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /question-banks/creator/{creator_id} [get]
func (h *QuestionBankHandler) GetQuestionBanksByCreator(c *gin.Context) {
	creatorID := ParseStringIDParam(c, "creator_id")
	if creatorID == "" {
		return
	}

	_, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	filters := h.parseQuestionBankFilters(c)

	response, err := h.service.GetByCreator(c.Request.Context(), creatorID, filters)
	if err != nil {
		h.handleServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// ===== HELPER METHODS =====

func (h *QuestionBankHandler) parseQuestionBankFilters(c *gin.Context) repositories.QuestionBankFilters {
	page := 1
	size := 10

	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if sizeStr := c.Query("size"); sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 && s <= 100 {
			size = s
		}
	}

	filters := repositories.QuestionBankFilters{
		Limit:     size,
		Offset:    (page - 1) * size,
		SortBy:    "created_at",
		SortOrder: "desc",
	}

	// Parse boolean filters
	if isPublic := c.Query("is_public"); isPublic != "" {
		if isPublic == "true" {
			filters.IsPublic = &[]bool{true}[0]
		} else if isPublic == "false" {
			filters.IsPublic = &[]bool{false}[0]
		}
	}

	if isShared := c.Query("is_shared"); isShared != "" {
		if isShared == "true" {
			filters.IsShared = &[]bool{true}[0]
		} else if isShared == "false" {
			filters.IsShared = &[]bool{false}[0]
		}
	}

	// Parse numeric filters
	if createdBy := c.Query("created_by"); createdBy != "" {
		filters.CreatedBy = &createdBy
	}

	// Parse string filters
	if name := c.Query("name"); name != "" {
		filters.Name = &name
	}

	// Parse sorting
	if sortBy := c.Query("sort_by"); sortBy != "" {
		// Validate sort field
		validSortFields := map[string]bool{
			"name":       true,
			"created_at": true,
			"updated_at": true,
		}
		if validSortFields[sortBy] {
			filters.SortBy = sortBy
		}
	}

	if sortOrder := c.Query("sort_order"); sortOrder != "" {
		if sortOrder == "asc" || sortOrder == "desc" {
			filters.SortOrder = sortOrder
		}
	}

	return filters
}

func (h *QuestionBankHandler) parseQuestionFilters(c *gin.Context) repositories.QuestionFilters {
	// Parse pagination using page and size (not limit and offset)
	page := 1
	size := 10

	if pageStr := c.Query("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if sizeStr := c.Query("size"); sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s > 0 && s <= 100 {
			size = s
		}
	}

	filters := repositories.QuestionFilters{
		Limit:     size,
		Offset:    (page - 1) * size,
		SortBy:    "created_at",
		SortOrder: "desc",
	}

	// Parse question type filter
	if qType := c.Query("type"); qType != "" {
		// TODO: Validate question type against models.QuestionType constants
		filters.Type = (*models.QuestionType)(&qType)
	}

	// Parse difficulty filter
	if difficulty := c.Query("difficulty"); difficulty != "" {
		// TODO: Validate difficulty against models.DifficultyLevel constants
		filters.Difficulty = (*models.DifficultyLevel)(&difficulty)
	}

	// Parse category ID filter
	if categoryID := c.Query("category_id"); categoryID != "" {
		if id, err := strconv.ParseUint(categoryID, 10, 32); err == nil {
			filters.CategoryID = &[]uint{uint(id)}[0]
		}
	}

	// Parse sorting
	if sortBy := c.Query("sort_by"); sortBy != "" {
		// Only allow sortBy values that map to allowed DB columns.
		validSortFields := map[string]bool{
			"created_at": true,
			"updated_at": true,
			"difficulty": true,
			"type":       true,
		}
		if validSortFields[sortBy] {
			filters.SortBy = sortBy
		}
	}

	if sortOrder := c.Query("sort_order"); sortOrder != "" {
		if sortOrder == "asc" || sortOrder == "desc" {
			filters.SortOrder = sortOrder
		}
	}

	return filters
}

func (h *QuestionBankHandler) handleServiceError(c *gin.Context, err error) {
	// Map service errors to HTTP status codes
	switch {
	case errors.Is(err, services.ErrQuestionBankNotFound):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Message: "Question bank not found",
		})
	case errors.Is(err, services.ErrQuestionBankAccessDenied):
		c.JSON(http.StatusForbidden, ErrorResponse{
			Message: "Access denied to question bank",
		})
	case errors.Is(err, services.ErrQuestionBankDuplicateName):
		c.JSON(http.StatusConflict, ErrorResponse{
			Message: "Question bank name already exists",
		})
	case errors.Is(err, services.ErrQuestionBankShareExists):
		c.JSON(http.StatusConflict, ErrorResponse{
			Message: "Question bank already shared with this user",
		})
	case errors.Is(err, services.ErrQuestionBankNotShared):
		c.JSON(http.StatusNotFound, ErrorResponse{
			Message: "Question bank is not shared with this user",
		})
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
