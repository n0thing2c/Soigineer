package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	alertDelivery "github.com/n0thing2c/Soigineer/internal/alerting/delivery"
	alertRedis "github.com/n0thing2c/Soigineer/internal/alerting/infrastructure/redis"
	"github.com/n0thing2c/Soigineer/internal/alerting/infrastructure/telegram"
	alertService "github.com/n0thing2c/Soigineer/internal/alerting/service"
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

	telegramNotifier, err := telegram.NewNotifier(
		cfg.TelegramBotToken,
		cfg.TelegramChatID,
		cfg.TelegramTimeout(),
	)
	if err != nil {
		log.Fatalf("create Telegram notifier: %v", err)
	}

	hub := realtimeService.NewHub(realtimeService.AllowAllAuthorizer{})
	alertPublisher := realtimeService.NewAlertPublisher(hub)

	deduplicator := alertRedis.NewDeduplicator(
		redisClient,
		time.Duration(cfg.AlertDedupPeriod)*time.Second,
		alertDedupKeyPrefix,
	)

	alertingService := alertService.NewAlertingService(
		deduplicator,
		[]alertService.ExternalNotifier{telegramNotifier},
		alertPublisher,
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

	wsHandler := realtimeDelivery.NewWebSocketHandler(hub)
	engine := gin.Default()
	engine.GET("/healthz", wsHandler.Health)

	v1 := engine.Group("/v1")
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
