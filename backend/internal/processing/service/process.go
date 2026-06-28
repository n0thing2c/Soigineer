package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/n0thing2c/Soigineer/internal/processing/domain"
	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
)

type LogRepository interface {
	Save(ctx context.Context, log []*domain.LogModel) error
}

type AlertProducer interface {
	ProduceAlert(ctx context.Context, log sharedDomain.AlertEvent) error
}

type ProcessedLogProducer interface {
	ProduceProcessedLogs(ctx context.Context, logs []sharedDomain.ProcessedLogEvent) error
}

type ProcessingService struct {
	Repo                 LogRepository
	Producer             AlertProducer
	ProcessedLogProducer ProcessedLogProducer
}

func NewProcessingService(r LogRepository, p AlertProducer, processedProducer ProcessedLogProducer) *ProcessingService {
	return &ProcessingService{
		Repo:                 r,
		Producer:             p,
		ProcessedLogProducer: processedProducer,
	}
}

func CreateLogModel(event sharedDomain.RawLogEvent) (*domain.LogModel, error) {
	appName := event.Payload.ApplicationName
	level := event.Payload.Level
	msg, normalizedMsg := NormalizeMessage(event.Payload.Message)

	timestamp, err := time.Parse(time.RFC3339Nano, event.Payload.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("invalid event timestamp: %w", err)
	}

	receivedAt, err := time.Parse(time.RFC3339Nano, event.ReceivedAt)
	if err != nil {
		return nil, fmt.Errorf("invalid receivedAt timestamp: %w", err)
	}

	traceID := event.Payload.TraceID
	category := Classify(normalizedMsg)

	model := domain.LogModel{
		EventID:           event.EventID,
		ApplicationName:   appName,
		Level:             level,
		Category:          string(category),
		Message:           msg,
		NormalizedMessage: normalizedMsg,
		Timestamp:         timestamp,
		ReceivedAt:        receivedAt,
		TraceID:           traceID,
		Fingerprint:       GenerateFingerprint(appName, level, string(category), normalizedMsg),
	}
	return &model, nil
}

func CreateAlertEvent(model *domain.LogModel) sharedDomain.AlertEvent {
	return sharedDomain.AlertEvent{
		EventID:         model.EventID,
		ApplicationName: model.ApplicationName,
		Level:           model.Level,
		Category:        model.Category,
		Message:         model.Message,
		TraceID:         model.TraceID,
		Fingerprint:     model.Fingerprint,
		Timestamp:       model.Timestamp,
	}
}

func CreateProcessedLogEvent(model *domain.LogModel) sharedDomain.ProcessedLogEvent {
	return sharedDomain.ProcessedLogEvent{
		EventID:           model.EventID,
		ApplicationName:   model.ApplicationName,
		Level:             model.Level,
		Category:          model.Category,
		Message:           model.Message,
		NormalizedMessage: model.NormalizedMessage,
		Timestamp:         model.Timestamp,
		ReceivedAt:        model.ReceivedAt,
		TraceID:           model.TraceID,
		Fingerprint:       model.Fingerprint,
	}
}

func (p *ProcessingService) ProcessLog(ctx context.Context, events []sharedDomain.RawLogEvent) error {
	if len(events) == 0 {
		return nil
	}

	models := make([]*domain.LogModel, 0, len(events))

	for _, event := range events {
		model, err := CreateLogModel(event)
		if err != nil {
			log.Printf(
				"[WARNING] Skipping invalid log event. event_id=%s trace_id=%s err=%v",
				event.EventID,
				event.Payload.TraceID,
				err,
			)
			continue
		}
		if model.Level == "ERROR" || model.Level == "CRITICAL" {
			alertEvent := CreateAlertEvent(model)
			if err := p.Producer.ProduceAlert(ctx, alertEvent); err != nil {
				return fmt.Errorf("produce alert event_id=%s: %w", event.EventID, err)
			}
		}
		models = append(models, model)
	}

	if len(models) == 0 {
		log.Printf("[WARNING] No valid log records to persist after processing batch of %d events", len(events))
		return nil
	}

	if err := p.Repo.Save(ctx, models); err != nil {
		return err
	}

	processedEvents := make([]sharedDomain.ProcessedLogEvent, 0, len(models))
	for _, model := range models {
		processedEvents = append(processedEvents, CreateProcessedLogEvent(model))
	}

	if err := p.ProcessedLogProducer.ProduceProcessedLogs(ctx, processedEvents); err != nil {
		return fmt.Errorf("produce processed logs: %w", err)
	}

	return nil
}
