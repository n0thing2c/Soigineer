package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/n0thing2c/Soigineer/internal/processing/delivery"
	"github.com/n0thing2c/Soigineer/internal/processing/infrastructure/database"
	"github.com/n0thing2c/Soigineer/internal/processing/infrastructure/queue"
	"github.com/n0thing2c/Soigineer/internal/processing/repository"
	"github.com/n0thing2c/Soigineer/internal/processing/service"
	"github.com/n0thing2c/Soigineer/internal/shared/config"
)

func main() {
	cfg := config.LoadConfig()

	clickhouseDB, err := database.NewClickHouse(cfg)
	if err != nil {
		log.Fatalf("Fail to start ClickHouse Database: %v", err)
	}
	defer clickhouseDB.Close()

	LogRepo := repository.NewClickHouseLogRepo(clickhouseDB, cfg.ProcessorSaveTimeout())
	AlertProducer := queue.NewAlertProducer(cfg.KafkaBrokers, cfg.KafkaAlertTopic, cfg.AlertProducerTimeout())
	defer AlertProducer.Close()
	logProcessService := service.NewProcessingService(LogRepo, AlertProducer)
	logConsumer := delivery.NewLogConsumer(delivery.ConsumerConfig{
		Brokers:         cfg.KafkaBrokers,
		Topic:           cfg.KafkaRawLogsTopic,
		GroupID:         cfg.ProcessorConsumerGroup,
		MaxBatchRows:    cfg.ProcessorBatchRows,
		MaxBatchBytes:   cfg.ProcessorBatchBytes,
		FlushInterval:   cfg.ProcessorFlushInterval(),
		MessageBuffer:   cfg.ProcessorMessageBuffer,
		ShutdownTimeout: cfg.ProcessorShutdownTimeout(),
	}, logProcessService)
	defer logConsumer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		log.Println("[INFO] Starting Log Consumer Processor...")
		logConsumer.Start(ctx)
	}()

	sig := <-sigChan
	log.Printf("\n[INFO] Received OS signal: %v. Initiating graceful shutdown...\n", sig)

	cancel()
	wg.Wait()
	log.Println("[INFO] Shutdown complete. Goodbye!")
}
