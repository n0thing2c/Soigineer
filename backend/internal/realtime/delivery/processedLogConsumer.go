package delivery

import (
	"context"
	"encoding/json"
	"log"
	"time"

	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
	"github.com/segmentio/kafka-go"
)

const defaultRetryBackoff = time.Second

type LogPublisher interface {
	PublishLog(ctx context.Context, event sharedDomain.ProcessedLogEvent) error
}

type ProcessedLogConsumerConfig struct {
	Brokers      []string
	Topic        string
	GroupID      string
	RetryBackoff time.Duration
}

type ProcessedLogConsumer struct {
	reader       *kafka.Reader
	publisher    LogPublisher
	retryBackoff time.Duration
}

func NewProcessedLogConsumer(cfg ProcessedLogConsumerConfig, p LogPublisher) *ProcessedLogConsumer {
	retryBackoff := cfg.RetryBackoff
	if retryBackoff <= 0 {
		retryBackoff = defaultRetryBackoff
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          cfg.Topic,
		GroupID:        cfg.GroupID,
		CommitInterval: 0,
		MinBytes:       1,
		MaxBytes:       1 << 20,
	})

	return &ProcessedLogConsumer{
		reader:       reader,
		publisher:    p,
		retryBackoff: retryBackoff,
	}
}

func (c *ProcessedLogConsumer) Close() error {
	return c.reader.Close()
}

func (c *ProcessedLogConsumer) Start(ctx context.Context) {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[ERROR] Failed to fetch log: %v", err)
			continue
		}

		var event sharedDomain.ProcessedLogEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf(
				"[WARNING] Invalid processed log payload. partition=%d offset=%d err=%v",
				msg.Partition,
				msg.Offset,
				err,
			)

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				log.Printf(
					"[ERROR] Failed to commit invalid log. partition=%d offset=%d err=%v",
					msg.Partition,
					msg.Offset,
					err,
				)
			}
			continue
		}

		if !c.publishWithRetry(ctx, event, msg) {
			return
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			log.Printf(
				"[ERROR] Failed to commit log. partition=%d offset=%d err=%v",
				msg.Partition,
				msg.Offset,
				err,
			)
		}
	}
}

func (c *ProcessedLogConsumer) publishWithRetry(
	ctx context.Context,
	event sharedDomain.ProcessedLogEvent,
	msg kafka.Message,
) bool {
	for {
		if err := c.publisher.PublishLog(ctx, event); err == nil {
			return true
		} else {
			log.Printf(
				"[ERROR] Failed to publish processed log to realtime hub; retrying. event_id=%s partition=%d offset=%d backoff=%s err=%v",
				event.EventID,
				msg.Partition,
				msg.Offset,
				c.retryBackoff,
				err,
			)
		}

		timer := time.NewTimer(c.retryBackoff)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return false
		case <-timer.C:
		}
	}
}
