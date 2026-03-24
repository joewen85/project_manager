package handler

import (
	"net/http"
	"time"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
)

type projectRequest struct {
	Code          string `json:"code" binding:"required"`
	Name          string `json:"name" binding:"required"`
	Description   string `json:"description"`
	StartAt       string `json:"startAt"`
	EndAt         string `json:"endAt"`
	UserIDs       []uint `json:"userIds"`
	DepartmentIDs []uint `json:"departmentIds"`
}

func parseTimeOrNil(value string) *time.Time {
	if value == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	return &t
}

func (h *Handler) ListProjects(c *gin.Context) {
	var projects []model.Project
	if err := h.DB.Preload("Users").Preload("Departments").Find(&projects).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, projects)
}

func (h *Handler) CreateProject(c *gin.Context) {
	var req projectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	item := model.Project{
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		StartAt:     parseTimeOrNil(req.StartAt),
		EndAt:       parseTimeOrNil(req.EndAt),
	}
	if err := h.DB.Create(&item).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	if len(req.UserIDs) > 0 {
		var users []model.User
		h.DB.Where("id IN ?", req.UserIDs).Find(&users)
		h.DB.Model(&item).Association("Users").Replace(&users)
	}
	if len(req.DepartmentIDs) > 0 {
		var departments []model.Department
		h.DB.Where("id IN ?", req.DepartmentIDs).Find(&departments)
		h.DB.Model(&item).Association("Departments").Replace(&departments)
	}

	h.DB.Preload("Users").Preload("Departments").First(&item, item.ID)
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) ProjectDetail(c *gin.Context) {
	var project model.Project
	if err := h.DB.
		Preload("Users").
		Preload("Departments").
		Preload("Tasks.Assignees").
		Where("id = ?", c.Param("id")).
		First(&project).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "项目不存在"})
		return
	}
	c.JSON(http.StatusOK, project)
}
