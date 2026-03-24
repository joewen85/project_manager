package handler

import (
	"net/http"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
)

type roleRequest struct {
	Name          string `json:"name" binding:"required"`
	Description   string `json:"description"`
	PermissionIDs []uint `json:"permissionIds"`
}

func (h *Handler) ListPermissions(c *gin.Context) {
	var items []model.Permission
	if err := h.DB.Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) ListRoles(c *gin.Context) {
	var items []model.Role
	if err := h.DB.Preload("Permissions").Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateRole(c *gin.Context) {
	var req roleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	item := model.Role{Name: req.Name, Description: req.Description}
	if err := h.DB.Create(&item).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if len(req.PermissionIDs) > 0 {
		var permissions []model.Permission
		h.DB.Where("id IN ?", req.PermissionIDs).Find(&permissions)
		h.DB.Model(&item).Association("Permissions").Replace(&permissions)
	}
	h.DB.Preload("Permissions").First(&item, item.ID)
	c.JSON(http.StatusCreated, item)
}
