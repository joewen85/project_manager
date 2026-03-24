package handler

import (
	"net/http"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
)

func (h *Handler) ProgressList(c *gin.Context) {
	type progressItem struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	var items []progressItem
	if err := h.DB.Model(&model.Task{}).Select("status, count(*) as count").Group("status").Scan(&items).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "QUERY_PROGRESS_FAILED", err.Error())
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) DashboardStats(c *gin.Context) {
	var userCount, projectCount, taskCount, doneCount int64
	h.DB.Model(&model.User{}).Count(&userCount)
	h.DB.Model(&model.Project{}).Count(&projectCount)
	h.DB.Model(&model.Task{}).Count(&taskCount)
	h.DB.Model(&model.Task{}).Where("status = ?", model.TaskCompleted).Count(&doneCount)

	completionRate := 0.0
	if taskCount > 0 {
		completionRate = float64(doneCount) / float64(taskCount)
	}

	c.JSON(http.StatusOK, gin.H{
		"users":          userCount,
		"projects":       projectCount,
		"tasks":          taskCount,
		"completedTasks": doneCount,
		"completionRate": completionRate,
	})
}
