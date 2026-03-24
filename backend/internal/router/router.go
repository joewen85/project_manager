package router

import (
	"project-manager/backend/internal/config"
	"project-manager/backend/internal/handler"
	"project-manager/backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

func New(cfg config.Config, h *handler.Handler) *gin.Engine {
	r := gin.Default()
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
			rbac.GET("/roles", h.ListRoles)
			rbac.POST("/roles", h.CreateRole)
		}

		users := authGroup.Group("/users")
		users.Use(middleware.RequirePermission("users.read"))
		{
			users.GET("", h.ListUsers)
			users.POST("", middleware.RequirePermission("users.write"), h.CreateUser)
		}

		departments := authGroup.Group("/departments")
		departments.Use(middleware.RequirePermission("departments.read"))
		{
			departments.GET("", h.ListDepartments)
			departments.POST("", middleware.RequirePermission("departments.write"), h.CreateDepartment)
		}

		projects := authGroup.Group("/projects")
		projects.Use(middleware.RequirePermission("projects.read"))
		{
			projects.GET("", h.ListProjects)
			projects.GET("/:id", h.ProjectDetail)
			projects.POST("", middleware.RequirePermission("projects.write"), h.CreateProject)
			projects.GET("/:projectId/gantt", h.Gantt)
			projects.GET("/:projectId/task-tree", h.TaskTree)
		}

		tasks := authGroup.Group("/tasks")
		tasks.Use(middleware.RequirePermission("tasks.read"))
		{
			tasks.GET("", h.ListTasks)
			tasks.POST("", middleware.RequirePermission("tasks.write"), h.CreateTask)
			tasks.GET("/progress-list", h.ProgressList)
			tasks.GET("/me", h.MyTasks)
		}

		stats := authGroup.Group("/stats")
		stats.Use(middleware.RequirePermission("stats.read"))
		{
			stats.GET("/dashboard", h.DashboardStats)
		}
	}

	return r
}
