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
	h.writeAudit(c, "rbac", "create_role", item.ID, true, "创建角色")
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) UpdateRole(c *gin.Context) {
	var req roleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	var item model.Role
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "角色不存在"})
		return
	}
	item.Name = req.Name
	item.Description = req.Description
	if err := h.DB.Save(&item).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	var permissions []model.Permission
	if len(req.PermissionIDs) > 0 {
		h.DB.Where("id IN ?", req.PermissionIDs).Find(&permissions)
	}
	h.DB.Model(&item).Association("Permissions").Replace(&permissions)
	h.DB.Preload("Permissions").First(&item, item.ID)
	h.writeAudit(c, "rbac", "update_role", item.ID, true, "更新角色")
	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteRole(c *gin.Context) {
	var item model.Role
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "角色不存在"})
		return
	}
	if item.Name == "admin" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "内置角色不能删除"})
		return
	}
	if err := h.DB.Model(&item).Association("Permissions").Clear(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	if err := h.DB.Delete(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	h.writeAudit(c, "rbac", "delete_role", item.ID, true, "删除角色")
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
