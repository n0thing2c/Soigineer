package service

import (
	"context"
	"encoding/json"
	"testing"

	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
)

type fixedAuthorizer struct {
	allowLog   bool
	allowAlert bool
}

func (a fixedAuthorizer) AuthorizeLog(Principal, sharedDomain.ProcessedLogEvent) bool {
	return a.allowLog
}

func (a fixedAuthorizer) AuthorizeAlert(Principal, sharedDomain.AlertEvent) bool {
	return a.allowAlert
}

func newTestClient(stream StreamType, sub Subscription, buffer int) *Client {
	return &Client{
		send:         make(chan []byte, buffer),
		stream:       stream,
		principal:    Principal{UserID: "user-1", Role: "engineer"},
		subscription: sub,
	}
}

func TestHubBroadcastLogFiltersClients(t *testing.T) {
	hub := NewHub(fixedAuthorizer{allowLog: true, allowAlert: true})

	matching := newTestClient(StreamLogs, Subscription{
		Applications: map[string]bool{"payment": true},
		Levels:       map[string]bool{"ERROR": true},
	}, 1)
	wrongApp := newTestClient(StreamLogs, Subscription{
		Applications: map[string]bool{"billing": true},
	}, 1)
	alertClient := newTestClient(StreamAlerts, Subscription{}, 1)

	hub.logClients[matching] = struct{}{}
	hub.logClients[wrongApp] = struct{}{}
	hub.alertClients[alertClient] = struct{}{}

	event := sampleProcessedLog()
	hub.broadcastLog(event)

	payload := mustReceive(t, matching.send)
	var got sharedDomain.ProcessedLogEvent
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal broadcast log: %v", err)
	}
	if got.EventID != event.EventID {
		t.Fatalf("broadcast EventID = %q, want %q", got.EventID, event.EventID)
	}

	assertNoMessage(t, wrongApp.send)
	assertNoMessage(t, alertClient.send)
}

func TestHubBroadcastLogRespectsAuthorizer(t *testing.T) {
	hub := NewHub(fixedAuthorizer{allowLog: false, allowAlert: true})
	client := newTestClient(StreamLogs, Subscription{}, 1)
	hub.logClients[client] = struct{}{}

	hub.broadcastLog(sampleProcessedLog())

	assertNoMessage(t, client.send)
}

func TestHubBroadcastAlertUsesAlertClients(t *testing.T) {
	hub := NewHub(fixedAuthorizer{allowLog: true, allowAlert: true})

	alertClient := newTestClient(StreamAlerts, Subscription{
		Applications: map[string]bool{"payment": true},
		Levels:       map[string]bool{"ERROR": true},
	}, 1)
	logClient := newTestClient(StreamLogs, Subscription{}, 1)

	hub.alertClients[alertClient] = struct{}{}
	hub.logClients[logClient] = struct{}{}

	event := sampleAlert()
	hub.broadcastAlert(event)

	payload := mustReceive(t, alertClient.send)
	var got sharedDomain.AlertEvent
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal broadcast alert: %v", err)
	}
	if got.EventID != event.EventID {
		t.Fatalf("broadcast EventID = %q, want %q", got.EventID, event.EventID)
	}

	assertNoMessage(t, logClient.send)
}

func TestHubRemovesSlowClient(t *testing.T) {
	hub := NewHub(AllowAllAuthorizer{})
	client := newTestClient(StreamLogs, Subscription{}, 1)
	client.send <- []byte("queued")
	hub.logClients[client] = struct{}{}

	hub.broadcastLog(sampleProcessedLog())

	if _, ok := hub.logClients[client]; ok {
		t.Fatal("slow client still registered")
	}
}

func TestHubPublishReturnsCanceledAfterShutdown(t *testing.T) {
	hub := NewHub(AllowAllAuthorizer{})
	close(hub.done)

	if err := hub.PublishLog(context.Background(), sampleProcessedLog()); err == nil {
		t.Fatal("PublishLog() error = nil, want canceled")
	}
	if err := hub.PublishAlert(context.Background(), sampleAlert()); err == nil {
		t.Fatal("PublishAlert() error = nil, want canceled")
	}
}

func mustReceive(t *testing.T, ch <-chan []byte) []byte {
	t.Helper()
	select {
	case payload := <-ch:
		return payload
	default:
		t.Fatal("expected message, got none")
		return nil
	}
}

func assertNoMessage(t *testing.T, ch <-chan []byte) {
	t.Helper()
	select {
	case payload := <-ch:
		t.Fatalf("unexpected message: %s", string(payload))
	default:
	}
}
