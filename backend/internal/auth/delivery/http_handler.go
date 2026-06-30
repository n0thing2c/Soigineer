package delivery

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/n0thing2c/Soigineer/internal/auth/repository"
	"github.com/n0thing2c/Soigineer/internal/auth/service"
	"github.com/n0thing2c/Soigineer/internal/auth/token"
)

type Handler struct {
	auth *service.AuthService
}

func NewHandler(auth *service.AuthService) *Handler {
	return &Handler{auth: auth}
}

func RegisterRoutes(group *gin.RouterGroup, h *Handler) {
	group.POST("/auth/login", h.Login)
	group.GET("/auth/me", h.Me)
	group.GET("/admin/users", h.ListUsers)
	group.POST("/admin/users", h.CreateUser)
	group.PUT("/admin/users/:id/applications", h.ReplaceApplications)
	group.GET("/admin/applications", h.ListApplications)
}

func (h *Handler) Login(ctx *gin.Context) {
	var payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		respondError(ctx, http.StatusBadRequest, err)
		return
	}

	result, err := h.auth.Login(ctx.Request.Context(), payload.Username, payload.Password)
	if errors.Is(err, service.ErrInvalidCredentials) {
		respondError(ctx, http.StatusUnauthorized, err)
		return
	}
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, result)
}

func (h *Handler) Me(ctx *gin.Context) {
	user, ok := h.requireAuthenticated(ctx)
	if !ok {
		return
	}
	ctx.JSON(http.StatusOK, user)
}

func (h *Handler) ListUsers(ctx *gin.Context) {
	if _, ok := h.requireAdmin(ctx); !ok {
		return
	}

	users, err := h.auth.ListUsers(ctx.Request.Context())
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"items": users})
}

func (h *Handler) CreateUser(ctx *gin.Context) {
	if _, ok := h.requireAdmin(ctx); !ok {
		return
	}

	var payload struct {
		Username     string   `json:"username"`
		Password     string   `json:"password"`
		Role         string   `json:"role"`
		Applications []string `json:"applications"`
	}
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		respondError(ctx, http.StatusBadRequest, err)
		return
	}

	user, err := h.auth.CreateUser(
		ctx.Request.Context(),
		payload.Username,
		payload.Password,
		payload.Role,
		payload.Applications,
	)
	if errors.Is(err, service.ErrInvalidRole) {
		respondError(ctx, http.StatusBadRequest, err)
		return
	}
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusCreated, user)
}

func (h *Handler) ReplaceApplications(ctx *gin.Context) {
	if _, ok := h.requireAdmin(ctx); !ok {
		return
	}

	var payload struct {
		Applications []string `json:"applications"`
	}
	if err := ctx.ShouldBindJSON(&payload); err != nil {
		respondError(ctx, http.StatusBadRequest, err)
		return
	}

	user, err := h.auth.ReplaceApplications(ctx.Request.Context(), ctx.Param("id"), payload.Applications)
	if errors.Is(err, repository.ErrUserNotFound) {
		respondError(ctx, http.StatusNotFound, err)
		return
	}
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, user)
}

func (h *Handler) ListApplications(ctx *gin.Context) {
	if _, ok := h.requireAdmin(ctx); !ok {
		return
	}

	apps, err := h.auth.ListApplications(ctx.Request.Context())
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, err)
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"items": apps})
}

func (h *Handler) requireAuthenticated(ctx *gin.Context) (repository.User, bool) {
	rawToken, ok := bearerToken(ctx)
	if !ok {
		respondError(ctx, http.StatusUnauthorized, errors.New("bearer token is required"))
		return repository.User{}, false
	}

	user, err := h.auth.Me(ctx.Request.Context(), rawToken)
	if errors.Is(err, token.ErrInvalidToken) || errors.Is(err, token.ErrExpiredToken) {
		respondError(ctx, http.StatusUnauthorized, err)
		return repository.User{}, false
	}
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, err)
		return repository.User{}, false
	}
	return user, true
}

func (h *Handler) requireAdmin(ctx *gin.Context) (repository.User, bool) {
	rawToken, ok := bearerToken(ctx)
	if !ok {
		respondError(ctx, http.StatusUnauthorized, errors.New("bearer token is required"))
		return repository.User{}, false
	}

	user, err := h.auth.RequireAdmin(ctx.Request.Context(), rawToken)
	if errors.Is(err, service.ErrForbidden) {
		respondError(ctx, http.StatusForbidden, err)
		return repository.User{}, false
	}
	if errors.Is(err, token.ErrInvalidToken) || errors.Is(err, token.ErrExpiredToken) {
		respondError(ctx, http.StatusUnauthorized, err)
		return repository.User{}, false
	}
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, err)
		return repository.User{}, false
	}
	return user, true
}

func bearerToken(ctx *gin.Context) (string, bool) {
	header := strings.TrimSpace(ctx.GetHeader("Authorization"))
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return "", false
	}
	rawToken := strings.TrimSpace(header[len("Bearer "):])
	return rawToken, rawToken != ""
}

func respondError(ctx *gin.Context, status int, err error) {
	ctx.JSON(status, gin.H{"error": err.Error()})
}
