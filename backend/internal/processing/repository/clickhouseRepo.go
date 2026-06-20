package repository

import (
	"context"
	"fmt"
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

	query := `
        INSERT INTO logs_table (
            ApplicationName, Level, Message, NormalizedMessage,
            Timestamp, ReceivedAt, TraceID, Fingerprint
        )
    `
	batch, err := r.conn.PrepareBatch(saveCtx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare clickhouse batch for %d logs: %w", len(logs), err)
	}

	for _, l := range logs {
		err := batch.Append(
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
			return fmt.Errorf("failed to append log traceID %s in batch of %d logs: %w", l.TraceID, len(logs), err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send clickhouse batch of %d logs: %w", len(logs), err)
	}

	return nil
}
