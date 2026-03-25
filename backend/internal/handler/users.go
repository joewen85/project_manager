package handler

import (
	"net/http"
	"strings"

	"project-manager/backend/internal/model"
	"project-manager/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type createUserRequest struct {
	Username      string `json:"username" binding:"required"`
	Name          string `json:"name" binding:"required"`
	Email         string `json:"email" binding:"required,email"`
	Password      string `json:"password" binding:"required,min=6"`
	RoleIDs       []uint `json:"roleIds"`
	DepartmentIDs []uint `json:"departmentIds"`
}

type updateUserRequest struct {
	Name          string `json:"name" binding:"required"`
	Email         string `json:"email" binding:"required,email"`
	Password      string `json:"password"`
	IsActive      *bool  `json:"isActive"`
	RoleIDs       []uint `json:"roleIds"`
	DepartmentIDs []uint `json:"departmentIds"`
}

func (h *Handler) ListUsers(c *gin.Context) {
	page, pageSize := parsePage(c)
	var users []model.User
	query := h.DB.Model(&model.User{})
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("username LIKE ? OR name LIKE ? OR email LIKE ?", like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_USERS_FAILED", err)
		return
	}
	if err := query.Preload("Roles").Preload("Departments").Offset((page - 1) * pageSize).Limit(pageSize).Find(&users).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_USERS_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, pageResult[model.User]{List: users, Total: total, Page: page, PageSize: pageSize})
}

func (h *Handler) CreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}

	hash, err := util.HashPassword(req.Password)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "PASSWORD_HASH_FAILED", "密码加密失败")
		return
	}

	user := model.User{
		Username: req.Username,
		Name:     req.Name,
		Email:    req.Email,
		Password: hash,
		IsActive: true,
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		if len(req.RoleIDs) > 0 {
			roles, err := findRolesByIDs(tx, req.RoleIDs)
			if err != nil {
				return err
			}
			if err := replaceAssociation(tx, &user, "Roles", &roles); err != nil {
				return err
			}
		}
		if len(req.DepartmentIDs) > 0 {
			departments, err := findDepartmentsByIDs(tx, req.DepartmentIDs)
			if err != nil {
				return err
			}
			if err := replaceAssociation(tx, &user, "Departments", &departments); err != nil {
				return err
			}
		}
		if err := tx.Preload("Roles").Preload("Departments").First(&user, user.ID).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "users", "create", user.ID, true, "创建用户")
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_USER_FAILED", err)
		return
	}

	c.JSON(http.StatusCreated, user)
}

func (h *Handler) UpdateUser(c *gin.Context) {
	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}

	var user model.User
	if err := h.DB.First(&user, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "USER_NOT_FOUND", "用户不存在")
		return
	}

	user.Name = req.Name
	user.Email = req.Email
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}
	if req.Password != "" {
		hash, err := util.HashPassword(req.Password)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "PASSWORD_HASH_FAILED", "密码加密失败")
			return
		}
		user.Password = hash
	}

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&user).Error; err != nil {
			return err
		}

		roles, err := findRolesByIDs(tx, req.RoleIDs)
		if err != nil {
			return err
		}
		if err := replaceAssociation(tx, &user, "Roles", &roles); err != nil {
			return err
		}

		departments, err := findDepartmentsByIDs(tx, req.DepartmentIDs)
		if err != nil {
			return err
		}
		if err := replaceAssociation(tx, &user, "Departments", &departments); err != nil {
			return err
		}

		if err := tx.Preload("Roles").Preload("Departments").First(&user, user.ID).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "users", "update", user.ID, true, auditDetailf("更新用户(id=%d)", user.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "UPDATE_USER_FAILED", err)
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *Handler) DeleteUser(c *gin.Context) {
	var user model.User
	if err := h.DB.First(&user, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "USER_NOT_FOUND", "用户不存在")
		return
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := clearAssociation(tx, &user, "Roles"); err != nil {
			return err
		}
		if err := clearAssociation(tx, &user, "Departments"); err != nil {
			return err
		}
		if err := tx.Delete(&user).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "users", "delete", user.ID, true, auditDetailf("删除用户(id=%d)", user.ID))
	}); err != nil {
		respondDBError(c, http.StatusInternalServerError, "DELETE_USER_FAILED", err)
		return
	}

	respondMessage(c, http.StatusOK, "USER_DELETED", "删除成功")
}
