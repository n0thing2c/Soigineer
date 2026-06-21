package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunnerSingleModeApproximateRate(t *testing.T) {
	var requestCount atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	cfg := Config{
		BaseURL:        server.URL,
		ServerCount:    1,
		LogsPerSec:     20,
		Mode:           ModeSingle,
		BatchSize:      10,
		SingleRatio:    0.5,
		Duration:       1200 * time.Millisecond,
		Timeout:        2 * time.Second,
		ProgressEvery:  time.Hour,
		WorkerCount:    4,
		DispatchBuffer: 128,
		Seed:           42,
	}

	runner := NewRunner(cfg, io.Discard)
	result, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := requestCount.Load()
	if got < 20 || got > 28 {
		t.Fatalf("request count = %d, want roughly between 20 and 28", got)
	}
	if result.Snapshot.SentLogs != uint64(got) {
		t.Fatalf("SentLogs = %d, want %d", result.Snapshot.SentLogs, got)
	}
}

func decodeJSONBody(r *http.Request, target any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

func TestRunnerBatchModeApproximateRate(t *testing.T) {
	var totalLogs atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload BatchLogRequest
		if err := decodeJSONBody(r, &payload); err != nil {
			t.Fatalf("decodeJSONBody() error = %v", err)
		}
		totalLogs.Add(int64(len(payload.Logs)))
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	cfg := Config{
		BaseURL:        server.URL,
		ServerCount:    1,
		LogsPerSec:     25,
		Mode:           ModeBatch,
		BatchSize:      5,
		Duration:       1100 * time.Millisecond,
		Timeout:        2 * time.Second,
		ProgressEvery:  time.Hour,
		WorkerCount:    4,
		DispatchBuffer: 128,
		Seed:           99,
	}

	runner := NewRunner(cfg, io.Discard)
	result, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := totalLogs.Load()
	if got < 22 || got > 30 {
		t.Fatalf("batch logs = %d, want roughly between 22 and 30", got)
	}
	if result.Snapshot.SentLogs != uint64(got) {
		t.Fatalf("SentLogs = %d, want %d", result.Snapshot.SentLogs, got)
	}
}

func TestRunnerFixedDurationSendsExactTargetWithoutDropping(t *testing.T) {
	var totalLogs atomic.Int64
	var requestCount atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload BatchLogRequest
		if err := decodeJSONBody(r, &payload); err != nil {
			t.Fatalf("decodeJSONBody() error = %v", err)
		}
		requestCount.Add(1)
		totalLogs.Add(int64(len(payload.Logs)))
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	cfg := Config{
		BaseURL:        server.URL,
		ServerCount:    1,
		LogsPerSec:     50,
		Mode:           ModeBatch,
		BatchSize:      10,
		Duration:       time.Second,
		Timeout:        2 * time.Second,
		ProgressEvery:  time.Hour,
		WorkerCount:    4,
		DispatchBuffer: 128,
		Seed:           101,
	}

	runner := NewRunner(cfg, io.Discard)
	result, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if totalLogs.Load() != 50 {
		t.Fatalf("total logs = %d, want 50", totalLogs.Load())
	}
	if requestCount.Load() != 5 {
		t.Fatalf("request count = %d, want 5 full batches", requestCount.Load())
	}
	if result.Snapshot.PlannedLogs != 50 {
		t.Fatalf("PlannedLogs = %d, want 50", result.Snapshot.PlannedLogs)
	}
	if result.Snapshot.SentLogs != 50 {
		t.Fatalf("SentLogs = %d, want 50", result.Snapshot.SentLogs)
	}
	if result.Snapshot.DroppedLogs != 0 {
		t.Fatalf("DroppedLogs = %d, want 0", result.Snapshot.DroppedLogs)
	}
}

func TestRunnerMixedModeDistribution(t *testing.T) {
	var singleRequests atomic.Int64
	var batchRequests atomic.Int64
	var batchLogs atomic.Int64

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/ingest/logs", func(w http.ResponseWriter, r *http.Request) {
		singleRequests.Add(1)
		w.WriteHeader(http.StatusAccepted)
	})
	mux.HandleFunc("/v1/ingest/logs/batch", func(w http.ResponseWriter, r *http.Request) {
		var payload BatchLogRequest
		if err := decodeJSONBody(r, &payload); err != nil {
			t.Fatalf("decodeJSONBody() error = %v", err)
		}
		batchRequests.Add(1)
		batchLogs.Add(int64(len(payload.Logs)))
		w.WriteHeader(http.StatusAccepted)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	cfg := Config{
		BaseURL:        server.URL,
		ServerCount:    1,
		LogsPerSec:     40,
		Mode:           ModeMixed,
		BatchSize:      4,
		SingleRatio:    0.35,
		Duration:       1200 * time.Millisecond,
		Timeout:        2 * time.Second,
		ProgressEvery:  time.Hour,
		WorkerCount:    4,
		DispatchBuffer: 128,
		Seed:           123,
	}

	runner := NewRunner(cfg, io.Discard)
	result, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	totalSingles := singleRequests.Load()
	totalBatchLogs := batchLogs.Load()
	totalLogs := totalSingles + totalBatchLogs
	if totalLogs == 0 {
		t.Fatal("expected logs to be sent")
	}

	singleShare := float64(totalSingles) / float64(totalLogs)
	if singleRequests.Load() == 0 || batchRequests.Load() == 0 {
		t.Fatalf("expected both single and batch traffic, got single=%d batch=%d", totalSingles, batchRequests.Load())
	}
	if singleShare < 0.20 || singleShare > 0.50 {
		t.Fatalf("single share = %.2f, want roughly between 0.20 and 0.50", singleShare)
	}
	if result.Snapshot.SentLogs != uint64(totalLogs) {
		t.Fatalf("SentLogs = %d, want %d", result.Snapshot.SentLogs, totalLogs)
	}
}

func TestRunnerStopsOnContextCancel(t *testing.T) {
	var mu sync.Mutex
	requests := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	cfg := Config{
		BaseURL:        server.URL,
		ServerCount:    1,
		LogsPerSec:     50,
		Mode:           ModeSingle,
		BatchSize:      10,
		RunForever:     true,
		Timeout:        2 * time.Second,
		ProgressEvery:  time.Hour,
		WorkerCount:    4,
		DispatchBuffer: 128,
		Seed:           7,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	runner := NewRunner(cfg, io.Discard)
	done := make(chan error, 1)
	go func() {
		_, err := runner.Run(ctx)
		done <- err
	}()

	select {
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not stop after context cancellation")
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	}
}
