package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type taskRequest struct {
	TaskNo       string                   `json:"taskNo"`
	Title        string                   `json:"title" binding:"required"`
	Description  string                   `json:"description"`
	CustomField1 string                   `json:"customField1"`
	CustomField2 string                   `json:"customField2"`
	CustomField3 string                   `json:"customField3"`
	Status       string                   `json:"status"`
	Priority     string                   `json:"priority"`
	IsMilestone  bool                     `json:"isMilestone"`
	Progress     int                      `json:"progress"`
	StartAt      string                   `json:"startAt"`
	EndAt        string                   `json:"endAt"`
	Attachment   *attachmentRequest       `json:"attachment"`
	Attachments  *[]attachmentRequest     `json:"attachments"`
	ProjectID    uint                     `json:"projectId" binding:"required"`
	ParentID     *uint                    `json:"parentId"`
	AssigneeIDs  []uint                   `json:"assigneeIds"`
	TagIDs       []uint                   `json:"tagIds"`
	Dependencies *[]taskDependencyRequest `json:"dependencies"`
}

type taskDependencyRequest struct {
	DependsOnTaskID uint   `json:"dependsOnTaskId"`
	LagDays         int    `json:"lagDays"`
	Type            string `json:"type"`
}

type taskScheduleRequest struct {
	StartAt string `json:"startAt"`
	EndAt   string `json:"endAt"`
}

func normalizeStatus(status string) model.TaskStatus {
	switch model.TaskStatus(status) {
	case model.TaskQueued, model.TaskProcessing, model.TaskCompleted:
		return model.TaskStatus(status)
	default:
		return model.TaskPending
	}
}

func normalizePriority(priority string) model.TaskPriority {
	switch model.TaskPriority(priority) {
	case model.TaskPriorityMedium, model.TaskPriorityLow:
		return model.TaskPriority(priority)
	default:
		return model.TaskPriorityHigh
	}
}

func prioritySortClause(order string) string {
	switch strings.ToLower(strings.TrimSpace(order)) {
	case "medium":
		return "CASE tasks.priority WHEN 'medium' THEN 0 WHEN 'high' THEN 1 WHEN '' THEN 1 WHEN 'low' THEN 2 ELSE 1 END, tasks.created_at desc"
	case "low", "asc":
		return "CASE tasks.priority WHEN 'low' THEN 0 WHEN 'medium' THEN 1 WHEN 'high' THEN 2 WHEN '' THEN 2 ELSE 2 END, tasks.created_at desc"
	case "high", "desc":
		fallthrough
	default:
		return "CASE tasks.priority WHEN 'high' THEN 0 WHEN '' THEN 0 WHEN 'medium' THEN 1 WHEN 'low' THEN 2 ELSE 0 END, tasks.created_at desc"
	}
}

func statusSortClause(order string) string {
	switch strings.ToLower(strings.TrimSpace(order)) {
	case "desc":
		return "CASE tasks.status WHEN 'completed' THEN 0 WHEN 'processing' THEN 1 WHEN 'queued' THEN 2 WHEN 'pending' THEN 3 ELSE 4 END, tasks.created_at desc"
	default:
		return "CASE tasks.status WHEN 'pending' THEN 0 WHEN 'queued' THEN 1 WHEN 'processing' THEN 2 WHEN 'completed' THEN 3 ELSE 4 END, tasks.created_at desc"
	}
}

func parseTaskSort(c *gin.Context) string {
	sortBy := strings.TrimSpace(c.Query("sortBy"))
	if sortBy == "priority" {
		return prioritySortClause(c.Query("sortOrder"))
	}
	if sortBy == "status" {
		return statusSortClause(c.Query("sortOrder"))
	}
	return parseSort(c, "tasks.id desc", map[string]string{
		"taskNo":    "tasks.task_no",
		"title":     "tasks.title",
		"status":    "tasks.status",
		"progress":  "tasks.progress",
		"startAt":   "tasks.start_at",
		"endAt":     "tasks.end_at",
		"createdAt": "tasks.created_at",
	})
}

func generateTaskNo() string {
	return "TASK-" + uuid.NewString()[0:8]
}

func normalizeDependencyType(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "SS", "FF", "SF":
		return strings.ToUpper(strings.TrimSpace(value))
	default:
		return "FS"
	}
}

func parseProjectIDs(value string) []uint {
	parts := strings.Split(strings.TrimSpace(value), ",")
	out := make([]uint, 0, len(parts))
	seen := map[uint]struct{}{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parsed, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			continue
		}
		id := uint(parsed)
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func parseCSVValues(value string) []string {
	parts := strings.Split(strings.TrimSpace(value), ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}

func parseTaskStatuses(value string) []string {
	allowed := map[string]struct{}{
		string(model.TaskPending):    {},
		string(model.TaskQueued):     {},
		string(model.TaskProcessing): {},
		string(model.TaskCompleted):  {},
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

func parseTaskPriorities(value string) []string {
	allowed := map[string]struct{}{
		string(model.TaskPriorityHigh):   {},
		string(model.TaskPriorityMedium): {},
		string(model.TaskPriorityLow):    {},
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

func buildTaskKeywordQuery(keyword string, searchFields []string) (string, []interface{}) {
	allowed := map[string]string{
		"taskNo":       "tasks.task_no",
		"title":        "tasks.title",
		"description":  "tasks.description",
		"projectName":  "projects.name",
		"priority":     "tasks.priority",
		"status":       "tasks.status",
		"customField1": "tasks.custom_field1",
		"customField2": "tasks.custom_field2",
		"customField3": "tasks.custom_field3",
	}
	conditions := make([]string, 0, len(searchFields))
	args := make([]interface{}, 0, len(searchFields))
	for _, field := range searchFields {
		column, ok := allowed[field]
		if !ok {
			continue
		}
		conditions = append(conditions, column+" LIKE ?")
		args = append(args, keyword)
	}
	if len(conditions) == 0 {
		return "tasks.task_no LIKE ? OR tasks.title LIKE ? OR tasks.description LIKE ?", []interface{}{keyword, keyword, keyword}
	}
	return strings.Join(conditions, " OR "), args
}

func diffUserIDs(newIDs []uint, oldIDs []uint) (added []uint, removed []uint) {
	oldSet := map[uint]struct{}{}
	for _, id := range oldIDs {
		oldSet[id] = struct{}{}
	}
	newSet := map[uint]struct{}{}
	for _, id := range newIDs {
		newSet[id] = struct{}{}
		if _, ok := oldSet[id]; !ok {
			added = append(added, id)
		}
	}
	for _, id := range oldIDs {
		if _, ok := newSet[id]; !ok {
			removed = append(removed, id)
		}
	}
	return added, removed
}

type taskAssigneeOption struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type ganttDependencyItem struct {
	ID              uint   `json:"id"`
	TaskID          uint   `json:"taskId"`
	DependsOnTaskID uint   `json:"dependsOnTaskId"`
	LagDays         int    `json:"lagDays"`
	Type            string `json:"type"`
}

type ganttItem struct {
	ID           uint                  `json:"id"`
	TaskNo       string                `json:"taskNo"`
	Title        string                `json:"title"`
	StartAt      *time.Time            `json:"startAt"`
	EndAt        *time.Time            `json:"endAt"`
	Progress     int                   `json:"progress"`
	ParentID     *uint                 `json:"parentId"`
	Status       string                `json:"status"`
	Priority     string                `json:"priority"`
	IsMilestone  bool                  `json:"isMilestone"`
	ProjectID    uint                  `json:"projectId"`
	ProjectCode  string                `json:"projectCode"`
	ProjectName  string                `json:"projectName"`
	Assignees    []taskAssigneeOption  `json:"assignees"`
	Dependencies []ganttDependencyItem `json:"dependencies"`
}

type projectLite struct {
	ID   uint
	Code string
	Name string
}

func (h *Handler) syncTaskDependencies(tx *gorm.DB, taskID uint, projectID uint, deps []taskDependencyRequest) error {
	if err := tx.Where("task_id = ?", taskID).Delete(&model.TaskDependency{}).Error; err != nil {
		return err
	}
	if len(deps) == 0 {
		return nil
	}

	unique := map[uint]model.TaskDependency{}
	dependencyIDs := make([]uint, 0, len(deps))
	for _, dependency := range deps {
		if dependency.DependsOnTaskID == 0 || dependency.DependsOnTaskID == taskID {
			continue
		}
		if _, ok := unique[dependency.DependsOnTaskID]; ok {
			continue
		}
		entry := model.TaskDependency{
			TaskID:          taskID,
			DependsOnTaskID: dependency.DependsOnTaskID,
			LagDays:         dependency.LagDays,
			Type:            normalizeDependencyType(dependency.Type),
		}
		unique[dependency.DependsOnTaskID] = entry
		dependencyIDs = append(dependencyIDs, dependency.DependsOnTaskID)
	}
	if len(dependencyIDs) == 0 {
		return nil
	}

	var existingCount int64
	if err := tx.Model(&model.Task{}).
		Where("id IN ? AND project_id = ?", dependencyIDs, projectID).
		Count(&existingCount).Error; err != nil {
		return err
	}
	if int(existingCount) != len(dependencyIDs) {
		return gorm.ErrInvalidData
	}

	records := make([]model.TaskDependency, 0, len(unique))
	for _, dependency := range unique {
		records = append(records, dependency)
	}
	return tx.Create(&records).Error
}

func (h *Handler) collectVisibleProjects(c *gin.Context, projectIDs []uint) (map[uint]projectLite, error) {
	query := h.scopeProjectsQuery(c, h.DB.Model(&model.Project{}))
	if len(projectIDs) > 0 {
		query = query.Where("projects.id IN ?", projectIDs)
	}
	var rows []projectLite
	if err := query.Select("projects.id, projects.code, projects.name").Find(&rows).Error; err != nil {
		return nil, err
	}
	projectMap := make(map[uint]projectLite, len(rows))
	for _, row := range rows {
		projectMap[row.ID] = row
	}
	return projectMap, nil
}

func toGanttItems(tasks []model.Task, projectMeta map[uint]projectLite) []ganttItem {
	result := make([]ganttItem, 0, len(tasks))
	for _, task := range tasks {
		meta := projectMeta[task.ProjectID]
		assignees := make([]taskAssigneeOption, 0, len(task.Assignees))
		for _, assignee := range task.Assignees {
			assignees = append(assignees, taskAssigneeOption{
				ID:       assignee.ID,
				Name:     assignee.Name,
				Username: assignee.Username,
				Email:    assignee.Email,
			})
		}
		dependencies := make([]ganttDependencyItem, 0, len(task.Dependencies))
		for _, dependency := range task.Dependencies {
			dependencies = append(dependencies, ganttDependencyItem{
				ID:              dependency.ID,
				TaskID:          dependency.TaskID,
				DependsOnTaskID: dependency.DependsOnTaskID,
				LagDays:         dependency.LagDays,
				Type:            normalizeDependencyType(dependency.Type),
			})
		}
		result = append(result, ganttItem{
			ID:           task.ID,
			TaskNo:       task.TaskNo,
			Title:        task.Title,
			StartAt:      task.StartAt,
			EndAt:        task.EndAt,
			Progress:     task.Progress,
			ParentID:     task.ParentID,
			Status:       string(task.Status),
			Priority:     string(task.Priority),
			IsMilestone:  task.IsMilestone,
			ProjectID:    task.ProjectID,
			ProjectCode:  meta.Code,
			ProjectName:  meta.Name,
			Assignees:    assignees,
			Dependencies: dependencies,
		})
	}
	return result
}

func (h *Handler) ListTasks(c *gin.Context) {
	page, pageSize := parsePage(c)
	var tasks []model.Task
	query := h.DB.Model(&model.Task{}).
		Select("tasks.*, projects.name AS project_name").
		Joins("LEFT JOIN projects ON projects.id = tasks.project_id").
		Preload("Assignees").
		Preload("Creator").
		Preload("Dependencies").
		Preload("Tags", func(db *gorm.DB) *gorm.DB { return db.Order("tags.name asc") })
	query = h.scopeTasksQuery(c, query)
	if projectID := c.Query("projectId"); projectID != "" {
		query = query.Where("tasks.project_id = ?", projectID)
	}
	if statuses := parseTaskStatuses(c.Query("statuses")); len(statuses) > 0 {
		query = query.Where("tasks.status IN ?", statuses)
	} else if status := c.Query("status"); status != "" {
		query = query.Where("tasks.status = ?", status)
	}
	if priorities := parseTaskPriorities(c.Query("priorities")); len(priorities) > 0 {
		query = query.Where("tasks.priority IN ?", priorities)
	}
	if assigneeIDs := parseProjectIDs(c.Query("assigneeIds")); len(assigneeIDs) > 0 {
		query = query.Where("EXISTS (SELECT 1 FROM task_users task_filter_users WHERE task_filter_users.task_id = tasks.id AND task_filter_users.user_id IN ?)", assigneeIDs)
	}
	if tagIDs := parseProjectIDs(c.Query("tagIds")); len(tagIDs) > 0 {
		query = query.Where("EXISTS (SELECT 1 FROM task_tags task_filter_tags WHERE task_filter_tags.task_id = tasks.id AND task_filter_tags.tag_id IN ?)", tagIDs)
	}
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		whereClause, args := buildTaskKeywordQuery(like, parseCSVValues(c.Query("searchFields")))
		query = query.Where(whereClause, args...)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_TASKS_FAILED", err)
		return
	}
	orderBy := parseTaskSort(c)
	if err := query.Order(orderBy).Offset((page - 1) * pageSize).Limit(pageSize).Find(&tasks).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_TASKS_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, pageResult[model.Task]{List: tasks, Total: total, Page: page, PageSize: pageSize})
}

func (h *Handler) TaskAssigneeOptions(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("keyword"))
	pageSize := 100
	if value := strings.TrimSpace(c.Query("pageSize")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			if parsed > 100 {
				parsed = 100
			}
			pageSize = parsed
		}
	}

	query := h.DB.Model(&model.User{}).Select("id, name, username, email")
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR username LIKE ? OR email LIKE ?", like, like, like)
	}

	var users []taskAssigneeOption
	if err := query.Order("name asc").Limit(pageSize).Find(&users).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_TASK_ASSIGNEE_OPTIONS_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (h *Handler) CreateTask(c *gin.Context) {
	var req taskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
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
	attachments, _ := requestAttachments(req.Attachment, req.Attachments)
	if err := validateAttachments(attachments, h.Cfg.UploadPublicBase); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ATTACHMENT", err.Error())
		return
	}
	if !h.ensureProjectVisible(c, strconv.FormatUint(uint64(req.ProjectID), 10)) {
		return
	}

	creatorID := c.GetUint("userId")
	taskNo := req.TaskNo
	if taskNo == "" {
		taskNo = generateTaskNo()
	}
	modelAttachments := toModelAttachments(attachments)

	item := model.Task{
		TaskNo:       taskNo,
		Title:        req.Title,
		Description:  req.Description,
		CustomField1: req.CustomField1,
		CustomField2: req.CustomField2,
		CustomField3: req.CustomField3,
		Status:       normalizeStatus(req.Status),
		Priority:     normalizePriority(req.Priority),
		IsMilestone:  req.IsMilestone,
		Progress:     req.Progress,
		StartAt:      startAt,
		EndAt:        endAt,
		Attachment:   firstModelAttachment(modelAttachments),
		Attachments:  modelAttachments,
		CreatorID:    creatorID,
		ProjectID:    req.ProjectID,
		ParentID:     req.ParentID,
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}

		if len(req.AssigneeIDs) > 0 {
			users, err := findUsersByIDs(tx, req.AssigneeIDs)
			if err != nil {
				return err
			}
			if err := replaceAssociation(tx, &item, "Assignees", &users); err != nil {
				return err
			}
			if err := h.createNotificationsWithDB(tx, req.AssigneeIDs, "任务已指派给你", "任务 "+item.TaskNo+" - "+item.Title+" 已分配给你", "tasks", item.ID); err != nil {
				return err
			}
		}
		tags, err := findTagsByIDs(tx, req.TagIDs)
		if err != nil {
			return err
		}
		if err := replaceAssociation(tx, &item, "Tags", &tags); err != nil {
			return err
		}
		dependencies := []taskDependencyRequest{}
		if req.Dependencies != nil {
			dependencies = *req.Dependencies
		}
		if err := h.syncTaskDependencies(tx, item.ID, item.ProjectID, dependencies); err != nil {
			return err
		}
		if err := h.triggerFailpoint("tasks.create.after_assignees"); err != nil {
			return err
		}

		if err := tx.Preload("Assignees").
			Preload("Creator").
			Preload("Dependencies").
			Preload("Tags", func(db *gorm.DB) *gorm.DB { return db.Order("tags.name asc") }).
			First(&item, item.ID).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "tasks", "create", item.ID, true, auditDetailf("创建任务(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_TASK_FAILED", err)
		return
	}

	if len(item.Assignees) > 0 {
		assigneeIDs := make([]uint, 0, len(item.Assignees))
		for _, assignee := range item.Assignees {
			assigneeIDs = append(assigneeIDs, assignee.ID)
		}
		h.queueTaskChannelNotifications(assigneeIDs, "任务已指派给你", "任务 "+item.TaskNo+" - "+item.Title+" 已分配给你", item)
	}

	c.JSON(http.StatusCreated, item)
}

func (h *Handler) UpdateTask(c *gin.Context) {
	var req taskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}

	var item model.Task
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "TASK_NOT_FOUND", "任务不存在")
		return
	}
	if !h.ensureProjectVisible(c, strconv.FormatUint(uint64(item.ProjectID), 10)) {
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
	attachments, provided := requestAttachments(req.Attachment, req.Attachments)
	if err := validateAttachments(attachments, h.Cfg.UploadPublicBase); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ATTACHMENT", err.Error())
		return
	}

	if req.TaskNo != "" {
		item.TaskNo = req.TaskNo
	}
	item.Title = req.Title
	item.Description = req.Description
	item.CustomField1 = req.CustomField1
	item.CustomField2 = req.CustomField2
	item.CustomField3 = req.CustomField3
	item.Status = normalizeStatus(req.Status)
	item.Priority = normalizePriority(req.Priority)
	item.IsMilestone = req.IsMilestone
	item.Progress = req.Progress
	item.StartAt = startAt
	item.EndAt = endAt
	if provided {
		modelAttachments := toModelAttachments(attachments)
		item.Attachment = firstModelAttachment(modelAttachments)
		item.Attachments = modelAttachments
	}
	item.ProjectID = req.ProjectID
	item.ParentID = req.ParentID
	if !h.ensureProjectVisible(c, strconv.FormatUint(uint64(item.ProjectID), 10)) {
		return
	}

	var addedAssigneeIDs []uint
	var removedAssigneeIDs []uint
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		var oldAssignees []model.User
		if err := tx.Model(&item).Association("Assignees").Find(&oldAssignees); err != nil {
			return err
		}
		oldIDs := make([]uint, 0, len(oldAssignees))
		for _, user := range oldAssignees {
			oldIDs = append(oldIDs, user.ID)
		}

		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		users, err := findUsersByIDs(tx, req.AssigneeIDs)
		if err != nil {
			return err
		}
		if err := replaceAssociation(tx, &item, "Assignees", &users); err != nil {
			return err
		}
		tags, err := findTagsByIDs(tx, req.TagIDs)
		if err != nil {
			return err
		}
		if err := replaceAssociation(tx, &item, "Tags", &tags); err != nil {
			return err
		}
		if req.Dependencies != nil {
			if err := h.syncTaskDependencies(tx, item.ID, item.ProjectID, *req.Dependencies); err != nil {
				return err
			}
		}
		added, removed := diffUserIDs(req.AssigneeIDs, oldIDs)
		addedAssigneeIDs = append([]uint(nil), added...)
		removedAssigneeIDs = append([]uint(nil), removed...)
		if err := h.createNotificationsWithDB(tx, added, "你被加入任务执行人", "任务 "+item.TaskNo+" - "+item.Title+" 已将你设为执行人", "tasks", item.ID); err != nil {
			return err
		}
		if err := h.createNotificationsWithDB(tx, removed, "你已被移出任务执行人", "任务 "+item.TaskNo+" - "+item.Title+" 已将你移出执行人", "tasks", item.ID); err != nil {
			return err
		}
		if err := h.triggerFailpoint("tasks.update.after_assignees"); err != nil {
			return err
		}
		if err := tx.Preload("Assignees").
			Preload("Creator").
			Preload("Dependencies").
			Preload("Tags", func(db *gorm.DB) *gorm.DB { return db.Order("tags.name asc") }).
			First(&item, item.ID).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "tasks", "update", item.ID, true, auditDetailf("更新任务(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "UPDATE_TASK_FAILED", err)
		return
	}

	if len(addedAssigneeIDs) > 0 {
		h.queueTaskChannelNotifications(addedAssigneeIDs, "你被加入任务执行人", "任务 "+item.TaskNo+" - "+item.Title+" 已将你设为执行人", item)
	}
	if len(removedAssigneeIDs) > 0 {
		h.queueTaskChannelNotifications(removedAssigneeIDs, "你已被移出任务执行人", "任务 "+item.TaskNo+" - "+item.Title+" 已将你移出执行人", item)
	}

	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteTask(c *gin.Context) {
	var item model.Task
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "TASK_NOT_FOUND", "任务不存在")
		return
	}
	if !h.ensureProjectVisible(c, strconv.FormatUint(uint64(item.ProjectID), 10)) {
		return
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := clearAssociation(tx, &item, "Assignees"); err != nil {
			return err
		}
		if err := clearAssociation(tx, &item, "Tags"); err != nil {
			return err
		}
		if err := tx.Delete(&item).Error; err != nil {
			return err
		}
		if err := h.triggerFailpoint("tasks.delete.after_delete"); err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "tasks", "delete", item.ID, true, auditDetailf("删除任务(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusInternalServerError, "DELETE_TASK_FAILED", err)
		return
	}

	respondMessage(c, http.StatusOK, "TASK_DELETED", "删除成功")
}

func (h *Handler) TaskTree(c *gin.Context) {
	projectID := c.Param("id")
	if !h.ensureProjectVisible(c, projectID) {
		return
	}
	var roots []model.Task
	if err := h.DB.
		Preload("Children.Children.Children").
		Preload("Assignees").
		Where("project_id = ? AND parent_id IS NULL", projectID).
		Find(&roots).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_TASK_TREE_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, roots)
}

func (h *Handler) Gantt(c *gin.Context) {
	projectID := c.Param("id")
	if !h.ensureProjectVisible(c, projectID) {
		return
	}
	parsedProjectID, err := strconv.ParseUint(projectID, 10, 64)
	if err != nil || parsedProjectID == 0 {
		respondError(c, http.StatusBadRequest, "INVALID_PROJECT_ID", "非法项目ID")
		return
	}

	projectMap, err := h.collectVisibleProjects(c, []uint{uint(parsedProjectID)})
	if err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_GANTT_PROJECT_FAILED", err)
		return
	}
	var tasks []model.Task
	if err := h.DB.
		Where("project_id = ?", parsedProjectID).
		Preload("Assignees").
		Preload("Dependencies").
		Order("COALESCE(start_at, created_at) asc").
		Find(&tasks).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_GANTT_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, toGanttItems(tasks, projectMap))
}

func (h *Handler) GanttPortfolio(c *gin.Context) {
	projectIDs := parseProjectIDs(c.Query("projectIds"))
	projectMap, err := h.collectVisibleProjects(c, projectIDs)
	if err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_GANTT_PORTFOLIO_PROJECTS_FAILED", err)
		return
	}
	if len(projectMap) == 0 {
		c.JSON(http.StatusOK, []ganttItem{})
		return
	}

	visibleProjectIDs := make([]uint, 0, len(projectMap))
	for id := range projectMap {
		visibleProjectIDs = append(visibleProjectIDs, id)
	}
	var tasks []model.Task
	if err := h.DB.
		Where("project_id IN ?", visibleProjectIDs).
		Preload("Assignees").
		Preload("Dependencies").
		Order("project_id asc").
		Order("COALESCE(start_at, created_at) asc").
		Find(&tasks).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_GANTT_PORTFOLIO_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, toGanttItems(tasks, projectMap))
}

func scheduleDuration(task model.Task) time.Duration {
	if task.StartAt != nil && task.EndAt != nil && task.EndAt.After(*task.StartAt) {
		return task.EndAt.Sub(*task.StartAt)
	}
	return 24 * time.Hour
}

func dependencyStartAt(dependency model.TaskDependency, predecessorStartAt, predecessorEndAt *time.Time, duration time.Duration) *time.Time {
	lag := time.Duration(dependency.LagDays) * 24 * time.Hour
	depType := normalizeDependencyType(dependency.Type)
	switch depType {
	case "SS":
		if predecessorStartAt == nil {
			return nil
		}
		result := predecessorStartAt.Add(lag)
		return &result
	case "FF":
		if predecessorEndAt == nil {
			return nil
		}
		result := predecessorEndAt.Add(lag).Add(-duration)
		return &result
	case "SF":
		if predecessorStartAt == nil {
			return nil
		}
		result := predecessorStartAt.Add(lag).Add(-duration)
		return &result
	default:
		if predecessorEndAt == nil {
			return nil
		}
		result := predecessorEndAt.Add(lag)
		return &result
	}
}

func (h *Handler) autoResolveProjectDependencies(tx *gorm.DB, projectID uint) (int, error) {
	var tasks []model.Task
	if err := tx.Where("project_id = ?", projectID).Preload("Dependencies").Find(&tasks).Error; err != nil {
		return 0, err
	}
	if len(tasks) == 0 {
		return 0, nil
	}

	taskByID := make(map[uint]*model.Task, len(tasks))
	indegree := make(map[uint]int, len(tasks))
	nextMap := map[uint][]uint{}
	for index := range tasks {
		task := &tasks[index]
		taskByID[task.ID] = task
		indegree[task.ID] = 0
	}

	for _, task := range tasks {
		for _, dependency := range task.Dependencies {
			if _, ok := taskByID[dependency.DependsOnTaskID]; !ok {
				continue
			}
			indegree[task.ID] += 1
			nextMap[dependency.DependsOnTaskID] = append(nextMap[dependency.DependsOnTaskID], task.ID)
		}
	}

	queue := make([]uint, 0, len(tasks))
	for id, degree := range indegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	ordered := make([]uint, 0, len(tasks))
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		ordered = append(ordered, id)
		for _, childID := range nextMap[id] {
			indegree[childID] -= 1
			if indegree[childID] == 0 {
				queue = append(queue, childID)
			}
		}
	}

	if len(ordered) != len(tasks) {
		return 0, gorm.ErrInvalidData
	}

	changed := 0
	for _, id := range ordered {
		task := taskByID[id]
		duration := scheduleDuration(*task)
		var nextStartAt *time.Time
		if task.StartAt != nil {
			startCopy := *task.StartAt
			nextStartAt = &startCopy
		}

		for _, dependency := range task.Dependencies {
			predecessor := taskByID[dependency.DependsOnTaskID]
			if predecessor == nil {
				continue
			}
			candidate := dependencyStartAt(dependency, predecessor.StartAt, predecessor.EndAt, duration)
			if candidate == nil {
				continue
			}
			if nextStartAt == nil || nextStartAt.Before(*candidate) {
				candidateCopy := *candidate
				nextStartAt = &candidateCopy
			}
		}

		if nextStartAt == nil {
			continue
		}
		nextEndAt := nextStartAt.Add(duration)
		needUpdateStart := task.StartAt == nil || !task.StartAt.Equal(*nextStartAt)
		needUpdateEnd := task.EndAt == nil || !task.EndAt.Equal(nextEndAt)
		if !needUpdateStart && !needUpdateEnd {
			continue
		}
		task.StartAt = nextStartAt
		task.EndAt = &nextEndAt
		if err := tx.Model(&model.Task{}).
			Where("id = ?", task.ID).
			Updates(map[string]any{
				"start_at": task.StartAt,
				"end_at":   task.EndAt,
			}).Error; err != nil {
			return changed, err
		}
		changed += 1
	}

	return changed, nil
}

func (h *Handler) AutoResolveProjectDependencies(c *gin.Context) {
	projectID := c.Param("id")
	if !h.ensureProjectVisible(c, projectID) {
		return
	}
	parsedProjectID, err := strconv.ParseUint(projectID, 10, 64)
	if err != nil || parsedProjectID == 0 {
		respondError(c, http.StatusBadRequest, "INVALID_PROJECT_ID", "非法项目ID")
		return
	}

	updatedCount := 0
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		nextCount, err := h.autoResolveProjectDependencies(tx, uint(parsedProjectID))
		if err != nil {
			return err
		}
		updatedCount = nextCount
		return nil
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "AUTO_RESOLVE_DEPENDENCY_FAILED", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"updatedCount": updatedCount,
		"projectId":    parsedProjectID,
	})
}

func (h *Handler) UpdateTaskDependencies(c *gin.Context) {
	taskID := c.Param("id")
	var item model.Task
	if err := h.DB.First(&item, taskID).Error; err != nil {
		respondError(c, http.StatusNotFound, "TASK_NOT_FOUND", "任务不存在")
		return
	}

	if !h.ensureProjectVisible(c, strconv.FormatUint(uint64(item.ProjectID), 10)) {
		return
	}

	var req struct {
		Dependencies []taskDependencyRequest `json:"dependencies"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := h.syncTaskDependencies(tx, item.ID, item.ProjectID, req.Dependencies); err != nil {
			return err
		}
		return tx.Preload("Dependencies").First(&item, item.ID).Error
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "UPDATE_TASK_DEPENDENCIES_FAILED", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"taskId":       item.ID,
		"dependencies": item.Dependencies,
	})
}

func (h *Handler) UpdateTaskSchedule(c *gin.Context) {
	taskID := c.Param("id")
	var item model.Task
	if err := h.DB.First(&item, taskID).Error; err != nil {
		respondError(c, http.StatusNotFound, "TASK_NOT_FOUND", "任务不存在")
		return
	}
	if !h.ensureProjectVisible(c, strconv.FormatUint(uint64(item.ProjectID), 10)) {
		return
	}

	var req taskScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
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
	if startAt == nil || endAt == nil || !endAt.After(*startAt) {
		respondError(c, http.StatusBadRequest, "INVALID_SCHEDULE_RANGE", "开始和结束时间必须有效且结束晚于开始")
		return
	}

	autoResolve := strings.TrimSpace(c.Query("autoResolve")) != "false"
	updatedCount := 0
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		item.StartAt = startAt
		item.EndAt = endAt
		if err := tx.Model(&item).Updates(map[string]any{
			"start_at": item.StartAt,
			"end_at":   item.EndAt,
		}).Error; err != nil {
			return err
		}
		if autoResolve {
			nextCount, err := h.autoResolveProjectDependencies(tx, item.ProjectID)
			if err != nil {
				return err
			}
			updatedCount = nextCount
		}
		return tx.Preload("Assignees").Preload("Dependencies").First(&item, item.ID).Error
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "UPDATE_TASK_SCHEDULE_FAILED", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"updatedCount": updatedCount,
		"task":         item,
	})
}

func (h *Handler) MyTasks(c *gin.Context) {
	uid := c.GetUint("userId")
	type result struct {
		MyTasks       []model.Task `json:"myTasks"`
		MyCreated     []model.Task `json:"myCreated"`
		MyParticipate []model.Task `json:"myParticipate"`
	}
	out := result{
		MyTasks:       make([]model.Task, 0),
		MyCreated:     make([]model.Task, 0),
		MyParticipate: make([]model.Task, 0),
	}

	base := h.DB.Model(&model.Task{}).
		Select("tasks.*, projects.name AS project_name").
		Joins("LEFT JOIN projects ON projects.id = tasks.project_id")

	if err := base.Session(&gorm.Session{}).
		Joins("JOIN task_users tu ON tu.task_id = tasks.id").
		Where("tu.user_id = ?", uid).
		Find(&out.MyTasks).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_MY_TASKS_FAILED", err)
		return
	}
	if err := base.Session(&gorm.Session{}).
		Where("tasks.creator_id = ?", uid).
		Find(&out.MyCreated).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_MY_TASKS_FAILED", err)
		return
	}
	if err := base.Session(&gorm.Session{}).
		Joins("JOIN task_users tu ON tu.task_id = tasks.id").
		Where("tu.user_id = ? AND tasks.creator_id <> ?", uid, uid).
		Find(&out.MyParticipate).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_MY_TASKS_FAILED", err)
		return
	}

	c.JSON(http.StatusOK, out)
}
