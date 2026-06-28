package service

import (
	"context"
	"errors"
	"testing"
	"time"

	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
)

type fakeDeduplicator struct {
	shouldDispatch bool
	err            error
}

func (f fakeDeduplicator) ShouldDispatch(ctx context.Context, alert sharedDomain.AlertEvent) (bool, error) {
	return f.shouldDispatch, f.err
}

type fakeNotifier struct {
	calls int
	err   error
}

func (f *fakeNotifier) Notify(ctx context.Context, alert sharedDomain.AlertEvent) error {
	f.calls++
	return f.err
}

type fakePublisher struct {
	calls int
	err   error
}

func (f *fakePublisher) Publish(ctx context.Context, alert sharedDomain.AlertEvent) error {
	f.calls++
	return f.err
}

type fakeIncidentRecorder struct {
	calls      int
	dispatched bool
	err        error
}

func (f *fakeIncidentRecorder) Record(ctx context.Context, alert sharedDomain.AlertEvent, dispatched bool) error {
	f.calls++
	f.dispatched = dispatched
	return f.err
}

func sampleAlert() sharedDomain.AlertEvent {
	return sharedDomain.AlertEvent{
		EventID:         "event-1",
		ApplicationName: "payment-service",
		Level:           "ERROR",
		Category:        "DATABASE_ERROR",
		Message:         "database timeout",
		Fingerprint:     "fingerprint-1",
		TraceID:         "trace-1",
		Timestamp:       time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC),
	}
}

func TestAlertRecordsAndDispatchesFirstAlert(t *testing.T) {
	notifier := &fakeNotifier{}
	publisher := &fakePublisher{}
	incidents := &fakeIncidentRecorder{}
	service := NewAlertingService(
		fakeDeduplicator{shouldDispatch: true},
		[]ExternalNotifier{notifier},
		publisher,
		incidents,
	)

	if err := service.Alert(context.Background(), sampleAlert()); err != nil {
		t.Fatalf("Alert() error = %v", err)
	}

	if incidents.calls != 1 || !incidents.dispatched {
		t.Fatalf("incident calls/dispatched = %d/%v, want 1/true", incidents.calls, incidents.dispatched)
	}
	if publisher.calls != 1 || notifier.calls != 1 {
		t.Fatalf("publisher/notifier calls = %d/%d, want 1/1", publisher.calls, notifier.calls)
	}
}

func TestAlertRecordsSuppressedDuplicateWithoutDispatching(t *testing.T) {
	notifier := &fakeNotifier{}
	publisher := &fakePublisher{}
	incidents := &fakeIncidentRecorder{}
	service := NewAlertingService(
		fakeDeduplicator{shouldDispatch: false},
		[]ExternalNotifier{notifier},
		publisher,
		incidents,
	)

	if err := service.Alert(context.Background(), sampleAlert()); err != nil {
		t.Fatalf("Alert() error = %v", err)
	}

	if incidents.calls != 1 || incidents.dispatched {
		t.Fatalf("incident calls/dispatched = %d/%v, want 1/false", incidents.calls, incidents.dispatched)
	}
	if publisher.calls != 0 || notifier.calls != 0 {
		t.Fatalf("publisher/notifier calls = %d/%d, want 0/0", publisher.calls, notifier.calls)
	}
}

func TestAlertPropagatesIncidentRecordError(t *testing.T) {
	wantErr := errors.New("postgres unavailable")
	service := NewAlertingService(
		fakeDeduplicator{shouldDispatch: true},
		nil,
		&fakePublisher{},
		&fakeIncidentRecorder{err: wantErr},
	)

	if err := service.Alert(context.Background(), sampleAlert()); !errors.Is(err, wantErr) {
		t.Fatalf("Alert() error = %v, want %v", err, wantErr)
	}
}
