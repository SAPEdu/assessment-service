package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/SAP-F-2025/assessment-service/internal/config"
	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/SAP-F-2025/assessment-service/internal/services"
	"github.com/SAP-F-2025/assessment-service/internal/utils"
	"github.com/SAP-F-2025/assessment-service/internal/validator"
)

type HandlerManager struct {
	assessmentHandler   *AssessmentHandler
	questionHandler     *QuestionHandler
	questionBankHandler *QuestionBankHandler
	attemptHandler      *AttemptHandler
	gradingHandler      *GradingHandler
	dashboardHandler    *DashboardHandler
	studentHandler      *StudentHandler
	userHandler         *UserHandler
	authMiddleware      *CasdoorAuthMiddleware
}

func NewHandlerManager(
	serviceManager services.ServiceManager,
	validator *validator.Validator,
	logger utils.Logger,
	casdoorConfig config.CasdoorConfig,
	userRepo repositories.UserRepository,
) *HandlerManager {
	authMiddleware := NewCasdoorAuthMiddleware(casdoorConfig, userRepo)

	return &HandlerManager{
		assessmentHandler:   NewAssessmentHandler(serviceManager.Assessment(), validator, logger),
		questionHandler:     NewQuestionHandler(serviceManager.Question(), validator, logger),
		questionBankHandler: NewQuestionBankHandler(serviceManager.QuestionBank(), logger),
		attemptHandler:      NewAttemptHandler(serviceManager.Attempt(), validator, logger),
		gradingHandler:      NewGradingHandler(serviceManager.Grading(), validator, logger),
		dashboardHandler:    NewDashboardHandler(serviceManager.Dashboard(), logger),
		studentHandler:      NewStudentHandler(serviceManager.Student(), logger),
		userHandler:         NewUserHandler(userRepo, logger),
		authMiddleware:      authMiddleware,
	}
}

// SetupRoutes sets up all API routes
func (hm *HandlerManager) SetupRoutes(router *gin.Engine) {
	// Health check endpoint
	// router.GET("/health", HealthCheck)

	// API v1 routes with authentication
	v1 := router.Group("/api/v1")
	v1.Use(hm.authMiddleware.AuthMiddleware()) // Apply authentication to all API routes
	{
		// Assessment routes
		assessments := v1.Group("/assessments")
		{
			// Create/modify assessments - Teachers and Admins only
			assessments.POST("", hm.authMiddleware.RequireRoleMiddleware(models.RoleTeacher, models.RoleAdmin), hm.assessmentHandler.CreateAssessment)
			assessments.PUT("/:id", hm.authMiddleware.RequireRoleMiddleware(models.RoleTeacher, models.RoleAdmin), hm.assessmentHandler.UpdateAssessment)
			assessments.DELETE("/:id", hm.authMiddleware.RequireRoleMiddleware(models.RoleTeacher, models.RoleAdmin), hm.assessmentHandler.DeleteAssessment)
			assessments.PUT("/:id/status", hm.authMiddleware.RequireRoleMiddleware(models.RoleTeacher, models.RoleAdmin), hm.assessmentHandler.UpdateAssessmentStatus)
			assessments.POST("/:id/publish", hm.authMiddleware.RequireRoleMiddleware(models.RoleTeacher, models.RoleAdmin), hm.assessmentHandler.PublishAssessment)
			assessments.POST("/:id/archive", hm.authMiddleware.RequireRoleMiddleware(models.RoleTeacher, models.RoleAdmin), hm.assessmentHandler.ArchiveAssessment)

			// View assessments - All authenticated users
			assessments.GET("", hm.assessmentHandler.ListAssessments)
			assessments.GET("/search", hm.assessmentHandler.SearchAssessments)
			assessments.GET("/:id", hm.assessmentHandler.GetAssessment)
			assessments.GET("/:id/details", hm.assessmentHandler.GetAssessmentWithDetails)

			// Stats - Teachers and Admins only
			assessments.GET("/:id/stats", hm.authMiddleware.RequireRoleMiddleware(models.RoleTeacher, models.RoleAdmin), hm.assessmentHandler.GetAssessmentStats)

			// Assessment question management - Teachers and Admins only
			// Single question operations
			assessments.POST("/:id/questions/:question_id", hm.authMiddleware.RequireRoleMiddleware(models.RoleTeacher, models.RoleAdmin), hm.assessmentHandler.AddQuestionToAssessment)
			assessments.DELETE("/:id/questions/:question_id", hm.authMiddleware.RequireRoleMiddleware(models.RoleTeacher, models.RoleAdmin), hm.assessmentHandler.RemoveQuestionFromAssessment)
			assessments.PUT("/:id/questions/:question_id", hm.authMiddleware.RequireRoleMiddleware(models.RoleTeacher, models.RoleAdmin), hm.assessmentHandler.UpdateAssessmentQuestion)

			// Batch operations
			assessments.POST("/:id/questions/batch", hm.authMiddleware.RequireRoleMiddleware(models.RoleTeacher, models.RoleAdmin), hm.assessmentHandler.AddQuestionsToAssessment)
			assessments.POST("/:id/questions/auto-assign", hm.authMiddleware.RequireRoleMiddleware(models.RoleTeacher, models.RoleAdmin), hm.assessmentHandler.AutoAssignQuestionsToAssessment)
			assessments.DELETE("/:id/questions/batch", hm.authMiddleware.RequireRoleMiddleware(models.RoleTeacher, models.RoleAdmin), hm.assessmentHandler.RemoveQuestionsFromAssessment)
			assessments.PUT("/:id/questions/batch", hm.authMiddleware.RequireRoleMiddleware(models.RoleTeacher, models.RoleAdmin), hm.assessmentHandler.UpdateAssessmentQuestionsBatch)

			// Question ordering
			assessments.PUT("/:id/questions/reorder", hm.authMiddleware.RequireRoleMiddleware(models.RoleTeacher, models.RoleAdmin), hm.assessmentHandler.ReorderAssessmentQuestions)

			// Creator-specific routes - Teachers and Admins only
			assessments.GET("/creator/:creator_id", hm.authMiddleware.RequireRoleMiddleware(models.RoleTeacher, models.RoleAdmin), hm.assessmentHandler.GetAssessmentsByCreator)
			assessments.GET("/creator/:creator_id/stats", hm.authMiddleware.RequireRoleMiddleware(models.RoleTeacher, models.RoleAdmin), hm.assessmentHandler.GetCreatorStats)
		}

		// Question routes
		questions := v1.Group("/questions")
		{
			questions.POST("", hm.questionHandler.CreateQuestion)
			questions.POST("/batch", hm.questionHandler.CreateQuestionsBatch)
			questions.PUT("/batch", hm.questionHandler.UpdateQuestionsBatch)
			questions.GET("", hm.questionHandler.ListQuestions)
			questions.GET("/search", hm.questionHandler.SearchQuestions)
			questions.GET("/random", hm.questionHandler.GetRandomQuestions)
			questions.GET("/:id", hm.questionHandler.GetQuestion)
			questions.GET("/:id/details", hm.questionHandler.GetQuestionWithDetails)
			questions.PUT("/:id", hm.questionHandler.UpdateQuestion)
			questions.DELETE("/:id", hm.questionHandler.DeleteQuestion)
			questions.GET("/:id/stats", hm.questionHandler.GetQuestionStats)

			// Question bank management
			questions.GET("/bank/:bank_id", hm.questionHandler.GetQuestionsByBank)
			questions.POST("/:id/bank/:bank_id", hm.questionHandler.AddQuestionToBank)
			questions.DELETE("/:id/bank/:bank_id", hm.questionHandler.RemoveQuestionFromBank)

			// Creator-specific routes
			questions.GET("/creator/:creator_id", hm.questionHandler.GetQuestionsByCreator)
			questions.GET("/creator/:creator_id/usage-stats", hm.questionHandler.GetQuestionUsageStats)
		}

		// Question Bank routes
		questionBanks := v1.Group("/question-banks")
		{
			questionBanks.POST("", hm.questionBankHandler.CreateQuestionBank)
			questionBanks.GET("", hm.questionBankHandler.ListQuestionBanks)
			questionBanks.GET("/public", hm.questionBankHandler.GetPublicQuestionBanks)
			questionBanks.GET("/shared", hm.questionBankHandler.GetSharedQuestionBanks)
			questionBanks.GET("/search", hm.questionBankHandler.SearchQuestionBanks)
			questionBanks.GET("/:id", hm.questionBankHandler.GetQuestionBank)
			questionBanks.GET("/:id/details", hm.questionBankHandler.GetQuestionBankWithDetails)
			questionBanks.PUT("/:id", hm.questionBankHandler.UpdateQuestionBank)
			questionBanks.DELETE("/:id", hm.questionBankHandler.DeleteQuestionBank)
			questionBanks.GET("/:id/stats", hm.questionBankHandler.GetQuestionBankStats)

			// Sharing management
			questionBanks.POST("/:id/share", hm.questionBankHandler.ShareQuestionBank)
			questionBanks.DELETE("/:id/share/:user_id", hm.questionBankHandler.UnshareQuestionBank)
			questionBanks.PUT("/:id/share/:user_id/permissions", hm.questionBankHandler.UpdateSharePermissions)
			questionBanks.GET("/:id/shares", hm.questionBankHandler.GetQuestionBankShares)
			questionBanks.GET("/user/:user_id/shares", hm.questionBankHandler.GetUserShares)

			// Question management
			questionBanks.POST("/:id/questions", hm.questionBankHandler.AddQuestionsToBank)
			questionBanks.DELETE("/:id/questions", hm.questionBankHandler.RemoveQuestionsFromBank)
			questionBanks.GET("/:id/questions", hm.questionBankHandler.GetBankQuestions)

			// Creator-specific routes
			questionBanks.GET("/creator/:creator_id", hm.questionBankHandler.GetQuestionBanksByCreator)
		}

		// User routes (for sharing purposes)
		users := v1.Group("/users")
		{
			users.GET("", hm.userHandler.ListUsers)
			users.GET("/search", hm.userHandler.SearchUsers)
			users.GET("/:id", hm.userHandler.GetUser)
		}

		// Attempt routes
		attempts := v1.Group("/attempts")
		{
			attempts.POST("/start", hm.attemptHandler.StartAttempt)
			attempts.POST("/submit", hm.attemptHandler.SubmitAttempt)
			attempts.GET("", hm.attemptHandler.ListAttempts)
			attempts.GET("/:id", hm.attemptHandler.GetAttempt)
			attempts.GET("/:id/details", hm.attemptHandler.GetAttemptWithDetails)
			attempts.POST("/:id/resume", hm.attemptHandler.ResumeAttempt)
			attempts.POST("/:id/answer", hm.attemptHandler.SubmitAnswer)
			attempts.GET("/:id/time-remaining", hm.attemptHandler.GetTimeRemaining)
			attempts.POST("/:id/extend", hm.attemptHandler.ExtendTime)
			attempts.POST("/:id/timeout", hm.attemptHandler.HandleTimeout)
			attempts.GET("/:id/is-active", hm.attemptHandler.IsAttemptActive)

			// Assessment-specific routes
			attempts.GET("/current/:assessment_id", hm.attemptHandler.GetCurrentAttempt)
			attempts.GET("/can-start/:assessment_id", hm.attemptHandler.CanStartAttempt)
			attempts.GET("/count/:assessment_id", hm.attemptHandler.GetAttemptCount)
			attempts.GET("/assessment/:assessment_id", hm.attemptHandler.GetAttemptsByAssessment)
			attempts.GET("/stats/:assessment_id", hm.attemptHandler.GetAttemptStats)

			// Student-specific routes - Teachers and Admins only (students should use /students/me/attempts)
			attempts.GET("/student/:student_id", hm.authMiddleware.RequireRoleMiddleware(models.RoleTeacher, models.RoleAdmin), hm.attemptHandler.GetAttemptsByStudent)
		}

		// Grading routes - Teachers, Proctors and Admins only
		grading := v1.Group("/grading")
		grading.Use(hm.authMiddleware.RequireRoleMiddleware(models.RoleTeacher, models.RoleProctor, models.RoleAdmin))
		{
			// Manual grading
			grading.POST("/answers/:answer_id", hm.gradingHandler.GradeAnswer)
			grading.POST("/answers/batch", hm.gradingHandler.GradeMultipleAnswers)
			grading.POST("/attempts/:attempt_id", hm.gradingHandler.GradeAttempt)

			// Auto grading
			grading.POST("/answers/:answer_id/auto", hm.gradingHandler.AutoGradeAnswer)
			grading.POST("/attempts/:attempt_id/auto", hm.gradingHandler.AutoGradeAttempt)
			grading.POST("/assessments/:assessment_id/auto", hm.gradingHandler.AutoGradeAssessment)

			// Grading utilities
			grading.POST("/calculate-score", hm.gradingHandler.CalculateScore)
			grading.POST("/generate-feedback", hm.gradingHandler.GenerateFeedback)

			// Re-grading
			grading.POST("/questions/:question_id/regrade", hm.gradingHandler.ReGradeQuestion)
			grading.POST("/assessments/:assessment_id/regrade", hm.gradingHandler.ReGradeAssessment)

			// Grading overview
			grading.GET("/assessments/:assessment_id/overview", hm.gradingHandler.GetGradingOverview)
		}

		// Dashboard routes - Teachers and Admins only
		dashboard := v1.Group("/dashboard")
		dashboard.Use(hm.authMiddleware.RequireRoleMiddleware(models.RoleTeacher, models.RoleAdmin))
		{
			dashboard.GET("/stats", hm.dashboardHandler.GetDashboardStats)
			dashboard.GET("/activity-trends", hm.dashboardHandler.GetActivityTrends)
			dashboard.GET("/recent-activities", hm.dashboardHandler.GetRecentActivities)
			dashboard.GET("/question-distribution", hm.dashboardHandler.GetQuestionDistribution)
			dashboard.GET("/performance-by-subject", hm.dashboardHandler.GetPerformanceBySubject)
		}

		// Student routes - Students only
		students := v1.Group("/students")
		students.Use(hm.authMiddleware.RequireRoleMiddleware(models.RoleStudent))
		{
			students.GET("/me/stats", hm.studentHandler.GetStudentStats)
			students.GET("/me/assessments", hm.studentHandler.GetStudentAssessments)
			students.GET("/me/assessments/:id", hm.studentHandler.GetStudentAssessmentDetail)
			students.GET("/me/attempts", hm.studentHandler.GetStudentAttempts)
		}
	}

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "assessment-service",
		})
	})
}
