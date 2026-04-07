package handler

import (
	"net/http"
	"strconv"
	"strings"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type projectRequest struct {
	Code          string               `json:"code"`
	Name          string               `json:"name" binding:"required"`
	Description   string               `json:"description"`
	StartAt       string               `json:"startAt"`
	EndAt         string               `json:"endAt"`
	Attachment    *attachmentRequest   `json:"attachment"`
	Attachments   *[]attachmentRequest `json:"attachments"`
	UserIDs       []uint               `json:"userIds"`
	DepartmentIDs []uint               `json:"departmentIds"`
}

type projectEditorOptionUser struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type projectEditorOptionDepartment struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type projectEditorOptionsResponse struct {
	Users       []projectEditorOptionUser       `json:"users"`
	Departments []projectEditorOptionDepartment `json:"departments"`
}

func generateProjectCode() string {
	return "PROJ-" + strings.ToUpper(uuid.NewString()[0:8])
}

func buildProjectKeywordQuery(keyword string, searchFields []string) (string, []interface{}) {
	allowed := map[string]string{
		"code":        "projects.code",
		"name":        "projects.name",
		"description": "projects.description",
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
		return "projects.code LIKE ? OR projects.name LIKE ? OR projects.description LIKE ?", []interface{}{keyword, keyword, keyword}
	}
	return strings.Join(conditions, " OR "), args
}

func (h *Handler) ListProjects(c *gin.Context) {
	page, pageSize := parsePage(c)
	var projects []model.Project
	query := h.DB.Model(&model.Project{})
	query = h.scopeProjectsQuery(c, query)
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		whereClause, args := buildProjectKeywordQuery(like, parseCSVValues(c.Query("searchFields")))
		query = query.Where(whereClause, args...)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PROJECTS_FAILED", err)
		return
	}
	orderBy := parseSort(c, "projects.id desc", map[string]string{
		"code":      "projects.code",
		"name":      "projects.name",
		"startAt":   "projects.start_at",
		"endAt":     "projects.end_at",
		"createdAt": "projects.created_at",
	})
	if err := query.Preload("Users").Preload("Departments").Order(orderBy).Offset((page - 1) * pageSize).Limit(pageSize).Find(&projects).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PROJECTS_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, pageResult[model.Project]{List: projects, Total: total, Page: page, PageSize: pageSize})
}

func (h *Handler) ProjectEditorOptions(c *gin.Context) {
	keyword := strings.TrimSpace(c.Query("keyword"))
	userKeyword := strings.TrimSpace(c.Query("userKeyword"))
	departmentKeyword := strings.TrimSpace(c.Query("departmentKeyword"))
	if userKeyword == "" {
		userKeyword = keyword
	}
	if departmentKeyword == "" {
		departmentKeyword = keyword
	}
	pageSize := 100
	if value := strings.TrimSpace(c.Query("pageSize")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			if parsed > 100 {
				parsed = 100
			}
			pageSize = parsed
		}
	}

	usersQuery := h.DB.Model(&model.User{}).Select("id, name, username, email")
	departmentsQuery := h.DB.Model(&model.Department{}).Select("id, name")
	if userKeyword != "" {
		like := "%" + userKeyword + "%"
		usersQuery = usersQuery.Where("name LIKE ? OR username LIKE ? OR email LIKE ?", like, like, like)
	}
	if departmentKeyword != "" {
		like := "%" + departmentKeyword + "%"
		departmentsQuery = departmentsQuery.Where("name LIKE ?", like)
	}

	var users []projectEditorOptionUser
	if err := usersQuery.Order("name asc").Limit(pageSize).Find(&users).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PROJECT_EDITOR_USERS_FAILED", err)
		return
	}

	var departments []projectEditorOptionDepartment
	if err := departmentsQuery.Order("name asc").Limit(pageSize).Find(&departments).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PROJECT_EDITOR_DEPARTMENTS_FAILED", err)
		return
	}

	c.JSON(http.StatusOK, projectEditorOptionsResponse{
		Users:       users,
		Departments: departments,
	})
}

func (h *Handler) CreateProject(c *gin.Context) {
	var req projectRequest
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

	code := strings.TrimSpace(req.Code)
	if code == "" {
		code = generateProjectCode()
	}
	modelAttachments := toModelAttachments(attachments)

	item := model.Project{
		Code:        code,
		Name:        req.Name,
		Description: req.Description,
		StartAt:     startAt,
		EndAt:       endAt,
		Attachment:  firstModelAttachment(modelAttachments),
		Attachments: modelAttachments,
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}

		if len(req.UserIDs) > 0 {
			users, err := findUsersByIDs(tx, req.UserIDs)
			if err != nil {
				return err
			}
			if err := replaceAssociation(tx, &item, "Users", &users); err != nil {
				return err
			}
			if err := h.createNotificationsWithDB(tx, req.UserIDs, "你被设为项目负责人", "项目 "+item.Code+" - "+item.Name+" 已将你设为负责人", "projects", item.ID); err != nil {
				return err
			}
		}
		if len(req.DepartmentIDs) > 0 {
			departments, err := findDepartmentsByIDs(tx, req.DepartmentIDs)
			if err != nil {
				return err
			}
			if err := replaceAssociation(tx, &item, "Departments", &departments); err != nil {
				return err
			}
		}
		if err := h.triggerFailpoint("projects.create.after_relations"); err != nil {
			return err
		}

		if err := tx.Preload("Users").Preload("Departments").First(&item, item.ID).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "projects", "create", item.ID, true, auditDetailf("创建项目(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_PROJECT_FAILED", err)
		return
	}
	h.pushNotificationUpdates(req.UserIDs)

	c.JSON(http.StatusCreated, item)
}

func (h *Handler) UpdateProject(c *gin.Context) {
	var req projectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}

	code := strings.TrimSpace(req.Code)
	if code == "" {
		respondError(c, http.StatusBadRequest, "PROJECT_CODE_REQUIRED", "项目编码不能为空")
		return
	}

	var item model.Project
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "PROJECT_NOT_FOUND", "项目不存在")
		return
	}
	if !h.ensureProjectVisible(c, strconv.FormatUint(uint64(item.ID), 10)) {
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

	item.Code = code
	item.Name = req.Name
	item.Description = req.Description
	item.StartAt = startAt
	item.EndAt = endAt
	if provided {
		modelAttachments := toModelAttachments(attachments)
		item.Attachment = firstModelAttachment(modelAttachments)
		item.Attachments = modelAttachments
	}
	var addedUsers []uint
	var removedUsers []uint
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		var oldUsers []model.User
		if err := tx.Model(&item).Association("Users").Find(&oldUsers); err != nil {
			return err
		}
		oldUserIDs := make([]uint, 0, len(oldUsers))
		for _, user := range oldUsers {
			oldUserIDs = append(oldUserIDs, user.ID)
		}

		if err := tx.Save(&item).Error; err != nil {
			return err
		}

		users, err := findUsersByIDs(tx, req.UserIDs)
		if err != nil {
			return err
		}
		if err := replaceAssociation(tx, &item, "Users", &users); err != nil {
			return err
		}
		addedUsers, removedUsers = diffUserIDs(req.UserIDs, oldUserIDs)
		if err := h.createNotificationsWithDB(tx, addedUsers, "你被加入项目负责人", "项目 "+item.Code+" - "+item.Name+" 已将你设为负责人", "projects", item.ID); err != nil {
			return err
		}
		if err := h.createNotificationsWithDB(tx, removedUsers, "你已被移出项目负责人", "项目 "+item.Code+" - "+item.Name+" 已将你移出负责人", "projects", item.ID); err != nil {
			return err
		}

		departments, err := findDepartmentsByIDs(tx, req.DepartmentIDs)
		if err != nil {
			return err
		}
		if err := replaceAssociation(tx, &item, "Departments", &departments); err != nil {
			return err
		}
		if err := h.triggerFailpoint("projects.update.after_relations"); err != nil {
			return err
		}

		if err := tx.Preload("Users").Preload("Departments").First(&item, item.ID).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "projects", "update", item.ID, true, auditDetailf("更新项目(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "UPDATE_PROJECT_FAILED", err)
		return
	}
	h.pushNotificationUpdates(append(addedUsers, removedUsers...))

	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteProject(c *gin.Context) {
	var item model.Project
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "PROJECT_NOT_FOUND", "项目不存在")
		return
	}
	if !h.ensureProjectVisible(c, strconv.FormatUint(uint64(item.ID), 10)) {
		return
	}
	var taskCount int64
	h.DB.Model(&model.Task{}).Where("project_id = ?", item.ID).Count(&taskCount)
	if taskCount > 0 {
		respondError(c, http.StatusBadRequest, "PROJECT_HAS_TASKS", "请先删除或迁移项目下任务")
		return
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := clearAssociation(tx, &item, "Users"); err != nil {
			return err
		}
		if err := clearAssociation(tx, &item, "Departments"); err != nil {
			return err
		}
		if err := tx.Delete(&item).Error; err != nil {
			return err
		}
		if err := h.triggerFailpoint("projects.delete.after_delete"); err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "projects", "delete", item.ID, true, auditDetailf("删除项目(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusInternalServerError, "DELETE_PROJECT_FAILED", err)
		return
	}

	respondMessage(c, http.StatusOK, "PROJECT_DELETED", "删除成功")
}

func (h *Handler) ProjectDetail(c *gin.Context) {
	if !h.ensureProjectVisible(c, c.Param("id")) {
		return
	}
	var project model.Project
	if err := h.DB.
		Preload("Users").
		Preload("Departments").
		Preload("Tasks.Assignees").
		Where("id = ?", c.Param("id")).
		First(&project).Error; err != nil {
		respondError(c, http.StatusNotFound, "PROJECT_NOT_FOUND", "项目不存在")
		return
	}
	c.JSON(http.StatusOK, project)
}
