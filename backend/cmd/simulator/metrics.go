package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Metrics struct {
	mu sync.Mutex

	start        time.Time
	end          time.Time
	plannedLogs  uint64
	sentLogs     uint64
	failedLogs   uint64
	droppedLogs  uint64
	successReqs  uint64
	failedReqs   uint64
	droppedReqs  uint64
	statusCounts map[int]uint64
	errorCounts  map[string]uint64
	latencies    []time.Duration
}

type Snapshot struct {
	StartedAt     time.Time
	FinishedAt    time.Time
	Duration      time.Duration
	PlannedLogs   uint64
	SentLogs      uint64
	FailedLogs    uint64
	DroppedLogs   uint64
	SuccessReqs   uint64
	FailedReqs    uint64
	DroppedReqs   uint64
	ThroughputLPS float64
	MinLatency    time.Duration
	AvgLatency    time.Duration
	P95Latency    time.Duration
	MaxLatency    time.Duration
	StatusCounts  map[int]uint64
	NetworkErrors map[string]uint64
}

func NewMetrics(start time.Time) *Metrics {
	return &Metrics{
		start:        start,
		statusCounts: make(map[int]uint64),
		errorCounts:  make(map[string]uint64),
		latencies:    make([]time.Duration, 0, 1024),
	}
}

func (m *Metrics) AddPlanned(logs int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.plannedLogs += uint64(logs)
}

func (m *Metrics) Record(result Result) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.latencies = append(m.latencies, result.Latency)
	if result.Err != nil {
		m.failedReqs++
		m.failedLogs += uint64(result.Logs)
		if result.StatusCode > 0 {
			m.statusCounts[result.StatusCode]++
		} else {
			m.errorCounts[result.Err.Error()]++
		}
		return
	}

	m.successReqs++
	m.sentLogs += uint64(result.Logs)
	m.statusCounts[result.StatusCode]++
}

func (m *Metrics) RecordDropped(job Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.droppedReqs++
	m.droppedLogs += uint64(len(job.Logs))
}

func (m *Metrics) Finish(end time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.end = end
}

func (m *Metrics) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	finished := m.end
	if finished.IsZero() {
		finished = time.Now()
	}

	duration := finished.Sub(m.start)
	if duration <= 0 {
		duration = time.Millisecond
	}

	latencies := append([]time.Duration(nil), m.latencies...)
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	statusCounts := make(map[int]uint64, len(m.statusCounts))
	for key, value := range m.statusCounts {
		statusCounts[key] = value
	}

	errorCounts := make(map[string]uint64, len(m.errorCounts))
	for key, value := range m.errorCounts {
		errorCounts[key] = value
	}

	snapshot := Snapshot{
		StartedAt:     m.start,
		FinishedAt:    finished,
		Duration:      duration,
		PlannedLogs:   m.plannedLogs,
		SentLogs:      m.sentLogs,
		FailedLogs:    m.failedLogs,
		DroppedLogs:   m.droppedLogs,
		SuccessReqs:   m.successReqs,
		FailedReqs:    m.failedReqs,
		DroppedReqs:   m.droppedReqs,
		ThroughputLPS: float64(m.sentLogs) / duration.Seconds(),
		StatusCounts:  statusCounts,
		NetworkErrors: errorCounts,
	}

	if len(latencies) > 0 {
		snapshot.MinLatency = latencies[0]
		snapshot.MaxLatency = latencies[len(latencies)-1]
		snapshot.AvgLatency = averageLatency(latencies)
		snapshot.P95Latency = percentileLatency(latencies, 0.95)
	}

	return snapshot
}

func (s Snapshot) ErrorRate() float64 {
	total := s.SuccessReqs + s.FailedReqs
	if total == 0 {
		return 0
	}
	return float64(s.FailedReqs) / float64(total)
}

func (s Snapshot) SummaryLines() []string {
	lines := []string{
		fmt.Sprintf("duration=%s planned_logs=%d sent_logs=%d failed_logs=%d dropped_logs=%d throughput=%.2f logs/s", s.Duration.Round(time.Millisecond), s.PlannedLogs, s.SentLogs, s.FailedLogs, s.DroppedLogs, s.ThroughputLPS),
		fmt.Sprintf("requests success=%d failed=%d dropped=%d error_rate=%.2f%%", s.SuccessReqs, s.FailedReqs, s.DroppedReqs, s.ErrorRate()*100),
		fmt.Sprintf("latency min=%s avg=%s p95=%s max=%s", roundDuration(s.MinLatency), roundDuration(s.AvgLatency), roundDuration(s.P95Latency), roundDuration(s.MaxLatency)),
	}

	if len(s.StatusCounts) > 0 {
		lines = append(lines, "status_counts="+formatIntMap(s.StatusCounts))
	}
	if len(s.NetworkErrors) > 0 {
		lines = append(lines, "network_errors="+formatStringMap(s.NetworkErrors))
	}
	return lines
}

func averageLatency(latencies []time.Duration) time.Duration {
	var total time.Duration
	for _, latency := range latencies {
		total += latency
	}
	return total / time.Duration(len(latencies))
}

func percentileLatency(latencies []time.Duration, percentile float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	index := int(float64(len(latencies)-1) * percentile)
	return latencies[index]
}

func roundDuration(duration time.Duration) time.Duration {
	if duration <= 0 {
		return 0
	}
	return duration.Round(time.Microsecond)
}

func formatIntMap(values map[int]uint64) string {
	keys := make([]int, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Ints(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%d=%d", key, values[key]))
	}
	return strings.Join(parts, ",")
}

func formatStringMap(values map[string]uint64) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, values[key]))
	}
	return strings.Join(parts, ",")
}
