package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	alertDelivery "github.com/n0thing2c/Soigineer/internal/alerting/delivery"
	alertPostgres "github.com/n0thing2c/Soigineer/internal/alerting/infrastructure/postgres"
	alertRedis "github.com/n0thing2c/Soigineer/internal/alerting/infrastructure/redis"
	"github.com/n0thing2c/Soigineer/internal/alerting/infrastructure/telegram"
	alertService "github.com/n0thing2c/Soigineer/internal/alerting/service"
	authToken "github.com/n0thing2c/Soigineer/internal/auth/token"
	metadataPostgres "github.com/n0thing2c/Soigineer/internal/metadata/infrastructure/postgres"
	monitoringAccess "github.com/n0thing2c/Soigineer/internal/monitoring/access"
	monitoringDelivery "github.com/n0thing2c/Soigineer/internal/monitoring/delivery"
	monitoringRepository "github.com/n0thing2c/Soigineer/internal/monitoring/repository"
	monitoringService "github.com/n0thing2c/Soigineer/internal/monitoring/service"
	clickhouseDatabase "github.com/n0thing2c/Soigineer/internal/processing/infrastructure/database"
	realtimeDelivery "github.com/n0thing2c/Soigineer/internal/realtime/delivery"
	realtimeService "github.com/n0thing2c/Soigineer/internal/realtime/service"
	"github.com/n0thing2c/Soigineer/internal/shared/config"
	goredis "github.com/redis/go-redis/v9"
)

const (
	alertDedupKeyPrefix = "alert:dedup:"
	redisPingTimeout    = 5 * time.Second
	httpShutdownTimeout = 5 * time.Second
)

func main() {
	cfg := config.LoadConfig()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	redisClient := goredis.NewClient(&goredis.Options{
		Addr: cfg.RedisAddress,
	})
	defer redisClient.Close()

	pingCtx, cancelPing := context.WithTimeout(ctx, redisPingTimeout)
	err := redisClient.Ping(pingCtx).Err()
	cancelPing()
	if err != nil {
		log.Fatalf("connect to Redis at %s: %v", cfg.RedisAddress, err)
	}

	postgresDB, err := metadataPostgres.NewDB(cfg)
	if err != nil {
		log.Fatalf("connect to Postgres: %v", err)
	}
	defer postgresDB.Close()

	clickhouseDB, err := clickhouseDatabase.NewClickHouse(cfg)
	if err != nil {
		log.Fatalf("connect to ClickHouse: %v", err)
	}
	defer clickhouseDB.Close()

	notifiers := make([]alertService.ExternalNotifier, 0, 1)
	if strings.TrimSpace(cfg.TelegramBotToken) != "" && strings.TrimSpace(cfg.TelegramChatID) != "" {
		telegramNotifier, err := telegram.NewNotifier(
			cfg.TelegramBotToken,
			cfg.TelegramChatID,
			cfg.TelegramTimeout(),
		)
		if err != nil {
			log.Fatalf("create Telegram notifier: %v", err)
		}
		notifiers = append(notifiers, telegramNotifier)
	} else {
		log.Println("[INFO] Telegram notifier disabled because TELEGRAM_BOT_TOKEN or TELEGRAM_CHAT_ID is empty")
	}

	tokenManager := authToken.NewManager(cfg.AuthTokenSecret, cfg.AuthTokenTTL())
	principalStore := monitoringAccess.NewPrincipalStoreWithToken(postgresDB, tokenManager)
	hub := realtimeService.NewHub(realtimeService.RBACAuthorizer{})
	alertPublisher := realtimeService.NewAlertPublisher(hub)

	deduplicator := alertRedis.NewDeduplicator(
		redisClient,
		time.Duration(cfg.AlertDedupPeriod)*time.Second,
		alertDedupKeyPrefix,
	)

	alertingService := alertService.NewAlertingService(
		deduplicator,
		notifiers,
		alertPublisher,
		alertPostgres.NewIncidentRecorder(postgresDB),
	)

	alertConsumer := alertDelivery.NewAlertConsumer(alertDelivery.AlertConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Topic:   cfg.KafkaAlertTopic,
		GroupID: cfg.AlertConsumerGroup,
	}, alertingService)
	defer alertConsumer.Close()

	processedLogConsumer := realtimeDelivery.NewProcessedLogConsumer(
		realtimeDelivery.ProcessedLogConsumerConfig{
			Brokers: cfg.KafkaBrokers,
			Topic:   cfg.KafkaProcessedLogsTopic,
			GroupID: cfg.RealtimeLogConsumerGroup,
		},
		hub,
	)
	defer processedLogConsumer.Close()

	wsHandler := realtimeDelivery.NewWebSocketHandlerWithPrincipalLoader(hub, principalStore)
	apiService := monitoringService.NewMonitoringService(
		principalStore,
		monitoringRepository.NewClickHouseReader(clickhouseDB),
		monitoringRepository.NewPostgresReader(postgresDB),
	)
	apiHandler := monitoringDelivery.NewHandler(apiService)

	engine := gin.Default()
	engine.Use(corsMiddleware())
	engine.GET("/healthz", wsHandler.Health)

	v1 := engine.Group("/v1")
	monitoringDelivery.RegisterRoutes(v1, apiHandler)
	v1.GET("/realtime/logs", wsHandler.HandleLogs)
	v1.GET("/realtime/alerts", wsHandler.HandleAlerts)

	server := &http.Server{
		Addr:    ":" + cfg.MonitoringPort,
		Handler: engine,
	}

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		hub.Run(ctx)
	}()

	go func() {
		defer wg.Done()
		log.Printf("[INFO] Starting processed log consumer. topic=%s group=%s", cfg.KafkaProcessedLogsTopic, cfg.RealtimeLogConsumerGroup)
		processedLogConsumer.Start(ctx)
	}()

	go func() {
		defer wg.Done()
		log.Printf("[INFO] Starting alert consumer. topic=%s group=%s", cfg.KafkaAlertTopic, cfg.AlertConsumerGroup)
		alertConsumer.Start(ctx)
	}()

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("[INFO] Monitoring service is running at http://localhost:%s", cfg.MonitoringPort)
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[ERROR] Monitoring HTTP server stopped: %v", err)
			stop()
		}
	case <-ctx.Done():
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), httpShutdownTimeout)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] Monitoring HTTP server shutdown failed: %v", err)
	}

	stop()
	wg.Wait()
	log.Println("[INFO] Monitoring service stopped")
}

func corsMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		ctx.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-User-ID")
		ctx.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, OPTIONS")

		if ctx.Request.Method == http.MethodOptions {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}

		ctx.Next()
	}
}
