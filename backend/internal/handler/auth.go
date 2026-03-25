package handler

import (
	"net/http"

	"project-manager/backend/internal/auth"
	"project-manager/backend/internal/model"
	"project-manager/backend/internal/util"

	"github.com/gin-gonic/gin"
)

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}

	var user model.User
	if err := h.DB.Preload("Roles.Permissions").Where("username = ?", req.Username).First(&user).Error; err != nil {
		respondError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "用户名或密码错误")
		return
	}

	if !util.VerifyPassword(user.Password, req.Password) {
		respondError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "用户名或密码错误")
		return
	}

	permissionSet := map[string]struct{}{}
	for _, role := range user.Roles {
		for _, permission := range role.Permissions {
			permissionSet[permission.Code] = struct{}{}
		}
	}

	permissions := make([]string, 0, len(permissionSet))
	for code := range permissionSet {
		permissions = append(permissions, code)
	}

	token, err := auth.GenerateToken(h.Cfg.JWTSecret, user.ID, user.Username, permissions)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "TOKEN_GENERATE_FAILED", "token 生成失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":       token,
		"user":        user,
		"permissions": permissions,
	})
}

func (h *Handler) Profile(c *gin.Context) {
	uid := c.GetUint("userId")
	var user model.User
	if err := h.DB.Preload("Roles.Permissions").Preload("Departments").Where("id = ?", uid).First(&user).Error; err != nil {
		respondError(c, http.StatusNotFound, "USER_NOT_FOUND", "用户不存在")
		return
	}
	c.JSON(http.StatusOK, user)
}
