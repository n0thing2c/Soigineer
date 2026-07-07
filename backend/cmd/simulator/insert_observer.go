package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

type insertBatch struct {
	startedAt time.Time
	traceIDs  []string
}

type InsertTracker struct {
	mu      sync.Mutex
	batches []insertBatch
}

type trackedInsertBatch struct {
	id    int
	batch insertBatch
}

func NewInsertTracker() *InsertTracker {
	return &InsertTracker{
		batches: make([]insertBatch, 0, 1024),
	}
}

func (t *InsertTracker) Track(result Result, job Job) {
	if t == nil || result.Err != nil || result.Kind != jobBatch || result.StartedAt.IsZero() {
		return
	}

	traceIDs := make([]string, 0, len(job.Logs))
	for _, logRecord := range job.Logs {
		if logRecord.TraceID != "" {
			traceIDs = append(traceIDs, logRecord.TraceID)
		}
	}
	if len(traceIDs) == 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.batches = append(t.batches, insertBatch{
		startedAt: result.StartedAt,
		traceIDs:  traceIDs,
	})
}

func (t *InsertTracker) Count() int {
	if t == nil {
		return 0
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.batches)
}

func (t *InsertTracker) BatchesSince(index int) ([]trackedInsertBatch, int) {
	if t == nil {
		return nil, index
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if index >= len(t.batches) {
		return nil, len(t.batches)
	}

	batches := make([]trackedInsertBatch, 0, len(t.batches)-index)
	for i := index; i < len(t.batches); i++ {
		batches = append(batches, trackedInsertBatch{
			id: i,
			batch: insertBatch{
				startedAt: t.batches[i].startedAt,
				traceIDs:  append([]string(nil), t.batches[i].traceIDs...),
			},
		})
	}
	return batches, len(t.batches)
}

func observeBatchInserts(ctx context.Context, cfg Config, tracker *InsertTracker, loadDone <-chan struct{}, metrics *Metrics) error {
	if cfg.ReportWait <= 0 {
		return nil
	}
	if strings.TrimSpace(cfg.ClickHouseHost) == "" || strings.TrimSpace(cfg.ClickHousePort) == "" {
		return fmt.Errorf("clickhouse host or port not configured")
	}

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
		return err
	}
	defer conn.Close()

	pollEvery := cfg.InsertPollEvery
	if pollEvery <= 0 {
		pollEvery = 250 * time.Millisecond
	}

	pending := make(map[int]insertBatch)
	nextIndex := 0
	loadFinished := false
	var settleTimer *time.Timer
	var settleC <-chan time.Time

	ticker := time.NewTicker(pollEvery)
	defer ticker.Stop()
	defer func() {
		if settleTimer != nil {
			settleTimer.Stop()
		}
	}()

	for {
		newBatches, next := tracker.BatchesSince(nextIndex)
		nextIndex = next
		for _, tracked := range newBatches {
			pending[tracked.id] = tracked.batch
		}

		if len(pending) > 0 {
			found, err := queryInsertedTraceIDs(ctx, conn, pendingTraceIDs(pending))
			if err != nil {
				return err
			}

			now := time.Now()
			for id, batch := range pending {
				if batchComplete(batch, found) {
					metrics.RecordBatchInsertLatency(now.Sub(batch.startedAt))
					delete(pending, id)
				}
			}
		}

		if loadFinished && len(pending) == 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			metrics.RecordBatchInsertTimeout(len(pending))
			return nil
		case <-loadDone:
			if !loadFinished {
				loadFinished = true
				settleTimer = time.NewTimer(cfg.ReportWait)
				settleC = settleTimer.C
				loadDone = nil
			}
		case <-settleC:
			metrics.RecordBatchInsertTimeout(len(pending))
			return nil
		case <-ticker.C:
		}
	}
}

func pendingTraceIDs(pending map[int]insertBatch) []string {
	seen := make(map[string]struct{})
	traceIDs := make([]string, 0)
	for _, batch := range pending {
		for _, traceID := range batch.traceIDs {
			if _, ok := seen[traceID]; ok {
				continue
			}
			seen[traceID] = struct{}{}
			traceIDs = append(traceIDs, traceID)
		}
	}
	return traceIDs
}

func batchComplete(batch insertBatch, found map[string]struct{}) bool {
	for _, traceID := range batch.traceIDs {
		if _, ok := found[traceID]; !ok {
			return false
		}
	}
	return true
}

func queryInsertedTraceIDs(ctx context.Context, conn clickhouse.Conn, traceIDs []string) (map[string]struct{}, error) {
	found := make(map[string]struct{})
	const chunkSize = 1000

	for start := 0; start < len(traceIDs); start += chunkSize {
		end := minInt(start+chunkSize, len(traceIDs))
		chunk := traceIDs[start:end]

		placeholders := make([]string, len(chunk))
		args := make([]any, len(chunk))
		for i, traceID := range chunk {
			placeholders[i] = "?"
			args[i] = traceID
		}

		query := fmt.Sprintf(
			"SELECT TraceID FROM logs_table WHERE TraceID IN (%s)",
			strings.Join(placeholders, ","),
		)
		rows, err := conn.Query(ctx, query, args...)
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var traceID string
			if err := rows.Scan(&traceID); err != nil {
				rows.Close()
				return nil, err
			}
			found[traceID] = struct{}{}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}

	return found, nil
}
