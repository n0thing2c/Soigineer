package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
)

func TestNewNotifierValidatesRequiredConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		chatID  string
		wantErr string
	}{
		{
			name:    "missing token",
			chatID:  "123",
			wantErr: "bot token",
		},
		{
			name:    "missing chat ID",
			token:   "test-token",
			wantErr: "chat ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewNotifier(tt.token, tt.chatID, time.Second)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("NewNotifier() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestNotifierNotifySendsAlert(t *testing.T) {
	var received sendMessageRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/bottest-token/sendMessage" {
			t.Errorf("path = %s, want /bottest-token/sendMessage", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	defer server.Close()

	notifier, err := NewNotifier("test-token", "-100123", time.Second)
	if err != nil {
		t.Fatalf("NewNotifier() error = %v", err)
	}
	notifier.baseURL = server.URL

	alert := sharedDomain.AlertEvent{
		ApplicationName: "payment-service",
		Level:           "CRITICAL",
		Message:         "database connection failed",
		TraceID:         "trace-123",
		EventID:         "123abcx",
		Timestamp:       time.Date(2026, 6, 24, 10, 30, 0, 0, time.UTC),
	}

	if err := notifier.Notify(context.Background(), alert); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}

	if received.ChatID != "-100123" {
		t.Errorf("chat ID = %q, want -100123", received.ChatID)
	}
	for _, want := range []string{
		"CRITICAL ALERT",
		"payment-service",
		"database connection failed",
		"trace-123",
		"123abcx",
		"2026-06-24T10:30:00Z",
	} {
		if !strings.Contains(received.Text, want) {
			t.Errorf("message %q does not contain %q", received.Text, want)
		}
	}
}

func TestNotifierNotifyReturnsTelegramError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"description":"Bad Request: chat not found"}`))
	}))
	defer server.Close()

	notifier, err := NewNotifier("test-token", "invalid-chat", time.Second)
	if err != nil {
		t.Fatalf("NewNotifier() error = %v", err)
	}
	notifier.baseURL = server.URL

	err = notifier.Notify(context.Background(), sharedDomain.AlertEvent{})
	if err == nil {
		t.Fatal("Notify() error = nil, want Telegram API error")
	}
	if !strings.Contains(err.Error(), "chat not found") {
		t.Fatalf("Notify() error = %v, want chat not found", err)
	}
	if strings.Contains(err.Error(), "test-token") {
		t.Fatalf("Notify() leaked bot token: %v", err)
	}
}
