package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"

	"project-manager/backend/internal/model"
)

var taskNotifyHTTPClient = &http.Client{Timeout: 8 * time.Second}

type taskNotifyRecipient struct {
	ID       uint
	Name     string
	Username string
	Email    string
}

func (h *Handler) queueTaskChannelNotifications(userIDs []uint, title, content string, task model.Task) {
	if len(userIDs) == 0 || !h.hasAnyTaskNotifyChannelConfigured() {
		return
	}
	ids := append([]uint(nil), userIDs...)
	taskCopy := task
	go h.sendTaskChannelNotifications(ids, title, content, taskCopy)
}

func (h *Handler) hasAnyTaskNotifyChannelConfigured() bool {
	provider, _ := h.resolveNonEmailTaskProvider()
	return h.hasEmailTaskNotifyConfigured() || provider != ""
}

func (h *Handler) hasEmailTaskNotifyConfigured() bool {
	return strings.TrimSpace(h.Cfg.SMTPHost) != "" &&
		strings.TrimSpace(h.Cfg.SMTPPort) != "" &&
		strings.TrimSpace(h.Cfg.SMTPFrom) != ""
}

func (h *Handler) enabledNonEmailTaskProviders() []string {
	providers := make([]string, 0, 3)
	if strings.TrimSpace(h.Cfg.WeComCorpID) != "" &&
		strings.TrimSpace(h.Cfg.WeComCorpSecret) != "" &&
		strings.TrimSpace(h.Cfg.WeComAgentID) != "" {
		providers = append(providers, "wecom")
	}
	if strings.TrimSpace(h.Cfg.DingTalkWebhook) != "" {
		providers = append(providers, "dingtalk")
	}
	if strings.TrimSpace(h.Cfg.FeishuAppID) != "" &&
		strings.TrimSpace(h.Cfg.FeishuAppSecret) != "" {
		providers = append(providers, "feishu")
	}
	return providers
}

func (h *Handler) resolveNonEmailTaskProvider() (string, error) {
	selected := strings.ToLower(strings.TrimSpace(h.Cfg.TaskNotifyProvider))
	configured := h.enabledNonEmailTaskProviders()

	if selected == "" || selected == "auto" {
		if len(configured) == 0 {
			return "", nil
		}
		if len(configured) == 1 {
			return configured[0], nil
		}
		return "", fmt.Errorf("non-email task providers configured more than one: %s", strings.Join(configured, ","))
	}
	if selected == "none" {
		return "", nil
	}

	switch selected {
	case "wecom", "dingtalk", "feishu":
		for _, provider := range configured {
			if provider == selected {
				return selected, nil
			}
		}
		return "", nil
	default:
		return "", fmt.Errorf("invalid TASK_NOTIFY_PROVIDER: %s", selected)
	}
}

func (h *Handler) sendTaskChannelNotifications(userIDs []uint, title, content string, task model.Task) {
	recipients, err := h.loadTaskNotifyRecipients(userIDs)
	if err != nil {
		log.Printf("task notification recipients query failed: %v", err)
		return
	}
	message := buildTaskChannelMessage(title, content, task, recipients)
	if message == "" {
		return
	}

	if err := h.sendTaskEmailNotifications(recipients, title, message); err != nil {
		log.Printf("task email notification failed: %v", err)
	}
	if err := h.sendTaskNonEmailNotifications(title, message, recipients); err != nil {
		log.Printf("task external notification failed: %v", err)
	}
}

func (h *Handler) loadTaskNotifyRecipients(userIDs []uint) ([]taskNotifyRecipient, error) {
	ids := uniqueUint(userIDs)
	if len(ids) == 0 {
		return nil, nil
	}

	var users []model.User
	if err := h.DB.Select("id, name, username, email").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, nil
	}

	indexMap := make(map[uint]int, len(ids))
	for index, id := range ids {
		indexMap[id] = index
	}
	result := make([]taskNotifyRecipient, len(ids))
	filled := make([]bool, len(ids))
	for _, user := range users {
		position, ok := indexMap[user.ID]
		if !ok {
			continue
		}
		result[position] = taskNotifyRecipient{
			ID:       user.ID,
			Name:     strings.TrimSpace(user.Name),
			Username: strings.TrimSpace(user.Username),
			Email:    strings.TrimSpace(user.Email),
		}
		filled[position] = true
	}

	ordered := make([]taskNotifyRecipient, 0, len(users))
	for index := range result {
		if filled[index] {
			ordered = append(ordered, result[index])
		}
	}
	return ordered, nil
}

func buildTaskChannelMessage(title, content string, task model.Task, recipients []taskNotifyRecipient) string {
	taskCode := strings.TrimSpace(task.TaskNo)
	if taskCode == "" {
		taskCode = fmt.Sprintf("#%d", task.ID)
	}
	taskTitle := strings.TrimSpace(task.Title)
	if taskTitle == "" {
		taskTitle = "未命名任务"
	}

	lines := []string{
		fmt.Sprintf("通知：%s", strings.TrimSpace(title)),
		fmt.Sprintf("内容：%s", strings.TrimSpace(content)),
		fmt.Sprintf("任务：%s - %s", taskCode, taskTitle),
		fmt.Sprintf("状态：%s", string(task.Status)),
		fmt.Sprintf("优先级：%s", string(task.Priority)),
		fmt.Sprintf("进度：%d%%", task.Progress),
	}
	if task.StartAt != nil {
		lines = append(lines, fmt.Sprintf("开始时间：%s", task.StartAt.Format("2006-01-02 15:04:05")))
	}
	if task.EndAt != nil {
		lines = append(lines, fmt.Sprintf("结束时间：%s", task.EndAt.Format("2006-01-02 15:04:05")))
	}

	names := make([]string, 0, len(recipients))
	for _, recipient := range recipients {
		display := recipient.Name
		if display == "" {
			display = recipient.Username
		}
		if display == "" {
			display = strconv.FormatUint(uint64(recipient.ID), 10)
		}
		names = append(names, display)
	}
	if len(names) > 0 {
		lines = append(lines, fmt.Sprintf("通知对象：%s", strings.Join(names, "、")))
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (h *Handler) sendTaskEmailNotifications(recipients []taskNotifyRecipient, title, message string) error {
	if !h.hasEmailTaskNotifyConfigured() {
		return nil
	}

	host := strings.TrimSpace(h.Cfg.SMTPHost)
	port := strings.TrimSpace(h.Cfg.SMTPPort)
	from := strings.TrimSpace(h.Cfg.SMTPFrom)
	username := strings.TrimSpace(h.Cfg.SMTPUsername)
	password := h.Cfg.SMTPPassword
	addr := netJoinHostPort(host, port)

	emails := make([]string, 0, len(recipients))
	seen := make(map[string]struct{}, len(recipients))
	for _, recipient := range recipients {
		email := strings.TrimSpace(recipient.Email)
		if email == "" {
			continue
		}
		if _, exists := seen[email]; exists {
			continue
		}
		seen[email] = struct{}{}
		emails = append(emails, email)
	}
	if len(emails) == 0 {
		return nil
	}

	var auth smtp.Auth
	if username != "" && password != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}

	subject := mime.BEncoding.Encode("UTF-8", "[任务通知] "+title)
	failures := make([]error, 0)
	for _, email := range emails {
		content := strings.Join([]string{
			"From: " + from,
			"To: " + email,
			"Subject: " + subject,
			"MIME-Version: 1.0",
			"Content-Type: text/plain; charset=UTF-8",
			"",
			message,
		}, "\r\n")
		if err := smtp.SendMail(addr, auth, from, []string{email}, []byte(content)); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", email, err))
		}
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	return nil
}

func (h *Handler) sendTaskNonEmailNotifications(title, message string, recipients []taskNotifyRecipient) error {
	provider, err := h.resolveNonEmailTaskProvider()
	if err != nil {
		return err
	}
	if provider == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	switch provider {
	case "wecom":
		return h.sendWeComTaskNotification(ctx, message)
	case "dingtalk":
		return h.sendDingTalkTaskNotification(ctx, title, message)
	case "feishu":
		return h.sendFeishuTaskNotification(ctx, message, recipients)
	default:
		return nil
	}
}

func (h *Handler) sendWeComTaskNotification(ctx context.Context, message string) error {
	tokenURL := "https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=" +
		url.QueryEscape(strings.TrimSpace(h.Cfg.WeComCorpID)) +
		"&corpsecret=" + url.QueryEscape(strings.TrimSpace(h.Cfg.WeComCorpSecret))

	var tokenResp struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
	}
	if err := doTaskNotifyJSONRequest(ctx, http.MethodGet, tokenURL, nil, nil, &tokenResp); err != nil {
		return err
	}
	if tokenResp.ErrCode != 0 || strings.TrimSpace(tokenResp.AccessToken) == "" {
		return fmt.Errorf("wecom get token failed: errcode=%d errmsg=%s", tokenResp.ErrCode, tokenResp.ErrMsg)
	}

	agentID, err := strconv.ParseInt(strings.TrimSpace(h.Cfg.WeComAgentID), 10, 64)
	if err != nil || agentID <= 0 {
		return fmt.Errorf("invalid WECOM_AGENT_ID")
	}

	toUser := strings.TrimSpace(h.Cfg.WeComToUser)
	if toUser == "" {
		toUser = "@all"
	}

	sendURL := "https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=" + url.QueryEscape(tokenResp.AccessToken)
	reqBody := map[string]any{
		"touser":  toUser,
		"msgtype": "text",
		"agentid": agentID,
		"text": map[string]string{
			"content": message,
		},
		"safe": 0,
	}
	var sendResp struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := doTaskNotifyJSONRequest(ctx, http.MethodPost, sendURL, reqBody, nil, &sendResp); err != nil {
		return err
	}
	if sendResp.ErrCode != 0 {
		return fmt.Errorf("wecom send failed: errcode=%d errmsg=%s", sendResp.ErrCode, sendResp.ErrMsg)
	}
	return nil
}

func (h *Handler) sendDingTalkTaskNotification(ctx context.Context, title, message string) error {
	requestURL := strings.TrimSpace(h.Cfg.DingTalkWebhook)
	if requestURL == "" {
		return nil
	}

	secret := strings.TrimSpace(h.Cfg.DingTalkSecret)
	if secret != "" {
		timestamp := time.Now().UnixMilli()
		stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(stringToSign))
		sign := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
		separator := "?"
		if strings.Contains(requestURL, "?") {
			separator = "&"
		}
		requestURL = requestURL + separator + "timestamp=" + strconv.FormatInt(timestamp, 10) + "&sign=" + sign
	}

	markdown := "### 任务通知\n\n" + strings.ReplaceAll(strings.TrimSpace(message), "\n", "\n\n")
	reqBody := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  markdown,
		},
	}
	var resp struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := doTaskNotifyJSONRequest(ctx, http.MethodPost, requestURL, reqBody, nil, &resp); err != nil {
		return err
	}
	if resp.ErrCode != 0 {
		return fmt.Errorf("dingtalk send failed: errcode=%d errmsg=%s", resp.ErrCode, resp.ErrMsg)
	}
	return nil
}

func (h *Handler) sendFeishuTaskNotification(ctx context.Context, message string, recipients []taskNotifyRecipient) error {
	tokenURL := "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"
	reqTokenBody := map[string]string{
		"app_id":     strings.TrimSpace(h.Cfg.FeishuAppID),
		"app_secret": strings.TrimSpace(h.Cfg.FeishuAppSecret),
	}
	var tokenResp struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	if err := doTaskNotifyJSONRequest(ctx, http.MethodPost, tokenURL, reqTokenBody, nil, &tokenResp); err != nil {
		return err
	}
	if tokenResp.Code != 0 || strings.TrimSpace(tokenResp.TenantAccessToken) == "" {
		return fmt.Errorf("feishu get token failed: code=%d msg=%s", tokenResp.Code, tokenResp.Msg)
	}

	receiveType := strings.TrimSpace(h.Cfg.FeishuReceiveType)
	if receiveType == "" {
		receiveType = "email"
	}
	targets := make([]string, 0)
	configuredReceiveID := strings.TrimSpace(h.Cfg.FeishuReceiveID)
	if configuredReceiveID != "" {
		targets = append(targets, configuredReceiveID)
	} else if receiveType == "email" {
		seen := make(map[string]struct{}, len(recipients))
		for _, recipient := range recipients {
			email := strings.TrimSpace(recipient.Email)
			if email == "" {
				continue
			}
			if _, exists := seen[email]; exists {
				continue
			}
			seen[email] = struct{}{}
			targets = append(targets, email)
		}
	}
	if len(targets) == 0 {
		return nil
	}

	contentRaw, err := json.Marshal(map[string]string{"text": message})
	if err != nil {
		return err
	}
	headers := map[string]string{
		"Authorization": "Bearer " + strings.TrimSpace(tokenResp.TenantAccessToken),
	}

	failures := make([]error, 0)
	for _, receiveID := range targets {
		sendURL := "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=" + url.QueryEscape(receiveType)
		reqBody := map[string]string{
			"receive_id": receiveID,
			"msg_type":   "text",
			"content":    string(contentRaw),
		}
		var sendResp struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		if requestErr := doTaskNotifyJSONRequest(ctx, http.MethodPost, sendURL, reqBody, headers, &sendResp); requestErr != nil {
			failures = append(failures, fmt.Errorf("%s: %w", receiveID, requestErr))
			continue
		}
		if sendResp.Code != 0 {
			failures = append(failures, fmt.Errorf("%s: feishu code=%d msg=%s", receiveID, sendResp.Code, sendResp.Msg))
		}
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	return nil
}

func doTaskNotifyJSONRequest(ctx context.Context, method, requestURL string, payload interface{}, headers map[string]string, out interface{}) error {
	var bodyReader io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, bodyReader)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := taskNotifyHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(rawBody)))
	}
	if out == nil || len(rawBody) == 0 {
		return nil
	}
	return json.Unmarshal(rawBody, out)
}

func netJoinHostPort(host, port string) string {
	if strings.Contains(host, ":") {
		return host
	}
	return host + ":" + port
}
