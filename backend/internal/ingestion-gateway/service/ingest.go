package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/n0thing2c/Soigineer/internal/ingestion-gateway/infrastructure/queue"
	"github.com/n0thing2c/Soigineer/internal/shared/domain"
)

type Ingestion struct {
	producer        queue.Producer
	topic           string
	producerTimeout time.Duration
}

func NewIngestionService(p queue.Producer, t string, timeout time.Duration) *Ingestion {
	return &Ingestion{
		producer:        p,
		topic:           t,
		producerTimeout: timeout,
	}
}

func (i *Ingestion) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, i.producerTimeout)
}

func (i *Ingestion) IngestSingleLog(ctx context.Context, log domain.LogRecord) error {
	ctx, cancel := i.withTimeout(ctx)
	defer cancel()

	now := time.Now().UTC().Format(time.RFC3339)
	newEvent := domain.RawLogEvent{
		EventID:    uuid.NewString(),
		ReceivedAt: now,
		Source:     "ingestion-gateway",
		Payload:    log,
	}
	return i.producer.ProduceLog(ctx, i.topic, log.ApplicationName, newEvent)
}

func (i *Ingestion) IngestBatchLog(ctx context.Context, logs []domain.LogRecord) error {
	ctx, cancel := i.withTimeout(ctx)
	defer cancel()

	now := time.Now().UTC().Format(time.RFC3339)

	events := make([]domain.RawLogEvent, len(logs))
	for idx, log := range logs {
		events[idx] = domain.RawLogEvent{
			EventID:    uuid.NewString(),
			ReceivedAt: now,
			Source:     "ingestion-gateway",
			Payload:    log,
		}
	}

	return i.producer.ProduceBatchLog(ctx, i.topic, events)
}
