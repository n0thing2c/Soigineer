package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrRefreshTokenNotFound = errors.New("refresh token not found")

type RefreshTokenRepository struct {
	db *sql.DB
}

func NewRefreshTokenRepository(db *sql.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) EnsureSchema(ctx context.Context) error {
	statements := []string{
		`
		CREATE TABLE IF NOT EXISTS refresh_tokens (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token_hash TEXT NOT NULL UNIQUE,
			expires_at TIMESTAMPTZ NOT NULL,
			revoked_at TIMESTAMPTZ,
			replaced_by_token_hash TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
		`,
		`
		CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id
			ON refresh_tokens (user_id)
		`,
		`
		CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at
			ON refresh_tokens (expires_at)
		`,
	}

	for _, statement := range statements {
		if _, err := r.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("ensure refresh token schema: %w", err)
		}
	}
	return nil
}

func (r *RefreshTokenRepository) Save(
	ctx context.Context,
	userID string,
	tokenHash string,
	expiresAt time.Time,
) error {
	if _, err := r.db.ExecContext(
		ctx,
		`
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		`,
		userID,
		tokenHash,
		expiresAt,
	); err != nil {
		return fmt.Errorf("save refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) Rotate(
	ctx context.Context,
	oldTokenHash string,
	newTokenHash string,
	newExpiresAt time.Time,
	now time.Time,
) (string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin rotate refresh token transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var userID string
	err = tx.QueryRowContext(
		ctx,
		`
		UPDATE refresh_tokens
		SET revoked_at = $3, replaced_by_token_hash = $4
		WHERE token_hash = $1
			AND revoked_at IS NULL
			AND expires_at > $2
		RETURNING user_id::text
		`,
		oldTokenHash,
		now,
		now,
		newTokenHash,
	).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrRefreshTokenNotFound
	}
	if err != nil {
		return "", fmt.Errorf("rotate refresh token: %w", err)
	}

	if _, err = tx.ExecContext(
		ctx,
		`
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		`,
		userID,
		newTokenHash,
		newExpiresAt,
	); err != nil {
		return "", fmt.Errorf("save rotated refresh token: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("commit rotate refresh token transaction: %w", err)
	}

	return userID, nil
}

func (r *RefreshTokenRepository) Revoke(
	ctx context.Context,
	tokenHash string,
	now time.Time,
) error {
	if _, err := r.db.ExecContext(
		ctx,
		`
		UPDATE refresh_tokens
		SET revoked_at = $2
		WHERE token_hash = $1
			AND revoked_at IS NULL
		`,
		tokenHash,
		now,
	); err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}
