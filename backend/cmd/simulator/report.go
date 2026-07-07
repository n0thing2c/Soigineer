package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

type TopicOffsets struct {
	Total        int64
	ByPartition  map[int]int64
	CapturedAt   time.Time
	CaptureError error
}

type ConsumerLag struct {
	GroupID          string
	Topic            string
	TotalLag         int64
	ByPartition      map[int]int64
	EndOffsets       map[int]int64
	CommittedOffsets map[int]int64
	CapturedAt       time.Time
	CaptureError     error
}

type RunResult struct {
	RunID             string
	StartedAt         time.Time
	FinishedAt        time.Time
	Snapshot          Snapshot
	TopicBefore       TopicOffsets
	TopicAfter        TopicOffsets
	TopicDelta        int64
	ConsumerLagBefore ConsumerLag
	ConsumerLagAfter  ConsumerLag
	Settled           bool
	BenchmarkErr      error
}

type ReportData struct {
	Result            RunResult
	Config            Config
	ClickHouseCount   int64
	ClickHouseError   error
	ReportGeneratedAt time.Time
}

func newRunID() string {
	return fmt.Sprintf("bench-%s-%s", time.Now().UTC().Format("20060102-150405"), uuid.NewString()[:8])
}

func defaultReportPath(runID string) string {
	return filepath.Join("benchmark-reports", runID+".md")
}

func captureTopicOffsets(ctx context.Context, brokers []string, topic string) TopicOffsets {
	snapshot := TopicOffsets{
		ByPartition: make(map[int]int64),
		CapturedAt:  time.Now().UTC(),
	}
	if len(brokers) == 0 || strings.TrimSpace(topic) == "" {
		snapshot.CaptureError = fmt.Errorf("kafka brokers or topic not configured")
		return snapshot
	}

	conn, err := kafka.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		snapshot.CaptureError = err
		return snapshot
	}
	defer conn.Close()

	partitions, err := conn.ReadPartitions(topic)
	if err != nil {
		snapshot.CaptureError = err
		return snapshot
	}

	for _, partition := range partitions {
		leader, err := kafka.DialLeader(ctx, "tcp", brokers[0], topic, partition.ID)
		if err != nil {
			snapshot.CaptureError = err
			return snapshot
		}

		offset, err := leader.ReadLastOffset()
		leader.Close()
		if err != nil {
			snapshot.CaptureError = err
			return snapshot
		}

		snapshot.ByPartition[partition.ID] = offset
		snapshot.Total += offset
	}

	return snapshot
}

func captureConsumerLag(ctx context.Context, brokers []string, topic string, groupID string) ConsumerLag {
	snapshot := ConsumerLag{
		GroupID:          groupID,
		Topic:            topic,
		ByPartition:      make(map[int]int64),
		EndOffsets:       make(map[int]int64),
		CommittedOffsets: make(map[int]int64),
		CapturedAt:       time.Now().UTC(),
	}
	if len(brokers) == 0 || strings.TrimSpace(topic) == "" || strings.TrimSpace(groupID) == "" {
		snapshot.CaptureError = fmt.Errorf("kafka brokers, topic, or consumer group not configured")
		return snapshot
	}

	topicOffsets := captureTopicOffsets(ctx, brokers, topic)
	if topicOffsets.CaptureError != nil {
		snapshot.CaptureError = topicOffsets.CaptureError
		return snapshot
	}
	for partition, offset := range topicOffsets.ByPartition {
		snapshot.EndOffsets[partition] = offset
	}

	partitions := make([]int, 0, len(topicOffsets.ByPartition))
	for partition := range topicOffsets.ByPartition {
		partitions = append(partitions, partition)
	}

	client := kafka.Client{
		Addr:    kafka.TCP(brokers[0]),
		Timeout: 5 * time.Second,
	}
	offsets, err := client.OffsetFetch(ctx, &kafka.OffsetFetchRequest{
		GroupID: groupID,
		Topics: map[string][]int{
			topic: partitions,
		},
	})
	if err != nil {
		snapshot.CaptureError = err
		return snapshot
	}
	if offsets.Error != nil {
		snapshot.CaptureError = offsets.Error
		return snapshot
	}

	seenCommitted := make(map[int]struct{}, len(partitions))
	for _, offset := range offsets.Topics[topic] {
		if offset.Error != nil {
			snapshot.CaptureError = offset.Error
			return snapshot
		}

		committed := offset.CommittedOffset
		if committed < 0 {
			committed = 0
		}
		seenCommitted[offset.Partition] = struct{}{}
		endOffset := snapshot.EndOffsets[offset.Partition]
		lag := endOffset - committed
		if lag < 0 {
			lag = 0
		}

		snapshot.CommittedOffsets[offset.Partition] = committed
		snapshot.ByPartition[offset.Partition] = lag
		snapshot.TotalLag += lag
	}
	for _, partition := range partitions {
		if _, ok := seenCommitted[partition]; ok {
			continue
		}
		endOffset := snapshot.EndOffsets[partition]
		snapshot.CommittedOffsets[partition] = 0
		snapshot.ByPartition[partition] = endOffset
		snapshot.TotalLag += endOffset
	}

	return snapshot
}

func countClickHouseLogs(ctx context.Context, cfg Config, runID string) (int64, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{cfg.ClickHouseHost + ":" + cfg.ClickHousePort},
		Auth: clickhouse.Auth{
			Database: cfg.ClickHouseDB,
			Username: cfg.ClickHouseUser,
			Password: cfg.ClickHousePass,
		},
		DialTimeout: 10 * time.Second,
	})
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	query := "SELECT count() FROM logs_table WHERE startsWith(TraceID, ?)"
	row := conn.QueryRow(ctx, query, runID+"-")

	var count uint64
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return int64(count), nil
}

func buildReportData(ctx context.Context, cfg Config, result RunResult) ReportData {
	report := ReportData{
		Result:            result,
		Config:            cfg,
		ReportGeneratedAt: time.Now().UTC(),
	}

	if cfg.ReportWait > 0 && !result.Settled {
		timer := time.NewTimer(cfg.ReportWait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
		case <-timer.C:
		}
	}

	count, err := countClickHouseLogs(context.Background(), cfg, result.RunID)
	report.ClickHouseCount = count
	report.ClickHouseError = err
	return report
}

func writeMarkdownReport(cfg Config, report ReportData) (string, error) {
	path := cfg.ReportFile
	if strings.TrimSpace(path) == "" {
		path = defaultReportPath(report.Result.RunID)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}

	content := report.Markdown()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (r ReportData) Markdown() string {
	snapshot := r.Result.Snapshot
	queueObserved := r.Result.TopicDelta
	if queueObserved < 0 {
		queueObserved = 0
	}

	gatewayAccepted := int64(snapshot.SentLogs)
	gatewayGap := gatewayAccepted - queueObserved
	if gatewayGap < 0 {
		gatewayGap = 0
	}

	queueToDBGap := queueObserved - r.ClickHouseCount
	if queueToDBGap < 0 {
		queueToDBGap = 0
	}

	var builder strings.Builder
	builder.WriteString("# Benchmark Report\n\n")
	builder.WriteString(fmt.Sprintf("- Run ID: `%s`\n", r.Result.RunID))
	builder.WriteString(fmt.Sprintf("- Generated at: `%s`\n", r.ReportGeneratedAt.Format(time.RFC3339)))
	builder.WriteString(fmt.Sprintf("- Load window: `%s` -> `%s`\n", r.Result.StartedAt.Format(time.RFC3339), r.Result.FinishedAt.Format(time.RFC3339)))
	builder.WriteString(fmt.Sprintf("- Mode: `%s`\n", r.Config.Mode))
	builder.WriteString(fmt.Sprintf("- Servers: `%d`\n", r.Config.ServerCount))
	builder.WriteString(fmt.Sprintf("- Logs per second per server: `%d`\n", r.Config.LogsPerSec))
	builder.WriteString(fmt.Sprintf("- Batch size: `%d`\n", r.Config.BatchSize))
	builder.WriteString(fmt.Sprintf("- Report wait after load: `%s`\n\n", r.Config.ReportWait))

	builder.WriteString("## Summary\n\n")
	builder.WriteString("| Stage | Metric | Value |\n")
	builder.WriteString("| --- | --- | ---: |\n")
	builder.WriteString(fmt.Sprintf("| Simulator | Planned logs | %d |\n", snapshot.PlannedLogs))
	builder.WriteString(fmt.Sprintf("| Gateway | Accepted logs | %d |\n", snapshot.SentLogs))
	builder.WriteString(fmt.Sprintf("| Gateway | Failed logs | %d |\n", snapshot.FailedLogs))
	builder.WriteString(fmt.Sprintf("| Simulator | Dropped logs before send | %d |\n", snapshot.DroppedLogs))
	builder.WriteString(fmt.Sprintf("| Queue | Observed logs in `%s` | %d |\n", r.Config.KafkaTopic, queueObserved))
	if r.Result.ConsumerLagBefore.CaptureError == nil {
		builder.WriteString(fmt.Sprintf("| Queue | Consumer lag before `%s` | %d |\n", r.Config.ConsumerGroup, r.Result.ConsumerLagBefore.TotalLag))
	} else {
		builder.WriteString(fmt.Sprintf("| Queue | Consumer lag before `%s` | unavailable |\n", r.Config.ConsumerGroup))
	}
	if r.Result.ConsumerLagAfter.CaptureError == nil {
		builder.WriteString(fmt.Sprintf("| Queue | Consumer lag after `%s` | %d |\n", r.Config.ConsumerGroup, r.Result.ConsumerLagAfter.TotalLag))
	} else {
		builder.WriteString(fmt.Sprintf("| Queue | Consumer lag after `%s` | unavailable |\n", r.Config.ConsumerGroup))
	}
	if r.ClickHouseError != nil {
		builder.WriteString("| ClickHouse | Inserted logs | query failed |\n")
	} else {
		builder.WriteString(fmt.Sprintf("| ClickHouse | Inserted logs | %d |\n", r.ClickHouseCount))
	}
	builder.WriteString("\n")

	builder.WriteString("## Stage Analysis\n\n")
	builder.WriteString(fmt.Sprintf("- Gateway request failures: `%d` logs across `%d` requests.\n", snapshot.FailedLogs, snapshot.FailedReqs))
	builder.WriteString(fmt.Sprintf("- Logs dropped because the benchmark window ended before send: `%d` logs across `%d` jobs.\n", snapshot.DroppedLogs, snapshot.DroppedReqs))
	builder.WriteString(fmt.Sprintf("- Gateway accepted but not observed in queue delta: `%d` logs.\n", gatewayGap))
	if r.Result.ConsumerLagAfter.CaptureError == nil {
		builder.WriteString(fmt.Sprintf("- Raw log consumer lag for group `%s` after settle: `%d` messages.\n", r.Config.ConsumerGroup, r.Result.ConsumerLagAfter.TotalLag))
	}
	if r.ClickHouseError != nil {
		builder.WriteString(fmt.Sprintf("- ClickHouse verification failed: `%v`.\n", r.ClickHouseError))
	} else {
		builder.WriteString(fmt.Sprintf("- Queue to ClickHouse gap after `%s` settle time: `%d` logs.\n", r.Config.ReportWait, queueToDBGap))
	}
	builder.WriteString("\n")

	builder.WriteString("## Performance\n\n")
	for _, line := range snapshot.SummaryLines() {
		builder.WriteString("- " + line + "\n")
	}
	builder.WriteString("\n")

	builder.WriteString("## Notes\n\n")
	builder.WriteString("- `Gateway accepted` means the simulator received HTTP success from the ingestion gateway.\n")
	builder.WriteString("- `Observed logs in queue` is computed from Redpanda topic offset delta during this run.\n")
	builder.WriteString("- `Consumer lag` is computed as topic end offset minus committed offset for the configured consumer group.\n")
	builder.WriteString("- `Batch insert latency` is measured from simulator HTTP send start until every TraceID in that simulator batch is visible in ClickHouse.\n")
	builder.WriteString("- `Queue to ClickHouse gap` means logs were published to queue but were not visible in ClickHouse after the settle window. With the current system, this gap may be queue backlog, processor lag, retry, or DB write failure.\n")
	builder.WriteString("- `Dropped logs before send` are logs created by the simulator after the load window was already closed.\n")
	if r.Result.TopicBefore.CaptureError != nil || r.Result.TopicAfter.CaptureError != nil {
		builder.WriteString("- Queue observation had errors. Review the queue section carefully.\n")
	}
	if r.Result.ConsumerLagBefore.CaptureError != nil || r.Result.ConsumerLagAfter.CaptureError != nil {
		builder.WriteString("- Consumer lag observation had errors. Review the queue section carefully.\n")
	}
	if r.Result.BenchmarkErr != nil {
		builder.WriteString(fmt.Sprintf("- Benchmark ended with runtime error: `%v`.\n", r.Result.BenchmarkErr))
	}

	return builder.String()
}
