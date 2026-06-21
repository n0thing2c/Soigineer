package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMetricsRecordsSuccessFailureAndDrops(t *testing.T) {
	start := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	metrics := NewMetrics(start)

	metrics.AddPlanned(5)
	metrics.Record(Result{Kind: jobSingle, Logs: 2, StatusCode: 202, Latency: 2 * time.Millisecond})
	metrics.Record(Result{Kind: jobBatch, Logs: 3, StatusCode: 503, Latency: 4 * time.Millisecond, Err: errors.New("unexpected status code 503")})
	metrics.Record(Result{Kind: jobSingle, Logs: 1, Latency: time.Millisecond, Err: errors.New("dial failed")})
	metrics.RecordDropped(Job{Kind: jobBatch, Logs: []LogRequest{{}, {}}})
	metrics.Finish(start.Add(time.Second))

	snapshot := metrics.Snapshot()
	if snapshot.PlannedLogs != 5 || snapshot.SentLogs != 2 || snapshot.FailedLogs != 4 || snapshot.DroppedLogs != 2 {
		t.Fatalf("snapshot log counts = %+v", snapshot)
	}
	if snapshot.SuccessReqs != 1 || snapshot.FailedReqs != 2 || snapshot.DroppedReqs != 1 {
		t.Fatalf("snapshot request counts = %+v", snapshot)
	}
	if snapshot.StatusCounts[202] != 1 || snapshot.StatusCounts[503] != 1 {
		t.Fatalf("StatusCounts = %#v", snapshot.StatusCounts)
	}
	if snapshot.NetworkErrors["dial failed"] != 1 {
		t.Fatalf("NetworkErrors = %#v", snapshot.NetworkErrors)
	}
	if snapshot.MinLatency != time.Millisecond || snapshot.MaxLatency != 4*time.Millisecond {
		t.Fatalf("latency min/max = %s/%s", snapshot.MinLatency, snapshot.MaxLatency)
	}
	if snapshot.AvgLatency != (7*time.Millisecond)/3 {
		t.Fatalf("AvgLatency = %s, want %s", snapshot.AvgLatency, (7*time.Millisecond)/3)
	}
	if snapshot.ErrorRate() != float64(2)/3 {
		t.Fatalf("ErrorRate = %f, want 2/3", snapshot.ErrorRate())
	}
}

func TestSnapshotErrorRateZeroWithoutRequests(t *testing.T) {
	if got := (Snapshot{}).ErrorRate(); got != 0 {
		t.Fatalf("ErrorRate = %f, want 0", got)
	}
}

func TestSnapshotSummaryLinesIncludeSortedMaps(t *testing.T) {
	snapshot := Snapshot{
		Duration:      time.Second,
		StatusCounts:  map[int]uint64{503: 2, 202: 1},
		NetworkErrors: map[string]uint64{"z": 1, "a": 2},
	}

	lines := snapshot.SummaryLines()
	joined := strings.Join(lines, "\n")
	for _, want := range []string{"status_counts=202=1,503=2", "network_errors=a=2,z=1"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("SummaryLines() = %q, want %q", joined, want)
		}
	}
}

func TestFormatMapHelpers(t *testing.T) {
	if got, want := formatIntMap(map[int]uint64{2: 20, 1: 10}), "1=10,2=20"; got != want {
		t.Fatalf("formatIntMap() = %q, want %q", got, want)
	}
	if got, want := formatStringMap(map[string]uint64{"b": 2, "a": 1}), "a=1,b=2"; got != want {
		t.Fatalf("formatStringMap() = %q, want %q", got, want)
	}
}

func TestPercentileAndAverageLatency(t *testing.T) {
	latencies := []time.Duration{time.Millisecond, 2 * time.Millisecond, 10 * time.Millisecond}
	if got := averageLatency(latencies); got != (13*time.Millisecond)/3 {
		t.Fatalf("averageLatency() = %s", got)
	}
	if got := percentileLatency(latencies, 0.95); got != 2*time.Millisecond {
		t.Fatalf("percentileLatency() = %s, want 2ms", got)
	}
	if got := percentileLatency(nil, 0.95); got != 0 {
		t.Fatalf("percentileLatency(nil) = %s, want 0", got)
	}
}

func TestMetricsSnapshotCopiesMaps(t *testing.T) {
	metrics := NewMetrics(time.Now())
	metrics.Record(Result{Logs: 1, StatusCode: 202})
	snapshot := metrics.Snapshot()
	snapshot.StatusCounts[202] = 99

	next := metrics.Snapshot()
	if !reflect.DeepEqual(next.StatusCounts, map[int]uint64{202: 1}) {
		t.Fatalf("StatusCounts mutated through snapshot: %#v", next.StatusCounts)
	}
}
