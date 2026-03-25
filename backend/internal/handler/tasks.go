package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type taskRequest struct {
	TaskNo      string `json:"taskNo"`
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
	Progress    int    `json:"progress"`
	StartAt     string `json:"startAt"`
	EndAt       string `json:"endAt"`
	ProjectID   uint   `json:"projectId" binding:"required"`
	ParentID    *uint  `json:"parentId"`
	AssigneeIDs []uint `json:"assigneeIds"`
}

func normalizeStatus(status string) model.TaskStatus {
	switch model.TaskStatus(status) {
	case model.TaskQueued, model.TaskProcessing, model.TaskCompleted:
		return model.TaskStatus(status)
	default:
		return model.TaskPending
	}
}

func normalizePriority(priority string) model.TaskPriority {
	switch model.TaskPriority(priority) {
	case model.TaskPriorityMedium, model.TaskPriorityLow:
		return model.TaskPriority(priority)
	default:
		return model.TaskPriorityHigh
	}
}

func prioritySortClause(order string) string {
	switch strings.ToLower(strings.TrimSpace(order)) {
	case "medium":
		return "CASE tasks.priority WHEN 'medium' THEN 0 WHEN 'high' THEN 1 WHEN '' THEN 1 WHEN 'low' THEN 2 ELSE 1 END, tasks.created_at desc"
	case "low", "asc":
		return "CASE tasks.priority WHEN 'low' THEN 0 WHEN 'medium' THEN 1 WHEN 'high' THEN 2 WHEN '' THEN 2 ELSE 2 END, tasks.created_at desc"
	case "high", "desc":
		fallthrough
	default:
		return "CASE tasks.priority WHEN 'high' THEN 0 WHEN '' THEN 0 WHEN 'medium' THEN 1 WHEN 'low' THEN 2 ELSE 0 END, tasks.created_at desc"
	}
}

func parseTaskSort(c *gin.Context) string {
	sortBy := strings.TrimSpace(c.Query("sortBy"))
	if sortBy == "priority" {
		return prioritySortClause(c.Query("sortOrder"))
	}
	return parseSort(c, "tasks.id desc", map[string]string{
		"taskNo":    "tasks.task_no",
		"title":     "tasks.title",
		"status":    "tasks.status",
		"progress":  "tasks.progress",
		"startAt":   "tasks.start_at",
		"endAt":     "tasks.end_at",
		"createdAt": "tasks.created_at",
	})
}

func generateTaskNo() string {
	return "TASK-" + uuid.NewString()[0:8]
}

func diffUserIDs(newIDs []uint, oldIDs []uint) (added []uint, removed []uint) {
	oldSet := map[uint]struct{}{}
	for _, id := range oldIDs {
		oldSet[id] = struct{}{}
	}
	newSet := map[uint]struct{}{}
	for _, id := range newIDs {
		newSet[id] = struct{}{}
		if _, ok := oldSet[id]; !ok {
			added = append(added, id)
		}
	}
	for _, id := range oldIDs {
		if _, ok := newSet[id]; !ok {
			removed = append(removed, id)
		}
	}
	return added, removed
}

type taskAssigneeOption struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

func (h *Handler) ListTasks(c *gin.Context) {
	page, pageSize := parsePage(c)
	var tasks []model.Task
	query := h.DB.Model(&model.Task{}).Preload("Assignees").Preload("Creator")
	query = h.scopeTasksQuery(c, query)
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
		respondDBError(c, http.StatusInternalServerError, "QUERY_TASKS_FAILED", err)
		return
	}
	orderBy := parseTaskSort(c)
	if err := query.Order(orderBy).Offset((page - 1) * pageSize).Limit(pageSize).Find(&tasks).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_TASKS_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, pageResult[model.Task]{List: tasks, Total: total, Page: page, PageSize: pageSize})
}

func (h *Handler) TaskAssigneeOptions(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("keyword"))
	pageSize := 100
	if value := strings.TrimSpace(c.Query("pageSize")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			if parsed > 100 {
				parsed = 100
			}
			pageSize = parsed
		}
	}

	query := h.DB.Model(&model.User{}).Select("id, name, username, email")
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR username LIKE ? OR email LIKE ?", like, like, like)
	}

	var users []taskAssigneeOption
	if err := query.Order("name asc").Limit(pageSize).Find(&users).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_TASK_ASSIGNEE_OPTIONS_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (h *Handler) CreateTask(c *gin.Context) {
	var req taskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
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
		Priority:    normalizePriority(req.Priority),
		Progress:    req.Progress,
		StartAt:     startAt,
		EndAt:       endAt,
		CreatorID:   creatorID,
		ProjectID:   req.ProjectID,
		ParentID:    req.ParentID,
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}

		if len(req.AssigneeIDs) > 0 {
			users, err := findUsersByIDs(tx, req.AssigneeIDs)
			if err != nil {
				return err
			}
			if err := replaceAssociation(tx, &item, "Assignees", &users); err != nil {
				return err
			}
			if err := h.createNotificationsWithDB(tx, req.AssigneeIDs, "任务已指派给你", "任务 "+item.TaskNo+" - "+item.Title+" 已分配给你", "tasks", item.ID); err != nil {
				return err
			}
		}
		if err := h.triggerFailpoint("tasks.create.after_assignees"); err != nil {
			return err
		}

		if err := tx.Preload("Assignees").Preload("Creator").First(&item, item.ID).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "tasks", "create", item.ID, true, auditDetailf("创建任务(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_TASK_FAILED", err)
		return
	}

	c.JSON(http.StatusCreated, item)
}

func (h *Handler) UpdateTask(c *gin.Context) {
	var req taskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
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
	item.Priority = normalizePriority(req.Priority)
	item.Progress = req.Progress
	item.StartAt = startAt
	item.EndAt = endAt
	item.ProjectID = req.ProjectID
	item.ParentID = req.ParentID

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		var oldAssignees []model.User
		if err := tx.Model(&item).Association("Assignees").Find(&oldAssignees); err != nil {
			return err
		}
		oldIDs := make([]uint, 0, len(oldAssignees))
		for _, user := range oldAssignees {
			oldIDs = append(oldIDs, user.ID)
		}

		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		users, err := findUsersByIDs(tx, req.AssigneeIDs)
		if err != nil {
			return err
		}
		if err := replaceAssociation(tx, &item, "Assignees", &users); err != nil {
			return err
		}
		added, removed := diffUserIDs(req.AssigneeIDs, oldIDs)
		if err := h.createNotificationsWithDB(tx, added, "你被加入任务执行人", "任务 "+item.TaskNo+" - "+item.Title+" 已将你设为执行人", "tasks", item.ID); err != nil {
			return err
		}
		if err := h.createNotificationsWithDB(tx, removed, "你已被移出任务执行人", "任务 "+item.TaskNo+" - "+item.Title+" 已将你移出执行人", "tasks", item.ID); err != nil {
			return err
		}
		if err := h.triggerFailpoint("tasks.update.after_assignees"); err != nil {
			return err
		}
		if err := tx.Preload("Assignees").Preload("Creator").First(&item, item.ID).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "tasks", "update", item.ID, true, auditDetailf("更新任务(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "UPDATE_TASK_FAILED", err)
		return
	}

	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteTask(c *gin.Context) {
	var item model.Task
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "TASK_NOT_FOUND", "任务不存在")
		return
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := clearAssociation(tx, &item, "Assignees"); err != nil {
			return err
		}
		if err := tx.Delete(&item).Error; err != nil {
			return err
		}
		if err := h.triggerFailpoint("tasks.delete.after_delete"); err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "tasks", "delete", item.ID, true, auditDetailf("删除任务(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusInternalServerError, "DELETE_TASK_FAILED", err)
		return
	}

	respondMessage(c, http.StatusOK, "TASK_DELETED", "删除成功")
}

func (h *Handler) TaskTree(c *gin.Context) {
	projectID := c.Param("id")
	if !h.ensureProjectVisible(c, projectID) {
		return
	}
	var roots []model.Task
	if err := h.DB.
		Preload("Children.Children.Children").
		Preload("Assignees").
		Where("project_id = ? AND parent_id IS NULL", projectID).
		Find(&roots).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_TASK_TREE_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, roots)
}

func (h *Handler) Gantt(c *gin.Context) {
	projectID := c.Param("id")
	if !h.ensureProjectVisible(c, projectID) {
		return
	}
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
		respondDBError(c, http.StatusInternalServerError, "QUERY_GANTT_FAILED", err)
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
	h.DB.Joins("JOIN task_users tu ON tu.task_id = tasks.id").Where("tu.user_id = ? AND creator_id <> ?", uid, uid).Find(&out.MyParticipate)

	c.JSON(http.StatusOK, out)
}
