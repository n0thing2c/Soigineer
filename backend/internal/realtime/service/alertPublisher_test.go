package service

import (
	"context"
	"testing"
)

func TestAlertPublisherPublishesToHubAlertQueue(t *testing.T) {
	hub := NewHub(AllowAllAuthorizer{})
	publisher := NewAlertPublisher(hub)
	alert := sampleAlert()

	if err := publisher.Publish(context.Background(), alert); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	select {
	case got := <-hub.alertQueue:
		if got.EventID != alert.EventID {
			t.Fatalf("queued alert EventID = %q, want %q", got.EventID, alert.EventID)
		}
	default:
		t.Fatal("alert was not queued")
	}
}
