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

type AlertingService struct {
	Deduplicator AlertDeduplicator
	Notifiers    []ExternalNotifier
	Publisher    RealtimePublisher
}

func NewAlertingService(d AlertDeduplicator, n []ExternalNotifier, p RealtimePublisher) *AlertingService {
	return &AlertingService{
		Deduplicator: d,
		Notifiers:    n,
		Publisher:    p,
	}
}

func (s *AlertingService) Alert(ctx context.Context, alert sharedDomain.AlertEvent) error {
	isAlert, err := s.Deduplicator.ShouldDispatch(ctx, alert)
	if err != nil {
		return fmt.Errorf("deduplicate alert: %w", err)
	}

	if !isAlert {
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
