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

type workRequestCreateRequest struct {
	Type          string                         `json:"type" binding:"required"`
	Title         string                         `json:"title" binding:"required"`
	Description   string                         `json:"description"`
	Priority      string                         `json:"priority"`
	ProjectID     *uint                          `json:"projectId"`
	TargetTaskID  *uint                          `json:"targetTaskId"`
	ChangePayload model.WorkRequestChangePayload `json:"changePayload"`
}

type workRequestReviewRequest struct {
	Status string `json:"status" binding:"required"`
	Note   string `json:"note"`
}

type workRequestConvertTaskRequest struct {
	ProjectID   uint   `json:"projectId" binding:"required"`
	AssigneeIDs []uint `json:"assigneeIds"`
	ReviewerIDs []uint `json:"reviewerIds"`
	TagIDs      []uint `json:"tagIds"`
	StartAt     string `json:"startAt"`
	EndAt       string `json:"endAt"`
}

func normalizeWorkRequestType(value string) (model.WorkRequestType, bool) {
	switch model.WorkRequestType(strings.TrimSpace(value)) {
	case model.WorkRequestProject, model.WorkRequestTask, model.WorkRequestBug, model.WorkRequestChange:
		return model.WorkRequestType(strings.TrimSpace(value)), true
	default:
		return model.WorkRequestTask, false
	}
}

func normalizeWorkRequestStatus(value string) (model.WorkRequestStatus, bool) {
	switch model.WorkRequestStatus(strings.TrimSpace(value)) {
	case model.WorkRequestApproved, model.WorkRequestRejected:
		return model.WorkRequestStatus(strings.TrimSpace(value)), true
	default:
		return model.WorkRequestSubmitted, false
	}
}

func parseWorkRequestStatuses(value string) []string {
	allowed := map[string]struct{}{
		string(model.WorkRequestSubmitted): {},
		string(model.WorkRequestApproved):  {},
		string(model.WorkRequestRejected):  {},
		string(model.WorkRequestConverted): {},
		string(model.WorkRequestApplied):   {},
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

func normalizeOptionalTaskPriority(value model.TaskPriority) (model.TaskPriority, bool) {
	switch value {
	case "":
		return "", true
	case model.TaskPriorityHigh, model.TaskPriorityMedium, model.TaskPriorityLow:
		return value, true
	default:
		return "", false
	}
}

func normalizeWorkRequestChangePayload(payload model.WorkRequestChangePayload) (model.WorkRequestChangePayload, bool, string) {
	next := payload
	next.ScopeDescription = strings.TrimSpace(next.ScopeDescription)
	priority, ok := normalizeOptionalTaskPriority(next.Priority)
	if !ok {
		return next, false, "变更优先级必须是 high、medium 或 low"
	}
	next.Priority = priority
	next.AssigneeIDs = uniqueUint(next.AssigneeIDs)
	if next.StartAt != nil && next.EndAt != nil && !next.EndAt.After(*next.StartAt) {
		return next, false, "变更结束时间必须晚于开始时间"
	}
	hasChange := next.StartAt != nil ||
		next.EndAt != nil ||
		next.Priority != "" ||
		len(next.AssigneeIDs) > 0 ||
		next.ScopeDescription != ""
	if !hasChange {
		return next, false, "至少需要填写一个变更项"
	}
	return next, true, ""
}

func changePayloadDetail(payload model.WorkRequestChangePayload) string {
	lines := make([]string, 0, 5)
	if payload.StartAt != nil {
		lines = append(lines, "申请开始时间："+formatTaskActivityTime(payload.StartAt))
	}
	if payload.EndAt != nil {
		lines = append(lines, "申请结束时间："+formatTaskActivityTime(payload.EndAt))
	}
	if payload.Priority != "" {
		lines = append(lines, "申请优先级："+string(payload.Priority))
	}
	if len(payload.AssigneeIDs) > 0 {
		lines = append(lines, "申请执行人："+formatUintList(payload.AssigneeIDs))
	}
	if payload.ScopeDescription != "" {
		lines = append(lines, "范围说明："+payload.ScopeDescription)
	}
	return strings.Join(lines, "\n")
}

func (h *Handler) scopeWorkRequestsQuery(c *gin.Context, query *gorm.DB) *gorm.DB {
	if h.currentUserIsAdmin(c) || h.currentUserHasPermission(c, "requests.update") {
		return query
	}
	return query.Where("work_requests.requester_id = ?", c.GetUint("userId"))
}

func (h *Handler) ensureWorkRequestVisible(c *gin.Context, id string, preload bool) (*model.WorkRequest, bool) {
	var item model.WorkRequest
	query := h.scopeWorkRequestsQuery(c, h.DB.Model(&model.WorkRequest{}))
	if preload {
		query = query.Preload("Project").Preload("Requester").Preload("Reviewer").Preload("ConvertedTask").Preload("TargetTask").Preload("AppliedBy")
	}
	if err := query.Where("work_requests.id = ?", id).First(&item).Error; err != nil {
		respondError(c, http.StatusNotFound, "WORK_REQUEST_NOT_FOUND", "请求不存在")
		return nil, false
	}
	return &item, true
}

func (h *Handler) ListWorkRequests(c *gin.Context) {
	page, pageSize := parsePage(c)
	query := h.scopeWorkRequestsQuery(c, h.DB.Model(&model.WorkRequest{}))
	if statuses := parseWorkRequestStatuses(c.Query("statuses")); len(statuses) > 0 {
		query = query.Where("work_requests.status IN ?", statuses)
	} else if status := strings.TrimSpace(c.Query("status")); status != "" {
		if statuses := parseWorkRequestStatuses(status); len(statuses) > 0 {
			query = query.Where("work_requests.status IN ?", statuses)
		}
	}
	if requestType := strings.TrimSpace(c.Query("type")); requestType != "" {
		if parsed, ok := normalizeWorkRequestType(requestType); ok {
			query = query.Where("work_requests.type = ?", parsed)
		}
	}
	if projectID := strings.TrimSpace(c.Query("projectId")); projectID != "" {
		query = query.Where("work_requests.project_id = ?", projectID)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("work_requests.title LIKE ? OR work_requests.description LIKE ?", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_WORK_REQUESTS_FAILED", err)
		return
	}
	var items []model.WorkRequest
	if err := query.
		Preload("Project").
		Preload("Requester").
		Preload("Reviewer").
		Preload("ConvertedTask").
		Preload("TargetTask").
		Preload("AppliedBy").
		Order("work_requests.id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_WORK_REQUESTS_FAILED", err)
		return
	}

	c.JSON(http.StatusOK, pageResult[model.WorkRequest]{List: items, Total: total, Page: page, PageSize: pageSize})
}

func (h *Handler) CreateWorkRequest(c *gin.Context) {
	var req workRequestCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	requestType, ok := normalizeWorkRequestType(req.Type)
	if !ok {
		respondError(c, http.StatusBadRequest, "INVALID_WORK_REQUEST_TYPE", "请求类型必须是 project、task、bug 或 change")
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		respondError(c, http.StatusBadRequest, "WORK_REQUEST_TITLE_REQUIRED", "请求标题不能为空")
		return
	}
	if req.ProjectID != nil {
		if !h.ensureProjectVisible(c, strconv.FormatUint(uint64(*req.ProjectID), 10)) {
			return
		}
	}
	changePayload := model.WorkRequestChangePayload{}
	var targetTaskID *uint
	if requestType == model.WorkRequestChange {
		if req.TargetTaskID == nil || *req.TargetTaskID == 0 {
			respondError(c, http.StatusBadRequest, "CHANGE_TARGET_TASK_REQUIRED", "变更申请必须选择目标任务")
			return
		}
		targetTask, ok := h.ensureTaskVisible(c, strconv.FormatUint(uint64(*req.TargetTaskID), 10))
		if !ok {
			return
		}
		if req.ProjectID != nil && *req.ProjectID != targetTask.ProjectID {
			respondError(c, http.StatusBadRequest, "CHANGE_PROJECT_MISMATCH", "变更申请关联项目必须与目标任务一致")
			return
		}
		projectID := targetTask.ProjectID
		req.ProjectID = &projectID
		normalizedPayload, valid, message := normalizeWorkRequestChangePayload(req.ChangePayload)
		if !valid {
			respondError(c, http.StatusBadRequest, "INVALID_CHANGE_PAYLOAD", message)
			return
		}
		if len(normalizedPayload.AssigneeIDs) > 0 {
			users, err := findUsersByIDs(h.DB, normalizedPayload.AssigneeIDs)
			if err != nil {
				respondDBError(c, http.StatusInternalServerError, "QUERY_CHANGE_ASSIGNEES_FAILED", err)
				return
			}
			if len(users) != len(normalizedPayload.AssigneeIDs) {
				respondError(c, http.StatusBadRequest, "INVALID_CHANGE_ASSIGNEES", "变更执行人不存在")
				return
			}
		}
		changePayload = normalizedPayload
		targetTaskID = req.TargetTaskID
	}

	item := model.WorkRequest{
		Type:          requestType,
		Title:         title,
		Description:   strings.TrimSpace(req.Description),
		Priority:      normalizePriority(req.Priority),
		Status:        model.WorkRequestSubmitted,
		ProjectID:     req.ProjectID,
		RequesterID:   c.GetUint("userId"),
		TargetTaskID:  targetTaskID,
		ChangePayload: changePayload,
	}

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		if err := tx.Preload("Project").Preload("Requester").Preload("TargetTask").First(&item, item.ID).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "requests", "create", item.ID, true, auditDetailf("提交工作请求(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_WORK_REQUEST_FAILED", err)
		return
	}

	c.JSON(http.StatusCreated, item)
}

func (h *Handler) ReviewWorkRequest(c *gin.Context) {
	var req workRequestReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	nextStatus, ok := normalizeWorkRequestStatus(req.Status)
	if !ok {
		respondError(c, http.StatusBadRequest, "INVALID_WORK_REQUEST_STATUS", "审批状态必须是 approved 或 rejected")
		return
	}
	item, visible := h.ensureWorkRequestVisible(c, c.Param("id"), false)
	if !visible {
		return
	}
	if item.Status == model.WorkRequestConverted {
		respondError(c, http.StatusBadRequest, "WORK_REQUEST_ALREADY_CONVERTED", "已转为任务的请求不能再次审批")
		return
	}
	if item.Status == model.WorkRequestApplied {
		respondError(c, http.StatusBadRequest, "WORK_REQUEST_ALREADY_APPLIED", "已应用的变更请求不能再次审批")
		return
	}

	reviewerID := c.GetUint("userId")
	item.Status = nextStatus
	item.ReviewerID = &reviewerID
	item.ApprovalNote = strings.TrimSpace(req.Note)
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(item).Error; err != nil {
			return err
		}
		if err := tx.Preload("Project").Preload("Requester").Preload("Reviewer").Preload("TargetTask").Preload("AppliedBy").First(item, item.ID).Error; err != nil {
			return err
		}
		if err := h.createNotificationsWithDB(tx, []uint{item.RequesterID}, "请求已审批", "请求「"+item.Title+"」已更新为 "+string(item.Status), "requests", item.ID); err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "requests", "review", item.ID, true, auditDetailf("审批工作请求(id=%d,status=%s)", item.ID, item.Status))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "REVIEW_WORK_REQUEST_FAILED", err)
		return
	}
	h.pushNotificationUpdates([]uint{item.RequesterID})

	c.JSON(http.StatusOK, item)
}

func (h *Handler) ConvertWorkRequestToTask(c *gin.Context) {
	var req workRequestConvertTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	if !h.ensureProjectVisible(c, strconv.FormatUint(uint64(req.ProjectID), 10)) {
		return
	}
	startAt, err := parseRFC3339(req.StartAt)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_START_AT", "startAt 必须是 RFC3339 时间格式")
		return
	}
	endAt, err := parseRFC3339(req.EndAt)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_END_AT", "endAt 必须是 RFC3339 时间格式")
		return
	}

	requestItem, visible := h.ensureWorkRequestVisible(c, c.Param("id"), false)
	if !visible {
		return
	}
	if requestItem.ConvertedTaskID != nil {
		respondError(c, http.StatusBadRequest, "WORK_REQUEST_ALREADY_CONVERTED", "请求已转为任务")
		return
	}
	if requestItem.Type == model.WorkRequestChange {
		respondError(c, http.StatusBadRequest, "CHANGE_REQUEST_NOT_CONVERTIBLE", "变更申请应审批后应用，不能转为任务")
		return
	}
	if requestItem.Status == model.WorkRequestRejected {
		respondError(c, http.StatusBadRequest, "WORK_REQUEST_REJECTED", "已拒绝的请求不能转为任务")
		return
	}
	if requestItem.Status == model.WorkRequestApplied {
		respondError(c, http.StatusBadRequest, "WORK_REQUEST_ALREADY_APPLIED", "已应用的变更请求不能转为任务")
		return
	}

	currentUserID := c.GetUint("userId")
	task := model.Task{
		TaskNo:      generateTaskNo(),
		Title:       requestItem.Title,
		Description: requestItem.Description,
		Status:      model.TaskPending,
		Priority:    requestItem.Priority,
		Progress:    0,
		StartAt:     startAt,
		EndAt:       endAt,
		CreatorID:   currentUserID,
		ProjectID:   req.ProjectID,
	}
	var notifyIDs []uint
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		if len(req.AssigneeIDs) > 0 {
			users, err := findUsersByIDs(tx, req.AssigneeIDs)
			if err != nil {
				return err
			}
			if err := replaceAssociation(tx, &task, "Assignees", &users); err != nil {
				return err
			}
			notifyIDs = append(notifyIDs, req.AssigneeIDs...)
			if err := h.createNotificationsWithDB(tx, req.AssigneeIDs, "任务已指派给你", "任务 "+task.TaskNo+" - "+task.Title+" 已由请求转入并分配给你", "tasks", task.ID); err != nil {
				return err
			}
		}
		if len(req.ReviewerIDs) > 0 {
			reviewers, err := findUsersByIDs(tx, req.ReviewerIDs)
			if err != nil {
				return err
			}
			if err := replaceAssociation(tx, &task, "Reviewers", &reviewers); err != nil {
				return err
			}
			notifyIDs = append(notifyIDs, req.ReviewerIDs...)
			if err := h.createNotificationsWithDB(tx, req.ReviewerIDs, "你被设为任务审核人", "任务 "+task.TaskNo+" - "+task.Title+" 已由请求转入并将你设为审核人", "tasks", task.ID); err != nil {
				return err
			}
		}
		tags, err := findTagsByIDs(tx, req.TagIDs)
		if err != nil {
			return err
		}
		if err := replaceAssociation(tx, &task, "Tags", &tags); err != nil {
			return err
		}
		if err := h.preloadTaskResponse(tx, &task); err != nil {
			return err
		}
		requestItem.Status = model.WorkRequestConverted
		requestItem.ReviewerID = &currentUserID
		requestItem.ConvertedTaskID = &task.ID
		if err := tx.Save(requestItem).Error; err != nil {
			return err
		}
		if err := h.writeTaskActivityWithDB(tx, task.ID, currentUserID, "task.created_from_request", taskActivitySummary("由请求转入任务", task), "来源请求："+requestItem.Title, nil); err != nil {
			return err
		}
		if err := h.createNotificationsWithDB(tx, []uint{requestItem.RequesterID}, "请求已转为任务", "请求「"+requestItem.Title+"」已转为任务 "+task.TaskNo, "tasks", task.ID); err != nil {
			return err
		}
		notifyIDs = append(notifyIDs, requestItem.RequesterID)
		if err := h.writeAuditWithDB(c, tx, "tasks", "create_from_request", task.ID, true, auditDetailf("请求转任务(requestId=%d,taskId=%d)", requestItem.ID, task.ID)); err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "requests", "convert_to_task", requestItem.ID, true, auditDetailf("请求转任务(requestId=%d,taskId=%d)", requestItem.ID, task.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CONVERT_WORK_REQUEST_FAILED", err)
		return
	}
	h.pushNotificationUpdates(notifyIDs)
	if len(req.AssigneeIDs) > 0 {
		h.queueTaskChannelNotifications(req.AssigneeIDs, "任务已指派给你", "任务 "+task.TaskNo+" - "+task.Title+" 已由请求转入并分配给你", task)
	}

	c.JSON(http.StatusCreated, gin.H{
		"requestId": requestItem.ID,
		"task":      task,
	})
}

func (h *Handler) ApplyWorkRequestChange(c *gin.Context) {
	requestItem, visible := h.ensureWorkRequestVisible(c, c.Param("id"), false)
	if !visible {
		return
	}
	if requestItem.Type != model.WorkRequestChange {
		respondError(c, http.StatusBadRequest, "WORK_REQUEST_NOT_CHANGE", "只有变更申请可以应用")
		return
	}
	if requestItem.Status == model.WorkRequestApplied {
		respondError(c, http.StatusBadRequest, "WORK_REQUEST_ALREADY_APPLIED", "变更申请已应用")
		return
	}
	if requestItem.Status == model.WorkRequestRejected {
		respondError(c, http.StatusBadRequest, "WORK_REQUEST_REJECTED", "已拒绝的变更申请不能应用")
		return
	}
	if requestItem.Status != model.WorkRequestApproved {
		respondError(c, http.StatusBadRequest, "WORK_REQUEST_NOT_APPROVED", "变更申请审批通过后才能应用")
		return
	}
	if requestItem.TargetTaskID == nil || *requestItem.TargetTaskID == 0 {
		respondError(c, http.StatusBadRequest, "CHANGE_TARGET_TASK_REQUIRED", "变更申请缺少目标任务")
		return
	}

	var task model.Task
	if err := h.DB.First(&task, *requestItem.TargetTaskID).Error; err != nil {
		respondError(c, http.StatusNotFound, "TASK_NOT_FOUND", "任务不存在")
		return
	}
	if !h.ensureProjectVisible(c, strconv.FormatUint(uint64(task.ProjectID), 10)) {
		return
	}
	payload, valid, message := normalizeWorkRequestChangePayload(requestItem.ChangePayload)
	if !valid {
		respondError(c, http.StatusBadRequest, "INVALID_CHANGE_PAYLOAD", message)
		return
	}

	var oldAssignees []model.User
	if err := h.DB.Model(&task).Association("Assignees").Find(&oldAssignees); err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_TASK_ASSIGNEES_FAILED", err)
		return
	}
	oldAssigneeIDs := userIDsFromUsers(oldAssignees)
	oldItem := task
	currentUserID := c.GetUint("userId")
	now := time.Now()
	var addedAssigneeIDs []uint
	var removedAssigneeIDs []uint
	automationEffects := automationExecutionSideEffects{}

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if payload.StartAt != nil {
			task.StartAt = payload.StartAt
		}
		if payload.EndAt != nil {
			task.EndAt = payload.EndAt
		}
		if task.StartAt != nil && task.EndAt != nil && !task.EndAt.After(*task.StartAt) {
			return fmt.Errorf("INVALID_SCHEDULE_RANGE")
		}
		if payload.Priority != "" {
			task.Priority = payload.Priority
		}
		if err := tx.Save(&task).Error; err != nil {
			return err
		}
		if len(payload.AssigneeIDs) > 0 {
			users, err := findUsersByIDs(tx, payload.AssigneeIDs)
			if err != nil {
				return err
			}
			if len(users) != len(payload.AssigneeIDs) {
				return fmt.Errorf("INVALID_CHANGE_ASSIGNEES")
			}
			if err := replaceAssociation(tx, &task, "Assignees", &users); err != nil {
				return err
			}
			added, removed := diffUserIDs(payload.AssigneeIDs, oldAssigneeIDs)
			addedAssigneeIDs = append([]uint(nil), added...)
			removedAssigneeIDs = append([]uint(nil), removed...)
			if err := h.createNotificationsWithDB(tx, added, "你被加入任务执行人", "变更申请「"+requestItem.Title+"」已应用，任务 "+task.TaskNo+" - "+task.Title+" 已将你设为执行人", "tasks", task.ID); err != nil {
				return err
			}
			if err := h.createNotificationsWithDB(tx, removed, "你已被移出任务执行人", "变更申请「"+requestItem.Title+"」已应用，任务 "+task.TaskNo+" - "+task.Title+" 已将你移出执行人", "tasks", task.ID); err != nil {
				return err
			}
		}
		if err := h.preloadTaskResponse(tx, &task); err != nil {
			return err
		}
		nextAssigneeIDs := oldAssigneeIDs
		if len(payload.AssigneeIDs) > 0 {
			nextAssigneeIDs = payload.AssigneeIDs
		}
		detail := taskUpdateActivityDetail(oldItem, task, oldAssigneeIDs, nextAssigneeIDs, nil, nil, false, false)
		payloadDetail := changePayloadDetail(payload)
		if payloadDetail != "" {
			detail = strings.TrimSpace(detail + "\n" + payloadDetail)
		}
		if requestItem.ApprovalNote != "" {
			detail = strings.TrimSpace(detail + "\n审批意见：" + requestItem.ApprovalNote)
		}
		if err := h.writeTaskActivityWithDB(tx, task.ID, currentUserID, "change_request.applied", taskActivitySummary("应用变更申请", task), detail, nil); err != nil {
			return err
		}
		requestItem.Status = model.WorkRequestApplied
		requestItem.AppliedAt = &now
		requestItem.AppliedByID = &currentUserID
		if err := tx.Save(requestItem).Error; err != nil {
			return err
		}
		if err := h.createNotificationsWithDB(tx, []uint{requestItem.RequesterID}, "变更申请已应用", "变更申请「"+requestItem.Title+"」已应用到任务 "+task.TaskNo, "tasks", task.ID); err != nil {
			return err
		}
		if len(addedAssigneeIDs) > 0 || len(removedAssigneeIDs) > 0 {
			effects, err := h.executeTaskAssigneeChangedRulesWithDB(tx, task, addedAssigneeIDs, removedAssigneeIDs, currentUserID)
			if err != nil {
				return err
			}
			appendAutomationEffects(&automationEffects, effects)
		}
		if err := h.writeAuditWithDB(c, tx, "requests", "apply_change", requestItem.ID, true, auditDetailf("应用变更申请(requestId=%d,taskId=%d)", requestItem.ID, task.ID)); err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "tasks", "apply_change_request", task.ID, true, auditDetailf("应用变更申请(requestId=%d,taskId=%d)", requestItem.ID, task.ID))
	}); err != nil {
		message := err.Error()
		if message == "INVALID_SCHEDULE_RANGE" {
			respondError(c, http.StatusBadRequest, "INVALID_SCHEDULE_RANGE", "应用变更后任务开始和结束时间必须有效且结束晚于开始")
			return
		}
		if message == "INVALID_CHANGE_ASSIGNEES" {
			respondError(c, http.StatusBadRequest, "INVALID_CHANGE_ASSIGNEES", "变更执行人不存在")
			return
		}
		respondDBError(c, http.StatusBadRequest, "APPLY_CHANGE_REQUEST_FAILED", err)
		return
	}
	h.deliverAutomationWebhooks(automationEffects.WebhookJobs)
	if len(addedAssigneeIDs) > 0 {
		h.queueTaskChannelNotifications(addedAssigneeIDs, "你被加入任务执行人", "变更申请「"+requestItem.Title+"」已应用，任务 "+task.TaskNo+" - "+task.Title+" 已将你设为执行人", task)
	}
	if len(removedAssigneeIDs) > 0 {
		h.queueTaskChannelNotifications(removedAssigneeIDs, "你已被移出任务执行人", "变更申请「"+requestItem.Title+"」已应用，任务 "+task.TaskNo+" - "+task.Title+" 已将你移出执行人", task)
	}
	notifyIDs := append(append([]uint{}, addedAssigneeIDs...), removedAssigneeIDs...)
	notifyIDs = append(notifyIDs, requestItem.RequesterID)
	notifyIDs = append(notifyIDs, automationEffects.NotifiedIDs...)
	h.pushNotificationUpdates(notifyIDs)

	if err := h.DB.Preload("Project").Preload("Requester").Preload("Reviewer").Preload("TargetTask").Preload("AppliedBy").First(requestItem, requestItem.ID).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_CHANGE_REQUEST_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"request": requestItem,
		"task":    task,
	})
}
