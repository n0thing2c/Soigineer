package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func senderTestConfig(baseURL string) Config {
	return Config{
		BaseURL:     baseURL,
		Timeout:     time.Second,
		WorkerCount: 2,
	}
}

func TestSenderSendsSingleLog(t *testing.T) {
	want := LogRequest{
		ApplicationName: "app-001",
		Level:           "INFO",
		Message:         "hello",
		Timestamp:       "2026-06-21T10:00:00Z",
		TraceID:         "trace-1",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/ingest/logs" {
			t.Fatalf("path = %q, want /v1/ingest/logs", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}
		var got LogRequest
		if err := decodeJSONBody(r, &got); err != nil {
			t.Fatalf("decodeJSONBody() error = %v", err)
		}
		if got != want {
			t.Fatalf("body = %+v, want %+v", got, want)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	result := NewSender(senderTestConfig(server.URL)).Send(context.Background(), Job{
		Kind: jobSingle,
		Logs: []LogRequest{want},
	})
	if result.Err != nil {
		t.Fatalf("Send() error = %v", result.Err)
	}
	if result.StatusCode != http.StatusAccepted || result.Logs != 1 || result.Kind != jobSingle {
		t.Fatalf("result = %+v, want accepted single log", result)
	}
}

func TestSenderSendsBatchLogs(t *testing.T) {
	logs := []LogRequest{
		{ApplicationName: "app-001", Level: "INFO", Message: "first", Timestamp: "2026-06-21T10:00:00Z", TraceID: "trace-1"},
		{ApplicationName: "app-002", Level: "WARN", Message: "second", Timestamp: "2026-06-21T10:00:01Z", TraceID: "trace-2"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/ingest/logs/batch" {
			t.Fatalf("path = %q, want /v1/ingest/logs/batch", r.URL.Path)
		}
		var got BatchLogRequest
		if err := decodeJSONBody(r, &got); err != nil {
			t.Fatalf("decodeJSONBody() error = %v", err)
		}
		if len(got.Logs) != len(logs) {
			t.Fatalf("len(logs) = %d, want %d", len(got.Logs), len(logs))
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	result := NewSender(senderTestConfig(server.URL)).Send(context.Background(), Job{Kind: jobBatch, Logs: logs})
	if result.Err != nil {
		t.Fatalf("Send() error = %v", result.Err)
	}
	if result.StatusCode != http.StatusCreated || result.Logs != len(logs) {
		t.Fatalf("result = %+v, want created batch", result)
	}
}

func TestSenderReturnsUnexpectedStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	result := NewSender(senderTestConfig(server.URL)).Send(context.Background(), Job{
		Kind: jobSingle,
		Logs: []LogRequest{{ApplicationName: "app-001"}},
	})
	if result.Err == nil || !strings.Contains(result.Err.Error(), "unexpected status code 503") {
		t.Fatalf("expected status error, got %+v", result)
	}
	if result.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("StatusCode = %d, want 503", result.StatusCode)
	}
}

func TestSenderReturnsContextError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request should not be sent with canceled context")
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := NewSender(senderTestConfig(server.URL)).Send(ctx, Job{
		Kind: jobSingle,
		Logs: []LogRequest{{ApplicationName: "app-001"}},
	})
	if result.Err == nil {
		t.Fatal("expected context error, got nil")
	}
}

func TestSenderReturnsRequestBuildError(t *testing.T) {
	result := NewSender(senderTestConfig("http://[::1")).Send(context.Background(), Job{
		Kind: jobBatch,
		Logs: []LogRequest{{ApplicationName: "app-001"}},
	})
	if result.Err == nil {
		t.Fatal("expected request build error, got nil")
	}
}

func TestRunnerSendWithRetrySucceedsAfterTransientFailure(t *testing.T) {
	var attempts atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	runner := NewRunner(Config{
		BaseURL:      server.URL,
		Timeout:      time.Second,
		WorkerCount:  1,
		RetryCount:   2,
		RetryBackoff: time.Millisecond,
	}, nil)

	result := runner.sendWithRetry(context.Background(), Job{
		Kind: jobSingle,
		Logs: []LogRequest{{ApplicationName: "app-001"}},
	})
	if result.Err != nil {
		t.Fatalf("sendWithRetry() error = %v", result.Err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d, want 2", attempts.Load())
	}
}

func TestRunnerSendWithRetryExhaustsRetries(t *testing.T) {
	var attempts atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	runner := NewRunner(Config{
		BaseURL:      server.URL,
		Timeout:      time.Second,
		WorkerCount:  1,
		RetryCount:   2,
		RetryBackoff: time.Millisecond,
	}, nil)

	result := runner.sendWithRetry(context.Background(), Job{
		Kind: jobSingle,
		Logs: []LogRequest{{ApplicationName: "app-001"}},
	})
	if result.Err == nil {
		t.Fatal("expected retry exhaustion error, got nil")
	}
	if attempts.Load() != 3 {
		t.Fatalf("attempts = %d, want 3", attempts.Load())
	}
}

func TestRunnerSendWithRetryStopsWhenContextCanceled(t *testing.T) {
	var attempts atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	runner := NewRunner(Config{
		BaseURL:      server.URL,
		Timeout:      time.Second,
		WorkerCount:  1,
		RetryCount:   3,
		RetryBackoff: 50 * time.Millisecond,
	}, nil)

	cancel()
	result := runner.sendWithRetry(ctx, Job{
		Kind: jobSingle,
		Logs: []LogRequest{{ApplicationName: "app-001"}},
	})
	if result.Err == nil {
		t.Fatal("expected canceled context error")
	}
	if attempts.Load() != 0 {
		t.Fatalf("server attempts = %d, want 0 for pre-canceled context", attempts.Load())
	}
}

func TestMarshalBatchLogsJSONShape(t *testing.T) {
	body, err := marshalBatchLogs([]LogRequest{{ApplicationName: "app-001"}})
	if err != nil {
		t.Fatalf("marshalBatchLogs() error = %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if _, ok := raw["logs"]; !ok {
		t.Fatalf("batch payload keys = %#v, want logs key", raw)
	}
}
