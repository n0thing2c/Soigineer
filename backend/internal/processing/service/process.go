package service

import (
	"context"
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

func CreateLogModel(event sharedDomain.RawLogEvent) *domain.LogModel {
	appName := event.Payload.ApplicationName
	level := event.Payload.Level
	msg, normalizedMsg := NormalizeMessage(event.Payload.Message)
	timestamp, _ := time.Parse(time.RFC3339, event.Payload.Timestamp)
	receivedAt, _ := time.Parse(time.RFC3339, event.ReceivedAt)
	traceId := event.Payload.TraceID
	category := Classify(normalizedMsg)

	model := domain.LogModel{
		ApplicationName:   appName,
		Level:             level,
		Message:           msg,
		NormalizedMessage: normalizedMsg,
		Timestamp:         timestamp,
		ReceivedAt:        receivedAt,
		TraceID:           traceId,
		Fingerprint:       GenerateFingerprint(appName, level, string(category), normalizedMsg),
	}
	return &model
}

func (p *ProcessingService) ProcessLog(ctx context.Context, events []sharedDomain.RawLogEvent) error {
	if len(events) == 0 {
		return nil
	}

	models := make([]*domain.LogModel, 0, len(events))

	for _, event := range events {
		level := event.Payload.Level

		// 1. Fast-Path: Priority alert flow
		if level == "ERROR" || level == "CRITICAL" {
			alertEvent := sharedDomain.ToAlertEvent(event)
			if err := p.Producer.ProduceAlert(ctx, alertEvent); err != nil {
				log.Printf("[WARNING] Failed to produce alert for TraceID %s: %v\n", event.Payload.TraceID, err)
			}
		}
		model := CreateLogModel(event)
		models = append(models, model)
	}
	return p.Repo.Save(ctx, models)
}
