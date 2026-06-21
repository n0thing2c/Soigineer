package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseConfigValid(t *testing.T) {
	cfg, err := ParseConfig([]string{
		"--base-url=http://localhost:8080",
		"--server-count=12",
		"--logs-per-sec=40",
		"--mode=mixed",
		"--single-ratio=0.25",
		"--duration=15s",
		"--batch-size=20",
	})
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}

	if cfg.Mode != ModeMixed {
		t.Fatalf("Mode = %q, want %q", cfg.Mode, ModeMixed)
	}
	if cfg.ServerCount != 12 {
		t.Fatalf("ServerCount = %d, want 12", cfg.ServerCount)
	}
	if cfg.Duration != 15*time.Second {
		t.Fatalf("Duration = %s, want 15s", cfg.Duration)
	}
	if len(cfg.KafkaBrokers) != 1 || cfg.KafkaBrokers[0] != "localhost:19092" {
		t.Fatalf("KafkaBrokers = %#v, want localhost:19092", cfg.KafkaBrokers)
	}
}

func TestParseConfigDefaultsAndDerivedValues(t *testing.T) {
	cfg, err := ParseConfig([]string{"--duration=1s", "--server-count=6", "--batch-size=20"})
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}

	if cfg.BaseURL != "http://localhost:8080" {
		t.Fatalf("BaseURL = %q, want default", cfg.BaseURL)
	}
	if cfg.WorkerCount != 12 {
		t.Fatalf("WorkerCount = %d, want server-count*2", cfg.WorkerCount)
	}
	if cfg.DispatchBuffer != 1024 {
		t.Fatalf("DispatchBuffer = %d, want minimum 1024", cfg.DispatchBuffer)
	}
	if cfg.SingleEndpoint() != "http://localhost:8080/v1/ingest/logs" {
		t.Fatalf("SingleEndpoint() = %q", cfg.SingleEndpoint())
	}
}

func TestConfigEndpointsTrimTrailingSlash(t *testing.T) {
	cfg := Config{BaseURL: "http://localhost:8080/"}
	if got := cfg.SingleEndpoint(); got != "http://localhost:8080/v1/ingest/logs" {
		t.Fatalf("SingleEndpoint() = %q", got)
	}
	if got := cfg.BatchEndpoint(); got != "http://localhost:8080/v1/ingest/logs/batch" {
		t.Fatalf("BatchEndpoint() = %q", got)
	}
}

func TestParseConfigRejectsInvalidMode(t *testing.T) {
	_, err := ParseConfig([]string{"--mode=burst", "--run-forever"})
	if err == nil || !strings.Contains(err.Error(), "mode must be one of") {
		t.Fatalf("expected invalid mode error, got %v", err)
	}
}

func TestParseConfigRejectsInvalidBatchSize(t *testing.T) {
	_, err := ParseConfig([]string{"--batch-size=501", "--run-forever"})
	if err == nil || !strings.Contains(err.Error(), "batch-size must be between 1 and 500") {
		t.Fatalf("expected batch-size validation error, got %v", err)
	}
}

func TestParseConfigRejectsDurationConflict(t *testing.T) {
	_, err := ParseConfig([]string{"--duration=10s", "--run-forever"})
	if err == nil || !strings.Contains(err.Error(), "duration and run-forever cannot be used together") {
		t.Fatalf("expected duration conflict error, got %v", err)
	}
}

func TestParseConfigRejectsInvalidFlagValue(t *testing.T) {
	_, err := ParseConfig([]string{"--duration=not-a-duration"})
	if err == nil {
		t.Fatal("expected flag parse error, got nil")
	}
}

func TestConfigValidateRejectsInvalidValues(t *testing.T) {
	valid := Config{
		BaseURL:       "http://localhost:8080",
		ServerCount:   1,
		LogsPerSec:    1,
		Mode:          ModeSingle,
		BatchSize:     1,
		SingleRatio:   0.5,
		Duration:      time.Second,
		Timeout:       time.Second,
		RetryCount:    0,
		RetryBackoff:  0,
		ProgressEvery: time.Second,
		ReportWait:    0,
		KafkaBrokers:  []string{"localhost:19092"},
		KafkaTopic:    "raw-logs",
	}

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{"server count", func(c *Config) { c.ServerCount = 0 }, "server-count must be greater than 0"},
		{"logs per sec", func(c *Config) { c.LogsPerSec = 0 }, "logs-per-sec must be greater than 0"},
		{"batch size low", func(c *Config) { c.BatchSize = 0 }, "batch-size must be between 1 and 500"},
		{"batch size high", func(c *Config) { c.BatchSize = 501 }, "batch-size must be between 1 and 500"},
		{"mode", func(c *Config) { c.Mode = "burst" }, "mode must be one of"},
		{"single ratio low", func(c *Config) { c.Mode = ModeMixed; c.SingleRatio = -0.1 }, "single-ratio must be between 0 and 1"},
		{"single ratio high", func(c *Config) { c.Mode = ModeMixed; c.SingleRatio = 1.1 }, "single-ratio must be between 0 and 1"},
		{"duration", func(c *Config) { c.Duration = 0 }, "duration must be greater than 0 unless run-forever is enabled"},
		{"duration conflict", func(c *Config) { c.RunForever = true }, "duration and run-forever cannot be used together"},
		{"timeout", func(c *Config) { c.Timeout = 0 }, "timeout must be greater than 0"},
		{"retry count", func(c *Config) { c.RetryCount = -1 }, "retry-count must be greater than or equal to 0"},
		{"retry backoff", func(c *Config) { c.RetryBackoff = -1 }, "retry-backoff must be greater than or equal to 0"},
		{"progress every", func(c *Config) { c.ProgressEvery = 0 }, "progress-every must be greater than 0"},
		{"report wait", func(c *Config) { c.ReportWait = -1 }, "report-wait must be greater than or equal to 0"},
		{"brokers", func(c *Config) { c.KafkaBrokers = nil }, "kafka-brokers must contain at least one broker"},
		{"topic", func(c *Config) { c.KafkaTopic = " " }, "kafka-topic must not be empty"},
		{"base url", func(c *Config) { c.BaseURL = "://bad" }, "base-url must be a valid URL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			cfg.KafkaBrokers = append([]string(nil), valid.KafkaBrokers...)
			tt.mutate(&cfg)

			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestSplitAndTrim(t *testing.T) {
	got := splitAndTrim(" localhost:19092, ,redpanda:9092 ")
	if len(got) != 2 || got[0] != "localhost:19092" || got[1] != "redpanda:9092" {
		t.Fatalf("splitAndTrim() = %#v", got)
	}
}
