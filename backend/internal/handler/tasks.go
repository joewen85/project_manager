package handler

import (
	"net/http"
	"strings"
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
	page, pageSize := parsePage(c)
	var tasks []model.Task
	query := h.DB.Model(&model.Task{}).Preload("Assignees").Preload("Creator")
	if projectID := c.Query("projectId"); projectID != "" {
		query = query.Where("project_id = ?", projectID)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("task_no LIKE ? OR title LIKE ? OR description LIKE ?", like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	if err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pageResult[model.Task]{List: tasks, Total: total, Page: page, PageSize: pageSize})
}

func (h *Handler) CreateTask(c *gin.Context) {
	var req taskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	startAt, err := parseRFC3339(req.StartAt)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_START_AT", "startAt 必须是 RFC3339 时间格式")
		return
	}
	endAt, err := parseRFC3339(req.EndAt)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_END_AT", "endAt 必须是 RFC3339 时间格式")
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
		StartAt:     startAt,
		EndAt:       endAt,
		CreatorID:   creatorID,
		ProjectID:   req.ProjectID,
		ParentID:    req.ParentID,
	}
	if err := h.DB.Create(&item).Error; err != nil {
		respondError(c, http.StatusBadRequest, "CREATE_TASK_FAILED", err.Error())
		return
	}

	if len(req.AssigneeIDs) > 0 {
		var users []model.User
		h.DB.Where("id IN ?", req.AssigneeIDs).Find(&users)
		h.DB.Model(&item).Association("Assignees").Replace(&users)
	}

	h.DB.Preload("Assignees").Preload("Creator").First(&item, item.ID)
	h.writeAudit(c, "tasks", "create", item.ID, true, "创建任务")
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) UpdateTask(c *gin.Context) {
	var req taskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	var item model.Task
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "TASK_NOT_FOUND", "任务不存在")
		return
	}
	startAt, err := parseRFC3339(req.StartAt)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_START_AT", "startAt 必须是 RFC3339 时间格式")
		return
	}
	endAt, err := parseRFC3339(req.EndAt)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_END_AT", "endAt 必须是 RFC3339 时间格式")
		return
	}

	if req.TaskNo != "" {
		item.TaskNo = req.TaskNo
	}
	item.Title = req.Title
	item.Description = req.Description
	item.Status = normalizeStatus(req.Status)
	item.Progress = req.Progress
	item.StartAt = startAt
	item.EndAt = endAt
	item.ProjectID = req.ProjectID
	item.ParentID = req.ParentID

	if err := h.DB.Save(&item).Error; err != nil {
		respondError(c, http.StatusBadRequest, "UPDATE_TASK_FAILED", err.Error())
		return
	}
	var users []model.User
	if len(req.AssigneeIDs) > 0 {
		h.DB.Where("id IN ?", req.AssigneeIDs).Find(&users)
	}
	h.DB.Model(&item).Association("Assignees").Replace(&users)
	h.DB.Preload("Assignees").Preload("Creator").First(&item, item.ID)
	h.writeAudit(c, "tasks", "update", item.ID, true, "更新任务")
	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteTask(c *gin.Context) {
	var item model.Task
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "TASK_NOT_FOUND", "任务不存在")
		return
	}
	if err := h.DB.Model(&item).Association("Assignees").Clear(); err != nil {
		respondError(c, http.StatusInternalServerError, "CLEAR_TASK_ASSIGNEES_FAILED", err.Error())
		return
	}
	if err := h.DB.Delete(&item).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "DELETE_TASK_FAILED", err.Error())
		return
	}
	h.writeAudit(c, "tasks", "delete", item.ID, true, "删除任务")
	respondMessage(c, http.StatusOK, "TASK_DELETED", "删除成功")
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
