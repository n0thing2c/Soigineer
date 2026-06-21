package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMarshalSingleLog(t *testing.T) {
	logRecord := LogRequest{
		ApplicationName: "app-001",
		Level:           "INFO",
		Message:         "hello",
		Timestamp:       "2026-06-21T10:00:00Z",
		TraceID:         "trace-1",
	}

	body, err := marshalSingleLog(logRecord)
	if err != nil {
		t.Fatalf("marshalSingleLog() error = %v", err)
	}

	var decoded LogRequest
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded != logRecord {
		t.Fatalf("decoded = %+v, want %+v", decoded, logRecord)
	}
}

func TestGeneratorPrefixesTraceIDWithRunID(t *testing.T) {
	generator := NewGenerator(42, "bench-test")
	logRecord := generator.NextLog("app-001")

	if !strings.HasPrefix(logRecord.TraceID, "bench-test-") {
		t.Fatalf("TraceID = %q, want prefix bench-test-", logRecord.TraceID)
	}
}

func TestMarshalBatchLogs(t *testing.T) {
	logs := []LogRequest{
		{ApplicationName: "app-001", Level: "INFO", Message: "first", Timestamp: "2026-06-21T10:00:00Z", TraceID: "trace-1"},
		{ApplicationName: "app-002", Level: "WARN", Message: "second", Timestamp: "2026-06-21T10:00:01Z", TraceID: "trace-2"},
	}

	body, err := marshalBatchLogs(logs)
	if err != nil {
		t.Fatalf("marshalBatchLogs() error = %v", err)
	}

	var decoded BatchLogRequest
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(decoded.Logs) != len(logs) {
		t.Fatalf("len(decoded.Logs) = %d, want %d", len(decoded.Logs), len(logs))
	}
	for idx := range logs {
		if decoded.Logs[idx] != logs[idx] {
			t.Fatalf("decoded.Logs[%d] = %+v, want %+v", idx, decoded.Logs[idx], logs[idx])
		}
	}
}
