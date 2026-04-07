package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"project-manager/backend/internal/auth"
	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

const (
	notificationSocketWriteWait      = 10 * time.Second
	notificationSocketPongWait       = 60 * time.Second
	notificationSocketPingPeriod     = (notificationSocketPongWait * 9) / 10
	notificationSocketMaxMessageSize = 512
)

var notificationSocketUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(_ *http.Request) bool {
		return true
	},
}

type notificationSocketEvent struct {
	Type string `json:"type"`
	At   string `json:"at"`
}

type notificationSocketClient struct {
	hub    *notificationSocketHub
	conn   *websocket.Conn
	userID uint
	send   chan []byte
}

type notificationSocketHub struct {
	mu      sync.RWMutex
	clients map[uint]map[*notificationSocketClient]struct{}
}

func newNotificationSocketHub() *notificationSocketHub {
	return &notificationSocketHub{
		clients: map[uint]map[*notificationSocketClient]struct{}{},
	}
}

func (h *notificationSocketHub) register(client *notificationSocketClient) {
	if h == nil || client == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	group := h.clients[client.userID]
	if group == nil {
		group = map[*notificationSocketClient]struct{}{}
		h.clients[client.userID] = group
	}
	group[client] = struct{}{}
}

func (h *notificationSocketHub) unregister(client *notificationSocketClient) {
	if h == nil || client == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.unregisterLocked(client, true)
}

func (h *notificationSocketHub) unregisterLocked(client *notificationSocketClient, closeSend bool) {
	group := h.clients[client.userID]
	if group == nil {
		return
	}
	if _, exists := group[client]; !exists {
		return
	}
	delete(group, client)
	if closeSend {
		close(client.send)
	}
	if len(group) == 0 {
		delete(h.clients, client.userID)
	}
}

func (h *notificationSocketHub) notifyUsers(userIDs []uint) {
	if h == nil {
		return
	}
	ids := uniqueUint(userIDs)
	if len(ids) == 0 {
		return
	}

	payload, err := json.Marshal(notificationSocketEvent{
		Type: "notifications.updated",
		At:   time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	for _, userID := range ids {
		group := h.clients[userID]
		if len(group) == 0 {
			continue
		}
		for client := range group {
			select {
			case client.send <- payload:
			default:
				h.unregisterLocked(client, true)
			}
		}
	}
}

func (client *notificationSocketClient) readPump() {
	defer func() {
		client.hub.unregister(client)
		_ = client.conn.Close()
	}()

	client.conn.SetReadLimit(notificationSocketMaxMessageSize)
	_ = client.conn.SetReadDeadline(time.Now().Add(notificationSocketPongWait))
	client.conn.SetPongHandler(func(string) error {
		return client.conn.SetReadDeadline(time.Now().Add(notificationSocketPongWait))
	})

	for {
		if _, _, err := client.conn.ReadMessage(); err != nil {
			break
		}
	}
}

func (client *notificationSocketClient) writePump() {
	ticker := time.NewTicker(notificationSocketPingPeriod)
	defer func() {
		ticker.Stop()
		_ = client.conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			_ = client.conn.SetWriteDeadline(time.Now().Add(notificationSocketWriteWait))
			if !ok {
				_ = client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = client.conn.SetWriteDeadline(time.Now().Add(notificationSocketWriteWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func socketTokenFromRequest(c *gin.Context) string {
	queryToken := strings.TrimSpace(c.Query("token"))
	if queryToken != "" {
		return queryToken
	}

	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if authHeader == "" {
		return ""
	}
	if len(authHeader) < len("Bearer ")+1 {
		return ""
	}
	if !strings.EqualFold(authHeader[:7], "Bearer ") {
		return ""
	}
	return strings.TrimSpace(authHeader[7:])
}

func hasNotificationReadPermission(perms []string) bool {
	for _, perm := range perms {
		if perm == "notifications.read" || perm == "notifications.write" {
			return true
		}
	}
	return false
}

func (h *Handler) resolveNotificationSocketPermissions(claims *auth.Claims) ([]string, error) {
	if h == nil {
		return nil, errors.New("handler is nil")
	}
	if claims == nil {
		return nil, errors.New("claims is nil")
	}
	if h.DB == nil {
		return claims.Permissions, nil
	}

	var user model.User
	if err := h.DB.Preload("Roles.Permissions").Where("id = ?", claims.UserID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, err
	}

	permissionSet := map[string]struct{}{}
	for _, role := range user.Roles {
		for _, permission := range role.Permissions {
			permissionSet[permission.Code] = struct{}{}
		}
	}
	if len(permissionSet) == 0 {
		for _, code := range claims.Permissions {
			permissionSet[code] = struct{}{}
		}
	}

	perms := make([]string, 0, len(permissionSet))
	for code := range permissionSet {
		perms = append(perms, code)
	}
	return perms, nil
}

func (h *Handler) pushNotificationUpdates(userIDs []uint) {
	if h == nil || h.NotificationHub == nil {
		return
	}
	h.NotificationHub.notifyUsers(userIDs)
}

func (h *Handler) NotificationSocket(c *gin.Context) {
	token := socketTokenFromRequest(c)
	if token == "" {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing token")
		return
	}

	claims, err := auth.ParseToken(h.Cfg.JWTSecret, token)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid token")
		return
	}

	perms, err := h.resolveNotificationSocketPermissions(claims)
	if err != nil {
		respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "user not found")
		return
	}
	if !hasNotificationReadPermission(perms) {
		respondError(c, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return
	}

	conn, err := notificationSocketUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	if h.NotificationHub == nil {
		_ = conn.Close()
		return
	}

	client := &notificationSocketClient{
		hub:    h.NotificationHub,
		conn:   conn,
		userID: claims.UserID,
		send:   make(chan []byte, 16),
	}
	h.NotificationHub.register(client)
	go client.writePump()
	client.readPump()
}
