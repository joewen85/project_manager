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
	OverdueDays *int   `json:"overdueDays"`
	ProjectIDs  []uint `json:"projectIds"`
}

type automationActionsRequest struct {
	NotifyAssignees     *bool `json:"notifyAssignees"`
	NotifyProjectOwners *bool `json:"notifyProjectOwners"`
}

func normalizeAutomationTrigger(value string) (model.AutomationTrigger, bool) {
	switch model.AutomationTrigger(strings.TrimSpace(value)) {
	case "", model.AutomationTriggerTaskOverdue:
		return model.AutomationTriggerTaskOverdue, true
	default:
		return model.AutomationTriggerTaskOverdue, false
	}
}

func normalizeAutomationConditions(req automationConditionsRequest) (model.AutomationConditions, error) {
	overdueDays := 1
	if req.OverdueDays != nil {
		overdueDays = *req.OverdueDays
	}
	if overdueDays < 0 {
		return model.AutomationConditions{}, fmt.Errorf("逾期天数不能小于 0")
	}
	return model.AutomationConditions{
		OverdueDays: overdueDays,
		ProjectIDs:  uniqueUint(req.ProjectIDs),
	}, nil
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func normalizeAutomationActions(req automationActionsRequest) (model.AutomationActions, error) {
	defaultActions := req.NotifyAssignees == nil && req.NotifyProjectOwners == nil
	actions := model.AutomationActions{
		NotifyAssignees:     boolValue(req.NotifyAssignees, defaultActions),
		NotifyProjectOwners: boolValue(req.NotifyProjectOwners, defaultActions),
	}
	if !actions.NotifyAssignees && !actions.NotifyProjectOwners {
		return model.AutomationActions{}, fmt.Errorf("至少需要启用一个通知对象")
	}
	return actions, nil
}

func buildAutomationRuleFromRequest(req automationRuleRequest, actorID uint) (model.AutomationRule, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return model.AutomationRule{}, fmt.Errorf("规则名称不能为空")
	}
	trigger, ok := normalizeAutomationTrigger(req.Trigger)
	if !ok {
		return model.AutomationRule{}, fmt.Errorf("触发器必须是 task_overdue")
	}
	conditions, err := normalizeAutomationConditions(req.Conditions)
	if err != nil {
		return model.AutomationRule{}, err
	}
	actions, err := normalizeAutomationActions(req.Actions)
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

func (h *Handler) executeTaskOverdueRule(tx *gorm.DB, c *gin.Context, rule model.AutomationRule, now time.Time) (matchedCount int, actionCount int, notifiedIDs []uint, message string, err error) {
	tasks, err := h.automationOverdueTasks(tx, c, rule.Conditions, now)
	if err != nil {
		return 0, 0, nil, "", err
	}
	for _, task := range tasks {
		recipients := automationTaskRecipients(task, rule.Actions)
		if len(recipients) == 0 {
			continue
		}
		days := overdueDays(now, task.EndAt)
		content := fmt.Sprintf("任务 %s - %s 已逾期 %d 天，请尽快处理", task.TaskNo, task.Title, days)
		if err := h.createNotificationsWithDB(tx, recipients, "任务已逾期", content, "tasks", task.ID); err != nil {
			return len(tasks), actionCount, notifiedIDs, "", err
		}
		actionCount += len(recipients)
		notifiedIDs = append(notifiedIDs, recipients...)
	}
	return len(tasks), actionCount, uniqueUint(notifiedIDs), fmt.Sprintf("匹配 %d 个逾期任务，发送 %d 条通知", len(tasks), actionCount), nil
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
			logItem.MatchedCount, logItem.ActionCount, notifiedIDs, logItem.Message, err = h.executeTaskOverdueRule(tx, c, rule, now)
		default:
			err = fmt.Errorf("不支持的自动化触发器：%s", rule.Trigger)
		}
		if err != nil {
			return err
		}
		logItem.Status = model.AutomationExecutionSuccess
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
	if err := h.DB.Where("is_enabled = ?", true).Find(&rules).Error; err != nil {
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
