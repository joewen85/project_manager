package handler

import (
	"net/http"
	"strings"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type tagRequest struct {
	Name string `json:"name" binding:"required"`
}

func (h *Handler) GetTag(c *gin.Context) {
	var item model.Tag
	if err := h.DB.
		Model(&model.Tag{}).
		Select("tags.*, COUNT(task_tags.task_id) AS task_count").
		Joins("LEFT JOIN task_tags ON task_tags.tag_id = tags.id").
		Where("tags.id = ?", c.Param("id")).
		Group("tags.id").
		First(&item).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(c, http.StatusNotFound, "TAG_NOT_FOUND", "标签不存在")
			return
		}
		respondDBError(c, http.StatusInternalServerError, "QUERY_TAG_FAILED", err)
		return
	}

	c.JSON(http.StatusOK, item)
}

func (h *Handler) ListTags(c *gin.Context) {
	page, pageSize := parsePage(c)
	var items []model.Tag
	query := h.DB.Model(&model.Tag{})
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ?", like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_TAGS_FAILED", err)
		return
	}

	if err := query.
		Select("tags.*, COUNT(task_tags.task_id) AS task_count").
		Joins("LEFT JOIN task_tags ON task_tags.tag_id = tags.id").
		Group("tags.id").
		Order("tags.name asc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_TAGS_FAILED", err)
		return
	}

	c.JSON(http.StatusOK, pageResult[model.Tag]{List: items, Total: total, Page: page, PageSize: pageSize})
}

func (h *Handler) CreateTag(c *gin.Context) {
	var req tagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}

	item := model.Tag{Name: strings.TrimSpace(req.Name)}
	if item.Name == "" {
		respondError(c, http.StatusBadRequest, "TAG_NAME_REQUIRED", "标签名称不能为空")
		return
	}

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "tags", "create", item.ID, true, auditDetailf("创建标签(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_TAG_FAILED", err)
		return
	}

	c.JSON(http.StatusCreated, item)
}

func (h *Handler) UpdateTag(c *gin.Context) {
	var req tagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}

	var item model.Tag
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "TAG_NOT_FOUND", "标签不存在")
		return
	}
	item.Name = strings.TrimSpace(req.Name)
	if item.Name == "" {
		respondError(c, http.StatusBadRequest, "TAG_NAME_REQUIRED", "标签名称不能为空")
		return
	}

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "tags", "update", item.ID, true, auditDetailf("更新标签(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "UPDATE_TAG_FAILED", err)
		return
	}

	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteTag(c *gin.Context) {
	var item model.Tag
	if err := h.DB.First(&item, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "TAG_NOT_FOUND", "标签不存在")
		return
	}

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := clearAssociation(tx, &item, "Tasks"); err != nil {
			return err
		}
		if err := tx.Delete(&item).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "tags", "delete", item.ID, true, auditDetailf("删除标签(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusInternalServerError, "DELETE_TAG_FAILED", err)
		return
	}

	respondMessage(c, http.StatusOK, "TAG_DELETED", "删除成功")
}
