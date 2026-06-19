package delivery

import (
	"context"
	"encoding/json"
	"log"
	"time"

	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
	"github.com/segmentio/kafka-go"
)

const (
	MaxBatchRows  = 50000
	MaxBatchBytes = 25 * 1024 * 1024
	FlushInterval = 3 * time.Second
)

type LogProcessor interface {
	ProcessLog(ctx context.Context, events []sharedDomain.RawLogEvent) error
}

type LogConsumer struct {
	reader    *kafka.Reader
	processor LogProcessor
}

func NewLogConsumer(brokers []string, topic string, groupID string, processor LogProcessor) *LogConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		CommitInterval: 0,
	})

	return &LogConsumer{reader: reader, processor: processor}
}

func (c *LogConsumer) Close() error {
	return c.reader.Close()
}

func (c *LogConsumer) Start(ctx context.Context) {
	msgChan := make(chan kafka.Message, MaxBatchRows)

	// Pull event from redpanda
	go func() {
		for {
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("[ERROR] Failed to fetch message: %v", err)
				continue
			}
			msgChan <- msg
		}
	}()

	batchEvents := make([]sharedDomain.RawLogEvent, 0, MaxBatchRows)
	batchMsgs := make([]kafka.Message, 0, MaxBatchRows)
	currentBatchBytes := 0

	ticker := time.NewTicker(FlushInterval)
	defer ticker.Stop()

	// Closure for flushing -> save DB and Commit
	flush := func() {
		if len(batchEvents) == 0 {
			return
		}

		err := c.processor.ProcessLog(ctx, batchEvents)
		if err != nil {
			log.Printf("[ERROR] Batch processing failed: %v", err)
			return
		}

		// Save sucessfuly -> Commit Offset
		if err := c.reader.CommitMessages(ctx, batchMsgs...); err != nil {
			log.Printf("[ERROR] Failed to commit offsets to Redpanda: %v", err)
		} else {
			log.Printf("[INFO] Flushed %d logs (%.2f MB)", len(batchEvents), float64(currentBatchBytes)/(1024*1024))
		}

		// Clean the buffer for new loop
		batchEvents = make([]sharedDomain.RawLogEvent, 0, MaxBatchRows)
		batchMsgs = make([]kafka.Message, 0, MaxBatchRows)
		currentBatchBytes = 0
	}

	// Batching and send
	for {
		select {
		case <-ctx.Done():
			// Graceful Shut down
			log.Println("Shutting down... Flushing remaining logs...")
			flush()
			return

		case msg := <-msgChan:
			var event sharedDomain.RawLogEvent
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				log.Printf("[WARNING] Invalid JSON payload. Skipping. Err: %v", err)
				// Commit failed log
				c.reader.CommitMessages(ctx, msg)
				continue
			}

			batchEvents = append(batchEvents, event)
			batchMsgs = append(batchMsgs, msg)
			currentBatchBytes += len(msg.Value)

			// If exceed size
			if len(batchEvents) >= MaxBatchRows || currentBatchBytes >= MaxBatchBytes {
				flush()
				ticker.Reset(FlushInterval)
			}

		case <-ticker.C:
			// When exceed flushInterval
			flush()
		}
	}
}
