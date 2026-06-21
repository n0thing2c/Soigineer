package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/n0thing2c/Soigineer/internal/shared/domain"
)

type fakeLogProducer struct {
	produceErr      error
	produceBatchErr error
	singleCtx       context.Context
	batchCtx        context.Context
	topic           string
	key             string
	event           domain.RawLogEvent
	batchTopic      string
	events          []domain.RawLogEvent
}

func (f *fakeLogProducer) ProduceLog(ctx context.Context, topic string, key string, event domain.RawLogEvent) error {
	f.singleCtx = ctx
	f.topic = topic
	f.key = key
	f.event = event
	return f.produceErr
}

func (f *fakeLogProducer) ProduceBatchLog(ctx context.Context, topic string, events []domain.RawLogEvent) error {
	f.batchCtx = ctx
	f.batchTopic = topic
	f.events = append([]domain.RawLogEvent(nil), events...)
	return f.produceBatchErr
}

func sampleLogRecord() domain.LogRecord {
	return domain.LogRecord{
		ApplicationName: "app-001",
		Level:           "INFO",
		Message:         "hello",
		Timestamp:       "2026-06-21T10:00:00Z",
		TraceID:         "trace-1",
	}
}

func TestIngestSingleLogProducesRawEvent(t *testing.T) {
	producer := &fakeLogProducer{}
	service := NewIngestionService(producer, "raw-logs", time.Second)
	logRecord := sampleLogRecord()

	if err := service.IngestSingleLog(context.Background(), logRecord); err != nil {
		t.Fatalf("IngestSingleLog() error = %v", err)
	}

	if producer.topic != "raw-logs" || producer.key != "app-001" {
		t.Fatalf("topic/key = %q/%q", producer.topic, producer.key)
	}
	if producer.event.EventID == "" {
		t.Fatal("EventID is empty")
	}
	if _, err := time.Parse(time.RFC3339, producer.event.ReceivedAt); err != nil {
		t.Fatalf("ReceivedAt = %q, want RFC3339: %v", producer.event.ReceivedAt, err)
	}
	if producer.event.Source != "ingestion-gateway" {
		t.Fatalf("Source = %q", producer.event.Source)
	}
	if producer.event.Payload != logRecord {
		t.Fatalf("Payload = %+v, want %+v", producer.event.Payload, logRecord)
	}
	if _, ok := producer.singleCtx.Deadline(); !ok {
		t.Fatal("expected producer context deadline")
	}
}

func TestIngestBatchLogProducesRawEvents(t *testing.T) {
	producer := &fakeLogProducer{}
	service := NewIngestionService(producer, "raw-logs", time.Second)
	logs := []domain.LogRecord{sampleLogRecord(), sampleLogRecord()}
	logs[1].ApplicationName = "app-002"
	logs[1].TraceID = "trace-2"

	if err := service.IngestBatchLog(context.Background(), logs); err != nil {
		t.Fatalf("IngestBatchLog() error = %v", err)
	}

	if producer.batchTopic != "raw-logs" {
		t.Fatalf("batchTopic = %q", producer.batchTopic)
	}
	if len(producer.events) != len(logs) {
		t.Fatalf("len(events) = %d, want %d", len(producer.events), len(logs))
	}
	for idx, event := range producer.events {
		if event.EventID == "" || event.Source != "ingestion-gateway" || event.Payload != logs[idx] {
			t.Fatalf("event[%d] = %+v", idx, event)
		}
	}
	if _, ok := producer.batchCtx.Deadline(); !ok {
		t.Fatal("expected producer batch context deadline")
	}
}

func TestIngestionPropagatesProducerErrors(t *testing.T) {
	wantErr := errors.New("producer failed")

	producer := &fakeLogProducer{produceErr: wantErr}
	service := NewIngestionService(producer, "raw-logs", time.Second)
	if err := service.IngestSingleLog(context.Background(), sampleLogRecord()); !errors.Is(err, wantErr) {
		t.Fatalf("IngestSingleLog() error = %v, want %v", err, wantErr)
	}

	producer = &fakeLogProducer{produceBatchErr: wantErr}
	service = NewIngestionService(producer, "raw-logs", time.Second)
	if err := service.IngestBatchLog(context.Background(), []domain.LogRecord{sampleLogRecord()}); !errors.Is(err, wantErr) {
		t.Fatalf("IngestBatchLog() error = %v, want %v", err, wantErr)
	}
}

func TestIngestionPassesCanceledContextToProducer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	producer := &fakeLogProducer{}
	service := NewIngestionService(producer, "raw-logs", time.Second)
	if err := service.IngestSingleLog(ctx, sampleLogRecord()); err != nil {
		t.Fatalf("IngestSingleLog() error = %v", err)
	}
	if producer.singleCtx.Err() == nil {
		t.Fatal("expected canceled context to reach producer")
	}
}
