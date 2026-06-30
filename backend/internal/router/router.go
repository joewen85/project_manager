package router

import (
	"strings"
	"time"

	"project-manager/backend/internal/config"
	"project-manager/backend/internal/handler"
	"project-manager/backend/internal/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func New(cfg config.Config, h *handler.Handler) *gin.Engine {
	r := gin.Default()
	r.StaticFS("/static/uploads", gin.Dir(cfg.UploadDir, false))
	corsOrigins := strings.TrimSpace(cfg.CORSAllowOrigins)
	if corsOrigins == "" {
		corsOrigins = "http://localhost:5173"
	}
	parts := strings.Split(corsOrigins, ",")
	allowedOrigins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin != "" {
			allowedOrigins = append(allowedOrigins, origin)
		}
	}
	r.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	api := r.Group("/api/v1")
	{
		api.POST("/auth/login", h.Login)
		api.GET("/notifications/ws", h.NotificationSocket)
	}

	authGroup := api.Group("")
	authGroup.Use(middleware.JWT(cfg.JWTSecret, h.DB))
	{
		authGroup.GET("/auth/profile", h.Profile)
		authGroup.POST("/auth/change-password", h.ChangePassword)
		authGroup.POST("/uploads", middleware.RequirePermission(h.DB, "uploads.create"), h.UploadFile)

		rbac := authGroup.Group("/rbac")
		{
			rbac.GET("/permissions", middleware.RequirePermission(h.DB, "rbac.read"), h.ListPermissions)
			rbac.POST("/permissions", middleware.RequirePermission(h.DB, "rbac.create"), h.CreatePermission)
			rbac.PUT("/permissions/:id", middleware.RequirePermission(h.DB, "rbac.update"), h.UpdatePermission)
			rbac.DELETE("/permissions/:id", middleware.RequirePermission(h.DB, "rbac.delete"), h.DeletePermission)
			rbac.GET("/roles", middleware.RequirePermission(h.DB, "rbac.read"), h.ListRoles)
			rbac.POST("/roles", middleware.RequirePermission(h.DB, "rbac.create"), h.CreateRole)
			rbac.PUT("/roles/:id", middleware.RequirePermission(h.DB, "rbac.update"), h.UpdateRole)
			rbac.DELETE("/roles/:id", middleware.RequirePermission(h.DB, "rbac.delete"), h.DeleteRole)
		}

		users := authGroup.Group("/users")
		{
			users.GET("", middleware.RequirePermission(h.DB, "users.read"), h.ListUsers)
			users.POST("", middleware.RequirePermission(h.DB, "users.create"), h.CreateUser)
			users.PUT("/:id", middleware.RequirePermission(h.DB, "users.update"), h.UpdateUser)
			users.DELETE("/:id", middleware.RequirePermission(h.DB, "users.delete"), h.DeleteUser)
		}

		departments := authGroup.Group("/departments")
		{
			departments.GET("", middleware.RequirePermission(h.DB, "departments.read"), h.ListDepartments)
			departments.POST("", middleware.RequirePermission(h.DB, "departments.create"), h.CreateDepartment)
			departments.PUT("/:id", middleware.RequirePermission(h.DB, "departments.update"), h.UpdateDepartment)
			departments.DELETE("/:id", middleware.RequirePermission(h.DB, "departments.delete"), h.DeleteDepartment)
		}

		tags := authGroup.Group("/tags")
		{
			tags.GET("", middleware.RequirePermission(h.DB, "tags.read"), h.ListTags)
			tags.GET("/:id", middleware.RequirePermission(h.DB, "tags.read"), h.GetTag)
			tags.POST("", middleware.RequirePermission(h.DB, "tags.create"), h.CreateTag)
			tags.PUT("/:id", middleware.RequirePermission(h.DB, "tags.update"), h.UpdateTag)
			tags.DELETE("/:id", middleware.RequirePermission(h.DB, "tags.delete"), h.DeleteTag)
		}

		projects := authGroup.Group("/projects")
		{
			projects.GET("", middleware.RequirePermission(h.DB, "projects.read"), h.ListProjects)
			projects.GET("/export", middleware.RequirePermission(h.DB, "projects.read"), h.ExportProjectsCSV)
			projects.GET("/editor-options", middleware.RequirePermission(h.DB, "projects.read"), h.ProjectEditorOptions)
			projects.GET("/gantt-portfolio", middleware.RequirePermission(h.DB, "projects.read"), h.GanttPortfolio)
			projects.POST("", middleware.RequirePermission(h.DB, "projects.create"), h.CreateProject)
			projects.PUT("/:id", middleware.RequirePermission(h.DB, "projects.update"), h.UpdateProject)
			projects.DELETE("/:id", middleware.RequirePermission(h.DB, "projects.delete"), h.DeleteProject)
			projects.GET("/:id/gantt", middleware.RequirePermission(h.DB, "projects.read"), h.Gantt)
			projects.POST("/:id/gantt/auto-resolve", middleware.RequirePermission(h.DB, "projects.update"), h.AutoResolveProjectDependencies)
			projects.GET("/:id/task-tree", middleware.RequirePermission(h.DB, "projects.read"), h.TaskTree)
			projects.GET("/:id/critical-path", middleware.RequirePermission(h.DB, "baselines.read"), h.ProjectCriticalPath)
			projects.GET("/:id", middleware.RequirePermission(h.DB, "projects.read"), h.ProjectDetail)
		}

		baselines := authGroup.Group("/project-baselines")
		{
			baselines.GET("", middleware.RequirePermission(h.DB, "baselines.read"), h.ListProjectBaselines)
			baselines.POST("", middleware.RequirePermission(h.DB, "baselines.create"), h.CreateProjectBaseline)
			baselines.GET("/:id", middleware.RequirePermission(h.DB, "baselines.read"), h.ProjectBaselineDetail)
			baselines.DELETE("/:id", middleware.RequirePermission(h.DB, "baselines.delete"), h.DeleteProjectBaseline)
		}

		registers := authGroup.Group("/project-registers")
		{
			registers.GET("", middleware.RequirePermission(h.DB, "registers.read"), h.ListProjectRegisters)
			registers.POST("", middleware.RequirePermission(h.DB, "registers.create"), h.CreateProjectRegister)
			registers.GET("/:id", middleware.RequirePermission(h.DB, "registers.read"), h.ProjectRegisterDetail)
			registers.PUT("/:id", middleware.RequirePermission(h.DB, "registers.update"), h.UpdateProjectRegister)
			registers.DELETE("/:id", middleware.RequirePermission(h.DB, "registers.delete"), h.DeleteProjectRegister)
			registers.GET("/:id/activities", middleware.RequirePermission(h.DB, "registers.read"), h.ListProjectRegisterActivities)
		}

		tasks := authGroup.Group("/tasks")
		{
			tasks.GET("", middleware.RequirePermission(h.DB, "tasks.read"), h.ListTasks)
			tasks.GET("/export", middleware.RequirePermission(h.DB, "tasks.read"), h.ExportTasksCSV)
			tasks.GET("/assignee-options", middleware.RequirePermission(h.DB, "tasks.read"), h.TaskAssigneeOptions)
			tasks.GET("/calendar", middleware.RequirePermission(h.DB, "tasks.read"), h.TaskCalendar)
			tasks.GET("/calendar.ics", middleware.RequirePermission(h.DB, "tasks.read"), h.ExportTaskCalendarICS)
			tasks.POST("", middleware.RequirePermission(h.DB, "tasks.create"), h.CreateTask)
			tasks.GET("/:id/comments", middleware.RequirePermission(h.DB, "comments.read"), h.ListTaskComments)
			tasks.POST("/:id/comments", middleware.RequirePermission(h.DB, "comments.create"), h.CreateTaskComment)
			tasks.DELETE("/:id/comments/:commentId", middleware.RequirePermission(h.DB, "comments.delete"), h.DeleteTaskComment)
			tasks.GET("/:id/activities", middleware.RequirePermission(h.DB, "comments.read"), h.ListTaskActivities)
			tasks.PATCH("/:id/progress", middleware.RequirePermission(h.DB, "tasks.read"), h.UpdateTaskProgress)
			tasks.PATCH("/:id/status", middleware.RequirePermission(h.DB, "tasks.read"), h.UpdateTaskStatus)
			tasks.PATCH("/:id/complete", middleware.RequirePermission(h.DB, "tasks.read"), h.CompleteTask)
			tasks.PUT("/:id", middleware.RequirePermission(h.DB, "tasks.update"), h.UpdateTask)
			tasks.PUT("/:id/dependencies", middleware.RequirePermission(h.DB, "tasks.update"), h.UpdateTaskDependencies)
			tasks.PATCH("/:id/schedule", middleware.RequirePermission(h.DB, "tasks.update"), h.UpdateTaskSchedule)
			tasks.DELETE("/:id", middleware.RequirePermission(h.DB, "tasks.delete"), h.DeleteTask)
			tasks.GET("/progress-list", middleware.RequirePermission(h.DB, "tasks.read"), h.ProgressList)
			tasks.GET("/me", middleware.RequirePermission(h.DB, "tasks.read"), h.MyTasks)
		}

		requests := authGroup.Group("/requests")
		{
			requests.GET("", middleware.RequirePermission(h.DB, "requests.read"), h.ListWorkRequests)
			requests.POST("", middleware.RequirePermission(h.DB, "requests.create"), h.CreateWorkRequest)
			requests.PATCH("/:id/review", middleware.RequirePermission(h.DB, "requests.update"), h.ReviewWorkRequest)
			requests.POST("/:id/apply-change", middleware.RequirePermission(h.DB, "requests.update"), h.ApplyWorkRequestChange)
			requests.POST("/:id/convert-task", middleware.RequirePermission(h.DB, "requests.update"), h.ConvertWorkRequestToTask)
		}

		templates := authGroup.Group("/project-templates")
		{
			templates.GET("", middleware.RequirePermission(h.DB, "templates.read"), h.ListProjectTemplates)
			templates.POST("", middleware.RequirePermission(h.DB, "templates.create"), h.CreateProjectTemplate)
			templates.PUT("/:id", middleware.RequirePermission(h.DB, "templates.update"), h.UpdateProjectTemplate)
			templates.DELETE("/:id", middleware.RequirePermission(h.DB, "templates.delete"), h.DeleteProjectTemplate)
			templates.POST("/:id/create-project", middleware.RequirePermission(h.DB, "projects.create"), h.CreateProjectFromTemplate)
		}

		reports := authGroup.Group("/reports")
		{
			reports.GET("", middleware.RequirePermission(h.DB, "reports.read"), h.ListSavedReports)
			reports.POST("", middleware.RequirePermission(h.DB, "reports.create"), h.CreateSavedReport)
			reports.GET("/:id", middleware.RequirePermission(h.DB, "reports.read"), h.SavedReportDetail)
			reports.PUT("/:id", middleware.RequirePermission(h.DB, "reports.update"), h.UpdateSavedReport)
			reports.DELETE("/:id", middleware.RequirePermission(h.DB, "reports.delete"), h.DeleteSavedReport)
		}

		sprints := authGroup.Group("/sprints")
		{
			sprints.GET("", middleware.RequirePermission(h.DB, "sprints.read"), h.ListSprints)
			sprints.POST("", middleware.RequirePermission(h.DB, "sprints.create"), h.CreateSprint)
			sprints.GET("/:id", middleware.RequirePermission(h.DB, "sprints.read"), h.SprintDetail)
			sprints.PUT("/:id", middleware.RequirePermission(h.DB, "sprints.update"), h.UpdateSprint)
			sprints.DELETE("/:id", middleware.RequirePermission(h.DB, "sprints.delete"), h.DeleteSprint)
			sprints.POST("/:id/tasks", middleware.RequirePermission(h.DB, "sprints.update"), h.AddSprintTasks)
			sprints.DELETE("/:id/tasks/:taskId", middleware.RequirePermission(h.DB, "sprints.update"), h.RemoveSprintTask)
		}

		webhooks := authGroup.Group("/webhooks")
		{
			webhooks.GET("", middleware.RequirePermission(h.DB, "webhooks.read"), h.ListWebhookSubscriptions)
			webhooks.POST("", middleware.RequirePermission(h.DB, "webhooks.create"), h.CreateWebhookSubscription)
			webhooks.GET("/deliveries", middleware.RequirePermission(h.DB, "webhooks.read"), h.ListWebhookDeliveries)
			webhooks.POST("/deliveries/:id/retry", middleware.RequirePermission(h.DB, "webhooks.update"), h.RetryWebhookDelivery)
			webhooks.GET("/:id", middleware.RequirePermission(h.DB, "webhooks.read"), h.WebhookSubscriptionDetail)
			webhooks.PUT("/:id", middleware.RequirePermission(h.DB, "webhooks.update"), h.UpdateWebhookSubscription)
			webhooks.DELETE("/:id", middleware.RequirePermission(h.DB, "webhooks.delete"), h.DeleteWebhookSubscription)
		}

		apiTokens := authGroup.Group("/api-tokens")
		{
			apiTokens.GET("", middleware.RequirePermission(h.DB, "api_tokens.read"), h.ListAPITokens)
			apiTokens.POST("", middleware.RequirePermission(h.DB, "api_tokens.create"), h.CreateAPIToken)
			apiTokens.GET("/permission-options", middleware.RequirePermission(h.DB, "api_tokens.read"), h.ListAPITokenPermissionOptions)
			apiTokens.GET("/:id", middleware.RequirePermission(h.DB, "api_tokens.read"), h.APITokenDetail)
			apiTokens.PUT("/:id", middleware.RequirePermission(h.DB, "api_tokens.update"), h.UpdateAPIToken)
			apiTokens.DELETE("/:id", middleware.RequirePermission(h.DB, "api_tokens.delete"), h.DeleteAPIToken)
		}

		automations := authGroup.Group("/automation-rules")
		{
			automations.GET("", middleware.RequirePermission(h.DB, "automations.read"), h.ListAutomationRules)
			automations.POST("", middleware.RequirePermission(h.DB, "automations.create"), h.CreateAutomationRule)
			automations.GET("/logs", middleware.RequirePermission(h.DB, "automations.read"), h.ListAutomationExecutionLogs)
			automations.PUT("/:id", middleware.RequirePermission(h.DB, "automations.update"), h.UpdateAutomationRule)
			automations.DELETE("/:id", middleware.RequirePermission(h.DB, "automations.delete"), h.DeleteAutomationRule)
			automations.POST("/:id/run", middleware.RequirePermission(h.DB, "automations.update"), h.RunAutomationRule)
		}

		stats := authGroup.Group("/stats")
		{
			stats.GET("/dashboard", middleware.RequirePermission(h.DB, "stats.read"), h.DashboardStats)
			stats.GET("/project-health", middleware.RequirePermission(h.DB, "stats.read"), h.ProjectHealth)
			stats.GET("/member-workload", middleware.RequirePermission(h.DB, "stats.read"), h.MemberWorkload)
		}

		notifications := authGroup.Group("/notifications")
		{
			notifications.GET("", middleware.RequirePermission(h.DB, "notifications.read"), h.ListNotifications)
			notifications.GET("/unread-count", middleware.RequirePermission(h.DB, "notifications.read"), h.UnreadNotificationCount)
			notifications.PATCH("/:id/read", middleware.RequirePermission(h.DB, "notifications.update"), h.MarkNotificationRead)
			notifications.PATCH("/read-all", middleware.RequirePermission(h.DB, "notifications.update"), h.MarkAllNotificationsRead)
		}

		audit := authGroup.Group("/audit")
		{
			audit.GET("/logs", middleware.RequirePermission(h.DB, "audit.read"), h.ListAuditLogs)
		}
	}

	return r
}
