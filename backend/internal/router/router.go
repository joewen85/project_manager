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

		rbac := authGroup.Group("/rbac")
		rbac.Use(middleware.RequirePermission("rbac.manage"))
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
		users.Use(middleware.RequirePermission("users.read"))
		{
			users.GET("", h.ListUsers)
			users.POST("", middleware.RequirePermission("users.write"), h.CreateUser)
			users.PUT("/:id", middleware.RequirePermission("users.write"), h.UpdateUser)
			users.DELETE("/:id", middleware.RequirePermission("users.write"), h.DeleteUser)
		}

		departments := authGroup.Group("/departments")
		departments.Use(middleware.RequirePermission("departments.read"))
		{
			departments.GET("", h.ListDepartments)
			departments.POST("", middleware.RequirePermission("departments.write"), h.CreateDepartment)
			departments.PUT("/:id", middleware.RequirePermission("departments.write"), h.UpdateDepartment)
			departments.DELETE("/:id", middleware.RequirePermission("departments.write"), h.DeleteDepartment)
		}

		projects := authGroup.Group("/projects")
		projects.Use(middleware.RequirePermission("projects.read"))
		{
			projects.GET("", h.ListProjects)
			projects.POST("", middleware.RequirePermission("projects.write"), h.CreateProject)
			projects.PUT("/:id", middleware.RequirePermission("projects.write"), h.UpdateProject)
			projects.DELETE("/:id", middleware.RequirePermission("projects.write"), h.DeleteProject)
			projects.GET("/:id/gantt", h.Gantt)
			projects.GET("/:id/task-tree", h.TaskTree)
			projects.GET("/:id", h.ProjectDetail)
		}

		tasks := authGroup.Group("/tasks")
		tasks.Use(middleware.RequirePermission("tasks.read"))
		{
			tasks.GET("", h.ListTasks)
			tasks.POST("", middleware.RequirePermission("tasks.write"), h.CreateTask)
			tasks.PUT("/:id", middleware.RequirePermission("tasks.write"), h.UpdateTask)
			tasks.DELETE("/:id", middleware.RequirePermission("tasks.write"), h.DeleteTask)
			tasks.GET("/progress-list", h.ProgressList)
			tasks.GET("/me", h.MyTasks)
		}

		stats := authGroup.Group("/stats")
		stats.Use(middleware.RequirePermission("stats.read"))
		{
			stats.GET("/dashboard", h.DashboardStats)
		}

		audit := authGroup.Group("/audit")
		audit.Use(middleware.RequirePermission("audit.read"))
		{
			audit.GET("/logs", h.ListAuditLogs)
		}
	}

	return r
}
