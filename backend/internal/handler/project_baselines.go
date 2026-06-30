package handler

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type projectBaselineRequest struct {
	ProjectID   uint   `json:"projectId" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type projectBaselineTaskVariance struct {
	TaskID             uint       `json:"taskId"`
	TaskNo             string     `json:"taskNo"`
	Title              string     `json:"title"`
	BaselineStartAt    *time.Time `json:"baselineStartAt"`
	BaselineEndAt      *time.Time `json:"baselineEndAt"`
	CurrentStartAt     *time.Time `json:"currentStartAt"`
	CurrentEndAt       *time.Time `json:"currentEndAt"`
	StartVarianceDays  int        `json:"startVarianceDays"`
	EndVarianceDays    int        `json:"endVarianceDays"`
	StatusChanged      bool       `json:"statusChanged"`
	ProgressChanged    bool       `json:"progressChanged"`
	MissingCurrentTask bool       `json:"missingCurrentTask"`
}

type projectBaselineCompare struct {
	BaselineTaskCount      int                           `json:"baselineTaskCount"`
	CurrentTaskCount       int                           `json:"currentTaskCount"`
	BaselineCompletedCount int                           `json:"baselineCompletedCount"`
	CurrentCompletedCount  int                           `json:"currentCompletedCount"`
	BaselinePlannedEndAt   *time.Time                    `json:"baselinePlannedEndAt"`
	CurrentPlannedEndAt    *time.Time                    `json:"currentPlannedEndAt"`
	EndVarianceDays        int                           `json:"endVarianceDays"`
	DelayedTaskCount       int                           `json:"delayedTaskCount"`
	MissingTaskCount       int                           `json:"missingTaskCount"`
	ChangedTasks           []projectBaselineTaskVariance `json:"changedTasks"`
}

type projectBaselineDetailResponse struct {
	model.ProjectBaseline
	Compare projectBaselineCompare `json:"compare"`
}

type criticalPathTask struct {
	ID           uint               `json:"id"`
	TaskNo       string             `json:"taskNo"`
	Title        string             `json:"title"`
	Status       model.TaskStatus   `json:"status"`
	Progress     int                `json:"progress"`
	Priority     model.TaskPriority `json:"priority"`
	IsMilestone  bool               `json:"isMilestone"`
	StartAt      *time.Time         `json:"startAt"`
	EndAt        *time.Time         `json:"endAt"`
	DurationDays int                `json:"durationDays"`
}

type criticalPathResult struct {
	ProjectID         uint               `json:"projectId"`
	ProjectEndAt      *time.Time         `json:"projectEndAt"`
	TotalDurationDays int                `json:"totalDurationDays"`
	CriticalTaskIDs   []uint             `json:"criticalTaskIds"`
	Tasks             []criticalPathTask `json:"tasks"`
	HasCycle          bool               `json:"hasCycle"`
}

func dayDiff(from, to *time.Time) int {
	if from == nil || to == nil {
		return 0
	}
	hours := to.Sub(*from).Hours()
	if hours > 0 {
		return int(math.Ceil(hours / 24))
	}
	if hours < 0 {
		return int(math.Floor(hours / 24))
	}
	return 0
}

func taskDurationDays(task model.Task) int {
	duration := scheduleDuration(task)
	days := int(math.Ceil(duration.Hours() / 24))
	if days < 1 {
		return 1
	}
	return days
}

func scheduleEndAt(tasks []model.Task) *time.Time {
	var endAt *time.Time
	for index := range tasks {
		if tasks[index].EndAt == nil {
			continue
		}
		if endAt == nil || tasks[index].EndAt.After(*endAt) {
			copyValue := *tasks[index].EndAt
			endAt = &copyValue
		}
	}
	return endAt
}

func scheduleStartAt(tasks []model.Task) *time.Time {
	var startAt *time.Time
	for index := range tasks {
		if tasks[index].StartAt == nil {
			continue
		}
		if startAt == nil || tasks[index].StartAt.Before(*startAt) {
			copyValue := *tasks[index].StartAt
			startAt = &copyValue
		}
	}
	return startAt
}

func baselineSnapshotFromTasks(tasks []model.Task) []model.ProjectBaselineTaskSnapshot {
	out := make([]model.ProjectBaselineTaskSnapshot, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, model.ProjectBaselineTaskSnapshot{
			TaskID:      task.ID,
			TaskNo:      task.TaskNo,
			Title:       task.Title,
			Status:      task.Status,
			Progress:    task.Progress,
			IsMilestone: task.IsMilestone,
			StartAt:     task.StartAt,
			EndAt:       task.EndAt,
			ParentID:    task.ParentID,
		})
	}
	return out
}

func completedTaskCount(tasks []model.Task) int {
	count := 0
	for _, task := range tasks {
		if task.Status == model.TaskCompleted {
			count += 1
		}
	}
	return count
}

func criticalDependencyOffsetDays(dependency model.TaskDependency) int {
	return dependency.LagDays
}

func buildCriticalPath(projectID uint, tasks []model.Task) (criticalPathResult, error) {
	result := criticalPathResult{ProjectID: projectID}
	if len(tasks) == 0 {
		return result, nil
	}
	taskByID := make(map[uint]model.Task, len(tasks))
	indegree := make(map[uint]int, len(tasks))
	nextMap := map[uint][]model.TaskDependency{}
	score := map[uint]int{}
	previous := map[uint]uint{}
	for _, task := range tasks {
		taskByID[task.ID] = task
		indegree[task.ID] = 0
		score[task.ID] = taskDurationDays(task)
	}
	for _, task := range tasks {
		for _, dependency := range task.Dependencies {
			if _, ok := taskByID[dependency.DependsOnTaskID]; !ok {
				continue
			}
			indegree[task.ID] += 1
			nextMap[dependency.DependsOnTaskID] = append(nextMap[dependency.DependsOnTaskID], dependency)
		}
	}
	queue := make([]uint, 0, len(tasks))
	for id, degree := range indegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}
	sort.Slice(queue, func(i, j int) bool { return queue[i] < queue[j] })
	ordered := make([]uint, 0, len(tasks))
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		ordered = append(ordered, id)
		for _, dependency := range nextMap[id] {
			successorID := dependency.TaskID
			successor := taskByID[successorID]
			candidate := score[id] + criticalDependencyOffsetDays(dependency) + taskDurationDays(successor)
			if candidate > score[successorID] {
				score[successorID] = candidate
				previous[successorID] = id
			}
			indegree[successorID] -= 1
			if indegree[successorID] == 0 {
				queue = append(queue, successorID)
				sort.Slice(queue, func(i, j int) bool { return queue[i] < queue[j] })
			}
		}
	}
	if len(ordered) != len(tasks) {
		result.HasCycle = true
		return result, gorm.ErrInvalidData
	}
	endID := ordered[0]
	for _, id := range ordered {
		if score[id] > score[endID] {
			endID = id
		}
	}
	pathIDs := []uint{endID}
	for {
		prevID, ok := previous[pathIDs[0]]
		if !ok {
			break
		}
		pathIDs = append([]uint{prevID}, pathIDs...)
	}
	result.CriticalTaskIDs = pathIDs
	result.TotalDurationDays = score[endID]
	result.ProjectEndAt = scheduleEndAt(tasks)
	result.Tasks = make([]criticalPathTask, 0, len(pathIDs))
	for _, id := range pathIDs {
		task := taskByID[id]
		result.Tasks = append(result.Tasks, criticalPathTask{
			ID:           task.ID,
			TaskNo:       task.TaskNo,
			Title:        task.Title,
			Status:       task.Status,
			Progress:     task.Progress,
			Priority:     task.Priority,
			IsMilestone:  task.IsMilestone,
			StartAt:      task.StartAt,
			EndAt:        task.EndAt,
			DurationDays: taskDurationDays(task),
		})
	}
	return result, nil
}

func criticalTaskIDSet(projectID uint, tasks []model.Task) map[uint]struct{} {
	result, err := buildCriticalPath(projectID, tasks)
	if err != nil {
		return map[uint]struct{}{}
	}
	out := map[uint]struct{}{}
	for _, id := range result.CriticalTaskIDs {
		out[id] = struct{}{}
	}
	return out
}

func (h *Handler) visibleProjectTasks(c *gin.Context, projectID uint) ([]model.Task, error) {
	var tasks []model.Task
	query := h.scopeTasksQuery(c, h.DB.Model(&model.Task{})).
		Where("tasks.project_id = ?", projectID).
		Preload("Dependencies").
		Order("COALESCE(tasks.start_at, tasks.created_at) asc, tasks.id asc")
	if err := query.Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

func (h *Handler) criticalOverdueByVisibleProject(c *gin.Context, now time.Time) (map[uint]int64, error) {
	var tasks []model.Task
	if err := h.scopeTasksQuery(c, h.DB.Model(&model.Task{})).
		Preload("Dependencies").
		Find(&tasks).Error; err != nil {
		return nil, err
	}
	tasksByProject := map[uint][]model.Task{}
	for _, task := range tasks {
		tasksByProject[task.ProjectID] = append(tasksByProject[task.ProjectID], task)
	}
	result := map[uint]int64{}
	for projectID, projectTasks := range tasksByProject {
		criticalIDs := criticalTaskIDSet(projectID, projectTasks)
		for _, task := range projectTasks {
			if _, ok := criticalIDs[task.ID]; !ok {
				continue
			}
			if task.EndAt != nil && task.EndAt.Before(now) && task.Status != model.TaskCompleted {
				result[projectID] += 1
			}
		}
	}
	return result, nil
}

func baselineCompare(baseline model.ProjectBaseline, currentTasks []model.Task) projectBaselineCompare {
	currentByID := map[uint]model.Task{}
	for _, task := range currentTasks {
		currentByID[task.ID] = task
	}
	changed := make([]projectBaselineTaskVariance, 0)
	delayedCount := 0
	missingCount := 0
	for _, snapshot := range baseline.Snapshot {
		current, ok := currentByID[snapshot.TaskID]
		variance := projectBaselineTaskVariance{
			TaskID:          snapshot.TaskID,
			TaskNo:          snapshot.TaskNo,
			Title:           snapshot.Title,
			BaselineStartAt: snapshot.StartAt,
			BaselineEndAt:   snapshot.EndAt,
		}
		if !ok {
			variance.MissingCurrentTask = true
			missingCount += 1
			changed = append(changed, variance)
			continue
		}
		variance.CurrentStartAt = current.StartAt
		variance.CurrentEndAt = current.EndAt
		variance.StartVarianceDays = dayDiff(snapshot.StartAt, current.StartAt)
		variance.EndVarianceDays = dayDiff(snapshot.EndAt, current.EndAt)
		variance.StatusChanged = snapshot.Status != current.Status
		variance.ProgressChanged = snapshot.Progress != current.Progress
		if variance.EndVarianceDays > 0 {
			delayedCount += 1
		}
		if variance.StartVarianceDays != 0 || variance.EndVarianceDays != 0 || variance.StatusChanged || variance.ProgressChanged {
			changed = append(changed, variance)
		}
	}
	currentCompleted := completedTaskCount(currentTasks)
	currentEndAt := scheduleEndAt(currentTasks)
	return projectBaselineCompare{
		BaselineTaskCount:      baseline.TaskCount,
		CurrentTaskCount:       len(currentTasks),
		BaselineCompletedCount: baseline.CompletedTaskCount,
		CurrentCompletedCount:  currentCompleted,
		BaselinePlannedEndAt:   baseline.PlannedEndAt,
		CurrentPlannedEndAt:    currentEndAt,
		EndVarianceDays:        dayDiff(baseline.PlannedEndAt, currentEndAt),
		DelayedTaskCount:       delayedCount,
		MissingTaskCount:       missingCount,
		ChangedTasks:           changed,
	}
}

func (h *Handler) scopeProjectBaselinesQuery(c *gin.Context, query *gorm.DB) *gorm.DB {
	if h.currentUserIsAdmin(c) {
		return query
	}
	return query.Where("project_baselines.created_by_id = ?", c.GetUint("userId"))
}

func (h *Handler) ListProjectBaselines(c *gin.Context) {
	page, pageSize := parsePage(c)
	query := h.scopeProjectBaselinesQuery(c, h.DB.Model(&model.ProjectBaseline{}))
	if projectID := strings.TrimSpace(c.Query("projectId")); projectID != "" {
		query = query.Where("project_baselines.project_id = ?", projectID)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("project_baselines.name LIKE ? OR project_baselines.description LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PROJECT_BASELINES_FAILED", err)
		return
	}
	var items []model.ProjectBaseline
	if err := query.Preload("Project").Preload("CreatedBy").
		Order("project_baselines.created_at desc, project_baselines.id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PROJECT_BASELINES_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, pageResult[model.ProjectBaseline]{List: items, Total: total, Page: page, PageSize: pageSize})
}

func (h *Handler) CreateProjectBaseline(c *gin.Context) {
	var req projectBaselineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		respondError(c, http.StatusBadRequest, "INVALID_BASELINE", "基线名称不能为空")
		return
	}
	projectID := strconv.FormatUint(uint64(req.ProjectID), 10)
	if !h.ensureProjectVisible(c, projectID) {
		return
	}
	tasks, err := h.visibleProjectTasks(c, req.ProjectID)
	if err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PROJECT_BASELINE_TASKS_FAILED", err)
		return
	}
	item := model.ProjectBaseline{
		ProjectID:          req.ProjectID,
		Name:               name,
		Description:        strings.TrimSpace(req.Description),
		TaskCount:          len(tasks),
		CompletedTaskCount: completedTaskCount(tasks),
		PlannedStartAt:     scheduleStartAt(tasks),
		PlannedEndAt:       scheduleEndAt(tasks),
		Snapshot:           baselineSnapshotFromTasks(tasks),
		CreatedByID:        c.GetUint("userId"),
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		if err := tx.Preload("Project").Preload("CreatedBy").First(&item, item.ID).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "project_baselines", "create", item.ID, true, auditDetailf("创建项目基线(id=%d, projectId=%d)", item.ID, item.ProjectID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_PROJECT_BASELINE_FAILED", err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) ProjectBaselineDetail(c *gin.Context) {
	var item model.ProjectBaseline
	query := h.scopeProjectBaselinesQuery(c, h.DB.Model(&model.ProjectBaseline{})).
		Preload("Project").
		Preload("CreatedBy")
	if err := query.Where("project_baselines.id = ?", c.Param("id")).First(&item).Error; err != nil {
		respondError(c, http.StatusNotFound, "PROJECT_BASELINE_NOT_FOUND", "项目基线不存在")
		return
	}
	if !h.ensureProjectVisible(c, strconv.FormatUint(uint64(item.ProjectID), 10)) {
		return
	}
	currentTasks, err := h.visibleProjectTasks(c, item.ProjectID)
	if err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PROJECT_BASELINE_COMPARE_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, projectBaselineDetailResponse{
		ProjectBaseline: item,
		Compare:         baselineCompare(item, currentTasks),
	})
}

func (h *Handler) DeleteProjectBaseline(c *gin.Context) {
	var item model.ProjectBaseline
	query := h.scopeProjectBaselinesQuery(c, h.DB.Model(&model.ProjectBaseline{}))
	if err := query.Where("project_baselines.id = ?", c.Param("id")).First(&item).Error; err != nil {
		respondError(c, http.StatusNotFound, "PROJECT_BASELINE_NOT_FOUND", "项目基线不存在")
		return
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&item).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "project_baselines", "delete", item.ID, true, auditDetailf("删除项目基线(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusInternalServerError, "DELETE_PROJECT_BASELINE_FAILED", err)
		return
	}
	respondMessage(c, http.StatusOK, "PROJECT_BASELINE_DELETED", "项目基线已删除")
}

func (h *Handler) ProjectCriticalPath(c *gin.Context) {
	projectIDText := c.Param("id")
	if !h.ensureProjectVisible(c, projectIDText) {
		return
	}
	parsedProjectID, err := strconv.ParseUint(projectIDText, 10, 64)
	if err != nil || parsedProjectID == 0 {
		respondError(c, http.StatusBadRequest, "INVALID_PROJECT_ID", "非法项目ID")
		return
	}
	tasks, err := h.visibleProjectTasks(c, uint(parsedProjectID))
	if err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_CRITICAL_PATH_TASKS_FAILED", err)
		return
	}
	result, err := buildCriticalPath(uint(parsedProjectID), tasks)
	if err != nil {
		respondError(c, http.StatusBadRequest, "CRITICAL_PATH_CYCLE", "任务依赖存在环，无法计算关键路径")
		return
	}
	c.JSON(http.StatusOK, result)
}

func formatCriticalReason(count int64) string {
	return fmt.Sprintf("%d个关键路径任务已逾期", count)
}
