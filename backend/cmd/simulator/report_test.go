package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultReportPath(t *testing.T) {
	if got, want := defaultReportPath("bench-test"), filepath.Join("benchmark-reports", "bench-test.md"); got != want {
		t.Fatalf("defaultReportPath() = %q, want %q", got, want)
	}
}

func TestReportMarkdownIncludesSuccessAndClampedGaps(t *testing.T) {
	report := ReportData{
		Result: RunResult{
			RunID:      "bench-test",
			StartedAt:  time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC),
			FinishedAt: time.Date(2026, 6, 21, 10, 0, 1, 0, time.UTC),
			Snapshot: Snapshot{
				PlannedLogs: 10,
				SentLogs:    5,
				FailedLogs:  1,
				DroppedLogs: 2,
				SuccessReqs: 3,
				FailedReqs:  1,
				DroppedReqs: 1,
				Duration:    time.Second,
			},
			TopicBefore: TopicOffsets{Total: 10},
			TopicAfter:  TopicOffsets{Total: 20},
			TopicDelta:  10,
		},
		Config: Config{
			Mode:           ModeBatch,
			ServerCount:    1,
			LogsPerSec:     10,
			BatchSize:      5,
			ReportWait:     time.Second,
			KafkaTopic:     "raw-logs",
			KafkaBrokers:   []string{"localhost:19092"},
			ClickHouseDB:   "logs_db",
			ClickHouseUser: "admin",
		},
		ClickHouseCount:   50,
		ReportGeneratedAt: time.Date(2026, 6, 21, 10, 0, 2, 0, time.UTC),
	}

	markdown := report.Markdown()
	for _, want := range []string{
		"# Benchmark Report",
		"| Gateway | Accepted logs | 5 |",
		"| Queue | Observed logs in `raw-logs` | 10 |",
		"| ClickHouse | Inserted logs | 50 |",
		"Gateway accepted but not observed in queue delta: `0` logs.",
		"Queue to ClickHouse gap after `1s` settle time: `0` logs.",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("Markdown() missing %q:\n%s", want, markdown)
		}
	}
}

func TestReportMarkdownIncludesErrors(t *testing.T) {
	report := ReportData{
		Result: RunResult{
			RunID:        "bench-test",
			StartedAt:    time.Now().UTC(),
			FinishedAt:   time.Now().UTC(),
			Snapshot:     Snapshot{Duration: time.Second},
			TopicBefore:  TopicOffsets{CaptureError: errors.New("before failed")},
			TopicAfter:   TopicOffsets{CaptureError: errors.New("after failed")},
			BenchmarkErr: errors.New("benchmark failed"),
		},
		Config:            Config{KafkaTopic: "raw-logs"},
		ClickHouseError:   errors.New("clickhouse failed"),
		ReportGeneratedAt: time.Now().UTC(),
	}

	markdown := report.Markdown()
	for _, want := range []string{
		"| ClickHouse | Inserted logs | query failed |",
		"ClickHouse verification failed: `clickhouse failed`.",
		"Queue observation had errors.",
		"Benchmark ended with runtime error: `benchmark failed`.",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("Markdown() missing %q:\n%s", want, markdown)
		}
	}
}

func TestWriteMarkdownReportUsesExplicitPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "report.md")
	cfg := Config{ReportFile: path, KafkaTopic: "raw-logs"}
	report := ReportData{
		Result:            RunResult{RunID: "bench-test", StartedAt: time.Now(), FinishedAt: time.Now(), Snapshot: Snapshot{Duration: time.Second}},
		Config:            cfg,
		ReportGeneratedAt: time.Now().UTC(),
	}

	got, err := writeMarkdownReport(cfg, report)
	if err != nil {
		t.Fatalf("writeMarkdownReport() error = %v", err)
	}
	if got != path {
		t.Fatalf("path = %q, want %q", got, path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(content), "# Benchmark Report") {
		t.Fatalf("report content = %q", string(content))
	}
}

func TestWriteMarkdownReportReturnsWriteError(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{ReportFile: dir, KafkaTopic: "raw-logs"}
	report := ReportData{
		Result:            RunResult{RunID: "bench-test", StartedAt: time.Now(), FinishedAt: time.Now(), Snapshot: Snapshot{Duration: time.Second}},
		Config:            cfg,
		ReportGeneratedAt: time.Now().UTC(),
	}

	if _, err := writeMarkdownReport(cfg, report); err == nil {
		t.Fatal("expected write error for directory path, got nil")
	}
}
