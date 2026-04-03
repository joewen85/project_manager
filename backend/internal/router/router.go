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
		authGroup.POST("/uploads", h.UploadFile)

		rbac := authGroup.Group("/rbac")
		rbac.Use(middleware.RequirePermission(h.DB, "rbac.manage"))
		{
			rbac.GET("/permissions", h.ListPermissions)
			rbac.POST("/permissions", h.CreatePermission)
			rbac.PUT("/permissions/:id", h.UpdatePermission)
			rbac.DELETE("/permissions/:id", h.DeletePermission)
			rbac.GET("/roles", h.ListRoles)
			rbac.POST("/roles", h.CreateRole)
			rbac.PUT("/roles/:id", h.UpdateRole)
			rbac.DELETE("/roles/:id", h.DeleteRole)
		}

		users := authGroup.Group("/users")
		users.Use(middleware.RequirePermission(h.DB, "users.read"))
		{
			users.GET("", h.ListUsers)
			users.POST("", middleware.RequirePermission(h.DB, "users.write"), h.CreateUser)
			users.PUT("/:id", middleware.RequirePermission(h.DB, "users.write"), h.UpdateUser)
			users.DELETE("/:id", middleware.RequirePermission(h.DB, "users.write"), h.DeleteUser)
		}

		departments := authGroup.Group("/departments")
		departments.Use(middleware.RequirePermission(h.DB, "departments.read"))
		{
			departments.GET("", h.ListDepartments)
			departments.POST("", middleware.RequirePermission(h.DB, "departments.write"), h.CreateDepartment)
			departments.PUT("/:id", middleware.RequirePermission(h.DB, "departments.write"), h.UpdateDepartment)
			departments.DELETE("/:id", middleware.RequirePermission(h.DB, "departments.write"), h.DeleteDepartment)
		}

		tags := authGroup.Group("/tags")
		tags.Use(middleware.RequirePermission(h.DB, "tags.read"))
		{
			tags.GET("", h.ListTags)
			tags.GET("/:id", h.GetTag)
			tags.POST("", middleware.RequirePermission(h.DB, "tags.write"), h.CreateTag)
			tags.PUT("/:id", middleware.RequirePermission(h.DB, "tags.write"), h.UpdateTag)
			tags.DELETE("/:id", middleware.RequirePermission(h.DB, "tags.write"), h.DeleteTag)
		}

		projects := authGroup.Group("/projects")
		projects.Use(middleware.RequirePermission(h.DB, "projects.read"))
		{
			projects.GET("", h.ListProjects)
			projects.GET("/export", h.ExportProjectsCSV)
			projects.GET("/editor-options", h.ProjectEditorOptions)
			projects.GET("/gantt-portfolio", h.GanttPortfolio)
			projects.POST("", middleware.RequirePermission(h.DB, "projects.write"), h.CreateProject)
			projects.PUT("/:id", middleware.RequirePermission(h.DB, "projects.write"), h.UpdateProject)
			projects.DELETE("/:id", middleware.RequirePermission(h.DB, "projects.write"), h.DeleteProject)
			projects.GET("/:id/gantt", h.Gantt)
			projects.POST("/:id/gantt/auto-resolve", middleware.RequirePermission(h.DB, "projects.write"), h.AutoResolveProjectDependencies)
			projects.GET("/:id/task-tree", h.TaskTree)
			projects.GET("/:id", h.ProjectDetail)
		}

		tasks := authGroup.Group("/tasks")
		tasks.Use(middleware.RequirePermission(h.DB, "tasks.read"))
		{
			tasks.GET("", h.ListTasks)
			tasks.GET("/export", h.ExportTasksCSV)
			tasks.GET("/assignee-options", h.TaskAssigneeOptions)
			tasks.POST("", middleware.RequirePermission(h.DB, "tasks.write"), h.CreateTask)
			tasks.PUT("/:id", middleware.RequirePermission(h.DB, "tasks.write"), h.UpdateTask)
			tasks.PUT("/:id/dependencies", middleware.RequirePermission(h.DB, "tasks.write"), h.UpdateTaskDependencies)
			tasks.PATCH("/:id/schedule", middleware.RequirePermission(h.DB, "tasks.write"), h.UpdateTaskSchedule)
			tasks.DELETE("/:id", middleware.RequirePermission(h.DB, "tasks.write"), h.DeleteTask)
			tasks.GET("/progress-list", h.ProgressList)
			tasks.GET("/me", h.MyTasks)
		}

		stats := authGroup.Group("/stats")
		stats.Use(middleware.RequirePermission(h.DB, "stats.read"))
		{
			stats.GET("/dashboard", h.DashboardStats)
		}

		notifications := authGroup.Group("/notifications")
		notifications.Use(middleware.RequirePermission(h.DB, "notifications.read"))
		{
			notifications.GET("", h.ListNotifications)
			notifications.GET("/unread-count", h.UnreadNotificationCount)
			notifications.PATCH("/:id/read", h.MarkNotificationRead)
			notifications.PATCH("/read-all", h.MarkAllNotificationsRead)
		}

		audit := authGroup.Group("/audit")
		audit.Use(middleware.RequirePermission(h.DB, "audit.read"))
		{
			audit.GET("/logs", h.ListAuditLogs)
		}
	}

	return r
}
