package database

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/n0thing2c/Soigineer/internal/shared/config"
)

func NewClickHouse(cfg *config.Config) (clickhouse.Conn, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{
			cfg.ClickHouseHost + ":" + cfg.ClickHousePort,
		},
		Auth: clickhouse.Auth{
			Database: cfg.ClickHouseDatabase,
			Username: cfg.ClickHouseUser,
			Password: cfg.ClickHousePassword,
		},
		DialTimeout:     10 * time.Second,
		MaxOpenConns:    cfg.ClickHouseMaxOpenConns,
		MaxIdleConns:    cfg.ClickHouseMaxIdleConns,
		ConnMaxLifetime: cfg.ClickHouseConnMaxLifetime(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ClickHouse options: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping ClickHouse at startup: %w", err)
	}

	return conn, nil
}
