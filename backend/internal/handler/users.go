package handler

import (
	"net/http"
	"strings"

	"project-manager/backend/internal/model"
	"project-manager/backend/internal/util"

	"github.com/gin-gonic/gin"
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
		respondError(c, http.StatusInternalServerError, "QUERY_USERS_FAILED", err.Error())
		return
	}
	if err := query.Preload("Roles").Preload("Departments").Offset((page - 1) * pageSize).Limit(pageSize).Find(&users).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "QUERY_USERS_FAILED", err.Error())
		return
	}
	c.JSON(http.StatusOK, pageResult[model.User]{List: users, Total: total, Page: page, PageSize: pageSize})
}

func (h *Handler) CreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
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
	if err := h.DB.Create(&user).Error; err != nil {
		respondError(c, http.StatusBadRequest, "CREATE_USER_FAILED", err.Error())
		return
	}

	if len(req.RoleIDs) > 0 {
		var roles []model.Role
		h.DB.Where("id IN ?", req.RoleIDs).Find(&roles)
		h.DB.Model(&user).Association("Roles").Replace(&roles)
	}
	if len(req.DepartmentIDs) > 0 {
		var departments []model.Department
		h.DB.Where("id IN ?", req.DepartmentIDs).Find(&departments)
		h.DB.Model(&user).Association("Departments").Replace(&departments)
	}

	h.DB.Preload("Roles").Preload("Departments").First(&user, user.ID)
	h.writeAudit(c, "users", "create", user.ID, true, "创建用户")
	c.JSON(http.StatusCreated, user)
}

func (h *Handler) UpdateUser(c *gin.Context) {
	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
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

	if err := h.DB.Save(&user).Error; err != nil {
		respondError(c, http.StatusBadRequest, "UPDATE_USER_FAILED", err.Error())
		return
	}

	var roles []model.Role
	if len(req.RoleIDs) > 0 {
		h.DB.Where("id IN ?", req.RoleIDs).Find(&roles)
	}
	h.DB.Model(&user).Association("Roles").Replace(&roles)

	var departments []model.Department
	if len(req.DepartmentIDs) > 0 {
		h.DB.Where("id IN ?", req.DepartmentIDs).Find(&departments)
	}
	h.DB.Model(&user).Association("Departments").Replace(&departments)

	h.DB.Preload("Roles").Preload("Departments").First(&user, user.ID)
	h.writeAudit(c, "users", "update", user.ID, true, "更新用户")
	c.JSON(http.StatusOK, user)
}

func (h *Handler) DeleteUser(c *gin.Context) {
	var user model.User
	if err := h.DB.First(&user, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "USER_NOT_FOUND", "用户不存在")
		return
	}
	if err := h.DB.Model(&user).Association("Roles").Clear(); err != nil {
		respondError(c, http.StatusInternalServerError, "CLEAR_USER_ROLES_FAILED", err.Error())
		return
	}
	if err := h.DB.Model(&user).Association("Departments").Clear(); err != nil {
		respondError(c, http.StatusInternalServerError, "CLEAR_USER_DEPARTMENTS_FAILED", err.Error())
		return
	}
	if err := h.DB.Delete(&user).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "DELETE_USER_FAILED", err.Error())
		return
	}
	h.writeAudit(c, "users", "delete", user.ID, true, "删除用户")
	respondMessage(c, http.StatusOK, "USER_DELETED", "删除成功")
}
