package handler

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"mime"
	"net/http"
	"net/smtp"
	"sort"
	"strconv"
	"strings"
	"time"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type savedReportRequest struct {
	Name        string                       `json:"name" binding:"required"`
	Description string                       `json:"description"`
	Type        string                       `json:"type" binding:"required"`
	Filters     model.SavedReportFilters     `json:"filters"`
	ChartConfig model.SavedReportChartConfig `json:"chartConfig"`
}

type reportSubscriptionRequest struct {
	IsEnabled        bool     `json:"isEnabled"`
	Schedule         string   `json:"schedule"`
	Weekday          int      `json:"weekday"`
	Hour             int      `json:"hour"`
	Channels         []string `json:"channels"`
	RecipientUserIDs []uint   `json:"recipientUserIds"`
}

type reportMetric struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Tone  string `json:"tone,omitempty"`
}

type reportColumn struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type reportRunResult struct {
	ReportID    uint                         `json:"reportId"`
	Name        string                       `json:"name"`
	Type        model.SavedReportType        `json:"type"`
	Filters     model.SavedReportFilters     `json:"filters"`
	ChartConfig model.SavedReportChartConfig `json:"chartConfig"`
	GeneratedAt time.Time                    `json:"generatedAt"`
	Summary     []reportMetric               `json:"summary"`
	Columns     []reportColumn               `json:"columns"`
	Rows        []map[string]any             `json:"rows"`
}

func normalizeSavedReportType(value string) (model.SavedReportType, bool) {
	switch model.SavedReportType(strings.TrimSpace(value)) {
	case model.SavedReportProjectHealth,
		model.SavedReportMemberWorkload,
		model.SavedReportTaskStatus,
		model.SavedReportTaskThroughput,
		model.SavedReportOverdueTrend,
		model.SavedReportDepartmentDistribution:
		return model.SavedReportType(strings.TrimSpace(value)), true
	default:
		return model.SavedReportProjectHealth, false
	}
}

func normalizeSavedReportFilters(filters model.SavedReportFilters) model.SavedReportFilters {
	return model.SavedReportFilters{
		ProjectID:    filters.ProjectID,
		DepartmentID: filters.DepartmentID,
		OwnerID:      filters.OwnerID,
		DateFrom:     strings.TrimSpace(filters.DateFrom),
		DateTo:       strings.TrimSpace(filters.DateTo),
		Keyword:      strings.TrimSpace(filters.Keyword),
		Statuses:     parseTaskStatuses(strings.Join(filters.Statuses, ",")),
	}
}

func normalizeSavedReportChartConfig(config model.SavedReportChartConfig) model.SavedReportChartConfig {
	displayMode := strings.TrimSpace(config.DisplayMode)
	if displayMode == "" {
		displayMode = "summary"
	}
	return model.SavedReportChartConfig{DisplayMode: displayMode}
}

func (h *Handler) validateSavedReportFilters(c *gin.Context, filters model.SavedReportFilters) bool {
	if filters.ProjectID == 0 {
		if filters.DepartmentID > 0 {
			var count int64
			if err := h.DB.Model(&model.Department{}).Where("id = ?", filters.DepartmentID).Count(&count).Error; err != nil {
				respondDBError(c, http.StatusInternalServerError, "QUERY_DEPARTMENT_FAILED", err)
				return false
			}
			if count == 0 {
				respondError(c, http.StatusBadRequest, "INVALID_REPORT_DEPARTMENT", "部门筛选不存在")
				return false
			}
		}
		if filters.OwnerID > 0 {
			var count int64
			if err := h.DB.Model(&model.User{}).Where("id = ?", filters.OwnerID).Count(&count).Error; err != nil {
				respondDBError(c, http.StatusInternalServerError, "QUERY_REPORT_OWNER_FAILED", err)
				return false
			}
			if count == 0 {
				respondError(c, http.StatusBadRequest, "INVALID_REPORT_OWNER", "负责人筛选不存在")
				return false
			}
		}
		if _, ok := parseReportDateFilter(filters.DateFrom); !ok && strings.TrimSpace(filters.DateFrom) != "" {
			respondError(c, http.StatusBadRequest, "INVALID_REPORT_DATE_FROM", "开始日期格式必须是 YYYY-MM-DD")
			return false
		}
		if _, ok := parseReportDateFilter(filters.DateTo); !ok && strings.TrimSpace(filters.DateTo) != "" {
			respondError(c, http.StatusBadRequest, "INVALID_REPORT_DATE_TO", "结束日期格式必须是 YYYY-MM-DD")
			return false
		}
		return true
	}
	if !h.ensureProjectVisible(c, strconv.FormatUint(uint64(filters.ProjectID), 10)) {
		return false
	}
	next := filters
	next.ProjectID = 0
	return h.validateSavedReportFilters(c, next)
}

func (h *Handler) scopeSavedReportsQuery(c *gin.Context, query *gorm.DB) *gorm.DB {
	if h.currentUserIsAdmin(c) {
		return query
	}
	return query.Where("saved_reports.created_by_id = ?", c.GetUint("userId"))
}

func (h *Handler) ensureSavedReportVisible(c *gin.Context, id string) (*model.SavedReport, bool) {
	var item model.SavedReport
	query := h.scopeSavedReportsQuery(c, h.DB.Model(&model.SavedReport{})).Preload("CreatedBy")
	if err := query.Where("saved_reports.id = ?", id).First(&item).Error; err != nil {
		respondError(c, http.StatusNotFound, "SAVED_REPORT_NOT_FOUND", "报表不存在")
		return nil, false
	}
	return &item, true
}

func (h *Handler) ListSavedReports(c *gin.Context) {
	page, pageSize := parsePage(c)
	query := h.scopeSavedReportsQuery(c, h.DB.Model(&model.SavedReport{}))
	if reportType, ok := normalizeSavedReportType(c.Query("type")); ok && strings.TrimSpace(c.Query("type")) != "" {
		query = query.Where("saved_reports.type = ?", reportType)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("saved_reports.name LIKE ? OR saved_reports.description LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_SAVED_REPORTS_FAILED", err)
		return
	}
	var items []model.SavedReport
	if err := query.Preload("CreatedBy").
		Order("saved_reports.updated_at desc, saved_reports.id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_SAVED_REPORTS_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, pageResult[model.SavedReport]{List: items, Total: total, Page: page, PageSize: pageSize})
}

func (h *Handler) SavedReportDetail(c *gin.Context) {
	item, visible := h.ensureSavedReportVisible(c, c.Param("id"))
	if !visible {
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) CreateSavedReport(c *gin.Context) {
	var req savedReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	reportType, ok := normalizeSavedReportType(req.Type)
	if !ok {
		respondError(c, http.StatusBadRequest, "INVALID_REPORT_TYPE", "报表类型必须是 project_health、member_workload 或 task_status")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		respondError(c, http.StatusBadRequest, "SAVED_REPORT_NAME_REQUIRED", "报表名称不能为空")
		return
	}
	filters := normalizeSavedReportFilters(req.Filters)
	if !h.validateSavedReportFilters(c, filters) {
		return
	}
	item := model.SavedReport{
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		Type:        reportType,
		Filters:     filters,
		ChartConfig: normalizeSavedReportChartConfig(req.ChartConfig),
		CreatedByID: c.GetUint("userId"),
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "reports", "create", item.ID, true, auditDetailf("创建保存报表(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_SAVED_REPORT_FAILED", err)
		return
	}
	if err := h.DB.Preload("CreatedBy").First(&item, item.ID).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_SAVED_REPORT_FAILED", err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) UpdateSavedReport(c *gin.Context) {
	var req savedReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	item, visible := h.ensureSavedReportVisible(c, c.Param("id"))
	if !visible {
		return
	}
	reportType, ok := normalizeSavedReportType(req.Type)
	if !ok {
		respondError(c, http.StatusBadRequest, "INVALID_REPORT_TYPE", "报表类型必须是 project_health、member_workload 或 task_status")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		respondError(c, http.StatusBadRequest, "SAVED_REPORT_NAME_REQUIRED", "报表名称不能为空")
		return
	}
	filters := normalizeSavedReportFilters(req.Filters)
	if !h.validateSavedReportFilters(c, filters) {
		return
	}
	item.Name = name
	item.Description = strings.TrimSpace(req.Description)
	item.Type = reportType
	item.Filters = filters
	item.ChartConfig = normalizeSavedReportChartConfig(req.ChartConfig)
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(item).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "reports", "update", item.ID, true, auditDetailf("更新保存报表(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "UPDATE_SAVED_REPORT_FAILED", err)
		return
	}
	if err := h.DB.Preload("CreatedBy").First(item, item.ID).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_SAVED_REPORT_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteSavedReport(c *gin.Context) {
	item, visible := h.ensureSavedReportVisible(c, c.Param("id"))
	if !visible {
		return
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(item).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "reports", "delete", item.ID, true, auditDetailf("删除保存报表(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusInternalServerError, "DELETE_SAVED_REPORT_FAILED", err)
		return
	}
	respondMessage(c, http.StatusOK, "SAVED_REPORT_DELETED", "删除成功")
}

func (h *Handler) RunSavedReport(c *gin.Context) {
	item, visible := h.ensureSavedReportVisible(c, c.Param("id"))
	if !visible {
		return
	}
	result, err := h.buildSavedReportResult(c, item)
	if err != nil {
		respondDBError(c, http.StatusInternalServerError, "RUN_SAVED_REPORT_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) ExportSavedReportCSV(c *gin.Context) {
	item, visible := h.ensureSavedReportVisible(c, c.Param("id"))
	if !visible {
		return
	}
	result, err := h.buildSavedReportResult(c, item)
	if err != nil {
		respondDBError(c, http.StatusInternalServerError, "EXPORT_SAVED_REPORT_FAILED", err)
		return
	}

	filename := strings.TrimSpace(item.Name)
	if filename == "" {
		filename = "report"
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename*=UTF-8''"+urlQueryEscape(filename)+".csv")
	_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	writer := csv.NewWriter(c.Writer)
	headers := make([]string, 0, len(result.Columns))
	for _, column := range result.Columns {
		headers = append(headers, column.Label)
	}
	_ = writer.Write(headers)
	for _, row := range result.Rows {
		record := make([]string, 0, len(result.Columns))
		for _, column := range result.Columns {
			record = append(record, reportCellText(row[column.Key]))
		}
		_ = writer.Write(record)
	}
	writer.Flush()
}

func (h *Handler) buildSavedReportResult(c *gin.Context, item *model.SavedReport) (reportRunResult, error) {
	base := reportRunResult{
		ReportID:    item.ID,
		Name:        item.Name,
		Type:        item.Type,
		Filters:     item.Filters,
		ChartConfig: item.ChartConfig,
		GeneratedAt: time.Now(),
	}
	switch item.Type {
	case model.SavedReportProjectHealth:
		return h.buildProjectHealthReport(c, item, base)
	case model.SavedReportMemberWorkload:
		return h.buildMemberWorkloadReport(c, item, base)
	case model.SavedReportTaskStatus:
		return h.buildTaskStatusReport(c, item, base)
	case model.SavedReportTaskThroughput:
		return h.buildTaskThroughputReport(c, item, base)
	case model.SavedReportOverdueTrend:
		return h.buildOverdueTrendReport(c, item, base)
	case model.SavedReportDepartmentDistribution:
		return h.buildDepartmentDistributionReport(c, item, base)
	default:
		return base, fmt.Errorf("unsupported report type: %s", item.Type)
	}
}

func (h *Handler) buildProjectHealthReport(c *gin.Context, item *model.SavedReport, result reportRunResult) (reportRunResult, error) {
	now := time.Now()
	query := h.applySavedReportTaskFilters(c, h.DB.Model(&model.Task{}), item.Filters).
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
	if err := query.Scan(&rows).Error; err != nil {
		return result, err
	}

	projects := map[uint]*projectHealthAccumulator{}
	for _, row := range rows {
		acc, ok := projects[row.ProjectID]
		if !ok {
			acc = &projectHealthAccumulator{projectID: row.ProjectID, projectCode: row.ProjectCode, projectName: row.ProjectName}
			projects[row.ProjectID] = acc
		}
		acc.addTask(row, now)
	}

	tasks, err := h.reportVisibleTasks(c, item.Filters)
	if err != nil {
		return result, err
	}
	for projectID, count := range criticalOverdueByProjectFromTasks(tasks, now) {
		if acc, ok := projects[projectID]; ok {
			acc.criticalOverdueTasks = count
		}
	}
	registerCounts, err := h.projectRegisterHealthCountsForReport(c, item.Filters)
	if err != nil {
		return result, err
	}
	for projectID, counts := range registerCounts {
		acc, ok := projects[projectID]
		if !ok {
			acc = &projectHealthAccumulator{projectID: projectID, projectCode: counts.ProjectCode, projectName: counts.ProjectName}
			projects[projectID] = acc
		}
		acc.highRiskRegisters = counts.HighRiskRegisters
		acc.unresolvedIssues = counts.UnresolvedIssues
	}

	items := make([]projectHealthItem, 0, len(projects))
	for _, acc := range projects {
		items = append(items, calculateProjectHealth(*acc))
	}
	sortProjectHealth(items)

	red, yellow, overdue := 0, 0, int64(0)
	result.Columns = []reportColumn{
		{Key: "project", Label: "项目"},
		{Key: "health", Label: "健康度"},
		{Key: "score", Label: "得分"},
		{Key: "completionRate", Label: "完成率"},
		{Key: "overdueTasks", Label: "逾期任务"},
		{Key: "criticalOverdueTasks", Label: "关键逾期"},
		{Key: "highRiskRegisters", Label: "高风险项"},
		{Key: "unresolvedIssues", Label: "未解决问题"},
		{Key: "reasons", Label: "原因"},
	}
	result.Rows = make([]map[string]any, 0, len(items))
	for _, health := range items {
		if health.Health == "red" {
			red++
		}
		if health.Health == "yellow" {
			yellow++
		}
		overdue += health.OverdueTasks
		result.Rows = append(result.Rows, map[string]any{
			"projectId":            health.ProjectID,
			"project":              strings.TrimSpace(health.ProjectCode + " - " + health.ProjectName),
			"health":               projectHealthLabel(health.Health),
			"score":                health.Score,
			"completionRate":       percentText(health.CompletionRate),
			"overdueTasks":         health.OverdueTasks,
			"criticalOverdueTasks": health.CriticalOverdueTasks,
			"highRiskRegisters":    health.HighRiskRegisters,
			"unresolvedIssues":     health.UnresolvedIssues,
			"reasons":              strings.Join(health.Reasons, "；"),
		})
	}
	result.Summary = []reportMetric{
		{Label: "项目数", Value: strconv.Itoa(len(items))},
		{Label: "高风险项目", Value: strconv.Itoa(red), Tone: "danger"},
		{Label: "关注项目", Value: strconv.Itoa(yellow), Tone: "warning"},
		{Label: "逾期任务", Value: strconv.FormatInt(overdue, 10), Tone: "danger"},
	}
	return result, nil
}

func (h *Handler) buildMemberWorkloadReport(c *gin.Context, item *model.SavedReport, result reportRunResult) (reportRunResult, error) {
	periodStart, periodEnd := reportPeriodRange(item.Filters, time.Now())
	query := h.applySavedReportTaskFilters(c, h.DB.Model(&model.Task{}), item.Filters).
		Select(`
			users.id AS user_id,
			users.name,
			users.username,
			users.email,
			COUNT(DISTINCT tasks.id) AS task_count,
			COALESCE(SUM(tasks.estimated_hours), 0) AS estimated_hours,
			COALESCE(SUM(tasks.actual_hours), 0) AS actual_hours,
			COALESCE(SUM(tasks.remaining_hours), 0) AS remaining_hours,
			COALESCE(users.weekly_capacity_hours, 40) AS capacity_hours
		`).
		Joins("JOIN task_users workload_task_users ON workload_task_users.task_id = tasks.id").
		Joins("JOIN users ON users.id = workload_task_users.user_id").
		Where("tasks.status <> ?", model.TaskCompleted).
		Where(`(
			(tasks.start_at IS NULL AND tasks.end_at IS NULL)
			OR (tasks.start_at IS NULL AND tasks.end_at >= ?)
			OR (tasks.end_at IS NULL AND tasks.start_at <= ?)
			OR (tasks.start_at <= ? AND tasks.end_at >= ?)
		)`, periodStart, periodEnd, periodEnd, periodStart).
		Group("users.id, users.name, users.username, users.email, users.weekly_capacity_hours").
		Order("estimated_hours desc, users.name asc")

	var items []memberWorkloadItem
	if err := query.Scan(&items).Error; err != nil {
		return result, err
	}
	overloaded := 0
	totalEstimated := 0.0
	result.Columns = []reportColumn{
		{Key: "member", Label: "成员"},
		{Key: "taskCount", Label: "任务数"},
		{Key: "estimatedHours", Label: "估算工时"},
		{Key: "actualHours", Label: "实际工时"},
		{Key: "remainingHours", Label: "剩余工时"},
		{Key: "capacityHours", Label: "周容量"},
		{Key: "utilizationRate", Label: "利用率"},
		{Key: "overloaded", Label: "是否过载"},
	}
	result.Rows = make([]map[string]any, 0, len(items))
	for index := range items {
		if items[index].CapacityHours > 0 {
			items[index].UtilizationRate = items[index].EstimatedHours / items[index].CapacityHours
			items[index].Overloaded = items[index].EstimatedHours > items[index].CapacityHours
		} else {
			items[index].Overloaded = items[index].EstimatedHours > 0
		}
		if items[index].Overloaded {
			overloaded++
		}
		totalEstimated += items[index].EstimatedHours
		result.Rows = append(result.Rows, map[string]any{
			"userId":          items[index].UserID,
			"member":          displayUserName(items[index].Name, items[index].Username),
			"taskCount":       items[index].TaskCount,
			"estimatedHours":  hoursText(items[index].EstimatedHours),
			"actualHours":     hoursText(items[index].ActualHours),
			"remainingHours":  hoursText(items[index].RemainingHours),
			"capacityHours":   hoursText(items[index].CapacityHours),
			"utilizationRate": percentText(items[index].UtilizationRate),
			"overloaded":      yesNoText(items[index].Overloaded),
		})
	}
	result.Summary = []reportMetric{
		{Label: "成员数", Value: strconv.Itoa(len(items))},
		{Label: "过载成员", Value: strconv.Itoa(overloaded), Tone: "danger"},
		{Label: "估算工时", Value: hoursText(totalEstimated)},
		{Label: "统计周期", Value: periodStart.Format("2006-01-02") + " 至 " + periodEnd.Format("2006-01-02")},
	}
	return result, nil
}

func (h *Handler) buildTaskStatusReport(c *gin.Context, item *model.SavedReport, result reportRunResult) (reportRunResult, error) {
	type statusRow struct {
		Status string
		Count  int64
	}
	var rows []statusRow
	query := h.applySavedReportTaskFilters(c, h.DB.Model(&model.Task{}), item.Filters).
		Select("tasks.status, count(*) as count").
		Group("tasks.status")
	if err := query.Scan(&rows).Error; err != nil {
		return result, err
	}
	byStatus := map[string]int64{}
	total := int64(0)
	for _, row := range rows {
		byStatus[row.Status] = row.Count
		total += row.Count
	}
	result.Columns = []reportColumn{
		{Key: "status", Label: "状态"},
		{Key: "count", Label: "任务数"},
		{Key: "share", Label: "占比"},
	}
	statuses := []model.TaskStatus{model.TaskPending, model.TaskQueued, model.TaskProcessing, model.TaskReviewing, model.TaskCompleted}
	result.Rows = make([]map[string]any, 0, len(statuses))
	for _, status := range statuses {
		count := byStatus[string(status)]
		share := 0.0
		if total > 0 {
			share = float64(count) / float64(total)
		}
		result.Rows = append(result.Rows, map[string]any{
			"status": taskStatusLabel(status),
			"count":  count,
			"share":  percentText(share),
		})
	}
	result.Summary = []reportMetric{
		{Label: "任务总数", Value: strconv.FormatInt(total, 10)},
		{Label: "已完成", Value: strconv.FormatInt(byStatus[string(model.TaskCompleted)], 10)},
		{Label: "待审核", Value: strconv.FormatInt(byStatus[string(model.TaskReviewing)], 10), Tone: "warning"},
	}
	return result, nil
}

func (h *Handler) buildTaskThroughputReport(c *gin.Context, item *model.SavedReport, result reportRunResult) (reportRunResult, error) {
	periodStart, periodEnd := reportPeriodRange(item.Filters, time.Now())
	created, err := h.taskCountByDate(c, item.Filters, "tasks.created_at", periodStart, periodEnd, "")
	if err != nil {
		return result, err
	}
	completed, err := h.taskCountByDate(c, item.Filters, "tasks.updated_at", periodStart, periodEnd, "tasks.status = 'completed'")
	if err != nil {
		return result, err
	}
	result.Columns = []reportColumn{
		{Key: "date", Label: "日期"},
		{Key: "createdTasks", Label: "新增任务"},
		{Key: "completedTasks", Label: "完成任务"},
		{Key: "netChange", Label: "净增"},
	}
	totalCreated, totalCompleted := int64(0), int64(0)
	for day := periodStart; !day.After(periodEnd); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		cCount := created[key]
		doneCount := completed[key]
		totalCreated += cCount
		totalCompleted += doneCount
		result.Rows = append(result.Rows, map[string]any{
			"date":           key,
			"createdTasks":   cCount,
			"completedTasks": doneCount,
			"netChange":      cCount - doneCount,
		})
	}
	result.Summary = []reportMetric{
		{Label: "新增任务", Value: strconv.FormatInt(totalCreated, 10)},
		{Label: "完成任务", Value: strconv.FormatInt(totalCompleted, 10)},
		{Label: "净增", Value: strconv.FormatInt(totalCreated-totalCompleted, 10)},
	}
	return result, nil
}

func (h *Handler) buildOverdueTrendReport(c *gin.Context, item *model.SavedReport, result reportRunResult) (reportRunResult, error) {
	periodStart, periodEnd := reportPeriodRange(item.Filters, time.Now())
	type overdueRow struct {
		Date  string
		Count int64
	}
	var rows []overdueRow
	query := h.applySavedReportTaskFilters(c, h.DB.Model(&model.Task{}), item.Filters).
		Select("DATE(tasks.end_at) AS date, COUNT(*) AS count").
		Where("tasks.end_at IS NOT NULL AND tasks.end_at BETWEEN ? AND ?", periodStart, periodEnd).
		Where("tasks.end_at < ? AND tasks.status <> ?", time.Now(), model.TaskCompleted).
		Group("DATE(tasks.end_at)").
		Order("date asc")
	if err := query.Scan(&rows).Error; err != nil {
		return result, err
	}
	byDate := map[string]int64{}
	total := int64(0)
	for _, row := range rows {
		byDate[row.Date] = row.Count
		total += row.Count
	}
	result.Columns = []reportColumn{
		{Key: "date", Label: "到期日期"},
		{Key: "overdueTasks", Label: "逾期任务"},
	}
	for day := periodStart; !day.After(periodEnd); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		result.Rows = append(result.Rows, map[string]any{
			"date":         key,
			"overdueTasks": byDate[key],
		})
	}
	result.Summary = []reportMetric{
		{Label: "逾期任务", Value: strconv.FormatInt(total, 10), Tone: "danger"},
		{Label: "统计周期", Value: periodStart.Format("2006-01-02") + " 至 " + periodEnd.Format("2006-01-02")},
	}
	return result, nil
}

func (h *Handler) buildDepartmentDistributionReport(c *gin.Context, item *model.SavedReport, result reportRunResult) (reportRunResult, error) {
	type departmentRow struct {
		DepartmentID       uint
		DepartmentName     string
		ProjectCount       int64
		TaskCount          int64
		CompletedTaskCount int64
	}
	visibleProjects := h.scopeProjectsQuery(c, h.DB.Model(&model.Project{}).Select("projects.id"))
	visibleTasks := h.applySavedReportTaskFilters(c, h.DB.Model(&model.Task{}), item.Filters).
		Select("tasks.id, tasks.project_id, tasks.status")

	query := h.DB.Table("departments").
		Select(`
			departments.id AS department_id,
			departments.name AS department_name,
			COUNT(DISTINCT projects.id) AS project_count,
			COUNT(DISTINCT visible_tasks.id) AS task_count,
			COALESCE(SUM(CASE WHEN visible_tasks.status = ? THEN 1 ELSE 0 END), 0) AS completed_task_count
		`, model.TaskCompleted).
		Joins("JOIN project_departments ON project_departments.department_id = departments.id").
		Joins("JOIN projects ON projects.id = project_departments.project_id").
		Joins("LEFT JOIN (?) visible_tasks ON visible_tasks.project_id = projects.id", visibleTasks).
		Where("projects.id IN (?)", visibleProjects)
	if item.Filters.ProjectID > 0 {
		query = query.Where("projects.id = ?", item.Filters.ProjectID)
	}
	if item.Filters.DepartmentID > 0 {
		query = query.Where("departments.id = ?", item.Filters.DepartmentID)
	}
	if keyword := strings.TrimSpace(item.Filters.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("departments.name LIKE ? OR departments.description LIKE ? OR projects.name LIKE ? OR projects.code LIKE ?", like, like, like, like)
	}
	query = query.Group("departments.id, departments.name").Order("project_count desc, task_count desc, departments.name asc")

	var rows []departmentRow
	if err := query.Scan(&rows).Error; err != nil {
		return result, err
	}
	result.Columns = []reportColumn{
		{Key: "department", Label: "部门"},
		{Key: "projectCount", Label: "项目数"},
		{Key: "taskCount", Label: "任务数"},
		{Key: "completedTaskCount", Label: "完成任务"},
		{Key: "completionRate", Label: "完成率"},
	}
	totalProjects, totalTasks := int64(0), int64(0)
	result.Rows = make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		rate := 0.0
		if row.TaskCount > 0 {
			rate = float64(row.CompletedTaskCount) / float64(row.TaskCount)
		}
		totalProjects += row.ProjectCount
		totalTasks += row.TaskCount
		result.Rows = append(result.Rows, map[string]any{
			"departmentId":       row.DepartmentID,
			"department":         row.DepartmentName,
			"projectCount":       row.ProjectCount,
			"taskCount":          row.TaskCount,
			"completedTaskCount": row.CompletedTaskCount,
			"completionRate":     percentText(rate),
		})
	}
	result.Summary = []reportMetric{
		{Label: "部门数", Value: strconv.Itoa(len(rows))},
		{Label: "项目数", Value: strconv.FormatInt(totalProjects, 10)},
		{Label: "任务数", Value: strconv.FormatInt(totalTasks, 10)},
	}
	return result, nil
}

func (h *Handler) applySavedReportTaskFilters(c *gin.Context, query *gorm.DB, filters model.SavedReportFilters) *gorm.DB {
	query = h.scopeTasksQuery(c, query)
	if filters.ProjectID > 0 {
		query = query.Where("tasks.project_id = ?", filters.ProjectID)
	}
	if filters.DepartmentID > 0 {
		query = query.Joins("JOIN project_departments report_pd ON report_pd.project_id = tasks.project_id").
			Where("report_pd.department_id = ?", filters.DepartmentID)
	}
	if filters.OwnerID > 0 {
		query = query.Where("EXISTS (SELECT 1 FROM task_users report_owner_tu WHERE report_owner_tu.task_id = tasks.id AND report_owner_tu.user_id = ?)", filters.OwnerID)
	}
	if len(filters.Statuses) > 0 {
		query = query.Where("tasks.status IN ?", filters.Statuses)
	}
	if keyword := strings.TrimSpace(filters.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Joins("JOIN projects report_projects ON report_projects.id = tasks.project_id").
			Where("tasks.task_no LIKE ? OR tasks.title LIKE ? OR tasks.description LIKE ? OR report_projects.code LIKE ? OR report_projects.name LIKE ?", like, like, like, like, like)
	}
	if from, ok := parseReportDateFilter(filters.DateFrom); ok {
		query = query.Where("tasks.end_at IS NULL OR tasks.end_at >= ?", from)
	}
	if to, ok := parseReportDateFilter(filters.DateTo); ok {
		query = query.Where("tasks.start_at IS NULL OR tasks.start_at <= ?", endOfDay(to))
	}
	return query
}

func (h *Handler) reportVisibleTasks(c *gin.Context, filters model.SavedReportFilters) ([]model.Task, error) {
	var tasks []model.Task
	query := h.applySavedReportTaskFilters(c, h.DB.Model(&model.Task{}), filters).
		Select("tasks.*").
		Preload("Dependencies").
		Order("tasks.project_id asc, COALESCE(tasks.start_at, tasks.created_at) asc, tasks.id asc")
	if err := query.Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

func criticalOverdueByProjectFromTasks(tasks []model.Task, now time.Time) map[uint]int64 {
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
	return result
}

func (h *Handler) projectRegisterHealthCountsForReport(c *gin.Context, filters model.SavedReportFilters) (map[uint]projectRegisterHealthCount, error) {
	var rows []projectRegisterHealthCount
	query := h.scopeProjectRegistersQuery(c, h.DB.Model(&model.ProjectRegister{})).
		Select(`
			project_registers.project_id AS project_id,
			projects.code AS project_code,
			projects.name AS project_name,
			SUM(CASE WHEN project_registers.type = ? AND project_registers.status IN ? AND project_registers.severity IN ? THEN 1 ELSE 0 END) AS high_risk_registers,
			SUM(CASE WHEN project_registers.type = ? AND project_registers.status IN ? THEN 1 ELSE 0 END) AS unresolved_issues
		`,
			model.ProjectRegisterRisk,
			[]model.ProjectRegisterStatus{model.ProjectRegisterOpen, model.ProjectRegisterInProgress},
			[]model.ProjectRegisterSeverity{model.ProjectRegisterSeverityHigh, model.ProjectRegisterSeverityCritical},
			model.ProjectRegisterIssue,
			[]model.ProjectRegisterStatus{model.ProjectRegisterOpen, model.ProjectRegisterInProgress},
		).
		Joins("JOIN projects ON projects.id = project_registers.project_id")
	if filters.ProjectID > 0 {
		query = query.Where("project_registers.project_id = ?", filters.ProjectID)
	}
	if filters.DepartmentID > 0 {
		query = query.Joins("JOIN project_departments report_register_pd ON report_register_pd.project_id = project_registers.project_id").
			Where("report_register_pd.department_id = ?", filters.DepartmentID)
	}
	if filters.OwnerID > 0 {
		query = query.Where("project_registers.owner_id = ?", filters.OwnerID)
	}
	if keyword := strings.TrimSpace(filters.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("projects.code LIKE ? OR projects.name LIKE ? OR project_registers.title LIKE ? OR project_registers.description LIKE ?", like, like, like, like)
	}
	query = query.Group("project_registers.project_id, projects.code, projects.name")
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[uint]projectRegisterHealthCount, len(rows))
	for _, row := range rows {
		out[row.ProjectID] = row
	}
	return out, nil
}

func (h *Handler) taskCountByDate(c *gin.Context, filters model.SavedReportFilters, dateColumn string, from, to time.Time, extraWhere string) (map[string]int64, error) {
	type dateCountRow struct {
		Date  string
		Count int64
	}
	var rows []dateCountRow
	query := h.applySavedReportTaskFilters(c, h.DB.Model(&model.Task{}), filters).
		Select("DATE("+dateColumn+") AS date, COUNT(*) AS count").
		Where(dateColumn+" BETWEEN ? AND ?", from, endOfDay(to)).
		Group("DATE(" + dateColumn + ")").
		Order("date asc")
	if extraWhere != "" {
		query = query.Where(extraWhere)
	}
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, row := range rows {
		out[row.Date] = row.Count
	}
	return out, nil
}

func parseReportDateFilter(value string) (time.Time, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, false
	}
	parsed, err := time.ParseInLocation("2006-01-02", trimmed, time.Local)
	return parsed, err == nil
}

func reportPeriodRange(filters model.SavedReportFilters, now time.Time) (time.Time, time.Time) {
	from, hasFrom := parseReportDateFilter(filters.DateFrom)
	to, hasTo := parseReportDateFilter(filters.DateTo)
	if !hasTo {
		to = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	}
	if !hasFrom {
		from = to.AddDate(0, 0, -13)
	}
	if from.After(to) {
		return to, to
	}
	return from, to
}

func endOfDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), value.Location())
}

func sortProjectHealth(items []projectHealthItem) {
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
}

func taskStatusLabel(status model.TaskStatus) string {
	switch status {
	case model.TaskQueued:
		return "排队中"
	case model.TaskProcessing:
		return "处理中"
	case model.TaskReviewing:
		return "待审核"
	case model.TaskCompleted:
		return "已完成"
	default:
		return "待处理"
	}
}

func projectHealthLabel(value string) string {
	switch value {
	case "red":
		return "高风险"
	case "yellow":
		return "关注"
	default:
		return "健康"
	}
}

func displayUserName(name, username string) string {
	if strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return strings.TrimSpace(username)
}

func percentText(value float64) string {
	return fmt.Sprintf("%.1f%%", clampPercent(value*100))
}

func hoursText(value float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", value), "0"), ".") + "h"
}

func yesNoText(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

func reportCellText(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func urlQueryEscape(value string) string {
	replacer := strings.NewReplacer("%", "%25", " ", "%20", "\"", "%22", "'", "%27", ";", "%3B", "\r", "", "\n", "")
	return replacer.Replace(value)
}

func normalizeReportSubscriptionRequest(req reportSubscriptionRequest, currentUserID uint) model.ReportSubscription {
	schedule := strings.ToLower(strings.TrimSpace(req.Schedule))
	if schedule == "" {
		schedule = "weekly"
	}
	if schedule != "weekly" {
		schedule = "weekly"
	}
	weekday := req.Weekday
	if weekday < 0 || weekday > 6 {
		weekday = 1
	}
	hour := req.Hour
	if hour < 0 || hour > 23 {
		hour = 9
	}
	channels := normalizeReportChannels(req.Channels)
	if len(channels) == 0 {
		channels = []string{"in_app"}
	}
	recipients := uniqueUint(req.RecipientUserIDs)
	if len(recipients) == 0 && currentUserID > 0 {
		recipients = []uint{currentUserID}
	}
	return model.ReportSubscription{
		IsEnabled:        req.IsEnabled,
		Schedule:         schedule,
		Weekday:          weekday,
		Hour:             hour,
		Channels:         channels,
		RecipientUserIDs: recipients,
	}
}

func normalizeReportChannels(values []string) []string {
	allowed := map[string]struct{}{
		"in_app":   {},
		"email":    {},
		"wecom":    {},
		"dingtalk": {},
		"feishu":   {},
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		channel := strings.ToLower(strings.TrimSpace(value))
		if _, ok := allowed[channel]; !ok {
			continue
		}
		if _, exists := seen[channel]; exists {
			continue
		}
		seen[channel] = struct{}{}
		out = append(out, channel)
	}
	return out
}

func (h *Handler) ReportSubscriptionDetail(c *gin.Context) {
	report, visible := h.ensureSavedReportVisible(c, c.Param("id"))
	if !visible {
		return
	}
	var item model.ReportSubscription
	if err := h.DB.Preload("CreatedBy").
		Where("report_id = ? AND created_by_id = ?", report.ID, c.GetUint("userId")).
		First(&item).Error; err != nil {
		respondError(c, http.StatusNotFound, "REPORT_SUBSCRIPTION_NOT_FOUND", "报表订阅不存在")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) UpsertReportSubscription(c *gin.Context) {
	report, visible := h.ensureSavedReportVisible(c, c.Param("id"))
	if !visible {
		return
	}
	var req reportSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	currentUserID := c.GetUint("userId")
	next := normalizeReportSubscriptionRequest(req, currentUserID)
	var item model.ReportSubscription
	err := h.DB.Where("report_id = ? AND created_by_id = ?", report.ID, currentUserID).First(&item).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		respondDBError(c, http.StatusInternalServerError, "QUERY_REPORT_SUBSCRIPTION_FAILED", err)
		return
	}
	item.ReportID = report.ID
	item.CreatedByID = currentUserID
	item.IsEnabled = next.IsEnabled
	item.Schedule = next.Schedule
	item.Weekday = next.Weekday
	item.Hour = next.Hour
	item.Channels = next.Channels
	item.RecipientUserIDs = next.RecipientUserIDs
	if err := h.DB.Save(&item).Error; err != nil {
		respondDBError(c, http.StatusBadRequest, "SAVE_REPORT_SUBSCRIPTION_FAILED", err)
		return
	}
	if err := h.DB.Preload("CreatedBy").First(&item, item.ID).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_REPORT_SUBSCRIPTION_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteReportSubscription(c *gin.Context) {
	report, visible := h.ensureSavedReportVisible(c, c.Param("id"))
	if !visible {
		return
	}
	var item model.ReportSubscription
	if err := h.DB.Where("report_id = ? AND created_by_id = ?", report.ID, c.GetUint("userId")).First(&item).Error; err != nil {
		respondError(c, http.StatusNotFound, "REPORT_SUBSCRIPTION_NOT_FOUND", "报表订阅不存在")
		return
	}
	if err := h.DB.Delete(&item).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "DELETE_REPORT_SUBSCRIPTION_FAILED", err)
		return
	}
	respondMessage(c, http.StatusOK, "REPORT_SUBSCRIPTION_DELETED", "订阅已取消")
}

func (h *Handler) RunReportSubscriptionNow(c *gin.Context) {
	report, visible := h.ensureSavedReportVisible(c, c.Param("id"))
	if !visible {
		return
	}
	var item model.ReportSubscription
	if err := h.DB.Preload("Report").Where("report_id = ? AND created_by_id = ?", report.ID, c.GetUint("userId")).First(&item).Error; err != nil {
		respondError(c, http.StatusNotFound, "REPORT_SUBSCRIPTION_NOT_FOUND", "报表订阅不存在")
		return
	}
	if err := h.deliverReportSubscription(&item, time.Now(), "manual"); err != nil {
		respondDBError(c, http.StatusBadRequest, "RUN_REPORT_SUBSCRIPTION_FAILED", err)
		return
	}
	if err := h.DB.Preload("CreatedBy").First(&item, item.ID).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_REPORT_SUBSCRIPTION_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) RunDueReportSubscriptions(now time.Time, trigger string) (int, error) {
	var items []model.ReportSubscription
	if err := h.DB.Preload("Report").Where("is_enabled = ?", true).Find(&items).Error; err != nil {
		return 0, err
	}
	executed := 0
	failures := make([]error, 0)
	for index := range items {
		if !reportSubscriptionDue(items[index], now) {
			continue
		}
		executed++
		if err := h.deliverReportSubscription(&items[index], now, trigger); err != nil {
			failures = append(failures, fmt.Errorf("report subscription %d: %w", items[index].ID, err))
		}
	}
	if len(failures) > 0 {
		return executed, errors.Join(failures...)
	}
	return executed, nil
}

func reportSubscriptionDue(item model.ReportSubscription, now time.Time) bool {
	if strings.TrimSpace(item.Schedule) != "weekly" {
		return false
	}
	if int(now.Weekday()) != item.Weekday || now.Hour() < item.Hour {
		return false
	}
	if item.LastRunAt == nil {
		return true
	}
	lastYear, lastWeek := item.LastRunAt.ISOWeek()
	nowYear, nowWeek := now.ISOWeek()
	return lastYear != nowYear || lastWeek != nowWeek
}

func (h *Handler) deliverReportSubscription(item *model.ReportSubscription, now time.Time, trigger string) error {
	reportContext := ginContextForUser(item.CreatedByID)
	result, err := h.buildSavedReportResult(reportContext, &item.Report)
	status := "success"
	errorMessage := ""
	if err == nil {
		title := "项目周报：" + item.Report.Name
		message := renderReportMessage(result, trigger)
		err = h.sendReportSubscriptionMessage(item, title, message)
	}
	if err != nil {
		status = "failed"
		errorMessage = err.Error()
	}
	updateErr := h.DB.Model(item).Updates(map[string]any{
		"last_run_at": now,
		"last_status": status,
		"last_error":  errorMessage,
	}).Error
	if updateErr != nil {
		if err != nil {
			return errors.Join(err, updateErr)
		}
		return updateErr
	}
	return err
}

func ginContextForUser(userID uint) *gin.Context {
	c := &gin.Context{}
	c.Set("userId", userID)
	return c
}

func renderReportMessage(result reportRunResult, trigger string) string {
	lines := []string{
		fmt.Sprintf("报表：%s", result.Name),
		fmt.Sprintf("类型：%s", result.Type),
		fmt.Sprintf("生成时间：%s", result.GeneratedAt.Format("2006-01-02 15:04:05")),
		fmt.Sprintf("触发方式：%s", trigger),
	}
	if len(result.Summary) > 0 {
		lines = append(lines, "", "摘要：")
		for _, metric := range result.Summary {
			lines = append(lines, fmt.Sprintf("- %s：%s", metric.Label, metric.Value))
		}
	}
	if len(result.Rows) > 0 {
		lines = append(lines, "", "明细：")
		limit := len(result.Rows)
		if limit > 8 {
			limit = 8
		}
		for index := 0; index < limit; index++ {
			parts := make([]string, 0, len(result.Columns))
			for _, column := range result.Columns {
				parts = append(parts, column.Label+"="+reportCellText(result.Rows[index][column.Key]))
			}
			lines = append(lines, "- "+strings.Join(parts, "，"))
		}
		if len(result.Rows) > limit {
			lines = append(lines, fmt.Sprintf("- 其余 %d 行请在报表中心查看。", len(result.Rows)-limit))
		}
	}
	return strings.Join(lines, "\n")
}

func (h *Handler) sendReportSubscriptionMessage(item *model.ReportSubscription, title, message string) error {
	channels := normalizeReportChannels(item.Channels)
	if len(channels) == 0 {
		channels = []string{"in_app"}
	}
	recipients := uniqueUint(item.RecipientUserIDs)
	if len(recipients) == 0 {
		recipients = []uint{item.CreatedByID}
	}
	failures := make([]error, 0)
	if containsReportChannel(channels, "in_app") {
		if err := h.createNotificationsWithDB(h.DB, recipients, title, message, "reports", item.ReportID); err != nil {
			failures = append(failures, err)
		} else {
			h.pushNotificationUpdates(recipients)
		}
	}
	recipientDetails, err := h.loadTaskNotifyRecipients(recipients)
	if err != nil {
		failures = append(failures, err)
	} else {
		if containsReportChannel(channels, "email") {
			if err := h.sendReportEmailNotifications(recipientDetails, title, message); err != nil {
				failures = append(failures, err)
			}
		}
		if err := h.sendReportExternalNotifications(channels, title, message, recipientDetails); err != nil {
			failures = append(failures, err)
		}
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	return nil
}

func containsReportChannel(channels []string, target string) bool {
	for _, channel := range channels {
		if channel == target {
			return true
		}
	}
	return false
}

func (h *Handler) sendReportEmailNotifications(recipients []taskNotifyRecipient, title, message string) error {
	if !h.hasEmailTaskNotifyConfigured() {
		return nil
	}
	host := strings.TrimSpace(h.Cfg.SMTPHost)
	port := strings.TrimSpace(h.Cfg.SMTPPort)
	from := strings.TrimSpace(h.Cfg.SMTPFrom)
	username := strings.TrimSpace(h.Cfg.SMTPUsername)
	password := h.Cfg.SMTPPassword
	addr := netJoinHostPort(host, port)

	emails := make([]string, 0, len(recipients))
	seen := make(map[string]struct{}, len(recipients))
	for _, recipient := range recipients {
		email := strings.TrimSpace(recipient.Email)
		if email == "" {
			continue
		}
		if _, exists := seen[email]; exists {
			continue
		}
		seen[email] = struct{}{}
		emails = append(emails, email)
	}
	if len(emails) == 0 {
		return nil
	}

	var auth smtp.Auth
	if username != "" && password != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}
	subject := mime.BEncoding.Encode("UTF-8", "[报表] "+title)
	failures := make([]error, 0)
	for _, email := range emails {
		content := strings.Join([]string{
			"From: " + from,
			"To: " + email,
			"Subject: " + subject,
			"MIME-Version: 1.0",
			"Content-Type: text/plain; charset=UTF-8",
			"",
			message,
		}, "\r\n")
		if err := smtp.SendMail(addr, auth, from, []string{email}, []byte(content)); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", email, err))
		}
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	return nil
}

func (h *Handler) sendReportExternalNotifications(channels []string, title, message string, recipients []taskNotifyRecipient) error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	failures := make([]error, 0)
	if containsReportChannel(channels, "wecom") {
		if err := h.sendWeComTaskNotification(ctx, message); err != nil {
			failures = append(failures, fmt.Errorf("wecom: %w", err))
		}
	}
	if containsReportChannel(channels, "dingtalk") {
		if err := h.sendDingTalkTaskNotification(ctx, title, message); err != nil {
			failures = append(failures, fmt.Errorf("dingtalk: %w", err))
		}
	}
	if containsReportChannel(channels, "feishu") {
		if err := h.sendFeishuTaskNotification(ctx, message, recipients); err != nil {
			failures = append(failures, fmt.Errorf("feishu: %w", err))
		}
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	return nil
}

func (h *Handler) LogReportSubscriptionJob(now time.Time) {
	executedCount, err := h.RunDueReportSubscriptions(now, "scheduled")
	if err != nil {
		log.Printf("report subscription job failed: %v", err)
		return
	}
	if executedCount > 0 {
		log.Printf("report subscription job executed %d subscription(s)", executedCount)
	}
}
