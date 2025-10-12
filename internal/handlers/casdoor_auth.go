package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"github.com/gin-gonic/gin"

	"github.com/SAP-F-2025/assessment-service/internal/config"
	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
)

// CasdoorAuthMiddleware provides authentication using Casdoor SDK
type CasdoorAuthMiddleware struct {
	client   *casdoorsdk.Client
	userRepo repositories.UserRepository
	config   config.CasdoorConfig
}

// NewCasdoorAuthMiddleware creates a new Casdoor authentication middleware
func NewCasdoorAuthMiddleware(cfg config.CasdoorConfig, userRepo repositories.UserRepository) *CasdoorAuthMiddleware {
	client := casdoorsdk.NewClient(
		cfg.Endpoint,
		cfg.ClientID,
		cfg.ClientSecret,
		cfg.Cert,
		cfg.Application,
		cfg.Organization,
	)

	return &CasdoorAuthMiddleware{
		client:   client,
		userRepo: userRepo,
		config:   cfg,
	}
}

// AuthMiddleware returns a Gin middleware function for Casdoor authentication
func (cam *CasdoorAuthMiddleware) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "authorization header missing",
			})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>" format
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || strings.ToLower(tokenParts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "invalid authorization header format",
			})
			c.Abort()
			return
		}

		token := tokenParts[1]

		// Parse and validate the token using Casdoor SDK
		claims, err := cam.client.ParseJwtToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": fmt.Sprintf("invalid token: %v", err),
			})
			c.Abort()
			return
		}

		// Extract user information from claims
		user, err := cam.extractUserFromClaims(c.Request.Context(), claims)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": fmt.Sprintf("failed to extract user info: %v", err),
			})
			c.Abort()
			return
		}

		// Set user information in context
		c.Set("user_id", user.ID)
		c.Set("user", user)
		c.Set("user_role", user.Role)
		c.Set("user_email", user.Email)

		// Continue with the request
		c.Next()
	}
}

// OptionalAuthMiddleware provides optional authentication (user info if token present)
func (cam *CasdoorAuthMiddleware) OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// No auth header, continue without user info
			c.Next()
			return
		}

		// Extract and validate token
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || strings.ToLower(tokenParts[0]) != "bearer" {
			// Invalid format, continue without user info
			c.Next()
			return
		}

		token := tokenParts[1]
		claims, err := casdoorsdk.ParseJwtToken(token)
		if err != nil {
			// Invalid token, continue without user info
			c.Next()
			return
		}

		// Extract user info if token is valid
		user, err := cam.extractUserFromClaims(c.Request.Context(), claims)
		if err == nil {
			c.Set("user_id", user.ID)
			c.Set("user", user)
			c.Set("user_role", user.Role)
			c.Set("user_email", user.Email)
		}

		c.Next()
	}
}

// RequireRoleMiddleware checks if user has required role
func (cam *CasdoorAuthMiddleware) RequireRoleMiddleware(requiredRoles ...models.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user role from context (should be set by AuthMiddleware)
		userRole, exists := c.Get("user_role")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "user role not found in context",
			})
			c.Abort()
			return
		}

		role, ok := userRole.(models.UserRole)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "invalid user role format",
			})
			c.Abort()
			return
		}

		// Check if user has any of the required roles
		hasRequiredRole := false
		for _, requiredRole := range requiredRoles {
			if role == requiredRole || role == models.RoleAdmin {
				hasRequiredRole = true
				break
			}
		}

		if !hasRequiredRole {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": fmt.Sprintf("insufficient permissions, required role: %v", requiredRoles),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// extractUserFromClaims extracts user information from JWT claims
func (cam *CasdoorAuthMiddleware) extractUserFromClaims(ctx context.Context, claims *casdoorsdk.Claims) (*models.User, error) {
	// Extract user ID from claims
	userID := claims.Id
	if userID == "" {
		return nil, fmt.Errorf("invalid user ID in token")
	}

	// Try to get user from repository (cache or Casdoor)
	user, err := cam.userRepo.GetByID(ctx, userID)
	if err != nil {
		// If user not found in repo, create from claims
		user = cam.createUserFromClaims(claims)
		if user == nil {
			return nil, fmt.Errorf("failed to create user from claims")
		}
	}

	// Update user activity
	if err := cam.updateUserActivity(ctx, user.ID); err != nil {
		// Log error but don't fail the request
		// TODO: Add proper logging
	}

	return user, nil
}

// createUserFromClaims creates a user model from JWT claims
func (cam *CasdoorAuthMiddleware) createUserFromClaims(claims *casdoorsdk.Claims) *models.User {
	userID := claims.Id
	if userID == "" {
		return nil
	}

	// Extract user info from claims
	email := ""
	fullName := ""
	//avatarURL := ""
	//role := models.RoleStudent // Default role

	// Extract user info from claims.User (which is a struct, not a pointer)
	email = claims.User.Email
	fullName = claims.User.DisplayName
	avatarURL := claims.User.Avatar

	// Map Casdoor role to internal role
	role := cam.mapCasdoorRoleToUserRole(claims.User.Type)

	return &models.User{
		ID:        userID,
		FullName:  fullName,
		Email:     email,
		Role:      role,
		AvatarURL: &avatarURL,
		//IsActive:      true,
		EmailVerified: true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		//Timezone:      "UTC",
		//Language:      "en",
	}
}

// mapCasdoorRoleToUserRole maps Casdoor user type to internal role
func (cam *CasdoorAuthMiddleware) mapCasdoorRoleToUserRole(casdoorType string) models.UserRole {
	switch strings.ToLower(casdoorType) {
	case "admin", "administrator":
		return models.RoleAdmin
	case "teacher", "instructor", "educator":
		return models.RoleTeacher
	case "proctor", "supervisor":
		return models.RoleProctor
	case "student", "learner":
		return models.RoleStudent
	default:
		return models.RoleStudent
	}
}

// updateUserActivity updates user's last activity time
func (cam *CasdoorAuthMiddleware) updateUserActivity(ctx context.Context, userID string) error {
	// This would typically update last_login_at in the database
	// Since we're using Casdoor, we might not need to implement this
	// or we could cache the activity in Redis
	return nil
}

// GetUserFromContext extracts user from Gin context
func GetUserFromContext(c *gin.Context) (*models.User, error) {
	user, exists := c.Get("user")
	if !exists {
		return nil, fmt.Errorf("user not found in context")
	}

	userModel, ok := user.(*models.User)
	if !ok {
		return nil, fmt.Errorf("invalid user type in context")
	}

	return userModel, nil
}

// GetUserIDFromContext extracts user ID from Gin context
func GetUserIDFromContext(c *gin.Context) (string, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", fmt.Errorf("user ID not found in context")
	}

	id, ok := userID.(string)
	if !ok {
		return "", fmt.Errorf("invalid user ID type in context")
	}

	return id, nil
}

// GetUserRoleFromContext extracts user role from Gin context
func GetUserRoleFromContext(c *gin.Context) (models.UserRole, error) {
	userRole, exists := c.Get("user_role")
	if !exists {
		return "", fmt.Errorf("user role not found in context")
	}

	role, ok := userRole.(models.UserRole)
	if !ok {
		return "", fmt.Errorf("invalid user role type in context")
	}

	return role, nil
}
