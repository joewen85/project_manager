package handler

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
)

func (h *Handler) ProgressList(c *gin.Context) {
	type progressItem struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	var items []progressItem
	query := h.DB.Model(&model.Task{})
	query = h.scopeTasksQuery(c, query)
	if err := query.Select("status, count(*) as count").Group("status").Scan(&items).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PROGRESS_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) DashboardStats(c *gin.Context) {
	var userCount, projectCount, taskCount, doneCount int64
	isAdmin := h.currentUserIsAdmin(c)
	if isAdmin {
		h.DB.Model(&model.User{}).Count(&userCount)
		h.DB.Model(&model.Project{}).Count(&projectCount)
		h.DB.Model(&model.Task{}).Count(&taskCount)
		h.DB.Model(&model.Task{}).Where("status = ?", model.TaskCompleted).Count(&doneCount)
	} else {
		uid := c.GetUint("userId")
		userCount = 1
		taskBase := h.scopeTasksQuery(c, h.DB.Model(&model.Task{}))
		taskBase.Count(&taskCount)
		taskBase.Where("status = ?", model.TaskCompleted).Count(&doneCount)
		taskBase.Distinct("project_id").Count(&projectCount)
		if uid == 0 {
			userCount = 0
		}
	}

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

type projectHealthItem struct {
	ProjectID        uint     `json:"projectId"`
	ProjectCode      string   `json:"projectCode"`
	ProjectName      string   `json:"projectName"`
	Health           string   `json:"health"`
	Score            int      `json:"score"`
	CompletionRate   float64  `json:"completionRate"`
	TotalTasks       int64    `json:"totalTasks"`
	CompletedTasks   int64    `json:"completedTasks"`
	OverdueTasks     int64    `json:"overdueTasks"`
	MilestoneOverdue int64    `json:"milestoneOverdue"`
	UnscheduledTasks int64    `json:"unscheduledTasks"`
	ReviewingTasks   int64    `json:"reviewingTasks"`
	Reasons          []string `json:"reasons"`
}

type projectHealthRow struct {
	ProjectID   uint
	ProjectCode string
	ProjectName string
	TaskID      uint
	Status      model.TaskStatus
	Priority    model.TaskPriority
	IsMilestone bool
	Progress    int
	StartAt     *time.Time
	EndAt       *time.Time
}

type projectHealthAccumulator struct {
	projectID        uint
	projectCode      string
	projectName      string
	totalTasks       int64
	completedTasks   int64
	overdueTasks     int64
	milestoneOverdue int64
	unscheduledTasks int64
	reviewingTasks   int64
	weightedScore    float64
	weightTotal      float64
}

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func taskHealthWeight(priority model.TaskPriority, isMilestone bool) float64 {
	weight := 1.0
	switch priority {
	case model.TaskPriorityHigh:
		weight = 3
	case model.TaskPriorityMedium:
		weight = 2
	}
	if isMilestone {
		weight += 1
	}
	return weight
}

func plannedProgress(now time.Time, startAt, endAt *time.Time) (float64, bool) {
	if startAt == nil || endAt == nil || !endAt.After(*startAt) {
		return 0, false
	}
	if !now.After(*startAt) {
		return 0, true
	}
	if !now.Before(*endAt) {
		return 100, true
	}
	elapsed := now.Sub(*startAt)
	duration := endAt.Sub(*startAt)
	return clampPercent(float64(elapsed) / float64(duration) * 100), true
}

func actualProgress(status model.TaskStatus, progress int) float64 {
	if status == model.TaskCompleted {
		return 100
	}
	return clampPercent(float64(progress))
}

func (acc *projectHealthAccumulator) addTask(row projectHealthRow, now time.Time) {
	acc.totalTasks += 1
	if row.Status == model.TaskCompleted {
		acc.completedTasks += 1
	}
	if row.Status == model.TaskReviewing {
		acc.reviewingTasks += 1
	}
	if row.StartAt == nil || row.EndAt == nil {
		acc.unscheduledTasks += 1
	}
	if row.EndAt != nil && row.EndAt.Before(now) && row.Status != model.TaskCompleted {
		acc.overdueTasks += 1
		if row.IsMilestone {
			acc.milestoneOverdue += 1
		}
	}

	planned, ok := plannedProgress(now, row.StartAt, row.EndAt)
	if !ok {
		return
	}
	actual := actualProgress(row.Status, row.Progress)
	lag := planned - actual
	taskScore := 100.0
	if lag > 0 {
		taskScore = clampPercent(100 - lag)
	}
	weight := taskHealthWeight(row.Priority, row.IsMilestone)
	acc.weightedScore += taskScore * weight
	acc.weightTotal += weight
}

func calculateProjectHealth(acc projectHealthAccumulator) projectHealthItem {
	completionRate := 0.0
	if acc.totalTasks > 0 {
		completionRate = float64(acc.completedTasks) / float64(acc.totalTasks)
	}

	score := 100
	if acc.weightTotal > 0 {
		score = int(acc.weightedScore/acc.weightTotal + 0.5)
	} else if acc.unscheduledTasks > 0 {
		score = 70
	}
	score = int(clampPercent(float64(score)))

	reasons := make([]string, 0, 5)
	if acc.overdueTasks > 0 {
		reasons = append(reasons, countReason(acc.overdueTasks, "个任务已逾期"))
	}
	if acc.milestoneOverdue > 0 {
		reasons = append(reasons, countReason(acc.milestoneOverdue, "个里程碑逾期"))
	}
	if acc.reviewingTasks >= 3 {
		reasons = append(reasons, countReason(acc.reviewingTasks, "个任务待审核"))
	}
	if acc.unscheduledTasks > 0 {
		reasons = append(reasons, countReason(acc.unscheduledTasks, "个任务未排期"))
	}
	if score < 85 && acc.weightTotal > 0 {
		reasons = append(reasons, "实际进度落后于计划进度")
	}

	health := "green"
	if acc.milestoneOverdue > 0 || acc.overdueTasks >= 3 || score < 60 {
		health = "red"
	} else if acc.overdueTasks > 0 || acc.reviewingTasks >= 3 || acc.unscheduledTasks > 0 || score < 85 {
		health = "yellow"
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "暂无明显风险")
	}

	return projectHealthItem{
		ProjectID:        acc.projectID,
		ProjectCode:      acc.projectCode,
		ProjectName:      acc.projectName,
		Health:           health,
		Score:            score,
		CompletionRate:   completionRate,
		TotalTasks:       acc.totalTasks,
		CompletedTasks:   acc.completedTasks,
		OverdueTasks:     acc.overdueTasks,
		MilestoneOverdue: acc.milestoneOverdue,
		UnscheduledTasks: acc.unscheduledTasks,
		ReviewingTasks:   acc.reviewingTasks,
		Reasons:          reasons,
	}
}

func countReason(count int64, suffix string) string {
	return strconv.FormatInt(count, 10) + suffix
}

func (h *Handler) ProjectHealth(c *gin.Context) {
	now := time.Now()
	taskQuery := h.scopeTasksQuery(c, h.DB.Model(&model.Task{})).
		Select(`
			tasks.project_id AS project_id,
			projects.code AS project_code,
			projects.name AS project_name,
			tasks.id AS task_id,
			tasks.status,
			tasks.priority,
			tasks.is_milestone,
			tasks.progress,
			tasks.start_at,
			tasks.end_at
		`).
		Joins("JOIN projects ON projects.id = tasks.project_id")

	var rows []projectHealthRow
	if err := taskQuery.Scan(&rows).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PROJECT_HEALTH_FAILED", err)
		return
	}

	projects := map[uint]*projectHealthAccumulator{}
	for _, row := range rows {
		acc, ok := projects[row.ProjectID]
		if !ok {
			acc = &projectHealthAccumulator{
				projectID:   row.ProjectID,
				projectCode: row.ProjectCode,
				projectName: row.ProjectName,
			}
			projects[row.ProjectID] = acc
		}
		acc.addTask(row, now)
	}

	items := make([]projectHealthItem, 0, len(projects))
	for _, acc := range projects {
		items = append(items, calculateProjectHealth(*acc))
	}
	sort.SliceStable(items, func(i, j int) bool {
		rank := map[string]int{"red": 0, "yellow": 1, "green": 2}
		leftRank := rank[items[i].Health]
		rightRank := rank[items[j].Health]
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if items[i].Score != items[j].Score {
			return items[i].Score < items[j].Score
		}
		return items[i].ProjectID < items[j].ProjectID
	})

	c.JSON(http.StatusOK, gin.H{"projects": items})
}
