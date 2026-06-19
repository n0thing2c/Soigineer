package repository

import (
	"context"
	"fmt"

	"github.com/n0thing2c/Soigineer/internal/processing/domain"
	"github.com/n0thing2c/Soigineer/internal/processing/infrastructure/database"
)

type clickHouseLogRepo struct {
	db *database.ClickHouseDB
}

func NewClickHouseLogRepo(db *database.ClickHouseDB) *clickHouseLogRepo {
	return &clickHouseLogRepo{db: db}
}

func (r *clickHouseLogRepo) Save(ctx context.Context, logs []*domain.LogModel) error {
	if len(logs) == 0 {
		return nil
	}

	// Prepare Batch: Khởi tạo một mẻ lệnh Insert (Lưu ý: Thay thế "your_database.logs_table" bằng tên bảng thực tế)
	query := `
        INSERT INTO logs_table (
            ApplicationName, Level, Message, NormalizedMessage, 
            Timestamp, ReceivedAt, TraceID, Fingerprint
        )
    `
	batch, err := r.db.Conn.PrepareBatch(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare clickhouse batch: %w", err)
	}

	// Append data: Load the entire slice into the ClickHouse Driver's RAM.
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
			return fmt.Errorf("failed to append log traceID %s to batch: %w", l.TraceID, err)
		}
	}

	// Send Batch:
	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send batch to clickhouse: %w", err)
	}

	return nil
}
