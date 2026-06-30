package handler

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"project-manager/backend/internal/model"
	"project-manager/backend/internal/util"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type apiTokenRequest struct {
	Name          string `json:"name" binding:"required"`
	Description   string `json:"description"`
	PermissionIDs []uint `json:"permissionIds" binding:"required"`
	IsEnabled     *bool  `json:"isEnabled"`
	ExpiresAt     string `json:"expiresAt"`
}

type apiTokenCreateResponse struct {
	model.APIToken
	Token string `json:"token"`
}

func permissionCodesFromPermissions(permissions []model.Permission) []string {
	codes := make([]string, 0, len(permissions))
	seen := map[string]struct{}{}
	for _, permission := range permissions {
		code := strings.TrimSpace(permission.Code)
		if code == "" {
			continue
		}
		if _, exists := seen[code]; exists {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func serviceAccountDisplayName(tokenName string) string {
	return "服务账号-" + truncateRunes(tokenName, 90)
}

func serviceRoleDescription(tokenName string) string {
	return "API Token 服务账号：" + truncateRunes(tokenName, 220)
}

func (h *Handler) scopeAPITokensQuery(c *gin.Context, query *gorm.DB) *gorm.DB {
	if h.currentUserIsAdmin(c) {
		return query
	}
	return query.Where("api_tokens.created_by_id = ?", c.GetUint("userId"))
}

func (h *Handler) ensureAPITokenReadable(c *gin.Context, id string) (*model.APIToken, bool) {
	var item model.APIToken
	query := h.scopeAPITokensQuery(c, h.DB.Model(&model.APIToken{})).
		Preload("CreatedBy").
		Preload("ServiceAccount")
	if err := query.Where("api_tokens.id = ?", id).First(&item).Error; err != nil {
		respondError(c, http.StatusNotFound, "API_TOKEN_NOT_FOUND", "API Token 不存在")
		return nil, false
	}
	return &item, true
}

func (h *Handler) ensureAPITokenWritable(c *gin.Context, id string) (*model.APIToken, bool) {
	item, ok := h.ensureAPITokenReadable(c, id)
	if !ok {
		return nil, false
	}
	if h.currentUserIsAdmin(c) || item.CreatedByID == c.GetUint("userId") {
		return item, true
	}
	respondError(c, http.StatusForbidden, "API_TOKEN_OWNER_REQUIRED", "只有 Token 创建人或管理员可以更新 API Token")
	return nil, false
}

func (h *Handler) parseAPITokenRequest(req apiTokenRequest) (model.APIToken, []model.Permission, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return model.APIToken{}, nil, fmt.Errorf("API Token 名称不能为空")
	}
	permissionIDs := uniqueUint(req.PermissionIDs)
	if len(permissionIDs) == 0 {
		return model.APIToken{}, nil, fmt.Errorf("至少选择一个权限")
	}
	permissions, err := findPermissionsByIDs(h.DB, permissionIDs)
	if err != nil {
		return model.APIToken{}, nil, err
	}
	if len(permissions) != len(permissionIDs) {
		return model.APIToken{}, nil, fmt.Errorf("权限不存在")
	}
	expiresAt, err := parseRFC3339(req.ExpiresAt)
	if err != nil {
		return model.APIToken{}, nil, fmt.Errorf("expiresAt 必须是 RFC3339 时间")
	}
	if expiresAt != nil && !expiresAt.After(time.Now()) {
		return model.APIToken{}, nil, fmt.Errorf("expiresAt 必须晚于当前时间")
	}
	return model.APIToken{
		Name:            name,
		Description:     strings.TrimSpace(req.Description),
		PermissionCodes: permissionCodesFromPermissions(permissions),
		IsEnabled:       boolValueDefault(req.IsEnabled, true),
		ExpiresAt:       expiresAt,
	}, permissions, nil
}

func (h *Handler) ListAPITokens(c *gin.Context) {
	page, pageSize := parsePage(c)
	query := h.scopeAPITokensQuery(c, h.DB.Model(&model.APIToken{}))
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("api_tokens.name LIKE ? OR api_tokens.description LIKE ? OR api_tokens.token_prefix LIKE ?", like, like, like)
	}
	if value := strings.TrimSpace(c.Query("isEnabled")); value != "" {
		query = query.Where("api_tokens.is_enabled = ?", value == "true" || value == "1")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_API_TOKENS_FAILED", err)
		return
	}
	var items []model.APIToken
	if err := query.Preload("CreatedBy").
		Preload("ServiceAccount").
		Order("api_tokens.updated_at desc, api_tokens.id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_API_TOKENS_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, pageResult[model.APIToken]{List: items, Total: total, Page: page, PageSize: pageSize})
}

func (h *Handler) ListAPITokenPermissionOptions(c *gin.Context) {
	var items []model.Permission
	if err := h.DB.Order("code asc").Find(&items).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_API_TOKEN_PERMISSIONS_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) APITokenDetail(c *gin.Context) {
	item, ok := h.ensureAPITokenReadable(c, c.Param("id"))
	if !ok {
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) CreateAPIToken(c *gin.Context) {
	var req apiTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	item, permissions, err := h.parseAPITokenRequest(req)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_API_TOKEN", err.Error())
		return
	}
	plainToken, err := util.GenerateAPIToken()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "API_TOKEN_GENERATE_FAILED", "API Token 生成失败")
		return
	}
	item.TokenPrefix = util.APITokenLookupPrefix(plainToken)
	item.TokenLastFour = util.APITokenLastFour(plainToken)
	item.TokenHash = util.HashAPIToken(plainToken)
	item.CreatedByID = c.GetUint("userId")

	serviceKey := strings.ReplaceAll(uuid.NewString(), "-", "")
	servicePassword, err := util.HashPassword(uuid.NewString() + uuid.NewString())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "PASSWORD_HASH_FAILED", "服务账号初始化失败")
		return
	}
	serviceAccount := model.User{
		Username:            "svc_token_" + serviceKey[:20],
		Name:                serviceAccountDisplayName(item.Name),
		Email:               "svc-token-" + serviceKey[:20] + "@service.local",
		Password:            servicePassword,
		IsActive:            item.IsEnabled,
		WeeklyCapacityHours: 0,
	}
	serviceRole := model.Role{
		Name:        "api-token-" + serviceKey[:24],
		Description: serviceRoleDescription(item.Name),
	}

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&serviceRole).Error; err != nil {
			return err
		}
		if err := replaceAssociation(tx, &serviceRole, "Permissions", &permissions); err != nil {
			return err
		}
		if err := tx.Create(&serviceAccount).Error; err != nil {
			return err
		}
		if err := replaceAssociation(tx, &serviceAccount, "Roles", &[]model.Role{serviceRole}); err != nil {
			return err
		}
		item.ServiceAccountID = serviceAccount.ID
		item.ServiceRoleID = serviceRole.ID
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		if err := tx.Preload("CreatedBy").Preload("ServiceAccount").First(&item, item.ID).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "api_tokens", "create", item.ID, true, auditDetailf("创建 API Token(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_API_TOKEN_FAILED", err)
		return
	}
	c.JSON(http.StatusCreated, apiTokenCreateResponse{APIToken: item, Token: plainToken})
}

func (h *Handler) UpdateAPIToken(c *gin.Context) {
	var req apiTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	item, ok := h.ensureAPITokenWritable(c, c.Param("id"))
	if !ok {
		return
	}
	if item.RevokedAt != nil {
		respondError(c, http.StatusBadRequest, "API_TOKEN_REVOKED", "已撤销的 API Token 不能更新")
		return
	}
	next, permissions, err := h.parseAPITokenRequest(req)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_API_TOKEN", err.Error())
		return
	}
	item.Name = next.Name
	item.Description = next.Description
	item.PermissionCodes = next.PermissionCodes
	item.IsEnabled = next.IsEnabled
	item.ExpiresAt = next.ExpiresAt

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(item).Error; err != nil {
			return err
		}
		var serviceRole model.Role
		if err := tx.First(&serviceRole, item.ServiceRoleID).Error; err != nil {
			return err
		}
		serviceRole.Description = serviceRoleDescription(item.Name)
		if err := tx.Save(&serviceRole).Error; err != nil {
			return err
		}
		if err := replaceAssociation(tx, &serviceRole, "Permissions", &permissions); err != nil {
			return err
		}
		if err := tx.Model(&model.User{}).Where("id = ?", item.ServiceAccountID).Updates(map[string]any{
			"name":      serviceAccountDisplayName(item.Name),
			"is_active": item.IsEnabled,
		}).Error; err != nil {
			return err
		}
		if err := tx.Preload("CreatedBy").Preload("ServiceAccount").First(item, item.ID).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "api_tokens", "update", item.ID, true, auditDetailf("更新 API Token(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "UPDATE_API_TOKEN_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteAPIToken(c *gin.Context) {
	item, ok := h.ensureAPITokenWritable(c, c.Param("id"))
	if !ok {
		return
	}
	now := time.Now()
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"is_enabled": false,
			"revoked_at": now,
		}
		if item.RevokedAt != nil {
			updates["revoked_at"] = item.RevokedAt
		}
		if err := tx.Model(&model.APIToken{}).Where("id = ?", item.ID).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.User{}).Where("id = ?", item.ServiceAccountID).Update("is_active", false).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "api_tokens", "delete", item.ID, true, auditDetailf("撤销 API Token(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusInternalServerError, "DELETE_API_TOKEN_FAILED", err)
		return
	}
	respondMessage(c, http.StatusOK, "API_TOKEN_REVOKED", "API Token 已撤销")
}
