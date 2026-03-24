package handler

import (
	"net/http"
	"strings"
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
	page, pageSize := parsePage(c)
	var projects []model.Project
	query := h.DB.Model(&model.Project{})
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("code LIKE ? OR name LIKE ? OR description LIKE ?", like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	if err := query.Preload("Users").Preload("Departments").Offset((page - 1) * pageSize).Limit(pageSize).Find(&projects).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pageResult[model.Project]{List: projects, Total: total, Page: page, PageSize: pageSize})
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
	h.writeAudit(c, "projects", "create", item.ID, true, "创建项目")
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) UpdateProject(c *gin.Context) {
	var req projectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	var item model.Project
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "项目不存在"})
		return
	}

	item.Code = req.Code
	item.Name = req.Name
	item.Description = req.Description
	item.StartAt = parseTimeOrNil(req.StartAt)
	item.EndAt = parseTimeOrNil(req.EndAt)
	if err := h.DB.Save(&item).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	var users []model.User
	if len(req.UserIDs) > 0 {
		h.DB.Where("id IN ?", req.UserIDs).Find(&users)
	}
	h.DB.Model(&item).Association("Users").Replace(&users)

	var departments []model.Department
	if len(req.DepartmentIDs) > 0 {
		h.DB.Where("id IN ?", req.DepartmentIDs).Find(&departments)
	}
	h.DB.Model(&item).Association("Departments").Replace(&departments)

	h.DB.Preload("Users").Preload("Departments").First(&item, item.ID)
	h.writeAudit(c, "projects", "update", item.ID, true, "更新项目")
	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteProject(c *gin.Context) {
	var item model.Project
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "项目不存在"})
		return
	}
	var taskCount int64
	h.DB.Model(&model.Task{}).Where("project_id = ?", item.ID).Count(&taskCount)
	if taskCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "请先删除或迁移项目下任务"})
		return
	}
	if err := h.DB.Model(&item).Association("Users").Clear(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	if err := h.DB.Model(&item).Association("Departments").Clear(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	if err := h.DB.Delete(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	h.writeAudit(c, "projects", "delete", item.ID, true, "删除项目")
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
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
