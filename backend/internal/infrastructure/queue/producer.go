package queue

import (
	"context"

	"github.com/n0thing2c/Soigineer/internal/shared/domain"
)

type Producer interface {
	ProduceLog(ctx context.Context, topic string, key string, event domain.RawLogEvent) error
	ProduceBatchLog(ctx context.Context, topic string, events []domain.RawLogEvent) error
}

