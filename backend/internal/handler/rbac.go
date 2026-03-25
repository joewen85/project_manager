package handler

import (
	"net/http"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type roleRequest struct {
	Name          string `json:"name" binding:"required"`
	Description   string `json:"description"`
	PermissionIDs []uint `json:"permissionIds"`
}

type permissionRequest struct {
	Code        string `json:"code" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

func (h *Handler) ListPermissions(c *gin.Context) {
	var items []model.Permission
	if err := h.DB.Find(&items).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PERMISSIONS_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) ListRoles(c *gin.Context) {
	var items []model.Role
	if err := h.DB.Preload("Permissions").Find(&items).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_ROLES_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateRole(c *gin.Context) {
	var req roleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	item := model.Role{Name: req.Name, Description: req.Description}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		permissions, err := findPermissionsByIDs(tx, req.PermissionIDs)
		if err != nil {
			return err
		}
		if err := replaceAssociation(tx, &item, "Permissions", &permissions); err != nil {
			return err
		}
		if err := h.triggerFailpoint("rbac.create_role.after_permissions"); err != nil {
			return err
		}
		if err := tx.Preload("Permissions").First(&item, item.ID).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "rbac", "create_role", item.ID, true, auditDetailf("创建角色(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_ROLE_FAILED", err)
		return
	}

	c.JSON(http.StatusCreated, item)
}

func (h *Handler) UpdateRole(c *gin.Context) {
	var req roleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	var item model.Role
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "ROLE_NOT_FOUND", "角色不存在")
		return
	}
	item.Name = req.Name
	item.Description = req.Description
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		permissions, err := findPermissionsByIDs(tx, req.PermissionIDs)
		if err != nil {
			return err
		}
		if err := replaceAssociation(tx, &item, "Permissions", &permissions); err != nil {
			return err
		}
		if err := h.triggerFailpoint("rbac.update_role.after_permissions"); err != nil {
			return err
		}
		if err := tx.Preload("Permissions").First(&item, item.ID).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "rbac", "update_role", item.ID, true, auditDetailf("更新角色(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "UPDATE_ROLE_FAILED", err)
		return
	}

	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteRole(c *gin.Context) {
	var item model.Role
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "ROLE_NOT_FOUND", "角色不存在")
		return
	}
	if item.Name == "admin" {
		respondError(c, http.StatusBadRequest, "SYSTEM_ROLE", "内置角色不能删除")
		return
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := clearAssociation(tx, &item, "Permissions"); err != nil {
			return err
		}
		if err := tx.Delete(&item).Error; err != nil {
			return err
		}
		if err := h.triggerFailpoint("rbac.delete_role.after_delete"); err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "rbac", "delete_role", item.ID, true, auditDetailf("删除角色(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusInternalServerError, "DELETE_ROLE_FAILED", err)
		return
	}

	respondMessage(c, http.StatusOK, "ROLE_DELETED", "删除成功")
}

func (h *Handler) CreatePermission(c *gin.Context) {
	var req permissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	item := model.Permission{Code: req.Code, Name: req.Name, Description: req.Description}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		if err := h.triggerFailpoint("rbac.create_permission.after_create"); err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "rbac", "create_permission", item.ID, true, auditDetailf("创建权限(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_PERMISSION_FAILED", err)
		return
	}

	c.JSON(http.StatusCreated, item)
}

func (h *Handler) UpdatePermission(c *gin.Context) {
	var req permissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	var item model.Permission
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "PERMISSION_NOT_FOUND", "权限不存在")
		return
	}
	item.Code = req.Code
	item.Name = req.Name
	item.Description = req.Description
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		if err := h.triggerFailpoint("rbac.update_permission.after_save"); err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "rbac", "update_permission", item.ID, true, auditDetailf("更新权限(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "UPDATE_PERMISSION_FAILED", err)
		return
	}

	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeletePermission(c *gin.Context) {
	var item model.Permission
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "PERMISSION_NOT_FOUND", "权限不存在")
		return
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&item).Error; err != nil {
			return err
		}
		if err := h.triggerFailpoint("rbac.delete_permission.after_delete"); err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "rbac", "delete_permission", item.ID, true, auditDetailf("删除权限(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusInternalServerError, "DELETE_PERMISSION_FAILED", err)
		return
	}

	respondMessage(c, http.StatusOK, "PERMISSION_DELETED", "删除成功")
}
