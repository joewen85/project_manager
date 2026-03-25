package handler

import (
	"net/http"
	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *Handler) currentUserIsAdmin(c *gin.Context) bool {
	uid := c.GetUint("userId")
	if uid == 0 {
		return false
	}

	var count int64
	err := h.DB.Table("user_roles").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("user_roles.user_id = ? AND roles.name = ?", uid, "admin").
		Count(&count).Error
	if err == nil {
		return count > 0
	}
	return false
}

func (h *Handler) scopeTasksQuery(c *gin.Context, query *gorm.DB) *gorm.DB {
	if h.currentUserIsAdmin(c) {
		return query
	}
	uid := c.GetUint("userId")
	return query.Where(
		"tasks.creator_id = ? OR EXISTS (SELECT 1 FROM task_users tu WHERE tu.task_id = tasks.id AND tu.user_id = ?)",
		uid,
		uid,
	)
}

func (h *Handler) scopeProjectsQuery(c *gin.Context, query *gorm.DB) *gorm.DB {
	if h.currentUserIsAdmin(c) {
		return query
	}
	uid := c.GetUint("userId")
	return query.Where(
		`EXISTS (SELECT 1 FROM project_users pu WHERE pu.project_id = projects.id AND pu.user_id = ?)
		OR EXISTS (
			SELECT 1
			FROM tasks t
			WHERE t.project_id = projects.id
			  AND (
				t.creator_id = ?
				OR EXISTS (SELECT 1 FROM task_users tu WHERE tu.task_id = t.id AND tu.user_id = ?)
			  )
		)`,
		uid,
		uid,
		uid,
	)
}

func (h *Handler) ensureProjectVisible(c *gin.Context, projectID string) bool {
	query := h.scopeProjectsQuery(c, h.DB.Model(&model.Project{})).Where("projects.id = ?", projectID)
	var count int64
	if err := query.Count(&count).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "QUERY_PROJECT_SCOPE_FAILED", err.Error())
		return false
	}
	if count == 0 {
		respondError(c, http.StatusNotFound, "PROJECT_NOT_FOUND", "项目不存在")
		return false
	}
	return true
}
