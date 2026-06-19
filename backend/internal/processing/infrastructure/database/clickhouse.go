package database

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/n0thing2c/Soigineer/internal/shared/config"
)

type ClickHouseDB struct {
	Conn clickhouse.Conn
}

func NewClickHouse(cfg *config.Config) (*ClickHouseDB, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{
			cfg.ClickHouseHost + ":" + cfg.ClickHousePort,
		},
		Auth: clickhouse.Auth{
			Database: cfg.ClickHouseDatabase,
			Username: cfg.ClickHouseUser,
			Password: cfg.ClickHousePassword,
		},
		DialTimeout: 10 * time.Second,

		// CONNECTION POOLING
		MaxOpenConns:    10,
		MaxIdleConns:    10,
		ConnMaxLifetime: time.Hour,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to initialize ClickHouse options: %w", err)
	}

	// FAIL-FAST:
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping ClickHouse at startup: %w", err)
	}

	return &ClickHouseDB{Conn: conn}, nil
}

func (db *ClickHouseDB) Close() error {
	if db.Conn != nil {
		return db.Conn.Close()
	}
	return nil
}

func (db *ClickHouseDB) Ping(ctx context.Context) error {
	return db.Conn.Ping(ctx)
}
