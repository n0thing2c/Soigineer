package producer

import (
	"context"

	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
)

type RedpandaAlertProducer struct {
}

func NewAlertProducer() *RedpandaAlertProducer {
	return &RedpandaAlertProducer{}
}

func (p *RedpandaAlertProducer) ProduceAlert(ctx context.Context, log sharedDomain.AlertEvent) error {
	return nil
}
