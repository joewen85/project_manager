package handler

import (
	"fmt"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
)

func (h *Handler) writeAudit(c *gin.Context, module, action string, targetID uint, success bool, detail string) {
	uid, _ := c.Get("userId")
	userID, _ := uid.(uint)

	log := model.AuditLog{
		UserID:    userID,
		Module:    module,
		Action:    action,
		TargetID:  targetID,
		Method:    c.Request.Method,
		Path:      c.FullPath(),
		Success:   success,
		Detail:    detail,
		ClientIP:  c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	}

	if err := h.DB.Create(&log).Error; err != nil {
		fmt.Printf("audit log write failed: %v\n", err)
	}
}

func (h *Handler) ListAuditLogs(c *gin.Context) {
	page, pageSize := parsePage(c)
	query := h.DB.Model(&model.AuditLog{})

	if module := c.Query("module"); module != "" {
		query = query.Where("module = ?", module)
	}
	if action := c.Query("action"); action != "" {
		query = query.Where("action = ?", action)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	var items []model.AuditLog
	if err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		c.JSON(500, gin.H{"message": err.Error()})
		return
	}

	c.JSON(200, pageResult[model.AuditLog]{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}
