package handler

import (
	"net/http"

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

func (h *Handler) ListUsers(c *gin.Context) {
	var users []model.User
	if err := h.DB.Preload("Roles").Preload("Departments").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, users)
}

func (h *Handler) CreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	hash, err := util.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "密码加密失败"})
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
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
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
	c.JSON(http.StatusCreated, user)
}
