package handler

import (
	"net/http"
	"strconv"
	"strings"

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

func normalizeSavedReportType(value string) (model.SavedReportType, bool) {
	switch model.SavedReportType(strings.TrimSpace(value)) {
	case model.SavedReportProjectHealth, model.SavedReportMemberWorkload, model.SavedReportTaskStatus:
		return model.SavedReportType(strings.TrimSpace(value)), true
	default:
		return model.SavedReportProjectHealth, false
	}
}

func normalizeSavedReportFilters(filters model.SavedReportFilters) model.SavedReportFilters {
	return model.SavedReportFilters{
		ProjectID: filters.ProjectID,
		Keyword:   strings.TrimSpace(filters.Keyword),
		Statuses:  parseTaskStatuses(strings.Join(filters.Statuses, ",")),
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
		return true
	}
	return h.ensureProjectVisible(c, strconv.FormatUint(uint64(filters.ProjectID), 10))
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
