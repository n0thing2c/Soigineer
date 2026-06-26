package queue

import (
	"context"
	"encoding/json"
	"time"

	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
	"github.com/segmentio/kafka-go"
)

type redpandaProcessedLogProducer struct {
	writer  *kafka.Writer
	topic   string
	timeout time.Duration
}

func NewProcessedLogProducer(brokers []string, topic string, timeout time.Duration) *redpandaProcessedLogProducer {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Balancer:     &kafka.LeastBytes{},
		BatchSize:    1000,
		BatchTimeout: 100 * time.Millisecond,
	}

	return &redpandaProcessedLogProducer{
		writer:  writer,
		topic:   topic,
		timeout: timeout,
	}
}

func (p *redpandaProcessedLogProducer) Close() error {
	return p.writer.Close()
}

func (p *redpandaProcessedLogProducer) ProduceProcessedLogs(ctx context.Context, events []sharedDomain.ProcessedLogEvent) error {
	if len(events) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	msgs := make([]kafka.Message, len(events))
	for idx, event := range events {
		jsonEvent, err := json.Marshal(event)
		if err != nil {
			return err
		}

		msgs[idx] = kafka.Message{
			Topic: p.topic,
			Key:   []byte(event.ApplicationName),
			Value: jsonEvent,
		}
	}

	return p.writer.WriteMessages(ctx, msgs...)
}
