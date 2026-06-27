package service

import (
	"context"

	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
)

type AlertPublisher struct {
	hub *Hub
}

func NewAlertPublisher(hub *Hub) *AlertPublisher {
	return &AlertPublisher{hub: hub}
}

func (p *AlertPublisher) Publish(ctx context.Context, alert sharedDomain.AlertEvent) error {
	return p.hub.PublishAlert(ctx, alert)
}
