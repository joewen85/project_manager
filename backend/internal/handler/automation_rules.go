package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type automationRuleRequest struct {
	Name       string                      `json:"name" binding:"required"`
	Trigger    string                      `json:"trigger"`
	IsEnabled  *bool                       `json:"isEnabled"`
	Conditions automationConditionsRequest `json:"conditions"`
	Actions    automationActionsRequest    `json:"actions"`
}

type automationConditionsRequest struct {
	OverdueDays         *int     `json:"overdueDays"`
	ProjectIDs          []uint   `json:"projectIds"`
	FromStatuses        []string `json:"fromStatuses"`
	ToStatuses          []string `json:"toStatuses"`
	FromProgressMin     *int     `json:"fromProgressMin"`
	FromProgressMax     *int     `json:"fromProgressMax"`
	ToProgressMin       *int     `json:"toProgressMin"`
	ToProgressMax       *int     `json:"toProgressMax"`
	AssigneeChangeTypes []string `json:"assigneeChangeTypes"`
}

type automationActionsRequest struct {
	NotifyAssignees     *bool  `json:"notifyAssignees"`
	NotifyProjectOwners *bool  `json:"notifyProjectOwners"`
	AddComment          *bool  `json:"addComment"`
	CommentContent      string `json:"commentContent"`
	AddTags             *bool  `json:"addTags"`
	TagIDs              []uint `json:"tagIds"`
	AssignAssignees     *bool  `json:"assignAssignees"`
	AssigneeIDs         []uint `json:"assigneeIds"`
}

func normalizeAutomationTrigger(value string) (model.AutomationTrigger, bool) {
	switch model.AutomationTrigger(strings.TrimSpace(value)) {
	case "", model.AutomationTriggerTaskOverdue:
		return model.AutomationTriggerTaskOverdue, true
	case model.AutomationTriggerTaskStatusChanged:
		return model.AutomationTriggerTaskStatusChanged, true
	case model.AutomationTriggerTaskProgressChanged:
		return model.AutomationTriggerTaskProgressChanged, true
	case model.AutomationTriggerTaskAssigneeChanged:
		return model.AutomationTriggerTaskAssigneeChanged, true
	default:
		return model.AutomationTriggerTaskOverdue, false
	}
}

func normalizeAutomationStatusList(values []string, fieldName string) ([]model.TaskStatus, error) {
	if len(values) == 0 {
		return nil, nil
	}
	result := make([]model.TaskStatus, 0, len(values))
	seen := map[model.TaskStatus]struct{}{}
	for _, value := range values {
		status, ok := parseExplicitTaskStatus(value)
		if !ok {
			return nil, fmt.Errorf("%s 只能包含 pending、queued、processing、reviewing 或 completed", fieldName)
		}
		if _, exists := seen[status]; exists {
			continue
		}
		seen[status] = struct{}{}
		result = append(result, status)
	}
	return result, nil
}

func validateAutomationProgressBound(value *int, fieldName string) error {
	if value == nil {
		return nil
	}
	if *value < 0 || *value > 100 {
		return fmt.Errorf("%s 必须在 0 到 100 之间", fieldName)
	}
	return nil
}

func validateAutomationProgressRange(minValue *int, maxValue *int, label string) error {
	if err := validateAutomationProgressBound(minValue, label+"Min"); err != nil {
		return err
	}
	if err := validateAutomationProgressBound(maxValue, label+"Max"); err != nil {
		return err
	}
	if minValue != nil && maxValue != nil && *minValue > *maxValue {
		return fmt.Errorf("%sMin 不能大于 %sMax", label, label)
	}
	return nil
}

func normalizeAutomationAssigneeChangeTypeList(values []string) ([]model.AssigneeChangeType, error) {
	if len(values) == 0 {
		return nil, nil
	}
	result := make([]model.AssigneeChangeType, 0, len(values))
	seen := map[model.AssigneeChangeType]struct{}{}
	for _, value := range values {
		changeType := model.AssigneeChangeType(strings.TrimSpace(value))
		switch changeType {
		case model.AssigneeChangeAdded, model.AssigneeChangeRemoved:
		default:
			return nil, fmt.Errorf("assigneeChangeTypes 只能包含 added 或 removed")
		}
		if _, exists := seen[changeType]; exists {
			continue
		}
		seen[changeType] = struct{}{}
		result = append(result, changeType)
	}
	return result, nil
}

func normalizeAutomationConditions(req automationConditionsRequest, trigger model.AutomationTrigger) (model.AutomationConditions, error) {
	overdueDays := 1
	if req.OverdueDays != nil {
		overdueDays = *req.OverdueDays
	}
	if overdueDays < 0 {
		return model.AutomationConditions{}, fmt.Errorf("逾期天数不能小于 0")
	}
	fromStatuses, err := normalizeAutomationStatusList(req.FromStatuses, "fromStatuses")
	if err != nil {
		return model.AutomationConditions{}, err
	}
	toStatuses, err := normalizeAutomationStatusList(req.ToStatuses, "toStatuses")
	if err != nil {
		return model.AutomationConditions{}, err
	}
	if trigger == model.AutomationTriggerTaskStatusChanged && len(req.FromStatuses) == 0 && len(req.ToStatuses) == 0 {
		return model.AutomationConditions{}, fmt.Errorf("状态变更规则至少需要设置变更前或变更后状态")
	}
	if err := validateAutomationProgressRange(req.FromProgressMin, req.FromProgressMax, "fromProgress"); err != nil {
		return model.AutomationConditions{}, err
	}
	if err := validateAutomationProgressRange(req.ToProgressMin, req.ToProgressMax, "toProgress"); err != nil {
		return model.AutomationConditions{}, err
	}
	if trigger == model.AutomationTriggerTaskProgressChanged &&
		req.FromProgressMin == nil && req.FromProgressMax == nil && req.ToProgressMin == nil && req.ToProgressMax == nil {
		return model.AutomationConditions{}, fmt.Errorf("进度变更规则至少需要设置变更前或变更后进度条件")
	}
	assigneeChangeTypes, err := normalizeAutomationAssigneeChangeTypeList(req.AssigneeChangeTypes)
	if err != nil {
		return model.AutomationConditions{}, err
	}
	if trigger == model.AutomationTriggerTaskAssigneeChanged && len(assigneeChangeTypes) == 0 {
		return model.AutomationConditions{}, fmt.Errorf("执行人变更规则至少需要选择新增或移除执行人")
	}
	return model.AutomationConditions{
		OverdueDays:         overdueDays,
		ProjectIDs:          uniqueUint(req.ProjectIDs),
		FromStatuses:        fromStatuses,
		ToStatuses:          toStatuses,
		FromProgressMin:     req.FromProgressMin,
		FromProgressMax:     req.FromProgressMax,
		ToProgressMin:       req.ToProgressMin,
		ToProgressMax:       req.ToProgressMax,
		AssigneeChangeTypes: assigneeChangeTypes,
	}, nil
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func normalizeAutomationActions(req automationActionsRequest, trigger model.AutomationTrigger) (model.AutomationActions, error) {
	defaultActions := req.NotifyAssignees == nil && req.NotifyProjectOwners == nil
	actions := model.AutomationActions{
		NotifyAssignees:     boolValue(req.NotifyAssignees, defaultActions),
		NotifyProjectOwners: boolValue(req.NotifyProjectOwners, defaultActions),
		AddComment:          boolValue(req.AddComment, false),
		CommentContent:      strings.TrimSpace(req.CommentContent),
		AddTags:             boolValue(req.AddTags, false),
		TagIDs:              uniqueUint(req.TagIDs),
		AssignAssignees:     boolValue(req.AssignAssignees, false),
		AssigneeIDs:         uniqueUint(req.AssigneeIDs),
	}
	if !actions.AddTags {
		actions.TagIDs = nil
	}
	if !actions.AssignAssignees {
		actions.AssigneeIDs = nil
	}
	if actions.AddTags && len(actions.TagIDs) == 0 {
		return model.AutomationActions{}, fmt.Errorf("添加标签动作至少需要选择一个标签")
	}
	if actions.AssignAssignees && len(actions.AssigneeIDs) == 0 {
		return model.AutomationActions{}, fmt.Errorf("指派执行人动作至少需要选择一个执行人")
	}
	if automationEventTriggerAllowsComment(trigger) && actions.AddComment {
		return actions, nil
	}
	if actions.AddTags {
		return actions, nil
	}
	if actions.AssignAssignees {
		return actions, nil
	}
	if !actions.NotifyAssignees && !actions.NotifyProjectOwners {
		return model.AutomationActions{}, fmt.Errorf("至少需要启用一个通知对象")
	}
	return actions, nil
}

func automationEventTriggerAllowsComment(trigger model.AutomationTrigger) bool {
	return trigger == model.AutomationTriggerTaskStatusChanged ||
		trigger == model.AutomationTriggerTaskProgressChanged ||
		trigger == model.AutomationTriggerTaskAssigneeChanged
}

func buildAutomationRuleFromRequest(req automationRuleRequest, actorID uint) (model.AutomationRule, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return model.AutomationRule{}, fmt.Errorf("规则名称不能为空")
	}
	trigger, ok := normalizeAutomationTrigger(req.Trigger)
	if !ok {
		return model.AutomationRule{}, fmt.Errorf("触发器必须是 task_overdue、task_status_changed、task_progress_changed 或 task_assignee_changed")
	}
	conditions, err := normalizeAutomationConditions(req.Conditions, trigger)
	if err != nil {
		return model.AutomationRule{}, err
	}
	actions, err := normalizeAutomationActions(req.Actions, trigger)
	if err != nil {
		return model.AutomationRule{}, err
	}
	return model.AutomationRule{
		Name:        name,
		Trigger:     trigger,
		IsEnabled:   boolValue(req.IsEnabled, true),
		Conditions:  conditions,
		Actions:     actions,
		CreatedByID: actorID,
	}, nil
}

func (h *Handler) validateAutomationActionTags(actions model.AutomationActions) error {
	if !actions.AddTags || len(actions.TagIDs) == 0 {
		return nil
	}
	var count int64
	if err := h.DB.Model(&model.Tag{}).Where("id IN ?", actions.TagIDs).Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(actions.TagIDs)) {
		return fmt.Errorf("添加标签动作包含不存在的标签")
	}
	return nil
}

func (h *Handler) validateAutomationActionAssignees(actions model.AutomationActions) error {
	if !actions.AssignAssignees || len(actions.AssigneeIDs) == 0 {
		return nil
	}
	var count int64
	if err := h.DB.Model(&model.User{}).Where("id IN ?", actions.AssigneeIDs).Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(actions.AssigneeIDs)) {
		return fmt.Errorf("指派执行人动作包含不存在的用户")
	}
	return nil
}

func (h *Handler) validateAutomationActionTargets(actions model.AutomationActions) error {
	if err := h.validateAutomationActionTags(actions); err != nil {
		return err
	}
	return h.validateAutomationActionAssignees(actions)
}

func (h *Handler) ListAutomationRules(c *gin.Context) {
	page, pageSize := parsePage(c)
	query := h.DB.Model(&model.AutomationRule{})
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ?", like)
	}
	if trigger := strings.TrimSpace(c.Query("trigger")); trigger != "" {
		if parsed, ok := normalizeAutomationTrigger(trigger); ok {
			query = query.Where("trigger = ?", parsed)
		}
	}
	if isEnabled := strings.TrimSpace(c.Query("isEnabled")); isEnabled != "" {
		parsed, err := strconv.ParseBool(isEnabled)
		if err == nil {
			query = query.Where("is_enabled = ?", parsed)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_AUTOMATION_RULES_FAILED", err)
		return
	}

	var items []model.AutomationRule
	if err := query.Preload("CreatedBy").Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_AUTOMATION_RULES_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, pageResult[model.AutomationRule]{List: items, Total: total, Page: page, PageSize: pageSize})
}

func (h *Handler) CreateAutomationRule(c *gin.Context) {
	var req automationRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	item, err := buildAutomationRuleFromRequest(req, c.GetUint("userId"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_AUTOMATION_RULE", err.Error())
		return
	}
	if err := h.validateAutomationActionTargets(item.Actions); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_AUTOMATION_RULE", err.Error())
		return
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		if err := tx.Preload("CreatedBy").First(&item, item.ID).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "automations", "create", item.ID, true, auditDetailf("创建自动化规则(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_AUTOMATION_RULE_FAILED", err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) UpdateAutomationRule(c *gin.Context) {
	var req automationRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	next, err := buildAutomationRuleFromRequest(req, c.GetUint("userId"))
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_AUTOMATION_RULE", err.Error())
		return
	}
	if err := h.validateAutomationActionTargets(next.Actions); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_AUTOMATION_RULE", err.Error())
		return
	}

	var item model.AutomationRule
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "AUTOMATION_RULE_NOT_FOUND", "自动化规则不存在")
		return
	}
	item.Name = next.Name
	item.Trigger = next.Trigger
	item.IsEnabled = next.IsEnabled
	item.Conditions = next.Conditions
	item.Actions = next.Actions

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		if err := tx.Preload("CreatedBy").First(&item, item.ID).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "automations", "update", item.ID, true, auditDetailf("更新自动化规则(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "UPDATE_AUTOMATION_RULE_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteAutomationRule(c *gin.Context) {
	var item model.AutomationRule
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "AUTOMATION_RULE_NOT_FOUND", "自动化规则不存在")
		return
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&item).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "automations", "delete", item.ID, true, auditDetailf("删除自动化规则(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusInternalServerError, "DELETE_AUTOMATION_RULE_FAILED", err)
		return
	}
	respondMessage(c, http.StatusOK, "AUTOMATION_RULE_DELETED", "删除成功")
}

func (h *Handler) ListAutomationExecutionLogs(c *gin.Context) {
	page, pageSize := parsePage(c)
	query := h.DB.Model(&model.AutomationExecutionLog{})
	if ruleID := strings.TrimSpace(c.Query("ruleId")); ruleID != "" {
		query = query.Where("rule_id = ?", ruleID)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}
	if trigger := strings.TrimSpace(c.Query("trigger")); trigger != "" {
		query = query.Where("trigger = ?", trigger)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_AUTOMATION_LOGS_FAILED", err)
		return
	}
	var items []model.AutomationExecutionLog
	if err := query.Preload("Rule").Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_AUTOMATION_LOGS_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, pageResult[model.AutomationExecutionLog]{List: items, Total: total, Page: page, PageSize: pageSize})
}

func projectIDsFromMap(projectMap map[uint]projectLite) []uint {
	ids := make([]uint, 0, len(projectMap))
	for id := range projectMap {
		ids = append(ids, id)
	}
	return ids
}

func (h *Handler) automationOverdueTasks(tx *gorm.DB, c *gin.Context, conditions model.AutomationConditions, now time.Time) ([]model.Task, error) {
	overdueDays := conditions.OverdueDays
	if overdueDays < 0 {
		overdueDays = 1
	}
	cutoff := now.AddDate(0, 0, -overdueDays)
	query := tx.Model(&model.Task{}).
		Where("tasks.status <> ?", model.TaskCompleted).
		Where("tasks.end_at IS NOT NULL AND tasks.end_at <= ?", cutoff)

	if len(conditions.ProjectIDs) > 0 {
		query = query.Where("tasks.project_id IN ?", conditions.ProjectIDs)
	}
	if c != nil && !h.currentUserIsAdmin(c) {
		projectMap, err := h.collectVisibleProjects(c, nil)
		if err != nil {
			return nil, err
		}
		visibleProjectIDs := projectIDsFromMap(projectMap)
		if len(visibleProjectIDs) == 0 {
			return []model.Task{}, nil
		}
		query = query.Where("tasks.project_id IN ?", visibleProjectIDs)
	}

	var tasks []model.Task
	if err := query.
		Preload("Assignees").
		Preload("Project.Users").
		Order("tasks.id asc").
		Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

func automationTaskRecipients(task model.Task, actions model.AutomationActions) []uint {
	recipients := make([]uint, 0)
	if actions.NotifyAssignees {
		for _, assignee := range task.Assignees {
			recipients = append(recipients, assignee.ID)
		}
	}
	if actions.NotifyProjectOwners {
		for _, owner := range task.Project.Users {
			recipients = append(recipients, owner.ID)
		}
	}
	return uniqueUint(recipients)
}

func overdueDays(now time.Time, endAt *time.Time) int {
	if endAt == nil || !now.After(*endAt) {
		return 0
	}
	return int(now.Sub(*endAt).Hours() / 24)
}

func automationTagNames(tags []model.Tag) string {
	if len(tags) == 0 {
		return ""
	}
	parts := make([]string, 0, len(tags))
	for _, tag := range tags {
		if name := strings.TrimSpace(tag.Name); name != "" {
			parts = append(parts, name)
			continue
		}
		parts = append(parts, strconv.FormatUint(uint64(tag.ID), 10))
	}
	return strings.Join(parts, "、")
}

func (h *Handler) appendAutomationTaskTagsWithDB(tx *gorm.DB, task model.Task, actions model.AutomationActions, actorID uint) (int, error) {
	if !actions.AddTags || len(actions.TagIDs) == 0 {
		return 0, nil
	}
	tagIDs := uniqueUint(actions.TagIDs)
	if len(tagIDs) == 0 {
		return 0, nil
	}

	var existingTagIDs []uint
	if err := tx.Table("task_tags").Where("task_id = ? AND tag_id IN ?", task.ID, tagIDs).Pluck("tag_id", &existingTagIDs).Error; err != nil {
		return 0, err
	}
	missingTagIDs := make([]uint, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		if !containsUint(existingTagIDs, tagID) {
			missingTagIDs = append(missingTagIDs, tagID)
		}
	}
	if len(missingTagIDs) == 0 {
		return 0, nil
	}

	tags, err := findTagsByIDs(tx, missingTagIDs)
	if err != nil {
		return 0, err
	}
	if len(tags) == 0 {
		return 0, nil
	}
	taskRef := model.Task{BaseModel: model.BaseModel{ID: task.ID}}
	if err := tx.Model(&taskRef).Association("Tags").Append(&tags); err != nil {
		return 0, err
	}
	tagNames := automationTagNames(tags)
	detail := "自动化添加标签"
	if tagNames != "" {
		detail += "：" + tagNames
	}
	if err := h.writeTaskActivityWithDB(tx, task.ID, actorID, "automation.tags_added", taskActivitySummary("自动化添加标签", task), detail, nil); err != nil {
		return 0, err
	}
	return len(tags), nil
}

func (h *Handler) appendAutomationTaskAssigneesWithDB(tx *gorm.DB, task model.Task, actions model.AutomationActions, actorID uint) (int, []uint, error) {
	if !actions.AssignAssignees || len(actions.AssigneeIDs) == 0 {
		return 0, nil, nil
	}
	assigneeIDs := uniqueUint(actions.AssigneeIDs)
	if len(assigneeIDs) == 0 {
		return 0, nil, nil
	}

	var existingAssigneeIDs []uint
	if err := tx.Table("task_users").Where("task_id = ? AND user_id IN ?", task.ID, assigneeIDs).Pluck("user_id", &existingAssigneeIDs).Error; err != nil {
		return 0, nil, err
	}
	missingAssigneeIDs := make([]uint, 0, len(assigneeIDs))
	for _, assigneeID := range assigneeIDs {
		if !containsUint(existingAssigneeIDs, assigneeID) {
			missingAssigneeIDs = append(missingAssigneeIDs, assigneeID)
		}
	}
	if len(missingAssigneeIDs) == 0 {
		return 0, nil, nil
	}

	users, err := findUsersByIDs(tx, missingAssigneeIDs)
	if err != nil {
		return 0, nil, err
	}
	if len(users) == 0 {
		return 0, nil, nil
	}
	taskRef := model.Task{BaseModel: model.BaseModel{ID: task.ID}}
	if err := tx.Model(&taskRef).Association("Assignees").Append(&users); err != nil {
		return 0, nil, err
	}
	addedAssigneeIDs := userIDsFromUsers(users)
	content := "自动化已将你设为任务 " + task.TaskNo + " - " + task.Title + " 的执行人"
	if err := h.createNotificationsWithDB(tx, addedAssigneeIDs, "你被加入任务执行人", content, "tasks", task.ID); err != nil {
		return 0, nil, err
	}
	assigneeNames := automationAssigneeNames(users, addedAssigneeIDs)
	detail := "自动化指派执行人"
	if assigneeNames != "" {
		detail += "：" + assigneeNames
	}
	if err := h.writeTaskActivityWithDB(tx, task.ID, actorID, "automation.assignees_added", taskActivitySummary("自动化指派执行人", task), detail, nil); err != nil {
		return 0, nil, err
	}
	return len(users), addedAssigneeIDs, nil
}

func (h *Handler) executeTaskOverdueRule(tx *gorm.DB, c *gin.Context, rule model.AutomationRule, now time.Time, actorID uint) (matchedCount int, actionCount int, notifiedIDs []uint, message string, err error) {
	tasks, err := h.automationOverdueTasks(tx, c, rule.Conditions, now)
	if err != nil {
		return 0, 0, nil, "", err
	}
	tagActionCount := 0
	assigneeActionCount := 0
	for _, task := range tasks {
		recipients := automationTaskRecipients(task, rule.Actions)
		if len(recipients) > 0 {
			days := overdueDays(now, task.EndAt)
			content := fmt.Sprintf("任务 %s - %s 已逾期 %d 天，请尽快处理", task.TaskNo, task.Title, days)
			if err := h.createNotificationsWithDB(tx, recipients, "任务已逾期", content, "tasks", task.ID); err != nil {
				return len(tasks), actionCount, notifiedIDs, "", err
			}
			actionCount += len(recipients)
			notifiedIDs = append(notifiedIDs, recipients...)
		}
		addedTags, err := h.appendAutomationTaskTagsWithDB(tx, task, rule.Actions, actorID)
		if err != nil {
			return len(tasks), actionCount, notifiedIDs, "", err
		}
		actionCount += addedTags
		tagActionCount += addedTags
		addedAssignees, assigneeNotifyIDs, err := h.appendAutomationTaskAssigneesWithDB(tx, task, rule.Actions, actorID)
		if err != nil {
			return len(tasks), actionCount, notifiedIDs, "", err
		}
		actionCount += addedAssignees
		assigneeActionCount += addedAssignees
		notifiedIDs = append(notifiedIDs, assigneeNotifyIDs...)
	}
	return len(tasks), actionCount, uniqueUint(notifiedIDs), fmt.Sprintf("匹配 %d 个逾期任务，发送 %d 条通知，添加 %d 个标签，指派 %d 个执行人", len(tasks), len(notifiedIDs), tagActionCount, assigneeActionCount), nil
}

func containsTaskStatus(values []model.TaskStatus, target model.TaskStatus) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func taskStatusChangeRuleMatches(conditions model.AutomationConditions, task model.Task, oldStatus model.TaskStatus, newStatus model.TaskStatus) bool {
	if oldStatus == newStatus {
		return false
	}
	if len(conditions.ProjectIDs) > 0 && !containsUint(conditions.ProjectIDs, task.ProjectID) {
		return false
	}
	if len(conditions.FromStatuses) > 0 && !containsTaskStatus(conditions.FromStatuses, oldStatus) {
		return false
	}
	if len(conditions.ToStatuses) > 0 && !containsTaskStatus(conditions.ToStatuses, newStatus) {
		return false
	}
	return true
}

func progressBoundMatches(value int, minValue *int, maxValue *int) bool {
	if minValue != nil && value < *minValue {
		return false
	}
	if maxValue != nil && value > *maxValue {
		return false
	}
	return true
}

func taskProgressChangeRuleMatches(conditions model.AutomationConditions, task model.Task, oldProgress int, newProgress int) bool {
	if oldProgress == newProgress {
		return false
	}
	if len(conditions.ProjectIDs) > 0 && !containsUint(conditions.ProjectIDs, task.ProjectID) {
		return false
	}
	if !progressBoundMatches(oldProgress, conditions.FromProgressMin, conditions.FromProgressMax) {
		return false
	}
	if !progressBoundMatches(newProgress, conditions.ToProgressMin, conditions.ToProgressMax) {
		return false
	}
	return true
}

func containsAssigneeChangeType(values []model.AssigneeChangeType, target model.AssigneeChangeType) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func taskAssigneeChangeRuleMatches(conditions model.AutomationConditions, task model.Task, addedAssigneeIDs []uint, removedAssigneeIDs []uint) bool {
	if len(addedAssigneeIDs) == 0 && len(removedAssigneeIDs) == 0 {
		return false
	}
	if len(conditions.ProjectIDs) > 0 && !containsUint(conditions.ProjectIDs, task.ProjectID) {
		return false
	}
	addedMatches := len(addedAssigneeIDs) > 0 && containsAssigneeChangeType(conditions.AssigneeChangeTypes, model.AssigneeChangeAdded)
	removedMatches := len(removedAssigneeIDs) > 0 && containsAssigneeChangeType(conditions.AssigneeChangeTypes, model.AssigneeChangeRemoved)
	return addedMatches || removedMatches
}

func renderAutomationCommentContent(template string, task model.Task, oldStatus model.TaskStatus, newStatus model.TaskStatus) string {
	content := strings.TrimSpace(template)
	if content == "" {
		content = "自动化：任务状态已从 {fromStatus} 更新为 {toStatus}"
	}
	replacer := strings.NewReplacer(
		"{taskNo}", task.TaskNo,
		"{title}", task.Title,
		"{fromStatus}", string(oldStatus),
		"{toStatus}", string(newStatus),
	)
	return replacer.Replace(content)
}

func renderAutomationProgressCommentContent(template string, task model.Task, oldProgress int, newProgress int) string {
	content := strings.TrimSpace(template)
	if content == "" {
		content = "自动化：任务进度已从 {fromProgress}% 更新为 {toProgress}%"
	}
	replacer := strings.NewReplacer(
		"{taskNo}", task.TaskNo,
		"{title}", task.Title,
		"{fromProgress}", strconv.Itoa(oldProgress),
		"{toProgress}", strconv.Itoa(newProgress),
	)
	return replacer.Replace(content)
}

func renderAutomationAssigneeCommentContent(template string, task model.Task, addedAssigneeNames string, removedAssigneeNames string) string {
	content := strings.TrimSpace(template)
	if content == "" {
		content = "自动化：任务执行人已变更"
	}
	replacer := strings.NewReplacer(
		"{taskNo}", task.TaskNo,
		"{title}", task.Title,
		"{addedAssignees}", addedAssigneeNames,
		"{removedAssignees}", removedAssigneeNames,
	)
	return replacer.Replace(content)
}

func taskStatusChangedNotificationContent(task model.Task, oldStatus model.TaskStatus, newStatus model.TaskStatus) string {
	return fmt.Sprintf("任务 %s - %s 状态已从 %s 更新为 %s", task.TaskNo, task.Title, oldStatus, newStatus)
}

func taskProgressChangedNotificationContent(task model.Task, oldProgress int, newProgress int) string {
	return fmt.Sprintf("任务 %s - %s 进度已从 %d%% 更新为 %d%%", task.TaskNo, task.Title, oldProgress, newProgress)
}

func automationAssigneeDisplayName(user model.User) string {
	if name := strings.TrimSpace(user.Name); name != "" {
		return name
	}
	if username := strings.TrimSpace(user.Username); username != "" {
		return username
	}
	if email := strings.TrimSpace(user.Email); email != "" {
		return email
	}
	return strconv.FormatUint(uint64(user.ID), 10)
}

func automationAssigneeNames(users []model.User, ids []uint) string {
	if len(ids) == 0 {
		return "无"
	}
	nameByID := make(map[uint]string, len(users))
	for _, user := range users {
		nameByID[user.ID] = automationAssigneeDisplayName(user)
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		if name := strings.TrimSpace(nameByID[id]); name != "" {
			parts = append(parts, name)
			continue
		}
		parts = append(parts, strconv.FormatUint(uint64(id), 10))
	}
	return strings.Join(parts, "、")
}

func taskAssigneeChangedNotificationContent(task model.Task, addedAssigneeNames string, removedAssigneeNames string) string {
	return fmt.Sprintf("任务 %s - %s 执行人已变更，新增：%s，移除：%s", task.TaskNo, task.Title, addedAssigneeNames, removedAssigneeNames)
}

func (h *Handler) executeTaskStatusChangedRulesWithDB(tx *gorm.DB, task model.Task, oldStatus model.TaskStatus, newStatus model.TaskStatus, actorID uint) ([]uint, error) {
	if oldStatus == newStatus {
		return nil, nil
	}

	if err := tx.Preload("Assignees").Preload("Project.Users").First(&task, task.ID).Error; err != nil {
		return nil, err
	}

	var rules []model.AutomationRule
	if err := tx.Where("is_enabled = ? AND trigger = ?", true, model.AutomationTriggerTaskStatusChanged).Order("id asc").Find(&rules).Error; err != nil {
		return nil, err
	}

	now := time.Now()
	notifiedIDs := make([]uint, 0)
	for _, rule := range rules {
		if !taskStatusChangeRuleMatches(rule.Conditions, task, oldStatus, newStatus) {
			continue
		}

		actionCount := 0
		recipients := automationTaskRecipients(task, rule.Actions)
		if len(recipients) > 0 {
			content := taskStatusChangedNotificationContent(task, oldStatus, newStatus)
			if err := h.createNotificationsWithDB(tx, recipients, "任务状态已变更", content, "tasks", task.ID); err != nil {
				return notifiedIDs, err
			}
			actionCount += len(recipients)
			notifiedIDs = append(notifiedIDs, recipients...)
		}

		if rule.Actions.AddComment {
			comment := model.TaskComment{
				TaskID:   task.ID,
				AuthorID: actorID,
				Content:  renderAutomationCommentContent(rule.Actions.CommentContent, task, oldStatus, newStatus),
			}
			if err := tx.Create(&comment).Error; err != nil {
				return notifiedIDs, err
			}
			commentID := comment.ID
			if err := h.writeTaskActivityWithDB(tx, task.ID, actorID, "automation.comment_created", "自动化评论", comment.Content, &commentID); err != nil {
				return notifiedIDs, err
			}
			actionCount += 1
		}
		addedTags, err := h.appendAutomationTaskTagsWithDB(tx, task, rule.Actions, actorID)
		if err != nil {
			return notifiedIDs, err
		}
		actionCount += addedTags
		addedAssignees, assigneeNotifyIDs, err := h.appendAutomationTaskAssigneesWithDB(tx, task, rule.Actions, actorID)
		if err != nil {
			return notifiedIDs, err
		}
		actionCount += addedAssignees
		notifiedIDs = append(notifiedIDs, assigneeNotifyIDs...)

		logItem := model.AutomationExecutionLog{
			RuleID:       rule.ID,
			Trigger:      rule.Trigger,
			Status:       model.AutomationExecutionSuccess,
			MatchedCount: 1,
			ActionCount:  actionCount,
			Message:      fmt.Sprintf("任务 %s 状态从 %s 更新为 %s，执行 %d 个动作", task.TaskNo, oldStatus, newStatus, actionCount),
			ActorID:      actorID,
			RunSource:    "event",
		}
		if err := tx.Create(&logItem).Error; err != nil {
			return notifiedIDs, err
		}
		if err := tx.Model(&model.AutomationRule{}).Where("id = ?", rule.ID).Update("last_run_at", now).Error; err != nil {
			return notifiedIDs, err
		}
	}
	return uniqueUint(notifiedIDs), nil
}

func (h *Handler) executeTaskProgressChangedRulesWithDB(tx *gorm.DB, task model.Task, oldProgress int, newProgress int, actorID uint) ([]uint, error) {
	if oldProgress == newProgress {
		return nil, nil
	}

	if err := tx.Preload("Assignees").Preload("Project.Users").First(&task, task.ID).Error; err != nil {
		return nil, err
	}

	var rules []model.AutomationRule
	if err := tx.Where("is_enabled = ? AND trigger = ?", true, model.AutomationTriggerTaskProgressChanged).Order("id asc").Find(&rules).Error; err != nil {
		return nil, err
	}

	now := time.Now()
	notifiedIDs := make([]uint, 0)
	for _, rule := range rules {
		if !taskProgressChangeRuleMatches(rule.Conditions, task, oldProgress, newProgress) {
			continue
		}

		actionCount := 0
		recipients := automationTaskRecipients(task, rule.Actions)
		if len(recipients) > 0 {
			content := taskProgressChangedNotificationContent(task, oldProgress, newProgress)
			if err := h.createNotificationsWithDB(tx, recipients, "任务进度已变更", content, "tasks", task.ID); err != nil {
				return notifiedIDs, err
			}
			actionCount += len(recipients)
			notifiedIDs = append(notifiedIDs, recipients...)
		}

		if rule.Actions.AddComment {
			comment := model.TaskComment{
				TaskID:   task.ID,
				AuthorID: actorID,
				Content:  renderAutomationProgressCommentContent(rule.Actions.CommentContent, task, oldProgress, newProgress),
			}
			if err := tx.Create(&comment).Error; err != nil {
				return notifiedIDs, err
			}
			commentID := comment.ID
			if err := h.writeTaskActivityWithDB(tx, task.ID, actorID, "automation.comment_created", "自动化评论", comment.Content, &commentID); err != nil {
				return notifiedIDs, err
			}
			actionCount += 1
		}
		addedTags, err := h.appendAutomationTaskTagsWithDB(tx, task, rule.Actions, actorID)
		if err != nil {
			return notifiedIDs, err
		}
		actionCount += addedTags
		addedAssignees, assigneeNotifyIDs, err := h.appendAutomationTaskAssigneesWithDB(tx, task, rule.Actions, actorID)
		if err != nil {
			return notifiedIDs, err
		}
		actionCount += addedAssignees
		notifiedIDs = append(notifiedIDs, assigneeNotifyIDs...)

		logItem := model.AutomationExecutionLog{
			RuleID:       rule.ID,
			Trigger:      rule.Trigger,
			Status:       model.AutomationExecutionSuccess,
			MatchedCount: 1,
			ActionCount:  actionCount,
			Message:      fmt.Sprintf("任务 %s 进度从 %d%% 更新为 %d%%，执行 %d 个动作", task.TaskNo, oldProgress, newProgress, actionCount),
			ActorID:      actorID,
			RunSource:    "event",
		}
		if err := tx.Create(&logItem).Error; err != nil {
			return notifiedIDs, err
		}
		if err := tx.Model(&model.AutomationRule{}).Where("id = ?", rule.ID).Update("last_run_at", now).Error; err != nil {
			return notifiedIDs, err
		}
	}
	return uniqueUint(notifiedIDs), nil
}

func (h *Handler) executeTaskAssigneeChangedRulesWithDB(tx *gorm.DB, task model.Task, addedAssigneeIDs []uint, removedAssigneeIDs []uint, actorID uint) ([]uint, error) {
	if len(addedAssigneeIDs) == 0 && len(removedAssigneeIDs) == 0 {
		return nil, nil
	}

	if err := tx.Preload("Assignees").Preload("Project.Users").First(&task, task.ID).Error; err != nil {
		return nil, err
	}

	var rules []model.AutomationRule
	if err := tx.Where("is_enabled = ? AND trigger = ?", true, model.AutomationTriggerTaskAssigneeChanged).Order("id asc").Find(&rules).Error; err != nil {
		return nil, err
	}

	changedAssigneeIDs := uniqueUint(append(append([]uint{}, addedAssigneeIDs...), removedAssigneeIDs...))
	var changedAssignees []model.User
	if len(changedAssigneeIDs) > 0 {
		if err := tx.Where("id IN ?", changedAssigneeIDs).Find(&changedAssignees).Error; err != nil {
			return nil, err
		}
	}
	addedAssigneeNames := automationAssigneeNames(changedAssignees, addedAssigneeIDs)
	removedAssigneeNames := automationAssigneeNames(changedAssignees, removedAssigneeIDs)

	now := time.Now()
	notifiedIDs := make([]uint, 0)
	for _, rule := range rules {
		if !taskAssigneeChangeRuleMatches(rule.Conditions, task, addedAssigneeIDs, removedAssigneeIDs) {
			continue
		}

		actionCount := 0
		recipients := automationTaskRecipients(task, rule.Actions)
		if len(recipients) > 0 {
			content := taskAssigneeChangedNotificationContent(task, addedAssigneeNames, removedAssigneeNames)
			if err := h.createNotificationsWithDB(tx, recipients, "任务执行人已变更", content, "tasks", task.ID); err != nil {
				return notifiedIDs, err
			}
			actionCount += len(recipients)
			notifiedIDs = append(notifiedIDs, recipients...)
		}

		if rule.Actions.AddComment {
			comment := model.TaskComment{
				TaskID:   task.ID,
				AuthorID: actorID,
				Content:  renderAutomationAssigneeCommentContent(rule.Actions.CommentContent, task, addedAssigneeNames, removedAssigneeNames),
			}
			if err := tx.Create(&comment).Error; err != nil {
				return notifiedIDs, err
			}
			commentID := comment.ID
			if err := h.writeTaskActivityWithDB(tx, task.ID, actorID, "automation.comment_created", "自动化评论", comment.Content, &commentID); err != nil {
				return notifiedIDs, err
			}
			actionCount += 1
		}
		addedTags, err := h.appendAutomationTaskTagsWithDB(tx, task, rule.Actions, actorID)
		if err != nil {
			return notifiedIDs, err
		}
		actionCount += addedTags
		addedAssignees, assigneeNotifyIDs, err := h.appendAutomationTaskAssigneesWithDB(tx, task, rule.Actions, actorID)
		if err != nil {
			return notifiedIDs, err
		}
		actionCount += addedAssignees
		notifiedIDs = append(notifiedIDs, assigneeNotifyIDs...)

		logItem := model.AutomationExecutionLog{
			RuleID:       rule.ID,
			Trigger:      rule.Trigger,
			Status:       model.AutomationExecutionSuccess,
			MatchedCount: 1,
			ActionCount:  actionCount,
			Message:      fmt.Sprintf("任务 %s 执行人变更，新增 %d 人，移除 %d 人，执行 %d 个动作", task.TaskNo, len(addedAssigneeIDs), len(removedAssigneeIDs), actionCount),
			ActorID:      actorID,
			RunSource:    "event",
		}
		if err := tx.Create(&logItem).Error; err != nil {
			return notifiedIDs, err
		}
		if err := tx.Model(&model.AutomationRule{}).Where("id = ?", rule.ID).Update("last_run_at", now).Error; err != nil {
			return notifiedIDs, err
		}
	}
	return uniqueUint(notifiedIDs), nil
}

func (h *Handler) recordAutomationFailure(rule model.AutomationRule, actorID uint, source string, execErr error) model.AutomationExecutionLog {
	logItem := model.AutomationExecutionLog{
		RuleID:    rule.ID,
		Trigger:   rule.Trigger,
		Status:    model.AutomationExecutionFailed,
		Message:   execErr.Error(),
		ActorID:   actorID,
		RunSource: source,
	}
	_ = h.DB.Create(&logItem).Error
	return logItem
}

func (h *Handler) executeAutomationRule(c *gin.Context, rule model.AutomationRule, now time.Time, actorID uint, source string) (model.AutomationExecutionLog, []uint, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		source = "manual"
	}
	logItem := model.AutomationExecutionLog{
		RuleID:    rule.ID,
		Trigger:   rule.Trigger,
		ActorID:   actorID,
		RunSource: source,
	}
	var notifiedIDs []uint

	if !rule.IsEnabled {
		logItem.Status = model.AutomationExecutionSkipped
		logItem.Message = "规则已停用"
		if err := h.DB.Create(&logItem).Error; err != nil {
			return logItem, nil, err
		}
		return logItem, nil, nil
	}

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		switch rule.Trigger {
		case model.AutomationTriggerTaskOverdue:
			logItem.MatchedCount, logItem.ActionCount, notifiedIDs, logItem.Message, err = h.executeTaskOverdueRule(tx, c, rule, now, actorID)
		case model.AutomationTriggerTaskStatusChanged:
			logItem.Status = model.AutomationExecutionSkipped
			logItem.Message = "状态变更规则仅在任务状态变更事件中执行"
		case model.AutomationTriggerTaskProgressChanged:
			logItem.Status = model.AutomationExecutionSkipped
			logItem.Message = "进度变更规则仅在任务进度变更事件中执行"
		case model.AutomationTriggerTaskAssigneeChanged:
			logItem.Status = model.AutomationExecutionSkipped
			logItem.Message = "执行人变更规则仅在任务执行人变更事件中执行"
		default:
			err = fmt.Errorf("不支持的自动化触发器：%s", rule.Trigger)
		}
		if err != nil {
			return err
		}
		if logItem.Status == "" {
			logItem.Status = model.AutomationExecutionSuccess
		}
		if err := tx.Create(&logItem).Error; err != nil {
			return err
		}
		return tx.Model(&model.AutomationRule{}).Where("id = ?", rule.ID).Update("last_run_at", now).Error
	})
	if err != nil {
		failedLog := h.recordAutomationFailure(rule, actorID, source, err)
		return failedLog, nil, err
	}
	return logItem, notifiedIDs, nil
}

func (h *Handler) RunAutomationRule(c *gin.Context) {
	var rule model.AutomationRule
	if err := h.DB.First(&rule, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "AUTOMATION_RULE_NOT_FOUND", "自动化规则不存在")
		return
	}

	logItem, notifiedIDs, err := h.executeAutomationRule(c, rule, time.Now(), c.GetUint("userId"), "manual")
	if err != nil {
		respondDBError(c, http.StatusBadRequest, "RUN_AUTOMATION_RULE_FAILED", err)
		return
	}
	h.pushNotificationUpdates(notifiedIDs)
	h.writeAudit(c, "automations", "run", rule.ID, true, auditDetailf("执行自动化规则(id=%d,logId=%d)", rule.ID, logItem.ID))
	c.JSON(http.StatusOK, logItem)
}

func (h *Handler) RunEnabledAutomationRules(now time.Time, source string) (int, error) {
	var rules []model.AutomationRule
	if err := h.DB.Where("is_enabled = ? AND trigger = ?", true, model.AutomationTriggerTaskOverdue).Find(&rules).Error; err != nil {
		return 0, err
	}
	var firstErr error
	executed := 0
	for _, rule := range rules {
		_, notifiedIDs, err := h.executeAutomationRule(nil, rule, now, 0, source)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		if err == nil {
			executed += 1
			h.pushNotificationUpdates(notifiedIDs)
		}
	}
	return executed, firstErr
}
