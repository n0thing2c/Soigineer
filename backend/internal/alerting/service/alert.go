package service

import (
	"context"
	"fmt"
	"log"
	"time"

	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
)

type AlertDeduplicator interface {
	ShouldDispatch(ctx context.Context, alert sharedDomain.AlertEvent, window time.Duration) (bool, error)
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

type AlertRule struct {
	Enabled         bool
	DedupWindow     time.Duration
	TelegramEnabled bool
}

type AlertRuleResolver interface {
	Resolve(ctx context.Context, alert sharedDomain.AlertEvent) (AlertRule, error)
}

type AlertingService struct {
	Deduplicator AlertDeduplicator
	Notifiers    []ExternalNotifier
	Publisher    RealtimePublisher
	Incidents    IncidentRecorder
	Rules        AlertRuleResolver
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

func NewAlertingServiceWithRules(
	d AlertDeduplicator,
	n []ExternalNotifier,
	p RealtimePublisher,
	i IncidentRecorder,
	r AlertRuleResolver,
) *AlertingService {
	service := NewAlertingService(d, n, p, i)
	service.Rules = r
	return service
}

func (s *AlertingService) Alert(ctx context.Context, alert sharedDomain.AlertEvent) error {
	rule := AlertRule{
		Enabled:         true,
		TelegramEnabled: true,
	}
	if s.Rules != nil {
		resolved, err := s.Rules.Resolve(ctx, alert)
		if err != nil {
			return fmt.Errorf("resolve alert rule: %w", err)
		}
		rule = resolved
	}

	if !rule.Enabled {
		if s.Incidents != nil {
			if err := s.Incidents.Record(ctx, alert, false); err != nil {
				return fmt.Errorf("record incident: %w", err)
			}
		}
		return nil
	}

	shouldDispatch, err := s.Deduplicator.ShouldDispatch(ctx, alert, rule.DedupWindow)
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

	if s.Publisher != nil {
		if err := s.Publisher.Publish(ctx, alert); err != nil {
			log.Printf("publish websocket: %v", err)
		}
	}

	if !rule.TelegramEnabled {
		return nil
	}

	for _, notifier := range s.Notifiers {
		if err := notifier.Notify(ctx, alert); err != nil {
			log.Printf("send notification: %v", err)
		}
	}

	return nil
}
