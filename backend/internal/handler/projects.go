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
	Code                  string                       `json:"code"`
	Name                  string                       `json:"name" binding:"required"`
	Description           string                       `json:"description"`
	StartAt               string                       `json:"startAt"`
	EndAt                 string                       `json:"endAt"`
	BudgetAmount          *float64                     `json:"budgetAmount"`
	ActualCostAmount      *float64                     `json:"actualCostAmount"`
	ExpectedRevenueAmount *float64                     `json:"expectedRevenueAmount"`
	ContractNo            *string                      `json:"contractNo"`
	ContractAttachments   *[]contractAttachmentRequest `json:"contractAttachments"`
	Attachment            *attachmentRequest           `json:"attachment"`
	Attachments           *[]attachmentRequest         `json:"attachments"`
	UserIDs               []uint                       `json:"userIds"`
	DepartmentIDs         []uint                       `json:"departmentIds"`
}

type contractAttachmentRequest struct {
	FileName     string `json:"fileName"`
	FilePath     string `json:"filePath"`
	RelativePath string `json:"relativePath"`
	FileSize     int64  `json:"fileSize"`
	MimeType     string `json:"mimeType"`
	Category     string `json:"category"`
	Version      string `json:"version"`
	AccessLevel  string `json:"accessLevel"`
	ExpiresAt    string `json:"expiresAt"`
}

type projectResponse struct {
	model.Project
	BudgetAmount          *float64                   `json:"budgetAmount,omitempty"`
	ActualCostAmount      *float64                   `json:"actualCostAmount,omitempty"`
	ExpectedRevenueAmount *float64                   `json:"expectedRevenueAmount,omitempty"`
	ContractNo            *string                    `json:"contractNo,omitempty"`
	ContractAttachments   []model.ContractAttachment `json:"contractAttachments,omitempty"`
	BudgetUsageRate       *float64                   `json:"budgetUsageRate,omitempty"`
	CostOverBudget        *bool                      `json:"costOverBudget,omitempty"`
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

func (h *Handler) canReadProjectFinance(c *gin.Context) bool {
	return h.currentUserIsAdmin(c) || h.currentUserHasPermission(c, "finance.read") || h.currentUserHasPermission(c, "finance.update")
}

func (h *Handler) canUpdateProjectFinance(c *gin.Context) bool {
	return h.currentUserIsAdmin(c) || h.currentUserHasPermission(c, "finance.update")
}

func hasProjectFinancePayload(req projectRequest) bool {
	return req.BudgetAmount != nil ||
		req.ActualCostAmount != nil ||
		req.ExpectedRevenueAmount != nil ||
		req.ContractNo != nil ||
		req.ContractAttachments != nil
}

func validateProjectAmount(value float64, fieldLabel string) (string, string, bool) {
	if value < 0 {
		return "INVALID_PROJECT_FINANCE_AMOUNT", fieldLabel + "不能小于0", false
	}
	return "", "", true
}

func normalizeContractCategory(value string) string {
	switch strings.TrimSpace(value) {
	case "invoice", "acceptance", "change", "other":
		return strings.TrimSpace(value)
	default:
		return "contract"
	}
}

func normalizeContractAccessLevel(value string) string {
	switch strings.TrimSpace(value) {
	case "internal", "external":
		return strings.TrimSpace(value)
	default:
		return "finance"
	}
}

func contractAttachmentToAttachmentRequest(item contractAttachmentRequest) attachmentRequest {
	return attachmentRequest{
		FileName:     item.FileName,
		FilePath:     item.FilePath,
		RelativePath: item.RelativePath,
		FileSize:     item.FileSize,
		MimeType:     item.MimeType,
	}
}

func isContractAttachmentEmpty(item contractAttachmentRequest) bool {
	return isAttachmentEmpty(contractAttachmentToAttachmentRequest(item)) &&
		strings.TrimSpace(item.Category) == "" &&
		strings.TrimSpace(item.Version) == "" &&
		strings.TrimSpace(item.AccessLevel) == "" &&
		strings.TrimSpace(item.ExpiresAt) == ""
}

func (h *Handler) normalizeContractAttachments(c *gin.Context, items []contractAttachmentRequest) ([]model.ContractAttachment, bool) {
	attachmentRequests := make([]attachmentRequest, 0, len(items))
	for _, item := range items {
		if isContractAttachmentEmpty(item) {
			continue
		}
		attachmentRequests = append(attachmentRequests, contractAttachmentToAttachmentRequest(item))
	}
	if err := validateAttachments(attachmentRequests, h.Cfg.UploadPublicBase); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_CONTRACT_ATTACHMENT", err.Error())
		return nil, false
	}
	out := make([]model.ContractAttachment, 0, len(items))
	for _, item := range items {
		if isContractAttachmentEmpty(item) {
			continue
		}
		expiresAt, err := parseRFC3339(item.ExpiresAt)
		if err != nil {
			respondError(c, http.StatusBadRequest, "INVALID_CONTRACT_ATTACHMENT_EXPIRES_AT", "合同附件到期时间必须是 RFC3339 格式")
			return nil, false
		}
		out = append(out, model.ContractAttachment{
			FileName:     strings.TrimSpace(item.FileName),
			FilePath:     normalizeAttachmentPath(item.FilePath),
			RelativePath: normalizeRelativeUploadPath(item.RelativePath),
			FileSize:     item.FileSize,
			MimeType:     strings.TrimSpace(item.MimeType),
			Category:     normalizeContractCategory(item.Category),
			Version:      strings.TrimSpace(item.Version),
			AccessLevel:  normalizeContractAccessLevel(item.AccessLevel),
			ExpiresAt:    expiresAt,
		})
	}
	return out, true
}

func (h *Handler) applyProjectFinanceRequest(c *gin.Context, item *model.Project, req projectRequest) (bool, bool) {
	if !hasProjectFinancePayload(req) {
		return false, true
	}
	if !h.canUpdateProjectFinance(c) {
		respondError(c, http.StatusForbidden, "PROJECT_FINANCE_PERMISSION_REQUIRED", "需要 finance.update 权限才能维护项目经营信息")
		return true, false
	}
	if req.BudgetAmount != nil {
		if code, message, ok := validateProjectAmount(*req.BudgetAmount, "项目预算"); !ok {
			respondError(c, http.StatusBadRequest, code, message)
			return true, false
		}
		item.BudgetAmount = *req.BudgetAmount
	}
	if req.ActualCostAmount != nil {
		if code, message, ok := validateProjectAmount(*req.ActualCostAmount, "项目成本"); !ok {
			respondError(c, http.StatusBadRequest, code, message)
			return true, false
		}
		item.ActualCostAmount = *req.ActualCostAmount
	}
	if req.ExpectedRevenueAmount != nil {
		if code, message, ok := validateProjectAmount(*req.ExpectedRevenueAmount, "预计收益"); !ok {
			respondError(c, http.StatusBadRequest, code, message)
			return true, false
		}
		item.ExpectedRevenueAmount = *req.ExpectedRevenueAmount
	}
	if req.ContractNo != nil {
		item.ContractNo = strings.TrimSpace(*req.ContractNo)
	}
	if req.ContractAttachments != nil {
		attachments, ok := h.normalizeContractAttachments(c, *req.ContractAttachments)
		if !ok {
			return true, false
		}
		item.ContractAttachments = attachments
	}
	return true, true
}

func projectCostOverBudget(project model.Project) bool {
	return project.BudgetAmount > 0 && project.ActualCostAmount > project.BudgetAmount
}

func projectBudgetUsageRate(project model.Project) float64 {
	if project.BudgetAmount <= 0 {
		return 0
	}
	return clampPercent(project.ActualCostAmount / project.BudgetAmount * 100)
}

func (h *Handler) projectResponse(c *gin.Context, project model.Project) projectResponse {
	response := projectResponse{Project: project}
	if h.canReadProjectFinance(c) {
		budgetAmount := project.BudgetAmount
		actualCostAmount := project.ActualCostAmount
		expectedRevenueAmount := project.ExpectedRevenueAmount
		contractNo := project.ContractNo
		budgetUsageRate := projectBudgetUsageRate(project)
		costOverBudget := projectCostOverBudget(project)
		response.BudgetAmount = &budgetAmount
		response.ActualCostAmount = &actualCostAmount
		response.ExpectedRevenueAmount = &expectedRevenueAmount
		response.ContractNo = &contractNo
		response.ContractAttachments = project.ContractAttachments
		response.BudgetUsageRate = &budgetUsageRate
		response.CostOverBudget = &costOverBudget
	}
	return response
}

func (h *Handler) projectResponses(c *gin.Context, projects []model.Project) []projectResponse {
	out := make([]projectResponse, 0, len(projects))
	for _, project := range projects {
		out = append(out, h.projectResponse(c, project))
	}
	return out
}

func (h *Handler) notifyProjectBudgetExceededWithDB(tx *gorm.DB, project model.Project) ([]uint, error) {
	if !projectCostOverBudget(project) {
		return nil, nil
	}
	recipients := make([]uint, 0, len(project.Users))
	for _, user := range project.Users {
		recipients = append(recipients, user.ID)
	}
	recipients = uniqueUint(recipients)
	if len(recipients) == 0 {
		return nil, nil
	}
	content := "项目 " + project.Code + " - " + project.Name + " 当前成本 " +
		strconv.FormatFloat(project.ActualCostAmount, 'f', 2, 64) +
		" 已超过预算 " + strconv.FormatFloat(project.BudgetAmount, 'f', 2, 64)
	if err := h.createNotificationsWithDB(tx, recipients, "项目成本超预算", content, "projects", project.ID); err != nil {
		return nil, err
	}
	return recipients, nil
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
		"updatedAt": "projects.updated_at",
	})
	if err := query.Preload("Users").Preload("Departments").Order(orderBy).Offset((page - 1) * pageSize).Limit(pageSize).Find(&projects).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_PROJECTS_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, pageResult[projectResponse]{List: h.projectResponses(c, projects), Total: total, Page: page, PageSize: pageSize})
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
	if _, ok := h.applyProjectFinanceRequest(c, &item, req); !ok {
		return
	}
	var notifyIDs []uint
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
		budgetRecipients, err := h.notifyProjectBudgetExceededWithDB(tx, item)
		if err != nil {
			return err
		}
		notifyIDs = append(notifyIDs, budgetRecipients...)
		return h.writeAuditWithDB(c, tx, "projects", "create", item.ID, true, auditDetailf("创建项目(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_PROJECT_FAILED", err)
		return
	}
	notifyIDs = append(notifyIDs, req.UserIDs...)
	h.pushNotificationUpdates(uniqueUint(notifyIDs))

	c.JSON(http.StatusCreated, h.projectResponse(c, item))
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
	wasOverBudget := projectCostOverBudget(item)
	if _, ok := h.applyProjectFinanceRequest(c, &item, req); !ok {
		return
	}
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
		if !wasOverBudget && projectCostOverBudget(item) {
			budgetRecipients, err := h.notifyProjectBudgetExceededWithDB(tx, item)
			if err != nil {
				return err
			}
			addedUsers = append(addedUsers, budgetRecipients...)
		}
		return h.writeAuditWithDB(c, tx, "projects", "update", item.ID, true, auditDetailf("更新项目(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "UPDATE_PROJECT_FAILED", err)
		return
	}
	h.pushNotificationUpdates(uniqueUint(append(addedUsers, removedUsers...)))

	c.JSON(http.StatusOK, h.projectResponse(c, item))
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
		Preload("Tasks.Reviewers").
		Where("id = ?", c.Param("id")).
		First(&project).Error; err != nil {
		respondError(c, http.StatusNotFound, "PROJECT_NOT_FOUND", "项目不存在")
		return
	}
	c.JSON(http.StatusOK, h.projectResponse(c, project))
}
