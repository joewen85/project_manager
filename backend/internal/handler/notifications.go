package handler

import (
	"net/http"
	"strings"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
)

func uniqueUint(values []uint) []uint {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[uint]struct{}, len(values))
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (h *Handler) createNotifications(userIDs []uint, title, content, module string, targetID uint) {
	ids := uniqueUint(userIDs)
	if len(ids) == 0 {
		return
	}
	items := make([]model.Notification, 0, len(ids))
	for _, userID := range ids {
		items = append(items, model.Notification{
			UserID:   userID,
			Title:    title,
			Content:  content,
			Module:   module,
			TargetID: targetID,
			IsRead:   false,
		})
	}
	_ = h.DB.Create(&items).Error
}

func (h *Handler) ListNotifications(c *gin.Context) {
	uid := c.GetUint("userId")
	page, pageSize := parsePage(c)
	var items []model.Notification
	query := h.DB.Model(&model.Notification{}).Where("user_id = ?", uid)
	if isRead := c.Query("isRead"); isRead == "true" {
		query = query.Where("is_read = ?", true)
	}
	if isRead := c.Query("isRead"); isRead == "false" {
		query = query.Where("is_read = ?", false)
	}
	if module := strings.TrimSpace(c.Query("module")); module != "" {
		query = query.Where("module = ?", module)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("title LIKE ? OR content LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "QUERY_NOTIFICATIONS_FAILED", err.Error())
		return
	}
	if err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "QUERY_NOTIFICATIONS_FAILED", err.Error())
		return
	}
	c.JSON(http.StatusOK, pageResult[model.Notification]{List: items, Total: total, Page: page, PageSize: pageSize})
}

func (h *Handler) UnreadNotificationCount(c *gin.Context) {
	uid := c.GetUint("userId")
	var count int64
	if err := h.DB.Model(&model.Notification{}).Where("user_id = ? AND is_read = ?", uid, false).Count(&count).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "QUERY_NOTIFICATION_COUNT_FAILED", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": count})
}

func (h *Handler) MarkNotificationRead(c *gin.Context) {
	uid := c.GetUint("userId")
	var item model.Notification
	if err := h.DB.Where("id = ? AND user_id = ?", c.Param("id"), uid).First(&item).Error; err != nil {
		respondError(c, http.StatusNotFound, "NOTIFICATION_NOT_FOUND", "通知不存在")
		return
	}
	item.IsRead = true
	if err := h.DB.Save(&item).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "MARK_NOTIFICATION_READ_FAILED", err.Error())
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) MarkAllNotificationsRead(c *gin.Context) {
	uid := c.GetUint("userId")
	if err := h.DB.Model(&model.Notification{}).Where("user_id = ? AND is_read = ?", uid, false).Update("is_read", true).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "MARK_ALL_NOTIFICATIONS_READ_FAILED", err.Error())
		return
	}
	respondMessage(c, http.StatusOK, "NOTIFICATIONS_MARKED_READ", "已全部标记为已读")
}
