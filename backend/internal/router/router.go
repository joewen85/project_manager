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
	}

	authGroup := api.Group("")
	authGroup.Use(middleware.JWT(cfg.JWTSecret))
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
			projects.GET("/:id", middleware.RequirePermission(h.DB, "projects.read"), h.ProjectDetail)
		}

		tasks := authGroup.Group("/tasks")
		{
			tasks.GET("", middleware.RequirePermission(h.DB, "tasks.read"), h.ListTasks)
			tasks.GET("/export", middleware.RequirePermission(h.DB, "tasks.read"), h.ExportTasksCSV)
			tasks.GET("/assignee-options", middleware.RequirePermission(h.DB, "tasks.read"), h.TaskAssigneeOptions)
			tasks.POST("", middleware.RequirePermission(h.DB, "tasks.create"), h.CreateTask)
			tasks.PUT("/:id", middleware.RequirePermission(h.DB, "tasks.update"), h.UpdateTask)
			tasks.PUT("/:id/dependencies", middleware.RequirePermission(h.DB, "tasks.update"), h.UpdateTaskDependencies)
			tasks.PATCH("/:id/schedule", middleware.RequirePermission(h.DB, "tasks.update"), h.UpdateTaskSchedule)
			tasks.DELETE("/:id", middleware.RequirePermission(h.DB, "tasks.delete"), h.DeleteTask)
			tasks.GET("/progress-list", middleware.RequirePermission(h.DB, "tasks.read"), h.ProgressList)
			tasks.GET("/me", middleware.RequirePermission(h.DB, "tasks.read"), h.MyTasks)
		}

		stats := authGroup.Group("/stats")
		{
			stats.GET("/dashboard", middleware.RequirePermission(h.DB, "stats.read"), h.DashboardStats)
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
