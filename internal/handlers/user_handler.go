package handlers

import (
	"net/http"
	"strconv"

	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/SAP-F-2025/assessment-service/internal/utils"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	BaseHandler
	userRepo repositories.UserRepository
}

func NewUserHandler(userRepo repositories.UserRepository, logger utils.Logger) *UserHandler {
	return &UserHandler{
		BaseHandler: NewBaseHandler(logger),
		userRepo:    userRepo,
	}
}

// ListUsers lists users with optional filtering
// @Summary List users
// @Description Get a paginated list of users (for sharing purposes)
// @Tags users
// @Accept json
// @Produce json
// @Param page query int false "Page number (default: 1)"
// @Param size query int false "Page size (default: 10, max: 100)"
// @Param q query string false "Search query (name or email)"
// @Param role query string false "Filter by role (student, teacher, proctor, admin)"
// @Success 200 {object} map[string]interface{} "User list response"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /users [get]
func (h *UserHandler) ListUsers(c *gin.Context) {
	h.LogRequest(c, "Listing users")

	// Check authentication
	_, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	// Parse filters
	filters := h.parseUserFilters(c)

	// Get users
	users, total, err := h.userRepo.List(c.Request.Context(), filters)
	if err != nil {
		h.LogError(c, err, "Failed to list users")
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Message: "Failed to list users",
			Details: err.Error(),
		})
		return
	}

	// Calculate page number
	page := (filters.Offset / max(filters.Limit, 1)) + 1

	response := map[string]interface{}{
		"users": users,
		"total": total,
		"page":  page,
		"size":  filters.Limit,
	}

	c.JSON(http.StatusOK, response)
}

// SearchUsers searches for users
// @Summary Search users
// @Description Search users by name or email
// @Tags users
// @Accept json
// @Produce json
// @Param q query string true "Search query"
// @Param page query int false "Page number (default: 1)"
// @Param size query int false "Page size (default: 10, max: 100)"
// @Param role query string false "Filter by role (student, teacher, proctor, admin)"
// @Success 200 {object} map[string]interface{} "User search results"
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /users/search [get]
func (h *UserHandler) SearchUsers(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "Search query parameter 'q' is required",
		})
		return
	}

	h.LogRequest(c, "Searching users", "query", query)

	// Check authentication
	_, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	// Parse filters
	filters := h.parseUserFilters(c)

	// Search users
	users, total, err := h.userRepo.Search(c.Request.Context(), query, filters)
	if err != nil {
		h.LogError(c, err, "Failed to search users")
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Message: "Failed to search users",
			Details: err.Error(),
		})
		return
	}

	// Calculate page number
	page := (filters.Offset / max(filters.Limit, 1)) + 1

	response := map[string]interface{}{
		"users": users,
		"total": total,
		"page":  page,
		"size":  filters.Limit,
	}

	c.JSON(http.StatusOK, response)
}

// GetUser retrieves a user by ID
// @Summary Get user by ID
// @Description Get user information by ID
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} models.User
// @Failure 400 {object} ErrorResponse "Bad request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 404 {object} ErrorResponse "Not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /users/{id} [get]
func (h *UserHandler) GetUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Message: "User ID is required",
		})
		return
	}

	h.LogRequest(c, "Getting user", "user_id", userID)

	// Check authentication
	_, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Message: "User not authenticated",
		})
		return
	}

	// Get user
	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		h.LogError(c, err, "Failed to get user")
		c.JSON(http.StatusNotFound, ErrorResponse{
			Message: "User not found",
			Details: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, user)
}

// ===== HELPER METHODS =====

func (h *UserHandler) parseUserFilters(c *gin.Context) repositories.UserFilters {
	// Parse pagination using page and size
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

	filters := repositories.UserFilters{
		Limit:  size,
		Offset: (page - 1) * size,
		Query:  c.Query("q"),
	}

	return filters
}
