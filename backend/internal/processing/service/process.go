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

type ProcessingService struct {
	Repo     LogRepository
	Producer AlertProducer
}

func NewProcessingService(r LogRepository, p AlertProducer) *ProcessingService {
	return &ProcessingService{
		Repo:     r,
		Producer: p,
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
		Message:           msg,
		NormalizedMessage: normalizedMsg,
		Timestamp:         timestamp,
		ReceivedAt:        receivedAt,
		TraceID:           traceID,
		Fingerprint:       GenerateFingerprint(appName, level, string(category), normalizedMsg),
	}
	return &model, nil
}

func (p *ProcessingService) ProcessLog(ctx context.Context, events []sharedDomain.RawLogEvent) error {
	if len(events) == 0 {
		return nil
	}

	models := make([]*domain.LogModel, 0, len(events))

	for _, event := range events {
		level := event.Payload.Level
		if level == "ERROR" || level == "CRITICAL" {
			alertEvent := sharedDomain.ToAlertEvent(event)
			if err := p.Producer.ProduceAlert(ctx, alertEvent); err != nil {
				log.Printf("[WARNING] Failed to produce alert for TraceID %s: %v\n", event.Payload.TraceID, err)
			}
		}

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
		models = append(models, model)
	}

	if len(models) == 0 {
		log.Printf("[WARNING] No valid log records to persist after processing batch of %d events", len(events))
		return nil
	}

	return p.Repo.Save(ctx, models)
}
