package handler

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	auditLogDefaultPageSize = 20
	auditLogRetentionMonths = 6
)

func auditDetailf(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

func auditRetentionCutoff(now time.Time) time.Time {
	return now.AddDate(0, -auditLogRetentionMonths, 0)
}

func (h *Handler) writeAudit(c *gin.Context, module, action string, targetID uint, success bool, detail string) {
	if err := h.writeAuditWithDB(c, h.DB, module, action, targetID, success, detail); err != nil {
		log.Printf("audit log write failed: %v", err)
	}
}

func (h *Handler) writeAuditWithDB(c *gin.Context, db *gorm.DB, module, action string, targetID uint, success bool, detail string) error {
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

	return db.Create(&log).Error
}

func (h *Handler) DeleteExpiredAuditLogs(now time.Time) (int64, error) {
	cutoff := auditRetentionCutoff(now)
	result := h.DB.Where("created_at < ?", cutoff).Delete(&model.AuditLog{})
	return result.RowsAffected, result.Error
}

func (h *Handler) ListAuditLogs(c *gin.Context) {
	page, pageSize := parsePage(c)
	if _, hasPageSize := c.GetQuery("pageSize"); !hasPageSize {
		pageSize = auditLogDefaultPageSize
	}
	query := h.DB.Model(&model.AuditLog{})

	if module := c.Query("module"); module != "" {
		query = query.Where("module = ?", module)
	}
	if action := c.Query("action"); action != "" {
		query = query.Where("action = ?", action)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_AUDIT_FAILED", err)
		return
	}

	var items []model.AuditLog
	if err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_AUDIT_FAILED", err)
		return
	}

	c.JSON(http.StatusOK, pageResult[model.AuditLog]{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}
