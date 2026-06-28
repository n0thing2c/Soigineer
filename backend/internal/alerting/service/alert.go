package service

import (
	"context"
	"fmt"
	"log"

	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
)

type AlertDeduplicator interface {
	ShouldDispatch(ctx context.Context, alert sharedDomain.AlertEvent) (bool, error)
}

type ExternalNotifier interface {
	Notify(ctx context.Context, alert sharedDomain.AlertEvent) error
}

type RealtimePublisher interface {
	Publish(ctx context.Context, alert sharedDomain.AlertEvent) error
}

type IncidentRecorder interface {
	Record(ctx context.Context, alert sharedDomain.AlertEvent, dispatched bool) error
}

type AlertingService struct {
	Deduplicator AlertDeduplicator
	Notifiers    []ExternalNotifier
	Publisher    RealtimePublisher
	Incidents    IncidentRecorder
}

func NewAlertingService(
	d AlertDeduplicator,
	n []ExternalNotifier,
	p RealtimePublisher,
	i IncidentRecorder,
) *AlertingService {
	return &AlertingService{
		Deduplicator: d,
		Notifiers:    n,
		Publisher:    p,
		Incidents:    i,
	}
}

func (s *AlertingService) Alert(ctx context.Context, alert sharedDomain.AlertEvent) error {
	shouldDispatch, err := s.Deduplicator.ShouldDispatch(ctx, alert)
	if err != nil {
		return fmt.Errorf("deduplicate alert: %w", err)
	}

	if s.Incidents != nil {
		if err := s.Incidents.Record(ctx, alert, shouldDispatch); err != nil {
			return fmt.Errorf("record incident: %w", err)
		}
	}

	if !shouldDispatch {
		return nil
	}

	if err := s.Publisher.Publish(ctx, alert); err != nil {
		log.Printf("publish websocket: %v", err)
	}

	for _, notifier := range s.Notifiers {
		if err := notifier.Notify(ctx, alert); err != nil {
			log.Printf("send notification: %v", err)
		}
	}

	return nil
}
