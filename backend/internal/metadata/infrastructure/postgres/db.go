package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"time"

	"github.com/n0thing2c/Soigineer/internal/shared/config"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const pingTimeout = 5 * time.Second

func NewDB(cfg *config.Config) (*sql.DB, error) {
	dsn := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.PostgresUser, cfg.PostgresPassword),
		Host:   cfg.PostgresHost + ":" + cfg.PostgresPort,
		Path:   cfg.PostgresDatabase,
	}

	query := dsn.Query()
	query.Set("sslmode", cfg.PostgresSSLMode)
	dsn.RawQuery = query.Encode()

	db, err := sql.Open("pgx", dsn.String())
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	db.SetMaxOpenConns(cfg.PostgresMaxOpenConns)
	db.SetMaxIdleConns(cfg.PostgresMaxIdleConns)
	db.SetConnMaxLifetime(cfg.PostgresConnMaxLifetime())

	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres at %s:%s/%s: %w", cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresDatabase, err)
	}

	return db, nil
}
