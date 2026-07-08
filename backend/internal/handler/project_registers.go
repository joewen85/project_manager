package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type projectRegisterRequest struct {
	Type           string                         `json:"type" binding:"required"`
	ProjectID      uint                           `json:"projectId" binding:"required"`
	TaskID         *uint                          `json:"taskId"`
	Title          string                         `json:"title" binding:"required"`
	Description    string                         `json:"description"`
	Status         string                         `json:"status"`
	Severity       string                         `json:"severity"`
	Probability    string                         `json:"probability"`
	Impact         string                         `json:"impact"`
	Source         string                         `json:"source"`
	ResponsePlan   string                         `json:"responsePlan"`
	Resolution     string                         `json:"resolution"`
	DecisionDetail string                         `json:"decisionDetail"`
	Background     string                         `json:"background"`
	ImpactScope    string                         `json:"impactScope"`
	DueAt          string                         `json:"dueAt"`
	Images         *[]projectRegisterImageRequest `json:"images"`
	OwnerID        *uint                          `json:"ownerId"`
	ParticipantIDs []uint                         `json:"participantIds"`
}

type projectRegisterImageRequest struct {
	FileName     string `json:"fileName"`
	FilePath     string `json:"filePath"`
	RelativePath string `json:"relativePath"`
	FileSize     int64  `json:"fileSize"`
	MimeType     string `json:"mimeType"`
	Remark       string `json:"remark"`
}

const maxProjectRegisterImageSize = 50 * 1024 * 1024
const maxProjectRegisterImageRemarkLength = 500

func normalizeProjectRegisterType(value string) (model.ProjectRegisterType, bool) {
	switch model.ProjectRegisterType(strings.TrimSpace(value)) {
	case model.ProjectRegisterRisk, model.ProjectRegisterIssue, model.ProjectRegisterDecision:
		return model.ProjectRegisterType(strings.TrimSpace(value)), true
	default:
		return model.ProjectRegisterRisk, false
	}
}

func normalizeProjectRegisterStatus(value string) (model.ProjectRegisterStatus, bool) {
	switch model.ProjectRegisterStatus(strings.TrimSpace(value)) {
	case "":
		return model.ProjectRegisterOpen, true
	case model.ProjectRegisterOpen, model.ProjectRegisterInProgress, model.ProjectRegisterResolved, model.ProjectRegisterClosed:
		return model.ProjectRegisterStatus(strings.TrimSpace(value)), true
	default:
		return model.ProjectRegisterOpen, false
	}
}

func normalizeProjectRegisterSeverity(value string) (model.ProjectRegisterSeverity, bool) {
	switch model.ProjectRegisterSeverity(strings.TrimSpace(value)) {
	case "":
		return model.ProjectRegisterSeverityMedium, true
	case model.ProjectRegisterSeverityLow, model.ProjectRegisterSeverityMedium, model.ProjectRegisterSeverityHigh, model.ProjectRegisterSeverityCritical:
		return model.ProjectRegisterSeverity(strings.TrimSpace(value)), true
	default:
		return model.ProjectRegisterSeverityMedium, false
	}
}

func normalizeOptionalProjectRegisterSeverity(value string) (model.ProjectRegisterSeverity, bool) {
	switch model.ProjectRegisterSeverity(strings.TrimSpace(value)) {
	case "":
		return "", true
	case model.ProjectRegisterSeverityLow, model.ProjectRegisterSeverityMedium, model.ProjectRegisterSeverityHigh, model.ProjectRegisterSeverityCritical:
		return model.ProjectRegisterSeverity(strings.TrimSpace(value)), true
	default:
		return "", false
	}
}

func normalizeProjectRegisterProbability(value string) (model.ProjectRegisterProbability, bool) {
	switch model.ProjectRegisterProbability(strings.TrimSpace(value)) {
	case "":
		return "", true
	case model.ProjectRegisterProbabilityLow, model.ProjectRegisterProbabilityMedium, model.ProjectRegisterProbabilityHigh:
		return model.ProjectRegisterProbability(strings.TrimSpace(value)), true
	default:
		return "", false
	}
}

func parseProjectRegisterStatuses(value string) []string {
	allowed := map[string]struct{}{
		string(model.ProjectRegisterOpen):       {},
		string(model.ProjectRegisterInProgress): {},
		string(model.ProjectRegisterResolved):   {},
		string(model.ProjectRegisterClosed):     {},
	}
	items := parseCSVValues(value)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := allowed[item]; ok {
			out = append(out, item)
		}
	}
	return out
}

func parseProjectRegisterSeverities(value string) []string {
	allowed := map[string]struct{}{
		string(model.ProjectRegisterSeverityLow):      {},
		string(model.ProjectRegisterSeverityMedium):   {},
		string(model.ProjectRegisterSeverityHigh):     {},
		string(model.ProjectRegisterSeverityCritical): {},
	}
	items := parseCSVValues(value)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := allowed[item]; ok {
			out = append(out, item)
		}
	}
	return out
}

func projectRegisterImageAttachment(item projectRegisterImageRequest) attachmentRequest {
	return attachmentRequest{
		FileName:     item.FileName,
		FilePath:     item.FilePath,
		RelativePath: item.RelativePath,
		FileSize:     item.FileSize,
		MimeType:     item.MimeType,
	}
}

func validateProjectRegisterImages(items []projectRegisterImageRequest, publicBase string) error {
	attachments := make([]attachmentRequest, 0, len(items))
	for _, item := range items {
		attachments = append(attachments, projectRegisterImageAttachment(item))
	}
	if err := validateAttachments(attachments, publicBase); err != nil {
		return err
	}
	for _, item := range items {
		attachment := projectRegisterImageAttachment(item)
		if isAttachmentEmpty(attachment) {
			if strings.TrimSpace(item.Remark) != "" {
				return errors.New("图片路径不能为空")
			}
			continue
		}
		if item.FileSize > maxProjectRegisterImageSize {
			return errors.New("单张图片不能大于50M")
		}
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(item.MimeType)), "image/") {
			return errors.New("登记项图片只支持图片文件")
		}
		if utf8.RuneCountInString(strings.TrimSpace(item.Remark)) > maxProjectRegisterImageRemarkLength {
			return errors.New("图片备注不能超过500字")
		}
	}
	return nil
}

func toProjectRegisterImages(items []projectRegisterImageRequest) []model.ProjectRegisterImage {
	if len(items) == 0 {
		return []model.ProjectRegisterImage{}
	}
	result := make([]model.ProjectRegisterImage, 0, len(items))
	for _, item := range items {
		attachment := projectRegisterImageAttachment(item)
		if isAttachmentEmpty(attachment) {
			continue
		}
		result = append(result, model.ProjectRegisterImage{
			FileName:     strings.TrimSpace(item.FileName),
			FilePath:     normalizeAttachmentPath(item.FilePath),
			RelativePath: normalizeRelativeUploadPath(item.RelativePath),
			FileSize:     item.FileSize,
			MimeType:     strings.TrimSpace(item.MimeType),
			Remark:       strings.TrimSpace(item.Remark),
		})
	}
	return result
}

func projectRegisterTypeLabel(value model.ProjectRegisterType) string {
	switch value {
	case model.ProjectRegisterIssue:
		return "问题"
	case model.ProjectRegisterDecision:
		return "决策"
	default:
		return "风险"
	}
}

func projectRegisterOpen(status model.ProjectRegisterStatus) bool {
	return status != model.ProjectRegisterResolved && status != model.ProjectRegisterClosed
}

func projectRegisterHighRisk(item model.ProjectRegister) bool {
	if item.Type != model.ProjectRegisterRisk || !projectRegisterOpen(item.Status) {
		return false
	}
	return item.Severity == model.ProjectRegisterSeverityHigh ||
		item.Severity == model.ProjectRegisterSeverityCritical ||
		(item.Probability == model.ProjectRegisterProbabilityHigh && (item.Impact == model.ProjectRegisterSeverityHigh || item.Impact == model.ProjectRegisterSeverityCritical))
}

func projectRegisterDetail(item model.ProjectRegister) string {
	lines := []string{
		"类型：" + projectRegisterTypeLabel(item.Type),
		"状态：" + string(item.Status),
		"等级：" + string(item.Severity),
	}
	if item.Probability != "" {
		lines = append(lines, "概率："+string(item.Probability))
	}
	if item.Impact != "" {
		lines = append(lines, "影响："+string(item.Impact))
	}
	if item.DueAt != nil {
		lines = append(lines, "截止时间："+formatTaskActivityTime(item.DueAt))
	}
	if item.OwnerID != nil {
		lines = append(lines, "负责人："+strconv.FormatUint(uint64(*item.OwnerID), 10))
	}
	return strings.Join(lines, "\n")
}

func projectRegisterUpdateDetail(oldItem model.ProjectRegister, nextItem model.ProjectRegister) string {
	lines := make([]string, 0, 8)
	lines = appendTaskChange(lines, "标题", oldItem.Title, nextItem.Title)
	lines = appendTaskChange(lines, "状态", oldItem.Status, nextItem.Status)
	lines = appendTaskChange(lines, "等级", oldItem.Severity, nextItem.Severity)
	lines = appendTaskChange(lines, "概率", oldItem.Probability, nextItem.Probability)
	lines = appendTaskChange(lines, "影响", oldItem.Impact, nextItem.Impact)
	lines = appendTaskChange(lines, "负责人", optionalUintText(oldItem.OwnerID), optionalUintText(nextItem.OwnerID))
	lines = appendTaskChange(lines, "截止时间", formatTaskActivityTime(oldItem.DueAt), formatTaskActivityTime(nextItem.DueAt))
	if len(lines) == 0 {
		return "登记项已保存"
	}
	return strings.Join(lines, "\n")
}

func optionalUintText(value *uint) string {
	if value == nil {
		return "未设置"
	}
	return strconv.FormatUint(uint64(*value), 10)
}

func (h *Handler) scopeProjectRegistersQuery(c *gin.Context, query *gorm.DB) *gorm.DB {
	if h.currentUserIsAdmin(c) {
		return query
	}
	visibleProjects := h.scopeProjectsQuery(c, h.DB.Model(&model.Project{}).Select("projects.id"))
	return query.Where("project_registers.project_id IN (?)", visibleProjects)
}

func (h *Handler) ensureProjectRegisterVisible(c *gin.Context, id string, preload bool) (*model.ProjectRegister, bool) {
	var item model.ProjectRegister
	query := h.scopeProjectRegistersQuery(c, h.DB.Model(&model.ProjectRegister{}))
	if preload {
		query = query.Preload("Project").Preload("Task").Preload("Owner").Preload("CreatedBy")
	}
	if err := query.Where("project_registers.id = ?", id).First(&item).Error; err != nil {
		respondError(c, http.StatusNotFound, "PROJECT_REGISTER_NOT_FOUND", "登记项不存在")
		return nil, false
	}
	return &item, true
}

func (h *Handler) preloadProjectRegisterResponse(tx *gorm.DB, item *model.ProjectRegister) error {
	return tx.Preload("Project").Preload("Project.Users").Preload("Task").Preload("Owner").Preload("CreatedBy").First(item, item.ID).Error
}

func (h *Handler) normalizeProjectRegisterRequest(c *gin.Context, req projectRegisterRequest) (model.ProjectRegister, bool) {
	registerType, ok := normalizeProjectRegisterType(req.Type)
	if !ok {
		respondError(c, http.StatusBadRequest, "INVALID_REGISTER_TYPE", "登记类型必须是 risk、issue 或 decision")
		return model.ProjectRegister{}, false
	}
	status, ok := normalizeProjectRegisterStatus(req.Status)
	if !ok {
		respondError(c, http.StatusBadRequest, "INVALID_REGISTER_STATUS", "登记状态必须是 open、in_progress、resolved 或 closed")
		return model.ProjectRegister{}, false
	}
	severity, ok := normalizeProjectRegisterSeverity(req.Severity)
	if !ok {
		respondError(c, http.StatusBadRequest, "INVALID_REGISTER_SEVERITY", "登记等级必须是 low、medium、high 或 critical")
		return model.ProjectRegister{}, false
	}
	impact, ok := normalizeOptionalProjectRegisterSeverity(req.Impact)
	if !ok {
		respondError(c, http.StatusBadRequest, "INVALID_REGISTER_IMPACT", "影响等级必须是 low、medium、high 或 critical")
		return model.ProjectRegister{}, false
	}
	probability, ok := normalizeProjectRegisterProbability(req.Probability)
	if !ok {
		respondError(c, http.StatusBadRequest, "INVALID_REGISTER_PROBABILITY", "概率必须是 low、medium 或 high")
		return model.ProjectRegister{}, false
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		respondError(c, http.StatusBadRequest, "REGISTER_TITLE_REQUIRED", "登记项标题不能为空")
		return model.ProjectRegister{}, false
	}
	if !h.ensureProjectVisible(c, strconv.FormatUint(uint64(req.ProjectID), 10)) {
		return model.ProjectRegister{}, false
	}
	if req.TaskID != nil && *req.TaskID == 0 {
		req.TaskID = nil
	}
	if req.TaskID != nil {
		task, visible := h.ensureTaskVisible(c, strconv.FormatUint(uint64(*req.TaskID), 10))
		if !visible {
			return model.ProjectRegister{}, false
		}
		if task.ProjectID != req.ProjectID {
			respondError(c, http.StatusBadRequest, "REGISTER_TASK_PROJECT_MISMATCH", "关联任务必须属于登记项项目")
			return model.ProjectRegister{}, false
		}
	}
	if req.OwnerID != nil && *req.OwnerID == 0 {
		req.OwnerID = nil
	}
	participantIDs := uniqueUint(req.ParticipantIDs)
	userIDs := append([]uint{}, participantIDs...)
	if req.OwnerID != nil {
		userIDs = append(userIDs, *req.OwnerID)
	}
	userIDs = uniqueUint(userIDs)
	if len(userIDs) > 0 {
		users, err := findUsersByIDs(h.DB, userIDs)
		if err != nil {
			respondDBError(c, http.StatusInternalServerError, "QUERY_REGISTER_USERS_FAILED", err)
			return model.ProjectRegister{}, false
		}
		if len(users) != len(userIDs) {
			respondError(c, http.StatusBadRequest, "INVALID_REGISTER_USERS", "登记项负责人或参与人不存在")
			return model.ProjectRegister{}, false
		}
	}
	dueAt, err := parseRFC3339(req.DueAt)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REGISTER_DUE_AT", "截止时间必须是 RFC3339 格式")
		return model.ProjectRegister{}, false
	}
	images := []projectRegisterImageRequest{}
	if req.Images != nil {
		images = *req.Images
	}
	if err := validateProjectRegisterImages(images, h.Cfg.UploadPublicBase); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_REGISTER_IMAGES", err.Error())
		return model.ProjectRegister{}, false
	}
	return model.ProjectRegister{
		Type:           registerType,
		ProjectID:      req.ProjectID,
		TaskID:         req.TaskID,
		Title:          title,
		Description:    strings.TrimSpace(req.Description),
		Status:         status,
		Severity:       severity,
		Probability:    probability,
		Impact:         impact,
		Source:         strings.TrimSpace(req.Source),
		ResponsePlan:   strings.TrimSpace(req.ResponsePlan),
		Resolution:     strings.TrimSpace(req.Resolution),
		DecisionDetail: strings.TrimSpace(req.DecisionDetail),
		Background:     strings.TrimSpace(req.Background),
		ImpactScope:    strings.TrimSpace(req.ImpactScope),
		Images:         toProjectRegisterImages(images),
		DueAt:          dueAt,
		OwnerID:        req.OwnerID,
		ParticipantIDs: participantIDs,
	}, true
}

func (h *Handler) writeProjectRegisterActivityWithDB(tx *gorm.DB, registerID, actorID uint, activityType, summary, detail string) error {
	now := time.Now()
	if err := tx.Create(&model.ProjectRegisterActivity{
		RegisterID: registerID,
		ActorID:    actorID,
		Type:       activityType,
		Summary:    summary,
		Detail:     detail,
	}).Error; err != nil {
		return err
	}
	return tx.Model(&model.ProjectRegister{}).Where("id = ?", registerID).Updates(map[string]any{"last_activity_at": now}).Error
}

func projectRegisterNotificationRecipients(item model.ProjectRegister) []uint {
	recipients := append([]uint{}, item.ParticipantIDs...)
	recipients = append(recipients, item.CreatedByID)
	if item.OwnerID != nil {
		recipients = append(recipients, *item.OwnerID)
	}
	for _, user := range item.Project.Users {
		recipients = append(recipients, user.ID)
	}
	return uniqueUint(recipients)
}

func (h *Handler) notifyProjectRegisterWithDB(tx *gorm.DB, item model.ProjectRegister, title, content string) ([]uint, error) {
	recipients := projectRegisterNotificationRecipients(item)
	if err := h.createNotificationsWithDB(tx, recipients, title, content, "project_registers", item.ID); err != nil {
		return nil, err
	}
	return recipients, nil
}

func (h *Handler) ListProjectRegisters(c *gin.Context) {
	page, pageSize := parsePage(c)
	query := h.scopeProjectRegistersQuery(c, h.DB.Model(&model.ProjectRegister{}))
	if registerType, ok := normalizeProjectRegisterType(c.Query("type")); ok && strings.TrimSpace(c.Query("type")) != "" {
		query = query.Where("project_registers.type = ?", registerType)
	}
	if status, ok := normalizeProjectRegisterStatus(c.Query("status")); ok && strings.TrimSpace(c.Query("status")) != "" {
		query = query.Where("project_registers.status = ?", status)
	}
	if statuses := parseProjectRegisterStatuses(c.Query("statuses")); len(statuses) > 0 {
		query = query.Where("project_registers.status IN ?", statuses)
	}
	if severity, ok := normalizeProjectRegisterSeverity(c.Query("severity")); ok && strings.TrimSpace(c.Query("severity")) != "" {
		query = query.Where("project_registers.severity = ?", severity)
	}
	if severities := parseProjectRegisterSeverities(c.Query("severities")); len(severities) > 0 {
		query = query.Where("project_registers.severity IN ?", severities)
	}
	if projectID := strings.TrimSpace(c.Query("projectId")); projectID != "" {
		query = query.Where("project_registers.project_id = ?", projectID)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("project_registers.title LIKE ? OR project_registers.description LIKE ? OR project_registers.source LIKE ?", like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PROJECT_REGISTERS_FAILED", err)
		return
	}
	var items []model.ProjectRegister
	if err := query.Preload("Project").Preload("Task").Preload("Owner").Preload("CreatedBy").
		Order("project_registers.last_activity_at desc, project_registers.updated_at desc, project_registers.id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PROJECT_REGISTERS_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, pageResult[model.ProjectRegister]{List: items, Total: total, Page: page, PageSize: pageSize})
}

func (h *Handler) ProjectRegisterDetail(c *gin.Context) {
	item, visible := h.ensureProjectRegisterVisible(c, c.Param("id"), true)
	if !visible {
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) CreateProjectRegister(c *gin.Context) {
	var req projectRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	item, ok := h.normalizeProjectRegisterRequest(c, req)
	if !ok {
		return
	}
	item.CreatedByID = c.GetUint("userId")
	var notifyIDs []uint
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		item.LastActivityAt = &now
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		if err := h.preloadProjectRegisterResponse(tx.Preload("Project.Users"), &item); err != nil {
			return err
		}
		if err := h.writeProjectRegisterActivityWithDB(tx, item.ID, item.CreatedByID, "register.created", "创建"+projectRegisterTypeLabel(item.Type)+"登记项", projectRegisterDetail(item)); err != nil {
			return err
		}
		title := projectRegisterTypeLabel(item.Type) + "登记项已创建"
		recipients, err := h.notifyProjectRegisterWithDB(tx, item, title, "项目「"+item.Project.Name+"」新增登记项「"+item.Title+"」")
		if err != nil {
			return err
		}
		notifyIDs = append(notifyIDs, recipients...)
		return h.writeAuditWithDB(c, tx, "project_registers", "create", item.ID, true, auditDetailf("创建项目登记项(id=%d,projectId=%d,type=%s)", item.ID, item.ProjectID, item.Type))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_PROJECT_REGISTER_FAILED", err)
		return
	}
	h.pushNotificationUpdates(notifyIDs)
	if err := h.preloadProjectRegisterResponse(h.DB, &item); err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PROJECT_REGISTER_FAILED", err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) UpdateProjectRegister(c *gin.Context) {
	var req projectRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	item, visible := h.ensureProjectRegisterVisible(c, c.Param("id"), false)
	if !visible {
		return
	}
	next, ok := h.normalizeProjectRegisterRequest(c, req)
	if !ok {
		return
	}
	oldItem := *item
	item.Type = next.Type
	item.ProjectID = next.ProjectID
	item.TaskID = next.TaskID
	item.Title = next.Title
	item.Description = next.Description
	item.Status = next.Status
	item.Severity = next.Severity
	item.Probability = next.Probability
	item.Impact = next.Impact
	item.Source = next.Source
	item.ResponsePlan = next.ResponsePlan
	item.Resolution = next.Resolution
	item.DecisionDetail = next.DecisionDetail
	item.Background = next.Background
	item.ImpactScope = next.ImpactScope
	if req.Images != nil {
		item.Images = next.Images
	}
	item.DueAt = next.DueAt
	item.OwnerID = next.OwnerID
	item.ParticipantIDs = next.ParticipantIDs

	var notifyIDs []uint
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(item).Error; err != nil {
			return err
		}
		if err := h.preloadProjectRegisterResponse(tx.Preload("Project.Users"), item); err != nil {
			return err
		}
		if err := h.writeProjectRegisterActivityWithDB(tx, item.ID, c.GetUint("userId"), "register.updated", "更新"+projectRegisterTypeLabel(item.Type)+"登记项", projectRegisterUpdateDetail(oldItem, *item)); err != nil {
			return err
		}
		title := projectRegisterTypeLabel(item.Type) + "登记项已更新"
		recipients, err := h.notifyProjectRegisterWithDB(tx, *item, title, "项目「"+item.Project.Name+"」登记项「"+item.Title+"」已更新")
		if err != nil {
			return err
		}
		notifyIDs = append(notifyIDs, recipients...)
		return h.writeAuditWithDB(c, tx, "project_registers", "update", item.ID, true, auditDetailf("更新项目登记项(id=%d,projectId=%d,type=%s)", item.ID, item.ProjectID, item.Type))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "UPDATE_PROJECT_REGISTER_FAILED", err)
		return
	}
	h.pushNotificationUpdates(notifyIDs)
	if err := h.preloadProjectRegisterResponse(h.DB, item); err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PROJECT_REGISTER_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteProjectRegister(c *gin.Context) {
	item, visible := h.ensureProjectRegisterVisible(c, c.Param("id"), true)
	if !visible {
		return
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("register_id = ?", item.ID).Delete(&model.ProjectRegisterActivity{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(item).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "project_registers", "delete", item.ID, true, auditDetailf("删除项目登记项(id=%d,projectId=%d,type=%s)", item.ID, item.ProjectID, item.Type))
	}); err != nil {
		respondDBError(c, http.StatusInternalServerError, "DELETE_PROJECT_REGISTER_FAILED", err)
		return
	}
	respondMessage(c, http.StatusOK, "PROJECT_REGISTER_DELETED", "删除成功")
}

func (h *Handler) ListProjectRegisterActivities(c *gin.Context) {
	item, visible := h.ensureProjectRegisterVisible(c, c.Param("id"), false)
	if !visible {
		return
	}
	page, pageSize := parsePage(c)
	query := h.DB.Model(&model.ProjectRegisterActivity{}).Where("register_id = ?", item.ID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PROJECT_REGISTER_ACTIVITIES_FAILED", err)
		return
	}
	var items []model.ProjectRegisterActivity
	if err := query.Preload("Actor").
		Order("project_register_activities.id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PROJECT_REGISTER_ACTIVITIES_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, pageResult[model.ProjectRegisterActivity]{List: items, Total: total, Page: page, PageSize: pageSize})
}
