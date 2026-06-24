package queue

import (
	"context"
	"encoding/json"
	"time"

	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
	"github.com/segmentio/kafka-go"
)

type redpandaAlertProducer struct {
	writer  *kafka.Writer
	topic   string
	timeout time.Duration
}

func NewAlertProducer(brokers []string, topic string, timeout time.Duration) *redpandaAlertProducer {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    1,
		BatchTimeout: 0,
	}

	return &redpandaAlertProducer{
		writer:  writer,
		topic:   topic,
		timeout: timeout,
	}
}

func (p *redpandaAlertProducer) Close() error {
	return p.writer.Close()
}

func (p *redpandaAlertProducer) ProduceAlert(ctx context.Context, event sharedDomain.AlertEvent) error {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	jsonEvent, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := kafka.Message{
		Topic: p.topic,
		Key:   []byte(event.Fingerprint),
		Value: []byte(jsonEvent),
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return err
	}
	return nil
}
