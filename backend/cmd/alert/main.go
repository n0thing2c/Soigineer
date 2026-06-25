package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/n0thing2c/Soigineer/internal/alerting/delivery"
	"github.com/n0thing2c/Soigineer/internal/alerting/infrastructure/redis"
	"github.com/n0thing2c/Soigineer/internal/alerting/infrastructure/telegram"
	alertService "github.com/n0thing2c/Soigineer/internal/alerting/service"
	"github.com/n0thing2c/Soigineer/internal/shared/config"
	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
	goredis "github.com/redis/go-redis/v9"
)

const (
	alertDedupKeyPrefix = "alert:dedup:"
	redisPingTimeout    = 5 * time.Second
)

// disabledRealtimePublisher keeps alert dispatch wiring explicit until the
// WebSocket publisher is implemented.
type disabledRealtimePublisher struct{}

func (disabledRealtimePublisher) Publish(
	context.Context,
	sharedDomain.AlertEvent,
) error {
	return nil
}

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

	deduplicator := redis.NewDeduplicator(
		redisClient,
		time.Duration(cfg.AlertDedupPeriod)*time.Second,
		alertDedupKeyPrefix,
	)

	service := alertService.NewAlertingService(
		deduplicator,
		[]alertService.ExternalNotifier{telegramNotifier},
		disabledRealtimePublisher{},
	)

	consumer := delivery.NewAlertConsumer(delivery.AlertConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Topic:   cfg.KafkaAlertTopic,
		GroupID: cfg.AlertConsumerGroup,
	}, service)
	defer consumer.Close()

	log.Printf(
		"[INFO] Starting alert consumer. topic=%s group=%s",
		cfg.KafkaAlertTopic,
		cfg.AlertConsumerGroup,
	)
	consumer.Start(ctx)
	log.Println("[INFO] Alert service stopped")
}
