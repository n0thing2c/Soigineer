package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/n0thing2c/Soigineer/internal/processing/domain"
)

type clickHouseLogRepo struct {
	conn        clickhouse.Conn
	saveTimeout time.Duration
}

func NewClickHouseLogRepo(conn clickhouse.Conn, saveTimeout time.Duration) *clickHouseLogRepo {
	return &clickHouseLogRepo{
		conn:        conn,
		saveTimeout: saveTimeout,
	}
}

func (r *clickHouseLogRepo) Save(ctx context.Context, logs []*domain.LogModel) error {
	if len(logs) == 0 {
		return nil
	}

	saveCtx := ctx
	if r.saveTimeout > 0 {
		var cancel context.CancelFunc
		saveCtx, cancel = context.WithTimeout(ctx, r.saveTimeout)
		defer cancel()
	}

	logsToInsert, err := r.filterNewLogs(saveCtx, logs)
	if err != nil {
		return err
	}
	if len(logsToInsert) == 0 {
		return nil
	}

	query := `
        INSERT INTO logs_table (
            EventID, ApplicationName, Level, Message, NormalizedMessage,
            Timestamp, ReceivedAt, TraceID, Fingerprint
        )
    `
	batch, err := r.conn.PrepareBatch(saveCtx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare clickhouse batch for %d logs: %w", len(logsToInsert), err)
	}

	for _, l := range logsToInsert {
		err := batch.Append(
			l.EventID,
			l.ApplicationName,
			l.Level,
			l.Message,
			l.NormalizedMessage,
			l.Timestamp,
			l.ReceivedAt,
			l.TraceID,
			l.Fingerprint,
		)
		if err != nil {
			return fmt.Errorf("failed to append log eventID %s traceID %s in batch of %d logs: %w", l.EventID, l.TraceID, len(logsToInsert), err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send clickhouse batch of %d logs: %w", len(logsToInsert), err)
	}

	return nil
}

func (r *clickHouseLogRepo) filterNewLogs(ctx context.Context, logs []*domain.LogModel) ([]*domain.LogModel, error) {
	seenInBatch := make(map[string]struct{}, len(logs))
	eventIDs := make([]string, 0, len(logs))

	for _, l := range logs {
		if l.EventID == "" {
			continue
		}
		if _, exists := seenInBatch[l.EventID]; exists {
			continue
		}
		seenInBatch[l.EventID] = struct{}{}
		eventIDs = append(eventIDs, l.EventID)
	}

	existing, err := r.existingEventIDs(ctx, eventIDs)
	if err != nil {
		return nil, err
	}

	seenInBatch = make(map[string]struct{}, len(logs))
	filtered := make([]*domain.LogModel, 0, len(logs))
	for _, l := range logs {
		if l.EventID == "" {
			filtered = append(filtered, l)
			continue
		}
		if _, exists := existing[l.EventID]; exists {
			continue
		}
		if _, exists := seenInBatch[l.EventID]; exists {
			continue
		}
		seenInBatch[l.EventID] = struct{}{}
		filtered = append(filtered, l)
	}

	return filtered, nil
}

func (r *clickHouseLogRepo) existingEventIDs(ctx context.Context, eventIDs []string) (map[string]struct{}, error) {
	existing := make(map[string]struct{})
	if len(eventIDs) == 0 {
		return existing, nil
	}

	placeholders := make([]string, len(eventIDs))
	args := make([]any, len(eventIDs))
	for i, eventID := range eventIDs {
		placeholders[i] = "?"
		args[i] = eventID
	}

	query := fmt.Sprintf(
		"SELECT EventID FROM logs_table WHERE EventID IN (%s)",
		strings.Join(placeholders, ","),
	)
	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query existing clickhouse event ids for %d logs: %w", len(eventIDs), err)
	}
	defer rows.Close()

	for rows.Next() {
		var eventID string
		if err := rows.Scan(&eventID); err != nil {
			return nil, fmt.Errorf("failed to scan existing clickhouse event id: %w", err)
		}
		existing[eventID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate existing clickhouse event ids: %w", err)
	}

	return existing, nil
}
