package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type webhookSubscriptionRequest struct {
	Name      string `json:"name" binding:"required"`
	Event     string `json:"event" binding:"required"`
	URL       string `json:"url" binding:"required"`
	IsEnabled *bool  `json:"isEnabled"`
}

type taskStatusWebhookPayload struct {
	Event       model.WebhookEvent      `json:"event"`
	TriggeredAt time.Time               `json:"triggeredAt"`
	ActorID     uint                    `json:"actorId"`
	Task        taskStatusWebhookTask   `json:"task"`
	Change      taskStatusWebhookChange `json:"change"`
}

type taskStatusWebhookTask struct {
	ID        uint             `json:"id"`
	TaskNo    string           `json:"taskNo"`
	Title     string           `json:"title"`
	Status    model.TaskStatus `json:"status"`
	Progress  int              `json:"progress"`
	ProjectID uint             `json:"projectId"`
	EndAt     *time.Time       `json:"endAt"`
}

type taskStatusWebhookChange struct {
	FromStatus model.TaskStatus `json:"fromStatus"`
	ToStatus   model.TaskStatus `json:"toStatus"`
}

func normalizeWebhookEvent(value string) (model.WebhookEvent, bool) {
	switch model.WebhookEvent(strings.TrimSpace(value)) {
	case model.WebhookEventTaskStatusChanged:
		return model.WebhookEventTaskStatusChanged, true
	default:
		return model.WebhookEventTaskStatusChanged, false
	}
}

func normalizeWebhookDeliveryStatus(value string) (model.WebhookDeliveryStatus, bool) {
	switch model.WebhookDeliveryStatus(strings.TrimSpace(value)) {
	case model.WebhookDeliveryPending, model.WebhookDeliverySuccess, model.WebhookDeliveryFailed:
		return model.WebhookDeliveryStatus(strings.TrimSpace(value)), true
	default:
		return model.WebhookDeliveryPending, false
	}
}

func boolValueDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func (h *Handler) scopeWebhookSubscriptionsQuery(c *gin.Context, query *gorm.DB) *gorm.DB {
	if h.currentUserIsAdmin(c) {
		return query
	}
	return query.Where("webhook_subscriptions.created_by_id = ?", c.GetUint("userId"))
}

func (h *Handler) ensureWebhookSubscriptionReadable(c *gin.Context, id string) (*model.WebhookSubscription, bool) {
	var item model.WebhookSubscription
	query := h.scopeWebhookSubscriptionsQuery(c, h.DB.Model(&model.WebhookSubscription{})).Preload("CreatedBy")
	if err := query.Where("webhook_subscriptions.id = ?", id).First(&item).Error; err != nil {
		respondError(c, http.StatusNotFound, "WEBHOOK_NOT_FOUND", "Webhook 订阅不存在")
		return nil, false
	}
	return &item, true
}

func (h *Handler) ensureWebhookSubscriptionWritable(c *gin.Context, id string) (*model.WebhookSubscription, bool) {
	item, ok := h.ensureWebhookSubscriptionReadable(c, id)
	if !ok {
		return nil, false
	}
	if h.currentUserIsAdmin(c) || item.CreatedByID == c.GetUint("userId") {
		return item, true
	}
	respondError(c, http.StatusForbidden, "WEBHOOK_OWNER_REQUIRED", "只有订阅创建人或管理员可以更新 Webhook 订阅")
	return nil, false
}

func (h *Handler) validateWebhookSubscriptionRequest(req webhookSubscriptionRequest) (model.WebhookSubscription, bool, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return model.WebhookSubscription{}, false, fmt.Errorf("Webhook 名称不能为空")
	}
	event, eventOK := normalizeWebhookEvent(req.Event)
	normalizedURL, err := normalizeAutomationWebhookURL(req.URL)
	if err != nil {
		return model.WebhookSubscription{}, eventOK, err
	}
	if err := validateAutomationWebhookEndpoint(normalizedURL, h.Cfg.WebhookPrivateOK); err != nil {
		return model.WebhookSubscription{}, eventOK, err
	}
	return model.WebhookSubscription{
		Name:      name,
		Event:     event,
		URL:       normalizedURL,
		IsEnabled: boolValueDefault(req.IsEnabled, true),
	}, eventOK, nil
}

func (h *Handler) ListWebhookSubscriptions(c *gin.Context) {
	page, pageSize := parsePage(c)
	query := h.scopeWebhookSubscriptionsQuery(c, h.DB.Model(&model.WebhookSubscription{}))
	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("webhook_subscriptions.name LIKE ? OR webhook_subscriptions.url LIKE ?", like, like)
	}
	if event, ok := normalizeWebhookEvent(c.Query("event")); ok && strings.TrimSpace(c.Query("event")) != "" {
		query = query.Where("webhook_subscriptions.event = ?", event)
	}
	if value := strings.TrimSpace(c.Query("isEnabled")); value != "" {
		query = query.Where("webhook_subscriptions.is_enabled = ?", value == "true" || value == "1")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_WEBHOOKS_FAILED", err)
		return
	}
	var items []model.WebhookSubscription
	if err := query.Preload("CreatedBy").
		Order("webhook_subscriptions.updated_at desc, webhook_subscriptions.id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_WEBHOOKS_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, pageResult[model.WebhookSubscription]{List: items, Total: total, Page: page, PageSize: pageSize})
}

func (h *Handler) WebhookSubscriptionDetail(c *gin.Context) {
	item, ok := h.ensureWebhookSubscriptionReadable(c, c.Param("id"))
	if !ok {
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) CreateWebhookSubscription(c *gin.Context) {
	var req webhookSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	item, eventOK, err := h.validateWebhookSubscriptionRequest(req)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_WEBHOOK", err.Error())
		return
	}
	if !eventOK {
		respondError(c, http.StatusBadRequest, "INVALID_WEBHOOK_EVENT", "Webhook 事件必须是 task_status_changed")
		return
	}
	item.CreatedByID = c.GetUint("userId")
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "webhooks", "create", item.ID, true, auditDetailf("创建 Webhook 订阅(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "CREATE_WEBHOOK_FAILED", err)
		return
	}
	if err := h.DB.Preload("CreatedBy").First(&item, item.ID).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_WEBHOOK_FAILED", err)
		return
	}
	c.JSON(http.StatusCreated, item)
}

func (h *Handler) UpdateWebhookSubscription(c *gin.Context) {
	var req webhookSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidationError(c, err)
		return
	}
	item, ok := h.ensureWebhookSubscriptionWritable(c, c.Param("id"))
	if !ok {
		return
	}
	next, eventOK, err := h.validateWebhookSubscriptionRequest(req)
	if err != nil {
		respondError(c, http.StatusBadRequest, "INVALID_WEBHOOK", err.Error())
		return
	}
	if !eventOK {
		respondError(c, http.StatusBadRequest, "INVALID_WEBHOOK_EVENT", "Webhook 事件必须是 task_status_changed")
		return
	}
	item.Name = next.Name
	item.Event = next.Event
	item.URL = next.URL
	item.IsEnabled = next.IsEnabled
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(item).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "webhooks", "update", item.ID, true, auditDetailf("更新 Webhook 订阅(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusBadRequest, "UPDATE_WEBHOOK_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteWebhookSubscription(c *gin.Context) {
	item, ok := h.ensureWebhookSubscriptionWritable(c, c.Param("id"))
	if !ok {
		return
	}
	if err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(item).Error; err != nil {
			return err
		}
		return h.writeAuditWithDB(c, tx, "webhooks", "delete", item.ID, true, auditDetailf("删除 Webhook 订阅(id=%d)", item.ID))
	}); err != nil {
		respondDBError(c, http.StatusInternalServerError, "DELETE_WEBHOOK_FAILED", err)
		return
	}
	respondMessage(c, http.StatusOK, "WEBHOOK_DELETED", "删除成功")
}

func (h *Handler) scopeWebhookDeliveriesQuery(c *gin.Context, query *gorm.DB) *gorm.DB {
	query = query.Joins("JOIN webhook_subscriptions ON webhook_subscriptions.id = webhook_deliveries.subscription_id")
	if h.currentUserIsAdmin(c) {
		return query
	}
	return query.Where("webhook_subscriptions.created_by_id = ?", c.GetUint("userId"))
}

func (h *Handler) ListWebhookDeliveries(c *gin.Context) {
	page, pageSize := parsePage(c)
	query := h.scopeWebhookDeliveriesQuery(c, h.DB.Model(&model.WebhookDelivery{}))
	if subscriptionID := strings.TrimSpace(c.Query("subscriptionId")); subscriptionID != "" {
		query = query.Where("webhook_deliveries.subscription_id = ?", subscriptionID)
	}
	if status, ok := normalizeWebhookDeliveryStatus(c.Query("status")); ok && strings.TrimSpace(c.Query("status")) != "" {
		query = query.Where("webhook_deliveries.status = ?", status)
	}
	if event, ok := normalizeWebhookEvent(c.Query("event")); ok && strings.TrimSpace(c.Query("event")) != "" {
		query = query.Where("webhook_deliveries.event = ?", event)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_WEBHOOK_DELIVERIES_FAILED", err)
		return
	}
	var items []model.WebhookDelivery
	if err := query.Preload("Subscription").
		Order("webhook_deliveries.created_at desc, webhook_deliveries.id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_WEBHOOK_DELIVERIES_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, pageResult[model.WebhookDelivery]{List: items, Total: total, Page: page, PageSize: pageSize})
}

func (h *Handler) postWebhookPayload(subscription model.WebhookSubscription, deliveryID uint, payload string) (int, error) {
	if err := validateAutomationWebhookEndpoint(subscription.URL, h.Cfg.WebhookPrivateOK); err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, subscription.URL, bytes.NewReader([]byte(payload)))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "project-manager-webhook/1.0")
	req.Header.Set("X-Project-Manager-Event", string(subscription.Event))
	req.Header.Set("X-Project-Manager-Webhook-ID", strconv.FormatUint(uint64(subscription.ID), 10))
	req.Header.Set("X-Project-Manager-Delivery-ID", strconv.FormatUint(uint64(deliveryID), 10))

	resp, err := newAutomationWebhookHTTPClient(h.Cfg.WebhookPrivateOK).Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	rawResp, err := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if err != nil {
		return resp.StatusCode, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		bodyText := strings.TrimSpace(string(rawResp))
		if bodyText == "" {
			return resp.StatusCode, fmt.Errorf("Webhook 返回状态 %d", resp.StatusCode)
		}
		return resp.StatusCode, fmt.Errorf("Webhook 返回状态 %d: %s", resp.StatusCode, bodyText)
	}
	return resp.StatusCode, nil
}

func nextWebhookRetryAt(attempts int) time.Time {
	delayMinutes := attempts
	if delayMinutes < 1 {
		delayMinutes = 1
	}
	if delayMinutes > 30 {
		delayMinutes = 30
	}
	return time.Now().Add(time.Duration(delayMinutes) * time.Minute)
}

func (h *Handler) attemptWebhookDelivery(delivery *model.WebhookDelivery, subscription model.WebhookSubscription) {
	statusCode, err := h.postWebhookPayload(subscription, delivery.ID, delivery.Payload)
	now := time.Now()
	attempts := delivery.Attempts + 1
	updates := map[string]any{
		"attempts":        attempts,
		"response_status": statusCode,
		"updated_at":      now,
	}
	subscriptionUpdates := map[string]any{
		"last_delivered_at": now,
	}
	if err != nil {
		message := automationWebhookErrorText(err)
		nextRetryAt := nextWebhookRetryAt(attempts)
		updates["status"] = model.WebhookDeliveryFailed
		updates["error_message"] = message
		updates["next_retry_at"] = &nextRetryAt
		subscriptionUpdates["last_delivery_status"] = model.WebhookDeliveryFailed
		subscriptionUpdates["last_error"] = message
	} else {
		updates["status"] = model.WebhookDeliverySuccess
		updates["error_message"] = ""
		updates["next_retry_at"] = nil
		updates["delivered_at"] = now
		subscriptionUpdates["last_delivery_status"] = model.WebhookDeliverySuccess
		subscriptionUpdates["last_error"] = ""
	}
	_ = h.DB.Model(&model.WebhookDelivery{}).Where("id = ?", delivery.ID).Updates(updates).Error
	_ = h.DB.Model(&model.WebhookSubscription{}).Where("id = ?", subscription.ID).Updates(subscriptionUpdates).Error
}

func (h *Handler) RetryWebhookDelivery(c *gin.Context) {
	var delivery model.WebhookDelivery
	query := h.scopeWebhookDeliveriesQuery(c, h.DB.Model(&model.WebhookDelivery{})).Preload("Subscription")
	if err := query.Where("webhook_deliveries.id = ?", c.Param("id")).First(&delivery).Error; err != nil {
		respondError(c, http.StatusNotFound, "WEBHOOK_DELIVERY_NOT_FOUND", "Webhook 投递记录不存在")
		return
	}
	if !h.currentUserIsAdmin(c) && delivery.Subscription.CreatedByID != c.GetUint("userId") {
		respondError(c, http.StatusForbidden, "WEBHOOK_OWNER_REQUIRED", "只有订阅创建人或管理员可以重试投递")
		return
	}
	if !delivery.Subscription.IsEnabled {
		respondError(c, http.StatusBadRequest, "WEBHOOK_DISABLED", "Webhook 订阅已停用")
		return
	}
	if delivery.Status == model.WebhookDeliverySuccess {
		respondError(c, http.StatusBadRequest, "WEBHOOK_DELIVERY_NOT_RETRYABLE", "成功投递记录无需重试")
		return
	}
	h.attemptWebhookDelivery(&delivery, delivery.Subscription)
	if err := h.DB.Preload("Subscription").First(&delivery, delivery.ID).Error; err != nil {
		respondDBError(c, http.StatusInternalServerError, "QUERY_WEBHOOK_DELIVERY_FAILED", err)
		return
	}
	c.JSON(http.StatusOK, delivery)
}

func buildTaskStatusWebhookPayload(task model.Task, oldStatus model.TaskStatus, newStatus model.TaskStatus, actorID uint) (string, error) {
	raw, err := json.Marshal(taskStatusWebhookPayload{
		Event:       model.WebhookEventTaskStatusChanged,
		TriggeredAt: time.Now(),
		ActorID:     actorID,
		Task: taskStatusWebhookTask{
			ID:        task.ID,
			TaskNo:    task.TaskNo,
			Title:     task.Title,
			Status:    task.Status,
			Progress:  task.Progress,
			ProjectID: task.ProjectID,
			EndAt:     task.EndAt,
		},
		Change: taskStatusWebhookChange{
			FromStatus: oldStatus,
			ToStatus:   newStatus,
		},
	})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (h *Handler) dispatchTaskStatusWebhooks(task model.Task, oldStatus model.TaskStatus, newStatus model.TaskStatus, actorID uint) {
	if oldStatus == newStatus {
		return
	}
	var subscriptions []model.WebhookSubscription
	if err := h.DB.Where(`
		is_enabled = ? AND event = ? AND (
			EXISTS (
				SELECT 1
				FROM user_roles webhook_ur
				JOIN roles webhook_roles ON webhook_roles.id = webhook_ur.role_id
				WHERE webhook_ur.user_id = webhook_subscriptions.created_by_id
				  AND webhook_roles.name = ?
			)
			OR webhook_subscriptions.created_by_id = ?
			OR EXISTS (
				SELECT 1
				FROM task_users webhook_tu
				WHERE webhook_tu.task_id = ?
				  AND webhook_tu.user_id = webhook_subscriptions.created_by_id
			)
			OR EXISTS (
				SELECT 1
				FROM task_reviewers webhook_tr
				WHERE webhook_tr.task_id = ?
				  AND webhook_tr.user_id = webhook_subscriptions.created_by_id
			)
		)`,
		true,
		model.WebhookEventTaskStatusChanged,
		"admin",
		task.CreatorID,
		task.ID,
		task.ID,
	).
		Order("id asc").
		Find(&subscriptions).Error; err != nil {
		return
	}
	if len(subscriptions) == 0 {
		return
	}
	payload, err := buildTaskStatusWebhookPayload(task, oldStatus, newStatus, actorID)
	if err != nil {
		return
	}
	for _, subscription := range subscriptions {
		delivery := model.WebhookDelivery{
			SubscriptionID: subscription.ID,
			Event:          subscription.Event,
			Status:         model.WebhookDeliveryPending,
			Payload:        payload,
		}
		if err := h.DB.Create(&delivery).Error; err != nil {
			continue
		}
		h.attemptWebhookDelivery(&delivery, subscription)
	}
}
