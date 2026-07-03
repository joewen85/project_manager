package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"project-manager/backend/internal/ai"
	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type aiTaskBreakdownPayload struct {
	Tasks []struct {
		Title            string `json:"title"`
		Description      string `json:"description"`
		Priority         string `json:"priority"`
		IsMilestone      bool   `json:"isMilestone"`
		RelativeStartDay int    `json:"relativeStartDay"`
		DurationDays     int    `json:"durationDays"`
	} `json:"tasks"`
}

// aiParseSuggestedTasks converts a raw LLM JSON reply into validated task
// suggestions. It strips common Markdown code fences, coerces out-of-range
// values, and returns ok=false when no usable task can be extracted so callers
// fall back to the deterministic template.
func aiParseSuggestedTasks(raw string, sources []aiSourceRef) ([]aiSuggestedTask, bool) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimPrefix(raw, "json")
		if idx := strings.LastIndex(raw, "```"); idx >= 0 {
			raw = raw[:idx]
		}
		raw = strings.TrimSpace(raw)
	}
	var payload aiTaskBreakdownPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, false
	}
	result := make([]aiSuggestedTask, 0, len(payload.Tasks))
	for _, item := range payload.Tasks {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			continue
		}
		priority := strings.ToLower(strings.TrimSpace(item.Priority))
		if priority != "high" && priority != "medium" && priority != "low" {
			priority = "medium"
		}
		start := item.RelativeStartDay
		if start < 0 {
			start = 0
		}
		duration := item.DurationDays
		if duration < 1 {
			duration = 1
		}
		result = append(result, aiSuggestedTask{
			Title:            title,
			Description:      strings.TrimSpace(item.Description),
			Priority:         priority,
			IsMilestone:      item.IsMilestone,
			RelativeStartDay: start,
			DurationDays:     duration,
			SourceRefs:       sources,
		})
		if len(result) >= 8 {
			break
		}
	}
	if len(result) == 0 {
		return nil, false
	}
	return result, true
}

// aiComposeNarrative asks the configured LLM gateway to turn read-only context
// into prose. It returns the fallback string unchanged when the assistant is
// not configured or the gateway call fails, so every endpoint still produces
// output. The context data is passed as a user message and the system prompt
// instructs the model to treat it strictly as data — comments and register
// entries are user-writable and must never be honoured as instructions.
func (h *Handler) aiComposeNarrative(ctx context.Context, systemPrompt, contextData, fallback string) string {
	if h.AIClient == nil {
		return fallback
	}
	out, err := h.AIClient.Chat(ctx, []ai.Message{
		{Role: ai.RoleSystem, Content: systemPrompt},
		{Role: ai.RoleUser, Content: contextData},
	})
	if err != nil || strings.TrimSpace(out) == "" {
		return fallback
	}
	return out
}

type aiProjectRequest struct {
	ProjectID uint   `json:"projectId" binding:"required"`
	WeekStart string `json:"weekStart"`
	WeekEnd   string `json:"weekEnd"`
}

type aiTaskBreakdownRequest struct {
	ProjectID   uint   `json:"projectId"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type aiSourceRef struct {
	Type  string `json:"type"`
	ID    uint   `json:"id,omitempty"`
	Label string `json:"label"`
	Path  string `json:"path,omitempty"`
}

type aiDraftResponse struct {
	Mode                 string        `json:"mode"`
	Title                string        `json:"title"`
	Draft                string        `json:"draft"`
	Highlights           []string      `json:"highlights"`
	Recommendations      []string      `json:"recommendations"`
	SourceRefs           []aiSourceRef `json:"sourceRefs"`
	RequiresConfirmation bool          `json:"requiresConfirmation"`
	GeneratedAt          time.Time     `json:"generatedAt"`
}

type aiSuggestedTask struct {
	Title            string        `json:"title"`
	Description      string        `json:"description"`
	Priority         string        `json:"priority"`
	IsMilestone      bool          `json:"isMilestone"`
	RelativeStartDay int           `json:"relativeStartDay"`
	DurationDays     int           `json:"durationDays"`
	SourceRefs       []aiSourceRef `json:"sourceRefs"`
}

type aiTaskBreakdownResponse struct {
	Mode                 string            `json:"mode"`
	Title                string            `json:"title"`
	Summary              string            `json:"summary"`
	Tasks                []aiSuggestedTask `json:"tasks"`
	SourceRefs           []aiSourceRef     `json:"sourceRefs"`
	RequiresConfirmation bool              `json:"requiresConfirmation"`
	GeneratedAt          time.Time         `json:"generatedAt"`
}

func aiProjectSource(project model.Project) aiSourceRef {
	return aiSourceRef{
		Type:  "project",
		ID:    project.ID,
		Label: strings.TrimSpace(project.Code + " " + project.Name),
		Path:  "/projects?projectId=" + strconv.FormatUint(uint64(project.ID), 10),
	}
}

func aiTaskSource(task model.Task) aiSourceRef {
	label := task.TaskNo
	if strings.TrimSpace(task.Title) != "" {
		label = strings.TrimSpace(task.TaskNo + " " + task.Title)
	}
	return aiSourceRef{
		Type:  "task",
		ID:    task.ID,
		Label: label,
		Path:  "/tasks?taskId=" + strconv.FormatUint(uint64(task.ID), 10) + "&view=1",
	}
}

func aiRegisterSource(item model.ProjectRegister) aiSourceRef {
	return aiSourceRef{
		Type:  "project_register",
		ID:    item.ID,
		Label: strings.TrimSpace(item.Title),
		Path:  "/registers?registerId=" + strconv.FormatUint(uint64(item.ID), 10),
	}
}

func aiActivitySource(item model.TaskActivity) aiSourceRef {
	return aiSourceRef{
		Type:  "task_activity",
		ID:    item.ID,
		Label: strings.TrimSpace(item.Summary),
		Path:  "/tasks?taskId=" + strconv.FormatUint(uint64(item.TaskID), 10) + "&view=1",
	}
}

func aiCommentSource(item model.TaskComment) aiSourceRef {
	return aiSourceRef{
		Type:  "task_comment",
		ID:    item.ID,
		Label: "评论 #" + strconv.FormatUint(uint64(item.ID), 10),
		Path:  "/tasks?taskId=" + strconv.FormatUint(uint64(item.TaskID), 10) + "&view=1",
	}
}

func appendAISource(out []aiSourceRef, seen map[string]bool, source aiSourceRef) []aiSourceRef {
	key := source.Type + ":" + strconv.FormatUint(uint64(source.ID), 10) + ":" + source.Label
	if seen[key] {
		return out
	}
	seen[key] = true
	return append(out, source)
}

func aiTaskIDs(tasks []model.Task) []uint {
	ids := make([]uint, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	return ids
}

func aiTaskStatusCounts(tasks []model.Task) map[model.TaskStatus]int {
	counts := map[model.TaskStatus]int{}
	for _, task := range tasks {
		counts[task.Status]++
	}
	return counts
}

func aiTaskCompletionRate(tasks []model.Task) float64 {
	if len(tasks) == 0 {
		return 0
	}
	done := 0
	for _, task := range tasks {
		if task.Status == model.TaskCompleted {
			done++
		}
	}
	return float64(done) / float64(len(tasks)) * 100
}

func aiFormatPercent(value float64) string {
	return strconv.FormatFloat(value, 'f', 1, 64) + "%"
}

func aiTaskStatusLabel(status model.TaskStatus) string {
	switch status {
	case model.TaskPending:
		return "待处理"
	case model.TaskQueued:
		return "排队中"
	case model.TaskProcessing:
		return "处理中"
	case model.TaskReviewing:
		return "待审核"
	case model.TaskCompleted:
		return "已完成"
	default:
		return string(status)
	}
}

func aiWeekRange(startText, endText string) (time.Time, time.Time, bool) {
	if strings.TrimSpace(startText) == "" && strings.TrimSpace(endText) == "" {
		start, end := currentWeekRange(time.Now())
		return start, end, true
	}
	startAt, err := parseRFC3339(startText)
	if err != nil || startAt == nil {
		return time.Time{}, time.Time{}, false
	}
	endAt, err := parseRFC3339(endText)
	if err != nil || endAt == nil || endAt.Before(*startAt) {
		return time.Time{}, time.Time{}, false
	}
	return *startAt, *endAt, true
}

func (h *Handler) canReadAIProjectSource(c *gin.Context) bool {
	return h.currentUserIsAdmin(c) || h.currentUserHasPermission(c, "projects.read")
}

func (h *Handler) canReadAITaskSource(c *gin.Context) bool {
	return h.currentUserIsAdmin(c) || h.currentUserHasPermission(c, "tasks.read")
}

func (h *Handler) canReadAICommentSource(c *gin.Context) bool {
	return h.currentUserIsAdmin(c) || h.currentUserHasPermission(c, "comments.read")
}

func (h *Handler) canReadAIRegisterSource(c *gin.Context) bool {
	return h.currentUserIsAdmin(c) || h.currentUserHasPermission(c, "registers.read")
}

func (h *Handler) aiProject(c *gin.Context, projectID uint) (model.Project, bool) {
	if projectID == 0 {
		respondError(c, http.StatusBadRequest, "PROJECT_ID_REQUIRED", "projectId 不能为空")
		return model.Project{}, false
	}
	if !h.canReadAIProjectSource(c) {
		respondError(c, http.StatusForbidden, "AI_SOURCE_PERMISSION_REQUIRED", "AI 助理读取项目上下文需要 projects.read 权限")
		return model.Project{}, false
	}
	if !h.ensureProjectVisible(c, strconv.FormatUint(uint64(projectID), 10)) {
		return model.Project{}, false
	}
	var project model.Project
	if err := h.DB.Preload("Users").Preload("Departments").First(&project, projectID).Error; err != nil {
		respondError(c, http.StatusNotFound, "PROJECT_NOT_FOUND", "项目不存在")
		return model.Project{}, false
	}
	return project, true
}

func (h *Handler) aiVisibleProjectTasks(c *gin.Context, projectID uint) ([]model.Task, error) {
	if !h.canReadAITaskSource(c) {
		return []model.Task{}, nil
	}
	var tasks []model.Task
	err := h.scopeTasksQuery(c, h.DB.Model(&model.Task{})).
		Where("tasks.project_id = ?", projectID).
		Preload("Assignees").
		Preload("Tags", func(db *gorm.DB) *gorm.DB { return db.Order("tags.name asc") }).
		Order("tasks.end_at asc, tasks.id asc").
		Find(&tasks).Error
	return tasks, err
}

func (h *Handler) aiRecentActivities(taskIDs []uint, startAt, endAt time.Time) ([]model.TaskActivity, error) {
	if len(taskIDs) == 0 {
		return []model.TaskActivity{}, nil
	}
	var items []model.TaskActivity
	err := h.DB.Model(&model.TaskActivity{}).
		Where("task_id IN ? AND created_at BETWEEN ? AND ?", taskIDs, startAt, endAt).
		Preload("Actor").
		Order("created_at desc").
		Limit(20).
		Find(&items).Error
	return items, err
}

func (h *Handler) aiRecentComments(taskIDs []uint, startAt, endAt time.Time) ([]model.TaskComment, error) {
	if len(taskIDs) == 0 {
		return []model.TaskComment{}, nil
	}
	var items []model.TaskComment
	err := h.DB.Model(&model.TaskComment{}).
		Where("task_id IN ? AND is_deleted = ? AND created_at BETWEEN ? AND ?", taskIDs, false, startAt, endAt).
		Preload("Author").
		Order("created_at desc").
		Limit(10).
		Find(&items).Error
	return items, err
}

func (h *Handler) aiProjectRegisters(c *gin.Context, projectID uint) ([]model.ProjectRegister, error) {
	if !h.canReadAIRegisterSource(c) {
		return []model.ProjectRegister{}, nil
	}
	var items []model.ProjectRegister
	err := h.scopeProjectRegistersQuery(c, h.DB.Model(&model.ProjectRegister{})).
		Where("project_registers.project_id = ?", projectID).
		Preload("Owner").
		Order("project_registers.updated_at desc").
		Limit(20).
		Find(&items).Error
	return items, err
}

func (h *Handler) AIProjectWeeklyReport(c *gin.Context) {
	var req aiProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	weekStart, weekEnd, ok := aiWeekRange(req.WeekStart, req.WeekEnd)
	if !ok {
		respondError(c, http.StatusBadRequest, "INVALID_AI_WEEK_RANGE", "weekStart/weekEnd 必须是有效 RFC3339 时间，且结束时间不能早于开始时间")
		return
	}
	project, ok := h.aiProject(c, req.ProjectID)
	if !ok {
		return
	}
	tasks, err := h.aiVisibleProjectTasks(c, project.ID)
	if err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_AI_TASKS_FAILED", err)
		return
	}
	taskIDs := aiTaskIDs(tasks)
	activities := []model.TaskActivity{}
	comments := []model.TaskComment{}
	if h.canReadAICommentSource(c) {
		activities, err = h.aiRecentActivities(taskIDs, weekStart, weekEnd)
		if err != nil {
			respondDBError(c, http.StatusInternalServerError, "QUERY_AI_ACTIVITIES_FAILED", err)
			return
		}
		comments, err = h.aiRecentComments(taskIDs, weekStart, weekEnd)
		if err != nil {
			respondDBError(c, http.StatusInternalServerError, "QUERY_AI_COMMENTS_FAILED", err)
			return
		}
	}
	registers, err := h.aiProjectRegisters(c, project.ID)
	if err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_AI_REGISTERS_FAILED", err)
		return
	}

	statusCounts := aiTaskStatusCounts(tasks)
	openRisks := 0
	openIssues := 0
	for _, item := range registers {
		if !projectRegisterOpen(item.Status) {
			continue
		}
		if item.Type == model.ProjectRegisterRisk {
			openRisks++
		}
		if item.Type == model.ProjectRegisterIssue {
			openIssues++
		}
	}

	highlights := []string{
		fmt.Sprintf("可见任务 %d 个，完成率 %s", len(tasks), aiFormatPercent(aiTaskCompletionRate(tasks))),
		fmt.Sprintf("本周期内记录 %d 条活动、%d 条评论", len(activities), len(comments)),
		fmt.Sprintf("未关闭风险 %d 个，未解决问题 %d 个", openRisks, openIssues),
	}
	recommendations := []string{
		"确认待审核任务是否需要集中处理",
		"对逾期或高风险项补充负责人和下一步动作",
	}
	if statusCounts[model.TaskReviewing] == 0 {
		recommendations[0] = "保持当前节奏，持续关注处理中任务的下一个里程碑"
	}

	var builder strings.Builder
	builder.WriteString("# 项目周报草稿：" + project.Code + " - " + project.Name + "\n\n")
	builder.WriteString("周期：" + weekStart.Format("2006-01-02") + " 至 " + weekEnd.Format("2006-01-02") + "\n\n")
	builder.WriteString("## 总体进展\n")
	builder.WriteString("- 可见任务：" + strconv.Itoa(len(tasks)) + " 个，完成率：" + aiFormatPercent(aiTaskCompletionRate(tasks)) + "\n")
	builder.WriteString("- 状态分布：")
	orderedStatuses := []model.TaskStatus{model.TaskPending, model.TaskQueued, model.TaskProcessing, model.TaskReviewing, model.TaskCompleted}
	statusParts := make([]string, 0, len(orderedStatuses))
	for _, status := range orderedStatuses {
		statusParts = append(statusParts, aiTaskStatusLabel(status)+" "+strconv.Itoa(statusCounts[status]))
	}
	builder.WriteString(strings.Join(statusParts, "，") + "\n\n")
	builder.WriteString("## 本周动态\n")
	if len(activities) == 0 && len(comments) == 0 {
		builder.WriteString("- 本周期暂无任务活动或评论记录。\n")
	} else {
		for index, activity := range activities {
			if index >= 5 {
				break
			}
			builder.WriteString("- " + activity.CreatedAt.Format("01-02") + "：" + activity.Summary + "\n")
		}
		for index, comment := range comments {
			if index >= 3 {
				break
			}
			content := strings.TrimSpace(comment.Content)
			if len(content) > 80 {
				content = content[:80] + "..."
			}
			builder.WriteString("- 评论：" + content + "\n")
		}
	}
	builder.WriteString("\n## 风险与问题\n")
	if openRisks == 0 && openIssues == 0 {
		builder.WriteString("- 当前没有未关闭风险或未解决问题登记项。\n")
	} else {
		for _, item := range registers {
			if !projectRegisterOpen(item.Status) {
				continue
			}
			builder.WriteString("- " + projectRegisterTypeLabel(item.Type) + "：" + item.Title + "（" + string(item.Severity) + "）\n")
		}
	}
	builder.WriteString("\n## 下周建议\n")
	for _, item := range recommendations {
		builder.WriteString("- " + item + "\n")
	}

	seen := map[string]bool{}
	sources := []aiSourceRef{}
	sources = appendAISource(sources, seen, aiProjectSource(project))
	for index, activity := range activities {
		if index >= 8 {
			break
		}
		sources = appendAISource(sources, seen, aiActivitySource(activity))
	}
	for index, comment := range comments {
		if index >= 5 {
			break
		}
		sources = appendAISource(sources, seen, aiCommentSource(comment))
	}
	for index, item := range registers {
		if index >= 8 {
			break
		}
		sources = appendAISource(sources, seen, aiRegisterSource(item))
	}

	fallbackDraft := builder.String()
	contextData := "<context>\n" + fallbackDraft + "\n</context>"
	draft := h.aiComposeNarrative(c.Request.Context(), h.aiPrompts.weeklyReport, contextData, fallbackDraft)

	c.JSON(http.StatusOK, aiDraftResponse{
		Mode:                 "weekly_report",
		Title:                "项目周报草稿",
		Draft:                draft,
		Highlights:           highlights,
		Recommendations:      recommendations,
		SourceRefs:           sources,
		RequiresConfirmation: true,
		GeneratedAt:          time.Now(),
	})
}

func (h *Handler) AIProjectRiskSummary(c *gin.Context) {
	var req aiProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	project, ok := h.aiProject(c, req.ProjectID)
	if !ok {
		return
	}
	tasks, err := h.aiVisibleProjectTasks(c, project.ID)
	if err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_AI_TASKS_FAILED", err)
		return
	}
	registers, err := h.aiProjectRegisters(c, project.ID)
	if err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_AI_REGISTERS_FAILED", err)
		return
	}

	now := time.Now()
	acc := projectHealthAccumulator{projectID: project.ID, projectCode: project.Code, projectName: project.Name}
	overdueTasks := make([]model.Task, 0)
	for _, task := range tasks {
		row := projectHealthRow{
			ProjectID:   project.ID,
			ProjectCode: project.Code,
			ProjectName: project.Name,
			TaskID:      task.ID,
			Status:      task.Status,
			Priority:    task.Priority,
			IsMilestone: task.IsMilestone,
			Progress:    task.Progress,
			StartAt:     task.StartAt,
			EndAt:       task.EndAt,
		}
		acc.addTask(row, now)
		if task.EndAt != nil && task.EndAt.Before(now) && task.Status != model.TaskCompleted {
			overdueTasks = append(overdueTasks, task)
		}
	}
	for _, item := range registers {
		if projectRegisterHighRisk(item) {
			acc.highRiskRegisters++
		}
		if item.Type == model.ProjectRegisterIssue && projectRegisterOpen(item.Status) {
			acc.unresolvedIssues++
		}
	}
	if h.canReadAITaskSource(c) {
		criticalCounts, err := h.criticalOverdueByVisibleProject(c, now)
		if err != nil {
			respondDBError(c, http.StatusInternalServerError, "QUERY_AI_CRITICAL_PATH_FAILED", err)
			return
		}
		acc.criticalOverdueTasks = criticalCounts[project.ID]
	}
	health := calculateProjectHealth(acc)

	recommendations := make([]string, 0, 4)
	if health.OverdueTasks > 0 {
		recommendations = append(recommendations, "优先确认逾期任务的阻塞原因，并给出新的负责人和日期承诺")
	}
	if health.CriticalOverdueTasks > 0 {
		recommendations = append(recommendations, "关键路径逾期任务需要单独升级，避免继续挤压项目完成时间")
	}
	if health.HighRiskRegisters > 0 {
		recommendations = append(recommendations, "将未关闭高风险登记项补齐缓解措施、责任人和截止时间")
	}
	if health.UnresolvedIssues > 0 {
		recommendations = append(recommendations, "把未解决问题拆成可执行任务，并纳入下一次周会跟踪")
	}
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "当前健康度未暴露明显风险，建议继续保持任务活动和登记册更新")
	}

	var builder strings.Builder
	builder.WriteString("# AI 风险摘要：" + project.Code + " - " + project.Name + "\n\n")
	builder.WriteString("健康状态：" + health.Health + "，评分：" + strconv.Itoa(health.Score) + "\n\n")
	builder.WriteString("## 主要原因\n")
	for _, reason := range health.Reasons {
		builder.WriteString("- " + reason + "\n")
	}
	if len(overdueTasks) > 0 {
		builder.WriteString("\n## 逾期任务\n")
		for index, task := range overdueTasks {
			if index >= 8 {
				break
			}
			builder.WriteString("- " + strings.TrimSpace(task.TaskNo+" "+task.Title) + "\n")
		}
	}
	relatedRegisters := make([]model.ProjectRegister, 0)
	for _, item := range registers {
		if projectRegisterHighRisk(item) || (item.Type == model.ProjectRegisterIssue && projectRegisterOpen(item.Status)) {
			relatedRegisters = append(relatedRegisters, item)
		}
	}
	if len(relatedRegisters) > 0 {
		builder.WriteString("\n## 关联登记项\n")
		for _, item := range relatedRegisters {
			builder.WriteString("- " + projectRegisterTypeLabel(item.Type) + "：" + item.Title + "（" + string(item.Severity) + "）\n")
		}
	}
	builder.WriteString("\n## 建议动作\n")
	for _, item := range recommendations {
		builder.WriteString("- " + item + "\n")
	}

	seen := map[string]bool{}
	sources := []aiSourceRef{}
	sources = appendAISource(sources, seen, aiProjectSource(project))
	for index, task := range overdueTasks {
		if index >= 8 {
			break
		}
		sources = appendAISource(sources, seen, aiTaskSource(task))
	}
	for _, item := range relatedRegisters {
		sources = appendAISource(sources, seen, aiRegisterSource(item))
	}

	fallbackDraft := builder.String()
	contextData := "<context>\n" + fallbackDraft + "\n</context>"
	draft := h.aiComposeNarrative(c.Request.Context(), h.aiPrompts.riskSummary, contextData, fallbackDraft)

	c.JSON(http.StatusOK, aiDraftResponse{
		Mode:                 "risk_summary",
		Title:                "AI 风险摘要",
		Draft:                draft,
		Highlights:           health.Reasons,
		Recommendations:      recommendations,
		SourceRefs:           sources,
		RequiresConfirmation: true,
		GeneratedAt:          time.Now(),
	})
}

func (h *Handler) AITaskBreakdown(c *gin.Context) {
	var req aiTaskBreakdownRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	title := strings.TrimSpace(req.Title)
	description := strings.TrimSpace(req.Description)
	var project model.Project
	sources := []aiSourceRef{}
	seen := map[string]bool{}
	if req.ProjectID > 0 {
		var ok bool
		project, ok = h.aiProject(c, req.ProjectID)
		if !ok {
			return
		}
		if title == "" {
			title = project.Name
		}
		if description == "" {
			description = project.Description
		}
		sources = appendAISource(sources, seen, aiProjectSource(project))
	}
	if title == "" && description == "" {
		respondError(c, http.StatusBadRequest, "AI_TASK_BREAKDOWN_INPUT_REQUIRED", "请提供项目或任务拆解描述")
		return
	}

	subject := title
	if subject == "" {
		subject = "项目任务"
	}
	tasks := []aiSuggestedTask{
		{
			Title:            subject + " - 需求确认与范围拆解",
			Description:      "确认目标、边界、验收标准和关键干系人，输出可执行范围清单。",
			Priority:         "high",
			RelativeStartDay: 0,
			DurationDays:     2,
			SourceRefs:       sources,
		},
		{
			Title:            subject + " - 计划排期与风险校准",
			Description:      "拆分里程碑、识别依赖和风险，确认负责人、评审人和排期。",
			Priority:         "high",
			RelativeStartDay: 2,
			DurationDays:     2,
			SourceRefs:       sources,
		},
		{
			Title:            subject + " - 核心交付执行",
			Description:      "按优先级推进核心任务，保持任务活动、评论和进度同步。",
			Priority:         "medium",
			RelativeStartDay: 4,
			DurationDays:     5,
			SourceRefs:       sources,
		},
		{
			Title:            subject + " - 验收与复盘",
			Description:      "完成验收确认、遗留问题归档和复盘结论沉淀。",
			Priority:         "medium",
			IsMilestone:      true,
			RelativeStartDay: 9,
			DurationDays:     1,
			SourceRefs:       sources,
		},
	}
	lower := strings.ToLower(description)
	if strings.Contains(description, "上线") || strings.Contains(lower, "release") || strings.Contains(lower, "launch") {
		tasks = append(tasks, aiSuggestedTask{
			Title:            subject + " - 上线准备与回滚预案",
			Description:      "确认发布窗口、回滚方案、通知对象和上线后观察指标。",
			Priority:         "high",
			IsMilestone:      true,
			RelativeStartDay: 8,
			DurationDays:     1,
			SourceRefs:       sources,
		})
	}

	if h.AIClient != nil {
		var ctxBuilder strings.Builder
		ctxBuilder.WriteString("标题：" + subject + "\n")
		if description != "" {
			ctxBuilder.WriteString("描述：" + description + "\n")
		}
		if req.ProjectID > 0 {
			ctxBuilder.WriteString("所属项目：" + project.Code + " - " + project.Name + "\n")
		}
		contextData := "<context>\n" + ctxBuilder.String() + "</context>"
		if reply, err := h.AIClient.Chat(c.Request.Context(), []ai.Message{
			{Role: ai.RoleSystem, Content: h.aiPrompts.taskBreakdown},
			{Role: ai.RoleUser, Content: contextData},
		}); err == nil {
			if parsed, ok := aiParseSuggestedTasks(reply, sources); ok {
				tasks = parsed
			}
		}
	}

	sort.SliceStable(tasks, func(i, j int) bool {
		return tasks[i].RelativeStartDay < tasks[j].RelativeStartDay
	})
	summary := "已生成 " + strconv.Itoa(len(tasks)) + " 条任务草稿，需项目经理确认后再创建真实任务。"
	c.JSON(http.StatusOK, aiTaskBreakdownResponse{
		Mode:                 "task_breakdown",
		Title:                "AI 任务拆解建议",
		Summary:              summary,
		Tasks:                tasks,
		SourceRefs:           sources,
		RequiresConfirmation: true,
		GeneratedAt:          time.Now(),
	})
}
