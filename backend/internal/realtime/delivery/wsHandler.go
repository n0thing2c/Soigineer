package delivery

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/n0thing2c/Soigineer/internal/realtime/service"
)

type WebSocketHandler struct {
	upgrader websocket.Upgrader
	hub      *service.Hub
	loader   PrincipalLoader
}

type PrincipalLoader interface {
	LoadRealtimePrincipal(ctx context.Context, identity string) (service.Principal, error)
}

func NewWebSocketHandler(hub *service.Hub) *WebSocketHandler {
	return NewWebSocketHandlerWithPrincipalLoader(hub, nil)
}

func NewWebSocketHandlerWithPrincipalLoader(
	hub *service.Hub,
	loader PrincipalLoader,
) *WebSocketHandler {
	return &WebSocketHandler{
		hub:    hub,
		loader: loader,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (h *WebSocketHandler) HandleLogs(ctx *gin.Context) {
	h.serveWebSocket(ctx, service.StreamLogs)
}

func (h *WebSocketHandler) HandleAlerts(ctx *gin.Context) {
	h.serveWebSocket(ctx, service.StreamAlerts)
}

func (h *WebSocketHandler) Health(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

func (h *WebSocketHandler) serveWebSocket(ctx *gin.Context, stream service.StreamType) {
	principal, err := h.resolvePrincipal(ctx)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid realtime user",
		})
		return
	}

	conn, err := h.upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}

	client := service.NewClient(
		h.hub,
		conn,
		stream,
		principal,
		parseSubscription(ctx),
	)

	h.hub.Register(client)
	go client.WriteLoop()
	client.ReadLoop()
}

func (h *WebSocketHandler) resolvePrincipal(ctx *gin.Context) (service.Principal, error) {
	if h.loader == nil {
		return parsePrincipal(ctx), nil
	}

	identity := parseIdentity(ctx)
	if identity == "" {
		return service.Principal{}, errors.New("missing identity")
	}

	return h.loader.LoadRealtimePrincipal(ctx.Request.Context(), identity)
}

func parseSubscription(ctx *gin.Context) service.Subscription {
	return service.Subscription{
		Applications: parseCSVSet(ctx.Query("app")),
		Levels:       parseCSVSet(ctx.Query("level")),
	}
}

func parsePrincipal(ctx *gin.Context) service.Principal {
	userID := parseIdentity(ctx)
	if userID == "" {
		userID = "anonymous"
	}

	role := ctx.GetHeader("X-User-Role")
	if role == "" {
		role = "anonymous"
	}

	return service.Principal{
		UserID:   userID,
		Username: userID,
		Role:     role,
		Apps:     parseCSVSet(ctx.GetHeader("X-User-Apps")),
	}
}

func parseIdentity(ctx *gin.Context) string {
	token := strings.TrimSpace(ctx.Query("token"))
	if token != "" {
		return token
	}

	authHeader := strings.TrimSpace(ctx.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[len("Bearer "):])
	}

	userID := strings.TrimSpace(ctx.GetHeader("X-User-ID"))
	if userID != "" {
		return userID
	}
	return strings.TrimSpace(ctx.Query("userId"))
}

func parseCSVSet(value string) map[string]bool {
	result := make(map[string]bool)

	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		result[item] = true
	}

	return result
}
