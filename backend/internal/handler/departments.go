package handler

import (
	"net/http"
	"strings"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
)

type departmentRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	UserIDs     []uint `json:"userIds"`
}

func (h *Handler) ListDepartments(c *gin.Context) {
	page, pageSize := parsePage(c)
	var items []model.Department
	query := h.DB.Model(&model.Department{})
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "QUERY_DEPARTMENTS_FAILED", err.Error())
		return
	}
	if err := query.Preload("Users").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "QUERY_DEPARTMENTS_FAILED", err.Error())
		return
	}
	c.JSON(http.StatusOK, pageResult[model.Department]{List: items, Total: total, Page: page, PageSize: pageSize})
}

func (h *Handler) CreateDepartment(c *gin.Context) {
	var req departmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	item := model.Department{Name: req.Name, Description: req.Description}
	if err := h.DB.Create(&item).Error; err != nil {
		respondError(c, http.StatusBadRequest, "CREATE_DEPARTMENT_FAILED", err.Error())
		return
	}
	if len(req.UserIDs) > 0 {
		var users []model.User
		h.DB.Where("id IN ?", req.UserIDs).Find(&users)
		h.DB.Model(&item).Association("Users").Replace(&users)
	}
	h.DB.Preload("Users").First(&item, item.ID)
	h.writeAudit(c, "departments", "create", item.ID, true, "创建部门")
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) UpdateDepartment(c *gin.Context) {
	var req departmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	var item model.Department
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "DEPARTMENT_NOT_FOUND", "部门不存在")
		return
	}
	item.Name = req.Name
	item.Description = req.Description
	if err := h.DB.Save(&item).Error; err != nil {
		respondError(c, http.StatusBadRequest, "UPDATE_DEPARTMENT_FAILED", err.Error())
		return
	}

	var users []model.User
	if len(req.UserIDs) > 0 {
		h.DB.Where("id IN ?", req.UserIDs).Find(&users)
	}
	h.DB.Model(&item).Association("Users").Replace(&users)
	h.DB.Preload("Users").First(&item, item.ID)
	h.writeAudit(c, "departments", "update", item.ID, true, "更新部门")
	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteDepartment(c *gin.Context) {
	var item model.Department
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "DEPARTMENT_NOT_FOUND", "部门不存在")
		return
	}
	if err := h.DB.Model(&item).Association("Users").Clear(); err != nil {
		respondError(c, http.StatusInternalServerError, "CLEAR_DEPARTMENT_USERS_FAILED", err.Error())
		return
	}
	if err := h.DB.Model(&item).Association("Projects").Clear(); err != nil {
		respondError(c, http.StatusInternalServerError, "CLEAR_DEPARTMENT_PROJECTS_FAILED", err.Error())
		return
	}
	if err := h.DB.Delete(&item).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "DELETE_DEPARTMENT_FAILED", err.Error())
		return
	}
	h.writeAudit(c, "departments", "delete", item.ID, true, "删除部门")
	respondMessage(c, http.StatusOK, "DEPARTMENT_DELETED", "删除成功")
}
