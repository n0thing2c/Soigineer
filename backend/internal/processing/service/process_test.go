package service

import (
	"context"
	"errors"
	"testing"
	"time"

	processingDomain "github.com/n0thing2c/Soigineer/internal/processing/domain"
	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
)

type fakeLogRepository struct {
	err    error
	saves  int
	models []*processingDomain.LogModel
}

func (f *fakeLogRepository) Save(ctx context.Context, models []*processingDomain.LogModel) error {
	f.saves++
	f.models = append([]*processingDomain.LogModel(nil), models...)
	return f.err
}

type fakeAlertProducer struct {
	err    error
	alerts []sharedDomain.AlertEvent
}

func (f *fakeAlertProducer) ProduceAlert(ctx context.Context, alert sharedDomain.AlertEvent) error {
	f.alerts = append(f.alerts, alert)
	return f.err
}

func sampleRawEvent(level string) sharedDomain.RawLogEvent {
	return sharedDomain.RawLogEvent{
		EventID:    "event-1",
		ReceivedAt: "2026-06-21T10:00:01Z",
		Source:     "ingestion-gateway",
		Payload: sharedDomain.LogRecord{
			ApplicationName: "app-001",
			Level:           level,
			Message:         "Database connection timeout after 3000ms",
			Timestamp:       "2026-06-21T10:00:00Z",
			TraceID:         "trace-1",
		},
	}
}

func TestCreateLogModelSuccess(t *testing.T) {
	event := sampleRawEvent("ERROR")

	model, err := CreateLogModel(event)
	if err != nil {
		t.Fatalf("CreateLogModel() error = %v", err)
	}

	if model.EventID != "event-1" || model.ApplicationName != "app-001" || model.Level != "ERROR" || model.TraceID != "trace-1" {
		t.Fatalf("model identity fields = %+v", model)
	}
	if model.Message != event.Payload.Message {
		t.Fatalf("Message = %q, want original %q", model.Message, event.Payload.Message)
	}
	if model.NormalizedMessage != "database connection timeout after 3000ms" {
		t.Fatalf("NormalizedMessage = %q", model.NormalizedMessage)
	}
	wantFingerprint := GenerateFingerprint("app-001", "ERROR", string(DatabaseError), model.NormalizedMessage)
	if model.Fingerprint != wantFingerprint {
		t.Fatalf("Fingerprint = %q, want %q", model.Fingerprint, wantFingerprint)
	}
	if model.Timestamp.IsZero() || model.ReceivedAt.IsZero() {
		t.Fatalf("timestamps were not parsed: %+v", model)
	}
}

func TestCreateLogModelRejectsInvalidTimestamps(t *testing.T) {
	event := sampleRawEvent("INFO")
	event.Payload.Timestamp = "bad"
	if _, err := CreateLogModel(event); err == nil {
		t.Fatal("expected invalid payload timestamp error")
	}

	event = sampleRawEvent("INFO")
	event.ReceivedAt = "bad"
	if _, err := CreateLogModel(event); err == nil {
		t.Fatal("expected invalid receivedAt error")
	}
}

func TestProcessLogReturnsNilForEmptyInput(t *testing.T) {
	repo := &fakeLogRepository{}
	producer := &fakeAlertProducer{}
	service := NewProcessingService(repo, producer)

	if err := service.ProcessLog(context.Background(), nil); err != nil {
		t.Fatalf("ProcessLog(nil) error = %v", err)
	}
	if repo.saves != 0 || len(producer.alerts) != 0 {
		t.Fatalf("repo saves/alerts = %d/%d", repo.saves, len(producer.alerts))
	}
}

func TestProcessLogSavesValidModelsAndAlertsErrors(t *testing.T) {
	repo := &fakeLogRepository{}
	producer := &fakeAlertProducer{}
	service := NewProcessingService(repo, producer)

	events := []sharedDomain.RawLogEvent{
		sampleRawEvent("INFO"),
		sampleRawEvent("ERROR"),
		sampleRawEvent("CRITICAL"),
	}

	if err := service.ProcessLog(context.Background(), events); err != nil {
		t.Fatalf("ProcessLog() error = %v", err)
	}
	if repo.saves != 1 || len(repo.models) != 3 {
		t.Fatalf("repo saves/models = %d/%d", repo.saves, len(repo.models))
	}
	if len(producer.alerts) != 2 {
		t.Fatalf("alerts = %d, want 2", len(producer.alerts))
	}
}

func TestProcessLogSkipsInvalidEvents(t *testing.T) {
	repo := &fakeLogRepository{}
	service := NewProcessingService(repo, &fakeAlertProducer{})

	valid := sampleRawEvent("INFO")
	invalid := sampleRawEvent("WARN")
	invalid.Payload.Timestamp = "bad"

	if err := service.ProcessLog(context.Background(), []sharedDomain.RawLogEvent{invalid, valid}); err != nil {
		t.Fatalf("ProcessLog() error = %v", err)
	}
	if repo.saves != 1 || len(repo.models) != 1 || repo.models[0].TraceID != valid.Payload.TraceID {
		t.Fatalf("repo saves/models = %d/%#v", repo.saves, repo.models)
	}
}

func TestProcessLogReturnsNilWhenAllEventsInvalid(t *testing.T) {
	repo := &fakeLogRepository{}
	service := NewProcessingService(repo, &fakeAlertProducer{})

	invalid := sampleRawEvent("INFO")
	invalid.Payload.Timestamp = "bad"

	if err := service.ProcessLog(context.Background(), []sharedDomain.RawLogEvent{invalid}); err != nil {
		t.Fatalf("ProcessLog() error = %v", err)
	}
	if repo.saves != 0 {
		t.Fatalf("repo saves = %d, want 0", repo.saves)
	}
}

func TestProcessLogPropagatesRepoError(t *testing.T) {
	wantErr := errors.New("save failed")
	service := NewProcessingService(&fakeLogRepository{err: wantErr}, &fakeAlertProducer{})

	if err := service.ProcessLog(context.Background(), []sharedDomain.RawLogEvent{sampleRawEvent("INFO")}); !errors.Is(err, wantErr) {
		t.Fatalf("ProcessLog() error = %v, want %v", err, wantErr)
	}
}

func TestProcessLogPropagatesAlertProducerError(t *testing.T) {
	wantErr := errors.New("alert failed")
	repo := &fakeLogRepository{}
	producer := &fakeAlertProducer{err: wantErr}
	service := NewProcessingService(repo, producer)

	err := service.ProcessLog(context.Background(), []sharedDomain.RawLogEvent{sampleRawEvent("ERROR")})
	if !errors.Is(err, wantErr) {
		t.Fatalf("ProcessLog() error = %v, want %v", err, wantErr)
	}
	if len(producer.alerts) != 1 || repo.saves != 0 {
		t.Fatalf("alerts/saves = %d/%d", len(producer.alerts), repo.saves)
	}
}

func TestCreateLogModelParsesRFC3339Nano(t *testing.T) {
	event := sampleRawEvent("INFO")
	event.Payload.Timestamp = "2026-06-21T10:00:00.123456789Z"
	event.ReceivedAt = "2026-06-21T10:00:01.987654321Z"

	model, err := CreateLogModel(event)
	if err != nil {
		t.Fatalf("CreateLogModel() error = %v", err)
	}
	if model.Timestamp.Nanosecond() != 123456789 || model.ReceivedAt.Nanosecond() != 987654321 {
		t.Fatalf("parsed timestamps = %s/%s", model.Timestamp.Format(time.RFC3339Nano), model.ReceivedAt.Format(time.RFC3339Nano))
	}
}
