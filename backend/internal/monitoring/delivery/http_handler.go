package delivery

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/n0thing2c/Soigineer/internal/monitoring/repository"
	monitoringService "github.com/n0thing2c/Soigineer/internal/monitoring/service"
)

type Handler struct {
	service *monitoringService.MonitoringService
}

func NewHandler(service *monitoringService.MonitoringService) *Handler {
	return &Handler{service: service}
}

func RegisterRoutes(group *gin.RouterGroup, h *Handler) {
	group.GET("/me", h.Me)
	group.GET("/applications", h.Applications)
	group.GET("/logs", h.Logs)
	group.GET("/incidents", h.Incidents)
	group.PATCH("/incidents/:id/status", h.UpdateIncidentStatus)
	group.GET("/analytics/health", h.Health)
	group.GET("/admin/alert-rules", h.AlertRules)
	group.PUT("/admin/alert-rules/:id", h.UpdateAlertRule)
}

func (h *Handler) Me(ctx *gin.Context) {
	principal, err := h.service.Me(ctx.Request.Context(), credentials(ctx))
	h.respondItem(ctx, principal, err)
}

func (h *Handler) Applications(ctx *gin.Context) {
	apps, err := h.service.ListApplications(ctx.Request.Context(), credentials(ctx))
	h.respondItems(ctx, apps, err)
}

func (h *Handler) Logs(ctx *gin.Context) {
	logs, err := h.service.ListLogs(ctx.Request.Context(), credentials(ctx), parseLogFilters(ctx))
	h.respondItems(ctx, logs, err)
}

func (h *Handler) Incidents(ctx *gin.Context) {
	incidents, err := h.service.ListIncidents(ctx.Request.Context(), credentials(ctx), repository.IncidentFilters{
		Applications: parseCSV(ctx.Query("app")),
		Levels:       parseCSV(ctx.Query("level")),
		Status:       strings.TrimSpace(ctx.Query("status")),
		Limit:        parseLimit(ctx.Query("limit")),
	})
	h.respondItems(ctx, incidents, err)
}

func (h *Handler) UpdateIncidentStatus(ctx *gin.Context) {
	var payload struct {
		Status string `json:"status"`
	}
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		respondError(ctx, http.StatusBadRequest, err)
		return
	}
	if err := h.service.UpdateIncidentStatus(
		ctx.Request.Context(),
		credentials(ctx),
		ctx.Param("id"),
		payload.Status,
	); err != nil {
		respondError(ctx, statusForError(err), err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (h *Handler) Health(ctx *gin.Context) {
	rows, err := h.service.Health(ctx.Request.Context(), credentials(ctx), parseLogFilters(ctx))
	h.respondItems(ctx, rows, err)
}

func (h *Handler) AlertRules(ctx *gin.Context) {
	rules, err := h.service.ListAlertRules(ctx.Request.Context(), credentials(ctx))
	h.respondItems(ctx, rules, err)
}

func (h *Handler) UpdateAlertRule(ctx *gin.Context) {
	var payload struct {
		Enabled            bool `json:"enabled"`
		DedupWindowSeconds int  `json:"dedupWindowSeconds"`
		TelegramEnabled    bool `json:"telegramEnabled"`
	}
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		respondError(ctx, http.StatusBadRequest, err)
		return
	}

	err := h.service.UpdateAlertRule(ctx.Request.Context(), credentials(ctx), ctx.Param("id"), repository.AlertRuleUpdate{
		Enabled:            payload.Enabled,
		DedupWindowSeconds: payload.DedupWindowSeconds,
		TelegramEnabled:    payload.TelegramEnabled,
	})
	if err != nil {
		respondError(ctx, statusForError(err), err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (h *Handler) respondItem(ctx *gin.Context, value any, err error) {
	if err != nil {
		respondError(ctx, statusForError(err), err)
		return
	}
	ctx.JSON(http.StatusOK, value)
}

func (h *Handler) respondItems(ctx *gin.Context, items any, err error) {
	if err != nil {
		respondError(ctx, statusForError(err), err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"items": items})
}

func credentials(ctx *gin.Context) monitoringService.Credentials {
	identity := strings.TrimSpace(ctx.GetHeader("X-User-ID"))
	if identity == "" {
		identity = strings.TrimSpace(ctx.Query("userId"))
	}
	return monitoringService.Credentials{
		BearerToken: bearerToken(ctx),
		Identity:    identity,
	}
}

func bearerToken(ctx *gin.Context) string {
	header := strings.TrimSpace(ctx.GetHeader("Authorization"))
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return ""
	}
	return strings.TrimSpace(header[len("Bearer "):])
}

func parseLogFilters(ctx *gin.Context) repository.LogFilters {
	return repository.LogFilters{
		Applications: parseCSV(ctx.Query("app")),
		Levels:       parseCSV(ctx.Query("level")),
		From:         parseTime(ctx.Query("from")),
		To:           parseTime(ctx.Query("to")),
		Limit:        parseLimit(ctx.Query("limit")),
	}
}

func parseCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func parseTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func parseLimit(value string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return limit
}

func respondError(ctx *gin.Context, status int, err error) {
	ctx.JSON(status, gin.H{
		"error": err.Error(),
	})
}

func statusForError(err error) int {
	switch {
	case errors.Is(err, monitoringService.ErrMissingIdentity),
		errors.Is(err, monitoringService.ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, monitoringService.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, monitoringService.ErrInvalidIncidentStatus),
		errors.Is(err, monitoringService.ErrInvalidAlertRuleUpdate):
		return http.StatusBadRequest
	case errors.Is(err, monitoringService.ErrNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}
