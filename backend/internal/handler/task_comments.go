package handler

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var mentionPattern = regexp.MustCompile(`@([A-Za-z0-9_.-]+)`)

type taskCommentRequest struct {
	Content     string               `json:"content" binding:"required"`
	Attachment  *attachmentRequest   `json:"attachment"`
	Attachments *[]attachmentRequest `json:"attachments"`
	MentionIDs  []uint               `json:"mentionIds"`
}

func (h *Handler) ensureTaskVisible(c *gin.Context, taskID string) (*model.Task, bool) {
	var item model.Task
	query := h.scopeTasksQuery(c, h.DB.Model(&model.Task{})).Where("tasks.id = ?", taskID)
	if err := query.First(&item).Error; err != nil {
		respondError(c, http.StatusNotFound, "TASK_NOT_FOUND", "任务不存在")
		return nil, false
	}
	return &item, true
}

func extractMentionUsernames(content string) []string {
	matches := mentionPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		username := strings.TrimSpace(match[1])
		if username == "" {
			continue
		}
		key := strings.ToLower(username)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, username)
	}
	return out
}

func mergeMentionUsers(fromIDs []model.User, fromUsernames []model.User) []model.User {
	seen := map[uint]struct{}{}
	out := make([]model.User, 0, len(fromIDs)+len(fromUsernames))
	for _, user := range append(fromIDs, fromUsernames...) {
		if user.ID == 0 {
			continue
		}
		if _, ok := seen[user.ID]; ok {
			continue
		}
		seen[user.ID] = struct{}{}
		out = append(out, user)
	}
	return out
}

func (h *Handler) resolveMentionUsers(tx *gorm.DB, content string, mentionIDs []uint) ([]model.User, error) {
	usersByID, err := findUsersByIDs(tx, mentionIDs)
	if err != nil {
		return nil, err
	}

	usernames := extractMentionUsernames(content)
	usersByUsername := []model.User{}
	if len(usernames) > 0 {
		if err := tx.Where("username IN ?", usernames).Find(&usersByUsername).Error; err != nil {
			return nil, err
		}
	}
	return mergeMentionUsers(usersByID, usersByUsername), nil
}

func mentionUserIDs(users []model.User, currentUserID uint) []uint {
	out := make([]uint, 0, len(users))
	for _, user := range users {
		if user.ID == 0 || user.ID == currentUserID {
			continue
		}
		out = append(out, user.ID)
	}
	return uniqueUint(out)
}

func (h *Handler) writeTaskActivityWithDB(tx *gorm.DB, taskID, actorID uint, activityType, summary, detail string, commentID *uint) error {
	activity := model.TaskActivity{
		TaskID:    taskID,
		ActorID:   actorID,
		Type:      activityType,
		Summary:   summary,
		Detail:    detail,
		CommentID: commentID,
	}
	return tx.Create(&activity).Error
}

func taskActivitySummary(action string, task model.Task) string {
	taskNo := strings.TrimSpace(task.TaskNo)
	if taskNo == "" {
		return action
	}
	return fmt.Sprintf("%s：%s - %s", action, taskNo, task.Title)
}

func (h *Handler) ListTaskComments(c *gin.Context) {
	task, ok := h.ensureTaskVisible(c, c.Param("id"))
	if !ok {
		return
	}

	page, pageSize := parsePage(c)
	var items []model.TaskComment
	query := h.DB.Model(&model.TaskComment{}).
		Where("task_id = ? AND is_deleted = ?", task.ID, false).
		Preload("Author").
		Preload("Mentions")

	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_TASK_COMMENTS_FAILED", err)
		return
	}
	if err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_TASK_COMMENTS_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, pageResult[model.TaskComment]{List: items, Total: total, Page: page, PageSize: pageSize})
}

func (h *Handler) CreateTaskComment(c *gin.Context) {
	task, ok := h.ensureTaskVisible(c, c.Param("id"))
	if !ok {
		return
	}

	var req taskCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		respondError(c, http.StatusBadRequest, "COMMENT_CONTENT_REQUIRED", "评论内容不能为空")
		return
	}

	attachments, _ := requestAttachments(req.Attachment, req.Attachments)
	if err := validateAttachments(attachments, h.Cfg.UploadPublicBase); err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_ATTACHMENT", err.Error())
		return
	}

	currentUserID := c.GetUint("userId")
	item := model.TaskComment{
		TaskID:      task.ID,
		AuthorID:    currentUserID,
		Content:     content,
		Attachments: toModelAttachments(attachments),
		IsDeleted:   false,
	}
	mentionIDs := []uint{}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		mentions, err := h.resolveMentionUsers(tx, content, req.MentionIDs)
		if err != nil {
			return err
		}
		if len(mentions) > 0 {
			if err := replaceAssociation(tx, &item, "Mentions", &mentions); err != nil {
				return err
			}
		}
		mentionIDs = mentionUserIDs(mentions, currentUserID)
		if err := h.createNotificationsWithDB(
			tx,
			mentionIDs,
			"你在任务评论中被提及",
			"任务 "+task.TaskNo+" - "+task.Title+" 的评论提到了你",
			"tasks",
			task.ID,
		); err != nil {
			return err
		}
		commentID := item.ID
		if err := h.writeTaskActivityWithDB(tx, task.ID, currentUserID, "comment.created", "新增评论", content, &commentID); err != nil {
			return err
		}
		if err := tx.Preload("Author").Preload("Mentions").First(&item, item.ID).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "comments", "create", item.ID, true, auditDetailf("创建任务评论(id=%d, task_id=%d)", item.ID, task.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_TASK_COMMENT_FAILED", err)
		return
	}
	h.pushNotificationUpdates(mentionIDs)
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) DeleteTaskComment(c *gin.Context) {
	task, ok := h.ensureTaskVisible(c, c.Param("id"))
	if !ok {
		return
	}
	commentID, err := strconv.ParseUint(c.Param("commentId"), 10, 64)
	if err != nil || commentID == 0 {
		respondError(c, http.StatusBadRequest, "INVALID_COMMENT_ID", "非法评论ID")
		return
	}

	var item model.TaskComment
	if err := h.DB.Where("id = ? AND task_id = ? AND is_deleted = ?", commentID, task.ID, false).First(&item).Error; err != nil {
		respondError(c, http.StatusNotFound, "COMMENT_NOT_FOUND", "评论不存在")
		return
	}
	currentUserID := c.GetUint("userId")
	if item.AuthorID != currentUserID && !h.currentUserIsAdmin(c) {
		respondError(c, http.StatusForbidden, "COMMENT_OWNER_REQUIRED", "只能删除自己的评论")
		return
	}

	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&item).Update("is_deleted", true).Error; err != nil {
			return err
		}
		if err := h.writeTaskActivityWithDB(tx, task.ID, currentUserID, "comment.deleted", "删除评论", "", nil); err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "comments", "delete", item.ID, true, auditDetailf("删除任务评论(id=%d, task_id=%d)", item.ID, task.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "DELETE_TASK_COMMENT_FAILED", err)
		return
	}

	respondMessage(c, http.StatusOK, "COMMENT_DELETED", "删除成功")
}

func (h *Handler) ListTaskActivities(c *gin.Context) {
	task, ok := h.ensureTaskVisible(c, c.Param("id"))
	if !ok {
		return
	}

	page, pageSize := parsePage(c)
	var items []model.TaskActivity
	query := h.DB.Model(&model.TaskActivity{}).
		Where("task_id = ?", task.ID).
		Preload("Actor").
		Preload("Comment", "is_deleted = ?", false).
		Preload("Comment.Author").
		Preload("Comment.Mentions")

	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_TASK_ACTIVITIES_FAILED", err)
		return
	}
	if err := query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_TASK_ACTIVITIES_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, pageResult[model.TaskActivity]{List: items, Total: total, Page: page, PageSize: pageSize})
}
