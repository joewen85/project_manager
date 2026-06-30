package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"project-manager/backend/internal/model"
	"project-manager/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type portalInviteRequest struct {
	Name               string               `json:"name"`
	Company            string               `json:"company"`
	ContactName        string               `json:"contactName"`
	ContactEmail       string               `json:"contactEmail"`
	ContactType        string               `json:"contactType"`
	IsEnabled          *bool                `json:"isEnabled"`
	ExpiresAt          string               `json:"expiresAt"`
	AllowedAttachments *[]attachmentRequest `json:"allowedAttachments"`
	ProjectID          uint                 `json:"projectId"`
}

type portalInviteCreateResponse struct {
	model.PortalInvite
	Token string `json:"token"`
}

type portalProjectView struct {
	ID          uint       `json:"id"`
	Code        string     `json:"code"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	StartAt     *time.Time `json:"startAt,omitempty"`
	EndAt       *time.Time `json:"endAt,omitempty"`
}

type portalTagView struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type portalTaskView struct {
	ID              uint               `json:"id"`
	TaskNo          string             `json:"taskNo"`
	Title           string             `json:"title"`
	Description     string             `json:"description"`
	Status          model.TaskStatus   `json:"status"`
	Priority        model.TaskPriority `json:"priority"`
	IsMilestone     bool               `json:"isMilestone"`
	ExternalVisible bool               `json:"externalVisible"`
	Progress        int                `json:"progress"`
	StartAt         *time.Time         `json:"startAt,omitempty"`
	EndAt           *time.Time         `json:"endAt,omitempty"`
	Tags            []portalTagView    `json:"tags"`
}

type portalCommentView struct {
	ID              uint               `json:"id"`
	TaskID          uint               `json:"taskId"`
	Content         string             `json:"content"`
	Attachments     []model.Attachment `json:"attachments"`
	ExternalName    string             `json:"externalName"`
	ExternalEmail   string             `json:"externalEmail"`
	ExternalCompany string             `json:"externalCompany"`
	CreatedAt       time.Time          `json:"createdAt"`
}

type portalStatusReport struct {
	GeneratedAt        time.Time `json:"generatedAt"`
	TaskCount          int       `json:"taskCount"`
	CompletedTaskCount int       `json:"completedTaskCount"`
	OverdueTaskCount   int       `json:"overdueTaskCount"`
	AverageProgress    float64   `json:"averageProgress"`
	CompletionRate     float64   `json:"completionRate"`
	Health             string    `json:"health"`
	Summary            string    `json:"summary"`
}

type portalStatusResponse struct {
	InviteID           uint                `json:"inviteId"`
	ContactName        string              `json:"contactName"`
	ContactEmail       string              `json:"contactEmail"`
	Company            string              `json:"company"`
	ContactType        string              `json:"contactType"`
	Project            portalProjectView   `json:"project"`
	StatusReport       portalStatusReport  `json:"statusReport"`
	Tasks              []portalTaskView    `json:"tasks"`
	Comments           []portalCommentView `json:"comments"`
	AllowedAttachments []model.Attachment  `json:"allowedAttachments"`
}

type portalWorkRequestRequest struct {
	Type          string                         `json:"type" binding:"required"`
	Title         string                         `json:"title" binding:"required"`
	Description   string                         `json:"description"`
	Priority      string                         `json:"priority"`
	TargetTaskID  *uint                          `json:"targetTaskId"`
	ChangePayload model.WorkRequestChangePayload `json:"changePayload"`
	ExternalName  string                         `json:"externalName"`
	ExternalEmail string                         `json:"externalEmail"`
	Attachment    *attachmentRequest             `json:"attachment"`
	Attachments   *[]attachmentRequest           `json:"attachments"`
}

type portalTaskCommentRequest struct {
	Content       string               `json:"content" binding:"required"`
	ExternalName  string               `json:"externalName"`
	ExternalEmail string               `json:"externalEmail"`
	Attachment    *attachmentRequest   `json:"attachment"`
	Attachments   *[]attachmentRequest `json:"attachments"`
}

func normalizePortalContactType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "supplier":
		return "supplier"
	default:
		return "customer"
	}
}

func portalInviteDefaultName(req portalInviteRequest, projectName string) string {
	for _, candidate := range []string{req.Name, req.Company, req.ContactName, projectName} {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			return trimmed
		}
	}
	return "外部门户邀请"
}

func parsePortalExpiresAt(value string) (*time.Time, error) {
	expiresAt, err := parseRFC3339(value)
	if err != nil {
		return nil, err
	}
	if expiresAt != nil && !expiresAt.After(time.Now()) {
		return nil, fmt.Errorf("expiresAt 必须晚于当前时间")
	}
	return expiresAt, nil
}

func (h *Handler) scopePortalInvitesQuery(c *gin.Context, query *gorm.DB) *gorm.DB {
	if h.currentUserIsAdmin(c) {
		return query
	}
	uid := c.GetUint("userId")
	if uid == 0 {
		return query.Where("1 = 0")
	}
	visibleProjects := h.scopeProjectsQuery(c, h.DB.Model(&model.Project{}).Select("projects.id"))
	return query.Where("portal_invites.created_by_id = ? OR portal_invites.project_id IN (?)", uid, visibleProjects)
}

func (h *Handler) ensurePortalInviteVisible(c *gin.Context, id string) (*model.PortalInvite, bool) {
	var item model.PortalInvite
	query := h.scopePortalInvitesQuery(c, h.DB.Model(&model.PortalInvite{})).
		Preload("Project").
		Preload("CreatedBy")
	if err := query.Where("portal_invites.id = ?", id).First(&item).Error; err != nil {
		respondError(c, http.StatusNotFound, "PORTAL_INVITE_NOT_FOUND", "外部门户邀请不存在")
		return nil, false
	}
	return &item, true
}

func (h *Handler) ListPortalInvites(c *gin.Context) {
	page, pageSize := parsePage(c)
	query := h.scopePortalInvitesQuery(c, h.DB.Model(&model.PortalInvite{}))
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("portal_invites.name LIKE ? OR portal_invites.company LIKE ? OR portal_invites.contact_name LIKE ? OR portal_invites.contact_email LIKE ? OR portal_invites.token_prefix LIKE ?", like, like, like, like, like)
	}
	if projectID := strings.TrimSpace(c.Query("projectId")); projectID != "" {
		query = query.Where("portal_invites.project_id = ?", projectID)
	}
	if enabled := strings.TrimSpace(c.Query("isEnabled")); enabled != "" {
		query = query.Where("portal_invites.is_enabled = ?", enabled == "true" || enabled == "1")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PORTAL_INVITES_FAILED", err)
		return
	}
	var items []model.PortalInvite
	if err := query.
		Preload("Project").
		Preload("CreatedBy").
		Order("portal_invites.id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PORTAL_INVITES_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, pageResult[model.PortalInvite]{List: items, Total: total, Page: page, PageSize: pageSize})
}

func (h *Handler) CreatePortalInvite(c *gin.Context) {
	var req portalInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	if req.ProjectID == 0 {
		respondError(c, http.StatusBadRequest, "PORTAL_PROJECT_REQUIRED", "请选择授权项目")
		return
	}
	if !h.ensureProjectVisible(c, strconv.FormatUint(uint64(req.ProjectID), 10)) {
		return
	}
	var project model.Project
	if err := h.DB.Select("id, code, name").First(&project, req.ProjectID).Error; err != nil {
		respondError(c, http.StatusNotFound, "PROJECT_NOT_FOUND", "项目不存在")
		return
	}
	expiresAt, err := parsePortalExpiresAt(req.ExpiresAt)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_PORTAL_EXPIRES_AT", err.Error())
		return
	}
	allowedAttachments := []attachmentRequest{}
	if req.AllowedAttachments != nil {
		allowedAttachments = *req.AllowedAttachments
	}
	if err := validateAttachments(allowedAttachments, h.Cfg.UploadPublicBase); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_PORTAL_ATTACHMENT", err.Error())
		return
	}
	plainToken, err := util.GeneratePortalToken()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "PORTAL_TOKEN_GENERATE_FAILED", "门户 Token 生成失败")
		return
	}
	item := model.PortalInvite{
		Name:               portalInviteDefaultName(req, project.Name),
		Company:            strings.TrimSpace(req.Company),
		ContactName:        strings.TrimSpace(req.ContactName),
		ContactEmail:       strings.TrimSpace(req.ContactEmail),
		ContactType:        normalizePortalContactType(req.ContactType),
		TokenPrefix:        util.PortalTokenLookupPrefix(plainToken),
		TokenLastFour:      util.PortalTokenLastFour(plainToken),
		TokenHash:          util.HashPortalToken(plainToken),
		IsEnabled:          boolValueDefault(req.IsEnabled, true),
		ExpiresAt:          expiresAt,
		AllowedAttachments: toModelAttachments(allowedAttachments),
		ProjectID:          req.ProjectID,
		CreatedByID:        c.GetUint("userId"),
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "portal", "create_invite", item.ID, true, auditDetailf("创建外部门户邀请(id=%d, project_id=%d)", item.ID, item.ProjectID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_PORTAL_INVITE_FAILED", err)
		return
	}
	_ = h.DB.Preload("Project").Preload("CreatedBy").First(&item, item.ID).Error
	c.JSON(http.StatusCreated, portalInviteCreateResponse{PortalInvite: item, Token: plainToken})
}

func (h *Handler) UpdatePortalInvite(c *gin.Context) {
	item, ok := h.ensurePortalInviteVisible(c, c.Param("id"))
	if !ok {
		return
	}
	var req portalInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	projectID := item.ProjectID
	if req.ProjectID != 0 {
		projectID = req.ProjectID
	}
	if !h.ensureProjectVisible(c, strconv.FormatUint(uint64(projectID), 10)) {
		return
	}
	var project model.Project
	if err := h.DB.Select("id, code, name").First(&project, projectID).Error; err != nil {
		respondError(c, http.StatusNotFound, "PROJECT_NOT_FOUND", "项目不存在")
		return
	}
	expiresAt, err := parsePortalExpiresAt(req.ExpiresAt)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_PORTAL_EXPIRES_AT", err.Error())
		return
	}
	item.Name = portalInviteDefaultName(req, project.Name)
	item.Company = strings.TrimSpace(req.Company)
	item.ContactName = strings.TrimSpace(req.ContactName)
	item.ContactEmail = strings.TrimSpace(req.ContactEmail)
	item.ContactType = normalizePortalContactType(req.ContactType)
	item.ProjectID = projectID
	item.ExpiresAt = expiresAt
	item.IsEnabled = boolValueDefault(req.IsEnabled, item.IsEnabled)
	if req.AllowedAttachments != nil {
		if err := validateAttachments(*req.AllowedAttachments, h.Cfg.UploadPublicBase); err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_PORTAL_ATTACHMENT", err.Error())
			return
		}
		item.AllowedAttachments = toModelAttachments(*req.AllowedAttachments)
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(item).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "portal", "update_invite", item.ID, true, auditDetailf("更新外部门户邀请(id=%d, project_id=%d)", item.ID, item.ProjectID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "UPDATE_PORTAL_INVITE_FAILED", err)
		return
	}
	_ = h.DB.Preload("Project").Preload("CreatedBy").First(item, item.ID).Error
	c.JSON(http.StatusOK, item)
}

func (h *Handler) RevokePortalInvite(c *gin.Context) {
	item, ok := h.ensurePortalInviteVisible(c, c.Param("id"))
	if !ok {
		return
	}
	now := time.Now()
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(item).Updates(map[string]any{
			"is_enabled": false,
			"revoked_at": now,
		}).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "portal", "revoke_invite", item.ID, true, auditDetailf("撤销外部门户邀请(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "REVOKE_PORTAL_INVITE_FAILED", err)
		return
	}
	respondMessage(c, http.StatusOK, "PORTAL_INVITE_REVOKED", "外部门户邀请已撤销")
}

func (h *Handler) DeletePortalInvite(c *gin.Context) {
	item, ok := h.ensurePortalInviteVisible(c, c.Param("id"))
	if !ok {
		return
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(item).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "portal", "delete_invite", item.ID, true, auditDetailf("删除外部门户邀请(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusInternalServerError, "DELETE_PORTAL_INVITE_FAILED", err)
		return
	}
	respondMessage(c, http.StatusOK, "PORTAL_INVITE_DELETED", "外部门户邀请已删除")
}

func (h *Handler) lookupPortalInvite(c *gin.Context) (*model.PortalInvite, bool) {
	token := strings.TrimSpace(c.Param("token"))
	if token == "" {
		respondError(c, http.StatusUnauthorized, "PORTAL_TOKEN_REQUIRED", "缺少外部门户访问 Token")
		return nil, false
	}
	prefix := util.PortalTokenLookupPrefix(token)
	var candidates []model.PortalInvite
	if err := h.DB.Preload("Project").Where("token_prefix = ?", prefix).Find(&candidates).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PORTAL_INVITE_FAILED", err)
		return nil, false
	}
	for index := range candidates {
		if !util.EqualPortalTokenHash(candidates[index].TokenHash, token) {
			continue
		}
		item := candidates[index]
		now := time.Now()
		if !item.IsEnabled || item.RevokedAt != nil {
			respondError(c, http.StatusForbidden, "PORTAL_INVITE_REVOKED", "外部门户邀请已停用")
			return nil, false
		}
		if item.ExpiresAt != nil && !item.ExpiresAt.After(now) {
			respondError(c, http.StatusGone, "PORTAL_INVITE_EXPIRED", "外部门户邀请已过期")
			return nil, false
		}
		_ = h.DB.Model(&model.PortalInvite{}).Where("id = ?", item.ID).Updates(map[string]any{
			"last_used_at": now,
			"last_used_ip": c.ClientIP(),
		}).Error
		item.LastUsedAt = &now
		item.LastUsedIP = c.ClientIP()
		return &item, true
	}
	respondError(c, http.StatusUnauthorized, "PORTAL_TOKEN_INVALID", "外部门户访问 Token 无效")
	return nil, false
}

func portalTaskViews(tasks []model.Task) []portalTaskView {
	out := make([]portalTaskView, 0, len(tasks))
	for _, task := range tasks {
		tags := make([]portalTagView, 0, len(task.Tags))
		for _, tag := range task.Tags {
			tags = append(tags, portalTagView{ID: tag.ID, Name: tag.Name})
		}
		out = append(out, portalTaskView{
			ID:              task.ID,
			TaskNo:          task.TaskNo,
			Title:           task.Title,
			Description:     task.Description,
			Status:          task.Status,
			Priority:        task.Priority,
			IsMilestone:     task.IsMilestone,
			ExternalVisible: task.ExternalVisible,
			Progress:        task.Progress,
			StartAt:         task.StartAt,
			EndAt:           task.EndAt,
			Tags:            tags,
		})
	}
	return out
}

func portalCommentViews(comments []model.TaskComment) []portalCommentView {
	out := make([]portalCommentView, 0, len(comments))
	for _, comment := range comments {
		out = append(out, portalCommentView{
			ID:              comment.ID,
			TaskID:          comment.TaskID,
			Content:         comment.Content,
			Attachments:     comment.Attachments,
			ExternalName:    comment.ExternalName,
			ExternalEmail:   comment.ExternalEmail,
			ExternalCompany: comment.ExternalCompany,
			CreatedAt:       comment.CreatedAt,
		})
	}
	return out
}

func buildPortalStatusReport(tasks []model.Task) portalStatusReport {
	now := time.Now()
	if len(tasks) == 0 {
		return portalStatusReport{
			GeneratedAt: now,
			Health:      "green",
			Summary:     "暂无对外可见任务",
		}
	}
	var completed int
	var overdue int
	var progressTotal int
	for _, task := range tasks {
		progressTotal += task.Progress
		if task.Status == model.TaskCompleted {
			completed += 1
		}
		if task.EndAt != nil && task.EndAt.Before(now) && task.Status != model.TaskCompleted {
			overdue += 1
		}
	}
	averageProgress := float64(progressTotal) / float64(len(tasks))
	completionRate := float64(completed) / float64(len(tasks))
	health := "green"
	summary := "项目对外可见任务推进正常"
	if overdue > 0 {
		health = "red"
		summary = fmt.Sprintf("有 %d 个对外可见任务已逾期，请关注交付风险", overdue)
	} else if completionRate < 0.5 && averageProgress < 50 {
		health = "yellow"
		summary = "项目对外可见任务仍处于推进早期"
	}
	return portalStatusReport{
		GeneratedAt:        now,
		TaskCount:          len(tasks),
		CompletedTaskCount: completed,
		OverdueTaskCount:   overdue,
		AverageProgress:    averageProgress,
		CompletionRate:     completionRate,
		Health:             health,
		Summary:            summary,
	}
}

func taskIDsFromVisibleTasks(tasks []model.Task) []uint {
	out := make([]uint, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, task.ID)
	}
	return out
}

func (h *Handler) portalVisibleTasks(projectID uint) ([]model.Task, error) {
	var tasks []model.Task
	err := h.DB.Model(&model.Task{}).
		Where("project_id = ? AND external_visible = ?", projectID, true).
		Preload("Tags", func(db *gorm.DB) *gorm.DB { return db.Order("tags.name asc") }).
		Order("COALESCE(start_at, end_at) asc, id asc").
		Find(&tasks).Error
	return tasks, err
}

func (h *Handler) PortalStatus(c *gin.Context) {
	invite, ok := h.lookupPortalInvite(c)
	if !ok {
		return
	}
	tasks, err := h.portalVisibleTasks(invite.ProjectID)
	if err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PORTAL_TASKS_FAILED", err)
		return
	}
	comments := []model.TaskComment{}
	taskIDs := taskIDsFromVisibleTasks(tasks)
	if len(taskIDs) > 0 {
		if err := h.DB.Model(&model.TaskComment{}).
			Where("task_id IN ? AND is_deleted = ? AND source = ?", taskIDs, false, "portal").
			Order("id desc").
			Find(&comments).Error; err != nil {
			respondDBError(c, http.StatusInternalServerError, "QUERY_PORTAL_COMMENTS_FAILED", err)
			return
		}
	}
	h.writeAudit(c, "portal", "view", invite.ID, true, auditDetailf("外部门户访问(project_id=%d)", invite.ProjectID))
	c.JSON(http.StatusOK, portalStatusResponse{
		InviteID:     invite.ID,
		ContactName:  invite.ContactName,
		ContactEmail: invite.ContactEmail,
		Company:      invite.Company,
		ContactType:  invite.ContactType,
		Project: portalProjectView{
			ID:          invite.Project.ID,
			Code:        invite.Project.Code,
			Name:        invite.Project.Name,
			Description: invite.Project.Description,
			StartAt:     invite.Project.StartAt,
			EndAt:       invite.Project.EndAt,
		},
		StatusReport:       buildPortalStatusReport(tasks),
		Tasks:              portalTaskViews(tasks),
		Comments:           portalCommentViews(comments),
		AllowedAttachments: invite.AllowedAttachments,
	})
}

func portalIdentity(invite model.PortalInvite, name, email string) (string, string, string) {
	externalName := strings.TrimSpace(name)
	if externalName == "" {
		externalName = strings.TrimSpace(invite.ContactName)
	}
	if externalName == "" {
		externalName = strings.TrimSpace(invite.Company)
	}
	if externalName == "" {
		externalName = "外部联系人"
	}
	externalEmail := strings.TrimSpace(email)
	if externalEmail == "" {
		externalEmail = strings.TrimSpace(invite.ContactEmail)
	}
	return externalName, externalEmail, strings.TrimSpace(invite.Company)
}

func (h *Handler) portalProjectRecipients(tx *gorm.DB, invite model.PortalInvite) ([]uint, error) {
	ids := []uint{invite.CreatedByID}
	var projectUserIDs []uint
	if err := tx.Table("project_users").Where("project_id = ?", invite.ProjectID).Pluck("user_id", &projectUserIDs).Error; err != nil {
		return nil, err
	}
	ids = append(ids, projectUserIDs...)
	return uniqueUint(ids), nil
}

func (h *Handler) portalTaskRecipients(tx *gorm.DB, task model.Task, fallbackID uint) ([]uint, error) {
	ids := []uint{task.CreatorID, fallbackID}
	var assigneeIDs []uint
	if err := tx.Table("task_users").Where("task_id = ?", task.ID).Pluck("user_id", &assigneeIDs).Error; err != nil {
		return nil, err
	}
	var reviewerIDs []uint
	if err := tx.Table("task_reviewers").Where("task_id = ?", task.ID).Pluck("user_id", &reviewerIDs).Error; err != nil {
		return nil, err
	}
	ids = append(ids, assigneeIDs...)
	ids = append(ids, reviewerIDs...)
	return uniqueUint(ids), nil
}

func (h *Handler) PortalCreateWorkRequest(c *gin.Context) {
	invite, ok := h.lookupPortalInvite(c)
	if !ok {
		return
	}
	var req portalWorkRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	requestType, validType := normalizeWorkRequestType(req.Type)
	if !validType {
		respondError(c, http.StatusBadRequest, "INVALID_WORK_REQUEST_TYPE", "请求类型必须是 project、task、bug 或 change")
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		respondError(c, http.StatusBadRequest, "WORK_REQUEST_TITLE_REQUIRED", "请求标题不能为空")
		return
	}
	attachments, _ := requestAttachments(req.Attachment, req.Attachments)
	if err := validateAttachments(attachments, h.Cfg.UploadPublicBase); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ATTACHMENT", err.Error())
		return
	}
	var targetTaskID *uint
	changePayload := model.WorkRequestChangePayload{}
	if requestType == model.WorkRequestChange {
		if req.TargetTaskID == nil || *req.TargetTaskID == 0 {
			respondError(c, http.StatusBadRequest, "CHANGE_TARGET_TASK_REQUIRED", "变更申请必须选择对外可见任务")
			return
		}
		var task model.Task
		if err := h.DB.Where("id = ? AND project_id = ? AND external_visible = ?", *req.TargetTaskID, invite.ProjectID, true).First(&task).Error; err != nil {
			respondError(c, http.StatusNotFound, "TASK_NOT_FOUND", "对外可见任务不存在")
			return
		}
		payload := req.ChangePayload
		payload.AssigneeIDs = nil
		if strings.TrimSpace(payload.ScopeDescription) == "" {
			payload.ScopeDescription = strings.TrimSpace(req.Description)
		}
		normalizedPayload, valid, message := normalizeWorkRequestChangePayload(payload)
		if !valid {
			respondError(c, http.StatusBadRequest, "INVALID_CHANGE_PAYLOAD", message)
			return
		}
		targetTaskID = req.TargetTaskID
		changePayload = normalizedPayload
	}
	externalName, externalEmail, externalCompany := portalIdentity(*invite, req.ExternalName, req.ExternalEmail)
	projectID := invite.ProjectID
	item := model.WorkRequest{
		Type:            requestType,
		Title:           title,
		Description:     strings.TrimSpace(req.Description),
		Attachments:     toModelAttachments(attachments),
		Priority:        normalizePriority(req.Priority),
		Status:          model.WorkRequestSubmitted,
		ProjectID:       &projectID,
		RequesterID:     invite.CreatedByID,
		Source:          "portal",
		PortalInviteID:  &invite.ID,
		ExternalName:    externalName,
		ExternalEmail:   externalEmail,
		ExternalCompany: externalCompany,
		TargetTaskID:    targetTaskID,
		ChangePayload:   changePayload,
	}
	var notifyIDs []uint
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		recipients, err := h.portalProjectRecipients(tx, *invite)
		if err != nil {
			return err
		}
		notifyIDs = recipients
		if err := h.createNotificationsWithDB(tx, recipients, "收到外部门户请求", "外部联系人提交了请求："+item.Title, "requests", item.ID); err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "portal", "create_request", item.ID, true, auditDetailf("外部门户提交请求(id=%d, invite_id=%d, attachments=%d)", item.ID, invite.ID, len(item.Attachments)))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_PORTAL_REQUEST_FAILED", err)
		return
	}
	h.pushNotificationUpdates(notifyIDs)
	_ = h.DB.Preload("Project").Preload("Requester").First(&item, item.ID).Error
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) PortalCreateTaskComment(c *gin.Context) {
	invite, ok := h.lookupPortalInvite(c)
	if !ok {
		return
	}
	taskID, err := strconv.ParseUint(c.Param("taskId"), 10, 64)
	if err != nil || taskID == 0 {
		respondError(c, http.StatusBadRequest, "INVALID_TASK_ID", "非法任务ID")
		return
	}
	var task model.Task
	if err := h.DB.Where("id = ? AND project_id = ? AND external_visible = ?", uint(taskID), invite.ProjectID, true).First(&task).Error; err != nil {
		respondError(c, http.StatusNotFound, "TASK_NOT_FOUND", "对外可见任务不存在")
		return
	}
	var req portalTaskCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		respondError(c, http.StatusBadRequest, "COMMENT_CONTENT_REQUIRED", "评论内容不能为空")
		return
	}
	attachments, _ := requestAttachments(req.Attachment, req.Attachments)
	if err := validateAttachments(attachments, h.Cfg.UploadPublicBase); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ATTACHMENT", err.Error())
		return
	}
	externalName, externalEmail, externalCompany := portalIdentity(*invite, req.ExternalName, req.ExternalEmail)
	item := model.TaskComment{
		TaskID:          task.ID,
		AuthorID:        invite.CreatedByID,
		Content:         content,
		Attachments:     toModelAttachments(attachments),
		Source:          "portal",
		PortalInviteID:  &invite.ID,
		ExternalName:    externalName,
		ExternalEmail:   externalEmail,
		ExternalCompany: externalCompany,
		IsDeleted:       false,
	}
	var notifyIDs []uint
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		commentID := item.ID
		if err := h.writeTaskActivityWithDB(tx, task.ID, 0, "portal.comment_created", "外部评论", content, &commentID); err != nil {
			return err
		}
		recipients, err := h.portalTaskRecipients(tx, task, invite.CreatedByID)
		if err != nil {
			return err
		}
		notifyIDs = recipients
		if err := h.createNotificationsWithDB(tx, recipients, "收到外部门户评论", "外部联系人在任务 "+task.TaskNo+" - "+task.Title+" 下发表了评论", "tasks", task.ID); err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "portal", "create_comment", item.ID, true, auditDetailf("外部门户评论(id=%d, invite_id=%d, task_id=%d, attachments=%d)", item.ID, invite.ID, task.ID, len(item.Attachments)))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_PORTAL_COMMENT_FAILED", err)
		return
	}
	h.pushNotificationUpdates(notifyIDs)
	c.JSON(http.StatusCreated, portalCommentViews([]model.TaskComment{item})[0])
}

func (h *Handler) saveCollectedUploadFiles(files []uploadSourceFile) ([]model.Attachment, error) {
	standaloneFiles := make([]uploadSourceFile, 0, len(files))
	folderGroups := make(map[string][]uploadSourceFile)
	folderOrder := make([]string, 0)
	for _, file := range files {
		if file.FileHeader == nil {
			continue
		}
		relativePath := file.RelativePath
		if strings.TrimSpace(relativePath) == "" {
			relativePath = file.FileHeader.Filename
		}
		_, _, relativePath = sanitizeUploadFileName(relativePath)
		folderName := topLevelFolder(relativePath)
		if folderName == "" {
			file.RelativePath = relativePath
			standaloneFiles = append(standaloneFiles, file)
			continue
		}
		if _, exists := folderGroups[folderName]; !exists {
			folderOrder = append(folderOrder, folderName)
		}
		file.RelativePath = relativePath
		folderGroups[folderName] = append(folderGroups[folderName], file)
	}

	now := time.Now()
	attachments := make([]model.Attachment, 0, len(standaloneFiles)+len(folderOrder))
	for _, file := range standaloneFiles {
		attachment, err := h.saveAttachmentFile(file.FileHeader, file.RelativePath, now)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}
	for _, folderName := range folderOrder {
		attachment, err := h.saveAttachmentZip(folderName, folderGroups[folderName], now)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}
	return attachments, nil
}

func (h *Handler) PortalUploadFile(c *gin.Context) {
	invite, ok := h.lookupPortalInvite(c)
	if !ok {
		return
	}
	files, err := collectUploadFiles(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, "UPLOAD_FILE_REQUIRED", "请上传文件")
		return
	}
	attachments, err := h.saveCollectedUploadFiles(files)
	if err != nil {
		respondDBError(c, http.StatusBadRequest, "UPLOAD_FILE_FAILED", err)
		return
	}
	h.writeAudit(c, "portal", "upload_attachment", invite.ID, true, auditDetailf("外部门户上传附件(invite_id=%d, count=%d)", invite.ID, len(attachments)))
	response := gin.H{"attachments": attachments}
	if len(attachments) == 1 {
		response["attachment"] = attachments[0]
	}
	c.JSON(http.StatusCreated, response)
}
