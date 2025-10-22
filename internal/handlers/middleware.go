package handlers

import (
	"net/http"

	"github.com/SAP-F-2025/assessment-service/internal/utils"
	"github.com/gin-gonic/gin"
	uuid2 "github.com/google/uuid"
)

// SetupMiddleware sets up common middleware for the Gin router
func SetupMiddleware(router *gin.Engine, logger utils.Logger) {
	// Request ID middleware (simplified)
	router.Use(RequestIDMiddleware())

	// CORS middleware (simplified)
	router.Use(CORSMiddleware())

	// Recovery middleware
	router.Use(gin.Recovery())

	// Context logger middleware (adds logger with request_id to context)
	router.Use(utils.ContextLogger(logger))

	// Custom logging middleware
	router.Use(utils.LoggerMiddleware(logger))

	// Security headers middleware
	router.Use(SecurityMiddleware())
}

// AuthMiddleware provides authentication middleware
// Deprecated: Use CasdoorAuthMiddleware.AuthMiddleware() instead
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// For now, this is a placeholder implementation
		// In a real application, you would:
		// 1. Extract JWT token from Authorization header
		// 2. Validate the token
		// 3. Extract user information from token
		// 4. Set user_id in context

		// Placeholder: Set a dummy user ID for development
		// Remove this in production and implement proper JWT validation
		c.Set("user_id", uint(1))

		c.Next()
	}
}

// LoggerMiddleware provides custom logging middleware
func LoggerMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return ""
	})
}

// SecurityMiddleware adds security headers
func SecurityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Next()
	}
}

// RateLimitMiddleware provides rate limiting (placeholder)
func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Implement rate limiting logic here
		c.Next()
	}
}

// RequestIDMiddleware generates a unique request ID for each request
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			// Generate a new request ID
			requestID = uuid2.New().String()
		}
		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)
		c.Next()
	}
}

// CORSMiddleware provides CORS support
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		c.Header("Access-Control-Expose-Headers", "Content-Length")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "43200")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// HealthCheck endpoint
//func HealthCheck(c *gin.Context) {
//	c.JSON(http.StatusOK, gin.H{
//		"status":    "healthy",
//		"timestamp": time.Now().UTC().Format(time.RFC3339),
//		"service":   "assessment-service",
//	})
//}
