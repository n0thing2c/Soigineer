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

type AlertHandler interface {
	Alert(ctx context.Context, alert sharedDomain.AlertEvent) error
}

type AlertConsumerConfig struct {
	Brokers      []string
	Topic        string
	GroupID      string
	RetryBackoff time.Duration
}

type AlertConsumer struct {
	reader       *kafka.Reader
	handler      AlertHandler
	retryBackoff time.Duration
}

func NewAlertConsumer(cfg AlertConsumerConfig, handler AlertHandler) *AlertConsumer {
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

	return &AlertConsumer{
		reader:       reader,
		handler:      handler,
		retryBackoff: retryBackoff,
	}
}

func (c *AlertConsumer) Close() error {
	return c.reader.Close()
}

func (c *AlertConsumer) Start(ctx context.Context) {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[ERROR] Failed to fetch alert: %v", err)
			continue
		}

		var event sharedDomain.AlertEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf(
				"[WARNING] Invalid alert payload. partition=%d offset=%d err=%v",
				msg.Partition,
				msg.Offset,
				err,
			)

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				log.Printf(
					"[ERROR] Failed to commit invalid alert. partition=%d offset=%d err=%v",
					msg.Partition,
					msg.Offset,
					err,
				)
			}
			continue
		}

		if !c.processWithRetry(ctx, event, msg) {
			return
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			log.Printf(
				"[ERROR] Failed to commit alert. partition=%d offset=%d err=%v",
				msg.Partition,
				msg.Offset,
				err,
			)
		}
	}
}

func (c *AlertConsumer) processWithRetry(
	ctx context.Context,
	event sharedDomain.AlertEvent,
	msg kafka.Message,
) bool {
	for {
		if err := c.handler.Alert(ctx, event); err == nil {
			return true
		} else {
			log.Printf(
				"[ERROR] Failed to process alert; retrying. event_id=%s partition=%d offset=%d backoff=%s err=%v",
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
