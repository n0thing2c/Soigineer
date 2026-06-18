package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/n0thing2c/Soigineer/internal/shared/domain"
	"github.com/segmentio/kafka-go"
)

type redpandaLogProducer struct {
	writer *kafka.Writer
}

func NewRedpandaLogProducer(brokers []string) *redpandaLogProducer {
	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    1000,
		BatchTimeout: 100 * time.Millisecond,
	}

	return &redpandaLogProducer{writer: writer}
}

func (p *redpandaLogProducer) Close() error {
	return p.writer.Close()
}

func (p *redpandaLogProducer) ProduceLog(ctx context.Context, topic string, key string, event domain.RawLogEvent) error {
	jsonEvent, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: []byte(jsonEvent),
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return err
	}
	return nil
}

func (p *redpandaLogProducer) ProduceBatchLog(ctx context.Context, topic string, events []domain.RawLogEvent) error {
	msgs := make([]kafka.Message, len(events))

	for idx, event := range events {
		jsonEvent, err := json.Marshal(event)
		if err != nil {
			return err
		}

		msg := kafka.Message{
			Topic: topic,
			Key:   []byte(event.Payload.ApplicationName),
			Value: jsonEvent,
		}
		msgs[idx] = msg
	}

	if err := p.writer.WriteMessages(ctx, msgs...); err != nil {
		return err
	}
	return nil
}
