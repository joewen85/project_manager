package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type sprintRequest struct {
	Name          string  `json:"name" binding:"required"`
	Goal          string  `json:"goal"`
	Status        string  `json:"status"`
	StartAt       string  `json:"startAt"`
	EndAt         string  `json:"endAt"`
	CapacityHours float64 `json:"capacityHours"`
}

type sprintTaskRequest struct {
	TaskIDs []uint `json:"taskIds"`
}

type sprintResponse struct {
	model.Sprint
	TaskCount          int64        `json:"taskCount"`
	CompletedTaskCount int64        `json:"completedTaskCount"`
	CompletionRate     float64      `json:"completionRate"`
	Tasks              []model.Task `json:"tasks,omitempty"`
}

func normalizeSprintStatus(value string) (model.SprintStatus, bool) {
	switch model.SprintStatus(strings.TrimSpace(value)) {
	case model.SprintActive, model.SprintClosed:
		return model.SprintStatus(strings.TrimSpace(value)), true
	case model.SprintPlanned, "":
		return model.SprintPlanned, true
	default:
		return model.SprintPlanned, false
	}
}

func validateSprintRequest(req sprintRequest) (*model.Sprint, bool, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, false, fmt.Errorf("迭代名称不能为空")
	}
	if req.CapacityHours < 0 {
		return nil, false, fmt.Errorf("迭代容量不能小于 0")
	}
	status, ok := normalizeSprintStatus(req.Status)
	startAt, err := parseRFC3339(req.StartAt)
	if err != nil {
		return nil, false, fmt.Errorf("startAt 必须是 RFC3339 时间格式")
	}
	endAt, err := parseRFC3339(req.EndAt)
	if err != nil {
		return nil, false, fmt.Errorf("endAt 必须是 RFC3339 时间格式")
	}
	if startAt != nil && endAt != nil && endAt.Before(*startAt) {
		return nil, false, fmt.Errorf("结束时间必须晚于开始时间")
	}
	return &model.Sprint{
		Name:          name,
		Goal:          strings.TrimSpace(req.Goal),
		Status:        status,
		StartAt:       startAt,
		EndAt:         endAt,
		CapacityHours: req.CapacityHours,
	}, ok, nil
}

func uniqueTaskIDs(values []uint) []uint {
	seen := map[uint]struct{}{}
	out := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (h *Handler) scopeSprintsQuery(c *gin.Context, query *gorm.DB) *gorm.DB {
	if h.currentUserIsAdmin(c) {
		return query
	}
	uid := c.GetUint("userId")
	return query.Where(
		`sprints.created_by_id = ?
		OR EXISTS (
			SELECT 1
			FROM sprint_tasks st
			JOIN tasks t ON t.id = st.task_id
			WHERE st.sprint_id = sprints.id
			  AND (
				t.creator_id = ?
				OR EXISTS (SELECT 1 FROM task_users tu WHERE tu.task_id = t.id AND tu.user_id = ?)
				OR EXISTS (SELECT 1 FROM task_reviewers tr WHERE tr.task_id = t.id AND tr.user_id = ?)
			  )
		)`,
		uid,
		uid,
		uid,
		uid,
	)
}

func (h *Handler) ensureSprintReadable(c *gin.Context, id string) (*model.Sprint, bool) {
	var item model.Sprint
	query := h.scopeSprintsQuery(c, h.DB.Model(&model.Sprint{})).Preload("CreatedBy")
	if err := query.Where("sprints.id = ?", id).First(&item).Error; err != nil {
		respondError(c, http.StatusNotFound, "SPRINT_NOT_FOUND", "迭代不存在")
		return nil, false
	}
	return &item, true
}

func (h *Handler) ensureSprintWritable(c *gin.Context, id string) (*model.Sprint, bool) {
	item, ok := h.ensureSprintReadable(c, id)
	if !ok {
		return nil, false
	}
	if h.currentUserIsAdmin(c) || item.CreatedByID == c.GetUint("userId") {
		return item, true
	}
	respondError(c, http.StatusForbidden, "SPRINT_OWNER_REQUIRED", "只有迭代创建人或管理员可以更新迭代")
	return nil, false
}

func (h *Handler) sprintTaskBaseQuery(c *gin.Context, sprintID uint) *gorm.DB {
	query := h.DB.Model(&model.Task{}).
		Joins("JOIN sprint_tasks ON sprint_tasks.task_id = tasks.id AND sprint_tasks.sprint_id = ?", sprintID)
	return h.scopeTasksQuery(c, query)
}

func (h *Handler) sprintVisibleStats(c *gin.Context, sprintID uint) (int64, int64, error) {
	var total int64
	if err := h.sprintTaskBaseQuery(c, sprintID).Count(&total).Error; err != nil {
		return 0, 0, err
	}
	var completed int64
	if err := h.sprintTaskBaseQuery(c, sprintID).Where("tasks.status = ?", model.TaskCompleted).Count(&completed).Error; err != nil {
		return 0, 0, err
	}
	return total, completed, nil
}

func (h *Handler) loadSprintVisibleTasks(c *gin.Context, sprintID uint) ([]model.Task, error) {
	var tasks []model.Task
	query := h.sprintTaskBaseQuery(c, sprintID).
		Select("tasks.*, projects.name AS project_name").
		Joins("LEFT JOIN projects ON projects.id = tasks.project_id").
		Preload("Assignees").
		Preload("Reviewers").
		Preload("Creator").
		Preload("Dependencies").
		Preload("Tags", func(db *gorm.DB) *gorm.DB { return db.Order("tags.name asc") })
	if err := query.Order("tasks.updated_at desc, tasks.id desc").Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

func toSprintResponse(item model.Sprint, total int64, completed int64, tasks []model.Task) sprintResponse {
	completionRate := 0.0
	if total > 0 {
		completionRate = float64(completed) / float64(total) * 100
	}
	return sprintResponse{
		Sprint:             item,
		TaskCount:          total,
		CompletedTaskCount: completed,
		CompletionRate:     completionRate,
		Tasks:              tasks,
	}
}

func (h *Handler) visibleTaskIDs(c *gin.Context, taskIDs []uint) ([]uint, error) {
	if len(taskIDs) == 0 {
		return []uint{}, nil
	}
	var visibleIDs []uint
	query := h.scopeTasksQuery(c, h.DB.Model(&model.Task{}))
	if err := query.Where("tasks.id IN ?", taskIDs).Pluck("tasks.id", &visibleIDs).Error; err != nil {
		return nil, err
	}
	return visibleIDs, nil
}

func (h *Handler) ListSprints(c *gin.Context) {
	page, pageSize := parsePage(c)
	query := h.scopeSprintsQuery(c, h.DB.Model(&model.Sprint{}))
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("sprints.name LIKE ? OR sprints.goal LIKE ?", like, like)
	}
	if status, ok := normalizeSprintStatus(c.Query("status")); ok && strings.TrimSpace(c.Query("status")) != "" {
		query = query.Where("sprints.status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_SPRINTS_FAILED", err)
		return
	}
	var items []model.Sprint
	if err := query.Preload("CreatedBy").
		Order("sprints.start_at desc, sprints.id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_SPRINTS_FAILED", err)
		return
	}
	result := make([]sprintResponse, 0, len(items))
	for _, item := range items {
		taskCount, completedCount, err := h.sprintVisibleStats(c, item.ID)
		if err != nil {
			respondDBError(c, http.StatusInternalServerError, "QUERY_SPRINT_STATS_FAILED", err)
			return
		}
		result = append(result, toSprintResponse(item, taskCount, completedCount, nil))
	}
	c.JSON(http.StatusOK, pageResult[sprintResponse]{List: result, Total: total, Page: page, PageSize: pageSize})
}

func (h *Handler) SprintDetail(c *gin.Context) {
	item, ok := h.ensureSprintReadable(c, c.Param("id"))
	if !ok {
		return
	}
	tasks, err := h.loadSprintVisibleTasks(c, item.ID)
	if err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_SPRINT_TASKS_FAILED", err)
		return
	}
	completed := int64(0)
	for _, task := range tasks {
		if task.Status == model.TaskCompleted {
			completed++
		}
	}
	c.JSON(http.StatusOK, toSprintResponse(*item, int64(len(tasks)), completed, tasks))
}

func (h *Handler) CreateSprint(c *gin.Context) {
	var req sprintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	next, statusOK, err := validateSprintRequest(req)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_SPRINT", err.Error())
		return
	}
	if !statusOK {
		respondError(c, http.StatusBadRequest, "INVALID_SPRINT_STATUS", "迭代状态必须是 planned、active 或 closed")
		return
	}
	next.CreatedByID = c.GetUint("userId")
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(next).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "sprints", "create", next.ID, true, auditDetailf("创建迭代(id=%d)", next.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_SPRINT_FAILED", err)
		return
	}
	if err := h.DB.Preload("CreatedBy").First(next, next.ID).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_SPRINT_FAILED", err)
		return
	}
	c.JSON(http.StatusCreated, toSprintResponse(*next, 0, 0, nil))
}

func (h *Handler) UpdateSprint(c *gin.Context) {
	var req sprintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	item, ok := h.ensureSprintWritable(c, c.Param("id"))
	if !ok {
		return
	}
	next, statusOK, err := validateSprintRequest(req)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_SPRINT", err.Error())
		return
	}
	if !statusOK {
		respondError(c, http.StatusBadRequest, "INVALID_SPRINT_STATUS", "迭代状态必须是 planned、active 或 closed")
		return
	}
	item.Name = next.Name
	item.Goal = next.Goal
	item.Status = next.Status
	item.StartAt = next.StartAt
	item.EndAt = next.EndAt
	item.CapacityHours = next.CapacityHours
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(item).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "sprints", "update", item.ID, true, auditDetailf("更新迭代(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "UPDATE_SPRINT_FAILED", err)
		return
	}
	tasks, err := h.loadSprintVisibleTasks(c, item.ID)
	if err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_SPRINT_TASKS_FAILED", err)
		return
	}
	completed := int64(0)
	for _, task := range tasks {
		if task.Status == model.TaskCompleted {
			completed++
		}
	}
	c.JSON(http.StatusOK, toSprintResponse(*item, int64(len(tasks)), completed, tasks))
}

func (h *Handler) DeleteSprint(c *gin.Context) {
	item, ok := h.ensureSprintWritable(c, c.Param("id"))
	if !ok {
		return
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(item).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "sprints", "delete", item.ID, true, auditDetailf("删除迭代(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusInternalServerError, "DELETE_SPRINT_FAILED", err)
		return
	}
	respondMessage(c, http.StatusOK, "SPRINT_DELETED", "删除成功")
}

func (h *Handler) AddSprintTasks(c *gin.Context) {
	item, ok := h.ensureSprintWritable(c, c.Param("id"))
	if !ok {
		return
	}
	if !h.currentUserIsAdmin(c) && !h.currentUserHasPermission(c, "tasks.read") {
		respondError(c, http.StatusForbidden, "TASK_READ_REQUIRED", "需要任务查看权限")
		return
	}
	var req sprintTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	taskIDs := uniqueTaskIDs(req.TaskIDs)
	if len(taskIDs) == 0 {
		respondError(c, http.StatusBadRequest, "SPRINT_TASKS_REQUIRED", "请选择要加入迭代的任务")
		return
	}
	visibleIDs, err := h.visibleTaskIDs(c, taskIDs)
	if err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_TASK_SCOPE_FAILED", err)
		return
	}
	if len(visibleIDs) != len(taskIDs) {
		respondError(c, http.StatusNotFound, "TASK_NOT_FOUND", "任务不存在或不可见")
		return
	}
	records := make([]model.SprintTask, 0, len(visibleIDs))
	for _, taskID := range visibleIDs {
		records = append(records, model.SprintTask{SprintID: item.ID, TaskID: taskID})
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&records).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "sprints", "add_tasks", item.ID, true, auditDetailf("迭代加入任务(id=%d,tasks=%d)", item.ID, len(records)))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "ADD_SPRINT_TASKS_FAILED", err)
		return
	}
	h.SprintDetail(c)
}

func (h *Handler) RemoveSprintTask(c *gin.Context) {
	item, ok := h.ensureSprintWritable(c, c.Param("id"))
	if !ok {
		return
	}
	if !h.currentUserIsAdmin(c) && !h.currentUserHasPermission(c, "tasks.read") {
		respondError(c, http.StatusForbidden, "TASK_READ_REQUIRED", "需要任务查看权限")
		return
	}
	parsed, err := strconv.ParseUint(c.Param("taskId"), 10, 64)
	if err != nil || parsed == 0 {
		respondError(c, http.StatusBadRequest, "INVALID_TASK_ID", "任务 ID 无效")
		return
	}
	taskID := uint(parsed)
	visibleIDs, err := h.visibleTaskIDs(c, []uint{taskID})
	if err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_TASK_SCOPE_FAILED", err)
		return
	}
	if len(visibleIDs) != 1 {
		respondError(c, http.StatusNotFound, "TASK_NOT_FOUND", "任务不存在或不可见")
		return
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("sprint_id = ? AND task_id = ?", item.ID, taskID).Delete(&model.SprintTask{}).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "sprints", "remove_task", item.ID, true, auditDetailf("迭代移除任务(id=%d,task=%d)", item.ID, taskID))
	}); err != nil {
		respondDBError(c, http.StatusInternalServerError, "REMOVE_SPRINT_TASK_FAILED", err)
		return
	}
	h.SprintDetail(c)
}
