package handler

import (
	"net/http"
	"strconv"
	"strings"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
)

type projectRequest struct {
	Code          string `json:"code" binding:"required"`
	Name          string `json:"name" binding:"required"`
	Description   string `json:"description"`
	StartAt       string `json:"startAt"`
	EndAt         string `json:"endAt"`
	UserIDs       []uint `json:"userIds"`
	DepartmentIDs []uint `json:"departmentIds"`
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

func (h *Handler) ListProjects(c *gin.Context) {
	page, pageSize := parsePage(c)
	var projects []model.Project
	query := h.DB.Model(&model.Project{})
	query = h.scopeProjectsQuery(c, query)
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("code LIKE ? OR name LIKE ? OR description LIKE ?", like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "QUERY_PROJECTS_FAILED", err.Error())
		return
	}
	if err := query.Preload("Users").Preload("Departments").Offset((page - 1) * pageSize).Limit(pageSize).Find(&projects).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "QUERY_PROJECTS_FAILED", err.Error())
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
		respondError(c, http.StatusInternalServerError, "QUERY_PROJECT_EDITOR_USERS_FAILED", err.Error())
		return
	}

	var departments []projectEditorOptionDepartment
	if err := departmentsQuery.Order("name asc").Limit(pageSize).Find(&departments).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "QUERY_PROJECT_EDITOR_DEPARTMENTS_FAILED", err.Error())
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
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
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

	item := model.Project{
		Code:        req.Code,
		Name:        req.Name,
		Description: req.Description,
		StartAt:     startAt,
		EndAt:       endAt,
	}
	if err := h.DB.Create(&item).Error; err != nil {
		respondError(c, http.StatusBadRequest, "CREATE_PROJECT_FAILED", err.Error())
		return
	}

	if len(req.UserIDs) > 0 {
		var users []model.User
		h.DB.Where("id IN ?", req.UserIDs).Find(&users)
		h.DB.Model(&item).Association("Users").Replace(&users)
		h.createNotifications(req.UserIDs, "你被设为项目负责人", "项目 "+item.Code+" - "+item.Name+" 已将你设为负责人", "projects", item.ID)
	}
	if len(req.DepartmentIDs) > 0 {
		var departments []model.Department
		h.DB.Where("id IN ?", req.DepartmentIDs).Find(&departments)
		h.DB.Model(&item).Association("Departments").Replace(&departments)
	}

	h.DB.Preload("Users").Preload("Departments").First(&item, item.ID)
	h.writeAudit(c, "projects", "create", item.ID, true, "创建项目")
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) UpdateProject(c *gin.Context) {
	var req projectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	var item model.Project
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "PROJECT_NOT_FOUND", "项目不存在")
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

	item.Code = req.Code
	item.Name = req.Name
	item.Description = req.Description
	item.StartAt = startAt
	item.EndAt = endAt
	if err := h.DB.Save(&item).Error; err != nil {
		respondError(c, http.StatusBadRequest, "UPDATE_PROJECT_FAILED", err.Error())
		return
	}

	var oldUsers []model.User
	h.DB.Model(&item).Association("Users").Find(&oldUsers)
	oldUserIDs := make([]uint, 0, len(oldUsers))
	for _, user := range oldUsers {
		oldUserIDs = append(oldUserIDs, user.ID)
	}

	var users []model.User
	if len(req.UserIDs) > 0 {
		h.DB.Where("id IN ?", req.UserIDs).Find(&users)
	}
	h.DB.Model(&item).Association("Users").Replace(&users)
	addedUsers, removedUsers := diffUserIDs(req.UserIDs, oldUserIDs)
	h.createNotifications(addedUsers, "你被加入项目负责人", "项目 "+item.Code+" - "+item.Name+" 已将你设为负责人", "projects", item.ID)
	h.createNotifications(removedUsers, "你已被移出项目负责人", "项目 "+item.Code+" - "+item.Name+" 已将你移出负责人", "projects", item.ID)

	var departments []model.Department
	if len(req.DepartmentIDs) > 0 {
		h.DB.Where("id IN ?", req.DepartmentIDs).Find(&departments)
	}
	h.DB.Model(&item).Association("Departments").Replace(&departments)

	h.DB.Preload("Users").Preload("Departments").First(&item, item.ID)
	h.writeAudit(c, "projects", "update", item.ID, true, "更新项目")
	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteProject(c *gin.Context) {
	var item model.Project
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "PROJECT_NOT_FOUND", "项目不存在")
		return
	}
	var taskCount int64
	h.DB.Model(&model.Task{}).Where("project_id = ?", item.ID).Count(&taskCount)
	if taskCount > 0 {
		respondError(c, http.StatusBadRequest, "PROJECT_HAS_TASKS", "请先删除或迁移项目下任务")
		return
	}
	if err := h.DB.Model(&item).Association("Users").Clear(); err != nil {
		respondError(c, http.StatusInternalServerError, "CLEAR_PROJECT_USERS_FAILED", err.Error())
		return
	}
	if err := h.DB.Model(&item).Association("Departments").Clear(); err != nil {
		respondError(c, http.StatusInternalServerError, "CLEAR_PROJECT_DEPARTMENTS_FAILED", err.Error())
		return
	}
	if err := h.DB.Delete(&item).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "DELETE_PROJECT_FAILED", err.Error())
		return
	}
	h.writeAudit(c, "projects", "delete", item.ID, true, "删除项目")
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
