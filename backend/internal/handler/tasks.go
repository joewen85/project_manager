package handler

import (
	"net/http"
	"time"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type taskRequest struct {
	TaskNo      string    `json:"taskNo"`
	Title       string    `json:"title" binding:"required"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Progress    int       `json:"progress"`
	StartAt     string    `json:"startAt"`
	EndAt       string    `json:"endAt"`
	ProjectID   uint      `json:"projectId" binding:"required"`
	ParentID    *uint     `json:"parentId"`
	AssigneeIDs []uint    `json:"assigneeIds"`
}

func normalizeStatus(status string) model.TaskStatus {
	switch model.TaskStatus(status) {
	case model.TaskQueued, model.TaskProcessing, model.TaskCompleted:
		return model.TaskStatus(status)
	default:
		return model.TaskPending
	}
}

func generateTaskNo() string {
	return "TASK-" + uuid.NewString()[0:8]
}

func (h *Handler) ListTasks(c *gin.Context) {
	var tasks []model.Task
	query := h.DB.Preload("Assignees").Preload("Creator")
	if projectID := c.Query("projectId"); projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tasks)
}

func (h *Handler) CreateTask(c *gin.Context) {
	var req taskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	creatorID := c.GetUint("userId")
	taskNo := req.TaskNo
	if taskNo == "" {
		taskNo = generateTaskNo()
	}

	item := model.Task{
		TaskNo:      taskNo,
		Title:       req.Title,
		Description: req.Description,
		Status:      normalizeStatus(req.Status),
		Progress:    req.Progress,
		StartAt:     parseTimeOrNil(req.StartAt),
		EndAt:       parseTimeOrNil(req.EndAt),
		CreatorID:   creatorID,
		ProjectID:   req.ProjectID,
		ParentID:    req.ParentID,
	}
	if err := h.DB.Create(&item).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	if len(req.AssigneeIDs) > 0 {
		var users []model.User
		h.DB.Where("id IN ?", req.AssigneeIDs).Find(&users)
		h.DB.Model(&item).Association("Assignees").Replace(&users)
	}

	h.DB.Preload("Assignees").Preload("Creator").First(&item, item.ID)
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) TaskTree(c *gin.Context) {
	projectID := c.Param("projectId")
	var roots []model.Task
	if err := h.DB.
		Preload("Children.Children.Children").
		Preload("Assignees").
		Where("project_id = ? AND parent_id IS NULL", projectID).
		Find(&roots).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, roots)
}

func (h *Handler) Gantt(c *gin.Context) {
	projectID := c.Param("projectId")
	type ganttItem struct {
		ID       uint       `json:"id"`
		TaskNo   string     `json:"taskNo"`
		Title    string     `json:"title"`
		StartAt  *time.Time `json:"startAt"`
		EndAt    *time.Time `json:"endAt"`
		Progress int        `json:"progress"`
		ParentID *uint      `json:"parentId"`
		Status   string     `json:"status"`
	}
	var result []ganttItem
	if err := h.DB.Model(&model.Task{}).Select("id, task_no, title, start_at, end_at, progress, parent_id, status").Where("project_id = ?", projectID).Scan(&result).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) MyTasks(c *gin.Context) {
	uid := c.GetUint("userId")
	type result struct {
		MyTasks       []model.Task `json:"myTasks"`
		MyCreated     []model.Task `json:"myCreated"`
		MyParticipate []model.Task `json:"myParticipate"`
	}
	out := result{}

	h.DB.Joins("JOIN task_users tu ON tu.task_id = tasks.id").Where("tu.user_id = ?", uid).Find(&out.MyTasks)
	h.DB.Where("creator_id = ?", uid).Find(&out.MyCreated)
	h.DB.Joins("JOIN task_users tu ON tu.task_id = tasks.id").Where("tu.user_id = ?", uid).Find(&out.MyParticipate)

	c.JSON(http.StatusOK, out)
}
