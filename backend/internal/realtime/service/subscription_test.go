package service

import (
	"testing"
	"time"

	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
)

func sampleProcessedLog() sharedDomain.ProcessedLogEvent {
	return sharedDomain.ProcessedLogEvent{
		EventID:         "event-1",
		ApplicationName: "payment",
		Level:           "ERROR",
		Message:         "database timeout",
		Timestamp:       time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC),
		ReceivedAt:      time.Date(2026, 6, 27, 10, 0, 1, 0, time.UTC),
		TraceID:         "trace-1",
		Fingerprint:     "fingerprint-1",
	}
}

func sampleAlert() sharedDomain.AlertEvent {
	return sharedDomain.AlertEvent{
		EventID:         "event-1",
		ApplicationName: "payment",
		Level:           "ERROR",
		Message:         "database timeout",
		Fingerprint:     "fingerprint-1",
		TraceID:         "trace-1",
		Timestamp:       time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC),
	}
}

func TestSubscriptionMatchLog(t *testing.T) {
	log := sampleProcessedLog()

	tests := []struct {
		name string
		sub  Subscription
		want bool
	}{
		{
			name: "empty subscription matches all",
			sub:  Subscription{},
			want: true,
		},
		{
			name: "matching application and level",
			sub: Subscription{
				Applications: map[string]bool{"payment": true},
				Levels:       map[string]bool{"ERROR": true},
			},
			want: true,
		},
		{
			name: "application mismatch",
			sub: Subscription{
				Applications: map[string]bool{"billing": true},
				Levels:       map[string]bool{"ERROR": true},
			},
			want: false,
		},
		{
			name: "level mismatch",
			sub: Subscription{
				Applications: map[string]bool{"payment": true},
				Levels:       map[string]bool{"INFO": true},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sub.MatchLog(log); got != tt.want {
				t.Fatalf("MatchLog() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubscriptionMatchAlert(t *testing.T) {
	alert := sampleAlert()

	tests := []struct {
		name string
		sub  Subscription
		want bool
	}{
		{
			name: "empty subscription matches all",
			sub:  Subscription{},
			want: true,
		},
		{
			name: "matching application and level",
			sub: Subscription{
				Applications: map[string]bool{"payment": true},
				Levels:       map[string]bool{"ERROR": true},
			},
			want: true,
		},
		{
			name: "application mismatch",
			sub: Subscription{
				Applications: map[string]bool{"billing": true},
				Levels:       map[string]bool{"ERROR": true},
			},
			want: false,
		},
		{
			name: "level mismatch",
			sub: Subscription{
				Applications: map[string]bool{"payment": true},
				Levels:       map[string]bool{"INFO": true},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sub.MatchAlert(alert); got != tt.want {
				t.Fatalf("MatchAlert() = %v, want %v", got, tt.want)
			}
		})
	}
}
