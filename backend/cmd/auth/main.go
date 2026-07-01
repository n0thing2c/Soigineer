package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	authDelivery "github.com/n0thing2c/Soigineer/internal/auth/delivery"
	authRepository "github.com/n0thing2c/Soigineer/internal/auth/repository"
	authService "github.com/n0thing2c/Soigineer/internal/auth/service"
	"github.com/n0thing2c/Soigineer/internal/auth/token"
	metadataPostgres "github.com/n0thing2c/Soigineer/internal/metadata/infrastructure/postgres"
	"github.com/n0thing2c/Soigineer/internal/shared/config"
)

const shutdownTimeout = 5 * time.Second

func main() {
	cfg := config.LoadConfig()

	db, err := metadataPostgres.NewDB(cfg)
	if err != nil {
		log.Fatalf("connect to Postgres: %v", err)
	}
	defer db.Close()

	userRepo := authRepository.NewUserRepository(db)
	refreshTokenRepo := authRepository.NewRefreshTokenRepository(db)
	auth := authService.NewAuthService(
		userRepo,
		refreshTokenRepo,
		token.NewManager(cfg.AuthTokenSecret, cfg.AuthTokenTTL()),
		cfg.AuthRefreshTokenTTL(),
	)

	if err := auth.BootstrapDefaults(
		context.Background(),
		cfg.BootstrapAdminPassword,
		cfg.BootstrapEngineerPassword,
	); err != nil {
		log.Fatalf("bootstrap auth defaults: %v", err)
	}

	engine := gin.Default()
	engine.Use(corsMiddleware())
	engine.GET("/healthz", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := engine.Group("/v1")
	authDelivery.RegisterRoutes(v1, authDelivery.NewHandler(auth))

	server := &http.Server{
		Addr:    ":" + cfg.AuthPort,
		Handler: engine,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("[INFO] Auth service is running at http://localhost:%s", cfg.AuthPort)
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[ERROR] Auth HTTP server stopped: %v", err)
			stop()
		}
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] Auth HTTP server shutdown failed: %v", err)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		ctx.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		ctx.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, OPTIONS")

		if ctx.Request.Method == http.MethodOptions {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}

		ctx.Next()
	}
}
