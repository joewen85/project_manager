package handler

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type projectTemplateRequest struct {
	Name        string                `json:"name" binding:"required"`
	Description string                `json:"description"`
	TaskTree    []templateTaskRequest `json:"taskTree" binding:"required"`
}

type templateTaskRequest struct {
	Key              string                          `json:"key"`
	Title            string                          `json:"title" binding:"required"`
	Description      string                          `json:"description"`
	Priority         string                          `json:"priority"`
	IsMilestone      bool                            `json:"isMilestone"`
	RelativeStartDay int                             `json:"relativeStartDay"`
	DurationDays     int                             `json:"durationDays"`
	Dependencies     []templateTaskDependencyRequest `json:"dependencies"`
	Children         []templateTaskRequest           `json:"children"`
}

type templateTaskDependencyRequest struct {
	DependsOnKey string `json:"dependsOnKey" binding:"required"`
	LagDays      int    `json:"lagDays"`
	Type         string `json:"type"`
}

type createProjectFromTemplateRequest struct {
	Code          string `json:"code"`
	Name          string `json:"name" binding:"required"`
	Description   string `json:"description"`
	StartAt       string `json:"startAt"`
	EndAt         string `json:"endAt"`
	UserIDs       []uint `json:"userIds"`
	DepartmentIDs []uint `json:"departmentIds"`
}

type createProjectFromTemplateResponse struct {
	TemplateID uint          `json:"templateId"`
	Project    model.Project `json:"project"`
	Tasks      []model.Task  `json:"tasks"`
}

var errTemplateValidation = errors.New("invalid project template")

func templateValidationError(message string) error {
	return errors.New(errTemplateValidation.Error() + ": " + message)
}

func isTemplateValidationError(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), errTemplateValidation.Error()+": ")
}

func templateValidationMessage(err error) string {
	return strings.TrimPrefix(err.Error(), errTemplateValidation.Error()+": ")
}

func buildTemplateTaskTree(req []templateTaskRequest) ([]model.TemplateTask, error) {
	if len(req) == 0 {
		return nil, templateValidationError("至少需要配置一个模板任务")
	}
	keys := map[string]struct{}{}
	tasks := make([]model.TemplateTask, 0, len(req))
	for index, item := range req {
		task, err := normalizeTemplateTask(item, "task"+strconv.Itoa(index+1), keys)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := validateTemplateDependencies(tasks, keys); err != nil {
		return nil, err
	}
	return tasks, nil
}

func normalizeTemplateTask(req templateTaskRequest, fallbackKey string, keys map[string]struct{}) (model.TemplateTask, error) {
	key := strings.TrimSpace(req.Key)
	if key == "" {
		key = fallbackKey
	}
	if _, exists := keys[key]; exists {
		return model.TemplateTask{}, templateValidationError("模板任务 key 不能重复：" + key)
	}
	keys[key] = struct{}{}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		return model.TemplateTask{}, templateValidationError("模板任务标题不能为空")
	}
	durationDays := req.DurationDays
	if durationDays <= 0 {
		durationDays = 1
	}
	if req.RelativeStartDay < 0 {
		return model.TemplateTask{}, templateValidationError("相对开始天数不能小于 0：" + key)
	}

	children := make([]model.TemplateTask, 0, len(req.Children))
	for index, child := range req.Children {
		childTask, err := normalizeTemplateTask(child, key+"."+strconv.Itoa(index+1), keys)
		if err != nil {
			return model.TemplateTask{}, err
		}
		children = append(children, childTask)
	}

	dependencies := make([]model.TemplateTaskDependency, 0, len(req.Dependencies))
	for _, dep := range req.Dependencies {
		depKey := strings.TrimSpace(dep.DependsOnKey)
		if depKey == "" {
			return model.TemplateTask{}, templateValidationError("模板任务依赖 key 不能为空：" + key)
		}
		if depKey == key {
			return model.TemplateTask{}, templateValidationError("模板任务不能依赖自身：" + key)
		}
		dependencies = append(dependencies, model.TemplateTaskDependency{
			DependsOnKey: depKey,
			LagDays:      dep.LagDays,
			Type:         normalizeDependencyType(dep.Type),
		})
	}

	return model.TemplateTask{
		Key:              key,
		Title:            title,
		Description:      strings.TrimSpace(req.Description),
		Priority:         normalizePriority(req.Priority),
		IsMilestone:      req.IsMilestone,
		RelativeStartDay: req.RelativeStartDay,
		DurationDays:     durationDays,
		Dependencies:     dependencies,
		Children:         children,
	}, nil
}

func validateTemplateDependencies(tasks []model.TemplateTask, keys map[string]struct{}) error {
	var walk func([]model.TemplateTask) error
	walk = func(items []model.TemplateTask) error {
		for _, item := range items {
			for _, dep := range item.Dependencies {
				if _, exists := keys[dep.DependsOnKey]; !exists {
					return templateValidationError("模板任务依赖不存在：" + dep.DependsOnKey)
				}
			}
			if err := walk(item.Children); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(tasks)
}

func buildProjectTemplateFromRequest(req projectTemplateRequest) (model.ProjectTemplate, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return model.ProjectTemplate{}, templateValidationError("模板名称不能为空")
	}
	taskTree, err := buildTemplateTaskTree(req.TaskTree)
	if err != nil {
		return model.ProjectTemplate{}, err
	}
	return model.ProjectTemplate{
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		TaskTree:    taskTree,
	}, nil
}

func (h *Handler) ListProjectTemplates(c *gin.Context) {
	page, pageSize := parsePage(c)
	query := h.DB.Model(&model.ProjectTemplate{})
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR description LIKE ?", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PROJECT_TEMPLATES_FAILED", err)
		return
	}

	var items []model.ProjectTemplate
	if err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PROJECT_TEMPLATES_FAILED", err)
		return
	}

	c.JSON(http.StatusOK, pageResult[model.ProjectTemplate]{List: items, Total: total, Page: page, PageSize: pageSize})
}

func (h *Handler) CreateProjectTemplate(c *gin.Context) {
	var req projectTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	item, err := buildProjectTemplateFromRequest(req)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_PROJECT_TEMPLATE", templateValidationMessage(err))
		return
	}

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "templates", "create", item.ID, true, auditDetailf("创建项目模板(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_PROJECT_TEMPLATE_FAILED", err)
		return
	}

	c.JSON(http.StatusCreated, item)
}

func (h *Handler) UpdateProjectTemplate(c *gin.Context) {
	var req projectTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	next, err := buildProjectTemplateFromRequest(req)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_PROJECT_TEMPLATE", templateValidationMessage(err))
		return
	}

	var item model.ProjectTemplate
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "PROJECT_TEMPLATE_NOT_FOUND", "项目模板不存在")
		return
	}

	item.Name = next.Name
	item.Description = next.Description
	item.TaskTree = next.TaskTree
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "templates", "update", item.ID, true, auditDetailf("更新项目模板(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "UPDATE_PROJECT_TEMPLATE_FAILED", err)
		return
	}

	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteProjectTemplate(c *gin.Context) {
	var item model.ProjectTemplate
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "PROJECT_TEMPLATE_NOT_FOUND", "项目模板不存在")
		return
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&item).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "templates", "delete", item.ID, true, auditDetailf("删除项目模板(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusInternalServerError, "DELETE_PROJECT_TEMPLATE_FAILED", err)
		return
	}

	respondMessage(c, http.StatusOK, "PROJECT_TEMPLATE_DELETED", "删除成功")
}

func applyRelativeDate(projectStart *time.Time, relativeStartDay int) *time.Time {
	if projectStart == nil {
		return nil
	}
	value := projectStart.AddDate(0, 0, relativeStartDay)
	return &value
}

func applyRelativeEndDate(projectStart *time.Time, relativeStartDay, durationDays int) *time.Time {
	if projectStart == nil {
		return nil
	}
	if durationDays <= 0 {
		durationDays = 1
	}
	value := projectStart.AddDate(0, 0, relativeStartDay+durationDays-1)
	return &value
}

func flattenCreatedTasks(taskByKey map[string]model.Task) []model.Task {
	tasks := make([]model.Task, 0, len(taskByKey))
	for _, item := range taskByKey {
		tasks = append(tasks, item)
	}
	sort.Slice(tasks, func(left, right int) bool {
		return tasks[left].ID < tasks[right].ID
	})
	return tasks
}

func (h *Handler) createTasksFromTemplateTree(tx *gorm.DB, projectID uint, creatorID uint, projectStart *time.Time, tree []model.TemplateTask) (map[string]model.Task, error) {
	taskByKey := map[string]model.Task{}
	var createRecursive func([]model.TemplateTask, *uint) error
	createRecursive = func(items []model.TemplateTask, parentID *uint) error {
		for _, templateTask := range items {
			task := model.Task{
				TaskNo:      generateTaskNo(),
				Title:       templateTask.Title,
				Description: templateTask.Description,
				Status:      model.TaskPending,
				Priority:    normalizePriority(string(templateTask.Priority)),
				IsMilestone: templateTask.IsMilestone,
				Progress:    0,
				StartAt:     applyRelativeDate(projectStart, templateTask.RelativeStartDay),
				EndAt:       applyRelativeEndDate(projectStart, templateTask.RelativeStartDay, templateTask.DurationDays),
				CreatorID:   creatorID,
				ProjectID:   projectID,
				ParentID:    parentID,
			}
			if err := tx.Create(&task).Error; err != nil {
				return err
			}
			taskByKey[templateTask.Key] = task
			if err := h.writeTaskActivityWithDB(tx, task.ID, creatorID, "task.created_from_template", taskActivitySummary("由模板生成任务", task), "来源模板任务："+templateTask.Key, nil); err != nil {
				return err
			}
			parent := task.ID
			if err := createRecursive(templateTask.Children, &parent); err != nil {
				return err
			}
		}
		return nil
	}
	if err := createRecursive(tree, nil); err != nil {
		return nil, err
	}
	return taskByKey, nil
}

func createDependenciesFromTemplateTree(tx *gorm.DB, projectID uint, tree []model.TemplateTask, taskByKey map[string]model.Task) error {
	var deps []model.TaskDependency
	seen := map[string]struct{}{}
	var walk func([]model.TemplateTask)
	walk = func(items []model.TemplateTask) {
		for _, item := range items {
			task, hasTask := taskByKey[item.Key]
			if hasTask {
				for _, dep := range item.Dependencies {
					dependsOn, hasDependsOn := taskByKey[dep.DependsOnKey]
					if !hasDependsOn || dependsOn.ID == task.ID {
						continue
					}
					uniqueKey := strconv.FormatUint(uint64(task.ID), 10) + ":" + strconv.FormatUint(uint64(dependsOn.ID), 10)
					if _, exists := seen[uniqueKey]; exists {
						continue
					}
					seen[uniqueKey] = struct{}{}
					deps = append(deps, model.TaskDependency{
						TaskID:          task.ID,
						DependsOnTaskID: dependsOn.ID,
						LagDays:         dep.LagDays,
						Type:            normalizeDependencyType(dep.Type),
					})
				}
			}
			walk(item.Children)
		}
	}
	walk(tree)
	if len(deps) == 0 {
		return nil
	}

	dependencyIDs := make([]uint, 0, len(deps))
	for _, dep := range deps {
		dependencyIDs = append(dependencyIDs, dep.DependsOnTaskID)
	}
	var existingCount int64
	if err := tx.Model(&model.Task{}).Where("id IN ? AND project_id = ?", dependencyIDs, projectID).Count(&existingCount).Error; err != nil {
		return err
	}
	if int(existingCount) != len(dependencyIDs) {
		return gorm.ErrInvalidData
	}
	return tx.Create(&deps).Error
}

func (h *Handler) CreateProjectFromTemplate(c *gin.Context) {
	var req createProjectFromTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	if !h.currentUserIsAdmin(c) && !h.currentUserHasPermission(c, "templates.read") {
		respondError(c, http.StatusForbidden, "PROJECT_TEMPLATE_READ_REQUIRED", "需要模板查看权限")
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

	var template model.ProjectTemplate
	if err := h.DB.First(&template, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "PROJECT_TEMPLATE_NOT_FOUND", "项目模板不存在")
		return
	}
	if len(template.TaskTree) == 0 {
		respondError(c, http.StatusBadRequest, "PROJECT_TEMPLATE_EMPTY", "项目模板任务树为空")
		return
	}

	code := strings.TrimSpace(req.Code)
	if code == "" {
		code = generateProjectCode()
	}
	project := model.Project{
		Code:        code,
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		StartAt:     startAt,
		EndAt:       endAt,
	}
	if project.Description == "" {
		project.Description = template.Description
	}

	creatorID := c.GetUint("userId")
	var tasks []model.Task
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&project).Error; err != nil {
			return err
		}
		if len(req.UserIDs) > 0 {
			users, err := findUsersByIDs(tx, req.UserIDs)
			if err != nil {
				return err
			}
			if err := replaceAssociation(tx, &project, "Users", &users); err != nil {
				return err
			}
			if err := h.createNotificationsWithDB(tx, req.UserIDs, "你被设为项目负责人", "项目 "+project.Code+" - "+project.Name+" 已由模板创建并将你设为负责人", "projects", project.ID); err != nil {
				return err
			}
		}
		if len(req.DepartmentIDs) > 0 {
			departments, err := findDepartmentsByIDs(tx, req.DepartmentIDs)
			if err != nil {
				return err
			}
			if err := replaceAssociation(tx, &project, "Departments", &departments); err != nil {
				return err
			}
		}

		taskByKey, err := h.createTasksFromTemplateTree(tx, project.ID, creatorID, startAt, template.TaskTree)
		if err != nil {
			return err
		}
		if err := createDependenciesFromTemplateTree(tx, project.ID, template.TaskTree, taskByKey); err != nil {
			return err
		}
		tasks = flattenCreatedTasks(taskByKey)
		for index := range tasks {
			if err := h.preloadTaskResponse(tx, &tasks[index]); err != nil {
				return err
			}
		}
		if err := tx.Preload("Users").Preload("Departments").First(&project, project.ID).Error; err != nil {
			return err
		}
		if err := h.writeAuditWithDB(c, tx, "projects", "create_from_template", project.ID, true, auditDetailf("模板创建项目(templateId=%d,projectId=%d)", template.ID, project.ID)); err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "templates", "create_project", template.ID, true, auditDetailf("模板创建项目(templateId=%d,projectId=%d)", template.ID, project.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_PROJECT_FROM_TEMPLATE_FAILED", err)
		return
	}
	h.pushNotificationUpdates(req.UserIDs)

	c.JSON(http.StatusCreated, createProjectFromTemplateResponse{
		TemplateID: template.ID,
		Project:    project,
		Tasks:      tasks,
	})
}
