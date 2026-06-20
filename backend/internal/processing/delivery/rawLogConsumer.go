package delivery

import (
	"context"
	"encoding/json"
	"log"
	"time"

	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
	"github.com/segmentio/kafka-go"
)

type LogProcessor interface {
	ProcessLog(ctx context.Context, events []sharedDomain.RawLogEvent) error
}

type ConsumerConfig struct {
	Brokers         []string
	Topic           string
	GroupID         string
	MaxBatchRows    int
	MaxBatchBytes   int
	FlushInterval   time.Duration
	MessageBuffer   int
	ShutdownTimeout time.Duration
}

type LogConsumer struct {
	reader          *kafka.Reader
	processor       LogProcessor
	maxBatchRows    int
	maxBatchBytes   int
	flushInterval   time.Duration
	messageBuffer   int
	shutdownTimeout time.Duration
}

func NewLogConsumer(cfg ConsumerConfig, processor LogProcessor) *LogConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          cfg.Topic,
		GroupID:        cfg.GroupID,
		CommitInterval: 0,
	})

	maxBatchRows := cfg.MaxBatchRows
	if maxBatchRows <= 0 {
		maxBatchRows = 1000
	}

	maxBatchBytes := cfg.MaxBatchBytes
	if maxBatchBytes <= 0 {
		maxBatchBytes = 4 * 1024 * 1024
	}

	flushInterval := cfg.FlushInterval
	if flushInterval <= 0 {
		flushInterval = 500 * time.Millisecond
	}

	messageBuffer := cfg.MessageBuffer
	if messageBuffer <= 0 {
		messageBuffer = maxBatchRows * 2
	}

	shutdownTimeout := cfg.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = 5 * time.Second
	}

	return &LogConsumer{
		reader:          reader,
		processor:       processor,
		maxBatchRows:    maxBatchRows,
		maxBatchBytes:   maxBatchBytes,
		flushInterval:   flushInterval,
		messageBuffer:   messageBuffer,
		shutdownTimeout: shutdownTimeout,
	}
}

func (c *LogConsumer) Close() error {
	return c.reader.Close()
}

func (c *LogConsumer) Start(ctx context.Context) {
	msgChan := make(chan kafka.Message, c.messageBuffer)
	go c.fetchLoop(ctx, msgChan)

	batchEvents := make([]sharedDomain.RawLogEvent, 0, c.maxBatchRows)
	batchMsgs := make([]kafka.Message, 0, c.maxBatchRows)
	currentBatchBytes := 0
	shuttingDown := false

	ticker := time.NewTicker(c.flushInterval)
	defer ticker.Stop()

	flush := func(flushCtx context.Context, reason string) {
		if len(batchEvents) == 0 {
			return
		}

		batchSize := len(batchEvents)
		batchMB := float64(currentBatchBytes) / (1024 * 1024)

		if err := c.processor.ProcessLog(flushCtx, batchEvents); err != nil {
			log.Printf(
				"[ERROR] Batch processing failed: reason=%s rows=%d bytes=%d err=%v",
				reason,
				batchSize,
				currentBatchBytes,
				err,
			)
			return
		}

		if err := c.reader.CommitMessages(flushCtx, batchMsgs...); err != nil {
			log.Printf(
				"[ERROR] Failed to commit offsets: reason=%s rows=%d bytes=%d err=%v",
				reason,
				batchSize,
				currentBatchBytes,
				err,
			)
			return
		}

		log.Printf("[INFO] Flushed rows=%d size_mb=%.2f reason=%s", batchSize, batchMB, reason)
		batchEvents = batchEvents[:0]
		batchMsgs = batchMsgs[:0]
		currentBatchBytes = 0
	}

	shutdownFlush := func(reason string) {
		flushCtx, cancel := context.WithTimeout(context.Background(), c.shutdownTimeout)
		defer cancel()
		flush(flushCtx, reason)
	}

	for {
		select {
		case <-ctx.Done():
			if !shuttingDown {
				log.Println("Shutdown signal received. Draining consumer before final flush...")
				shuttingDown = true
			}

		case msg, ok := <-msgChan:
			if !ok {
				shutdownFlush("shutdown")
				return
			}

			var event sharedDomain.RawLogEvent
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				log.Printf(
					"[WARNING] Invalid JSON payload. partition=%d offset=%d err=%v",
					msg.Partition,
					msg.Offset,
					err,
				)
				if err := c.reader.CommitMessages(context.Background(), msg); err != nil {
					log.Printf(
						"[ERROR] Failed to commit invalid payload. partition=%d offset=%d err=%v",
						msg.Partition,
						msg.Offset,
						err,
					)
				}
				continue
			}

			batchEvents = append(batchEvents, event)
			batchMsgs = append(batchMsgs, msg)
			currentBatchBytes += len(msg.Value)

			if len(batchEvents) >= c.maxBatchRows || currentBatchBytes >= c.maxBatchBytes {
				flush(ctx, "limit")
				ticker.Reset(c.flushInterval)
			}

		case <-ticker.C:
			if shuttingDown {
				continue
			}
			flush(ctx, "interval")
		}
	}
}

func (c *LogConsumer) fetchLoop(ctx context.Context, msgChan chan<- kafka.Message) {
	defer close(msgChan)

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[ERROR] Failed to fetch message: %v", err)
			continue
		}

		select {
		case msgChan <- msg:
		case <-ctx.Done():
			return
		}
	}
}
