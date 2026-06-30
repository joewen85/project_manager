package handler

import (
	"net/http"
	"strconv"
	"strings"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type workRequestCreateRequest struct {
	Type        string `json:"type" binding:"required"`
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	ProjectID   *uint  `json:"projectId"`
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
		query = query.Preload("Project").Preload("Requester").Preload("Reviewer").Preload("ConvertedTask")
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

	item := model.WorkRequest{
		Type:        requestType,
		Title:       title,
		Description: strings.TrimSpace(req.Description),
		Priority:    normalizePriority(req.Priority),
		Status:      model.WorkRequestSubmitted,
		ProjectID:   req.ProjectID,
		RequesterID: c.GetUint("userId"),
	}

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		if err := tx.Preload("Project").Preload("Requester").First(&item, item.ID).Error; err != nil {
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

	reviewerID := c.GetUint("userId")
	item.Status = nextStatus
	item.ReviewerID = &reviewerID
	item.ApprovalNote = strings.TrimSpace(req.Note)
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(item).Error; err != nil {
			return err
		}
		if err := tx.Preload("Project").Preload("Requester").Preload("Reviewer").First(item, item.ID).Error; err != nil {
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
	if requestItem.Status == model.WorkRequestRejected {
		respondError(c, http.StatusBadRequest, "WORK_REQUEST_REJECTED", "已拒绝的请求不能转为任务")
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
