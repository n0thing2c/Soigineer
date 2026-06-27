package delivery

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/n0thing2c/Soigineer/internal/realtime/service"
)

type WebSocketHandler struct {
	upgrader websocket.Upgrader
	hub      *service.Hub
}

func NewWebSocketHandler(hub *service.Hub) *WebSocketHandler {
	return &WebSocketHandler{
		hub: hub,
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
	conn, err := h.upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}

	client := service.NewClient(
		h.hub,
		conn,
		stream,
		parsePrincipal(ctx),
		parseSubscription(ctx),
	)

	h.hub.Register(client)
	go client.WriteLoop()
	client.ReadLoop()
}
func parseSubscription(ctx *gin.Context) service.Subscription {
	return service.Subscription{
		Applications: parseCSVSet(ctx.Query("app")),
		Levels:       parseCSVSet(ctx.Query("level")),
	}
}

func parsePrincipal(ctx *gin.Context) service.Principal {
	userID := ctx.GetHeader("X-User-ID")
	if userID == "" {
		userID = "anonymous"
	}

	role := ctx.GetHeader("X-User-Role")
	if role == "" {
		role = "anonymous"
	}

	return service.Principal{
		UserID: userID,
		Role:   role,
		Apps:   parseCSVSet(ctx.GetHeader("X-User-Apps")),
	}
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
