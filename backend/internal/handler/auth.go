package handler

import (
	"net/http"

	"project-manager/backend/internal/auth"
	"project-manager/backend/internal/model"
	"project-manager/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type changePasswordRequest struct {
	OldPassword     string `json:"oldPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required,min=6"`
	ConfirmPassword string `json:"confirmPassword" binding:"required"`
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
	if !user.IsActive {
		respondError(c, http.StatusForbidden, "USER_DISABLED", "账号已禁用")
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

func (h *Handler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	if req.NewPassword != req.ConfirmPassword {
		respondError(c, http.StatusBadRequest, "PASSWORD_CONFIRM_MISMATCH", "两次输入的新密码不一致")
		return
	}
	if req.OldPassword == req.NewPassword {
		respondError(c, http.StatusBadRequest, "PASSWORD_NOT_CHANGED", "新密码不能与旧密码一致")
		return
	}

	uid := c.GetUint("userId")
	var user model.User
	if err := h.DB.Where("id = ?", uid).First(&user).Error; err != nil {
		respondError(c, http.StatusNotFound, "USER_NOT_FOUND", "用户不存在")
		return
	}
	if !util.VerifyPassword(user.Password, req.OldPassword) {
		h.writeAudit(c, "auth", "change_password", user.ID, false, auditDetailf("用户修改个人密码失败(id=%d): 旧密码错误", user.ID))
		respondError(c, http.StatusBadRequest, "OLD_PASSWORD_INVALID", "旧密码错误")
		return
	}

	hash, err := util.HashPassword(req.NewPassword)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "PASSWORD_HASH_FAILED", "密码加密失败")
		return
	}

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&user).Update("password", hash).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "auth", "change_password", user.ID, true, auditDetailf("用户修改个人密码(id=%d)", user.ID))
	}); err != nil {
		respondDBError(c, http.StatusInternalServerError, "CHANGE_PASSWORD_FAILED", err)
		return
	}

	respondMessage(c, http.StatusOK, "PASSWORD_CHANGED", "密码修改成功")
}
