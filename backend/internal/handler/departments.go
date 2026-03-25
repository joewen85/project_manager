package handler

import (
	"net/http"
	"strings"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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
		respondDBError(c, http.StatusInternalServerError, "QUERY_DEPARTMENTS_FAILED", err)
		return
	}
	if err := query.Preload("Users").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_DEPARTMENTS_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, pageResult[model.Department]{List: items, Total: total, Page: page, PageSize: pageSize})
}

func (h *Handler) CreateDepartment(c *gin.Context) {
	var req departmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}

	item := model.Department{Name: req.Name, Description: req.Description}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		if len(req.UserIDs) > 0 {
			users, err := findUsersByIDs(tx, req.UserIDs)
			if err != nil {
				return err
			}
			if err := replaceAssociation(tx, &item, "Users", &users); err != nil {
				return err
			}
		}
		if err := tx.Preload("Users").First(&item, item.ID).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "departments", "create", item.ID, true, auditDetailf("创建部门(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_DEPARTMENT_FAILED", err)
		return
	}

	c.JSON(http.StatusCreated, item)
}

func (h *Handler) UpdateDepartment(c *gin.Context) {
	var req departmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}

	var item model.Department
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "DEPARTMENT_NOT_FOUND", "部门不存在")
		return
	}
	item.Name = req.Name
	item.Description = req.Description
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&item).Error; err != nil {
			return err
		}

		users, err := findUsersByIDs(tx, req.UserIDs)
		if err != nil {
			return err
		}
		if err := replaceAssociation(tx, &item, "Users", &users); err != nil {
			return err
		}
		if err := tx.Preload("Users").First(&item, item.ID).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "departments", "update", item.ID, true, auditDetailf("更新部门(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "UPDATE_DEPARTMENT_FAILED", err)
		return
	}

	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteDepartment(c *gin.Context) {
	var item model.Department
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "DEPARTMENT_NOT_FOUND", "部门不存在")
		return
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := clearAssociation(tx, &item, "Users"); err != nil {
			return err
		}
		if err := clearAssociation(tx, &item, "Projects"); err != nil {
			return err
		}
		if err := tx.Delete(&item).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "departments", "delete", item.ID, true, auditDetailf("删除部门(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusInternalServerError, "DELETE_DEPARTMENT_FAILED", err)
		return
	}

	respondMessage(c, http.StatusOK, "DEPARTMENT_DELETED", "删除成功")
}
