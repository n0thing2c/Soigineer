package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
)

const (
	defaultCategory = "UNKNOWN_ERROR"
	maxTitleLength  = 160
)

type IncidentRecorder struct {
	db *sql.DB
}

func NewIncidentRecorder(db *sql.DB) *IncidentRecorder {
	return &IncidentRecorder{db: db}
}

func (r *IncidentRecorder) Record(ctx context.Context, alert sharedDomain.AlertEvent, dispatched bool) error {
	if r == nil || r.db == nil {
		return nil
	}

	seenAt := alert.Timestamp
	if seenAt.IsZero() {
		seenAt = time.Now().UTC()
	}

	category := strings.TrimSpace(alert.Category)
	if category == "" {
		category = defaultCategory
	}

	suppressedIncrement := 0
	if !dispatched {
		suppressedIncrement = 1
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin incident transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(
		ctx,
		`
		INSERT INTO applications (name, display_name)
		VALUES ($1, $1)
		ON CONFLICT (name) DO NOTHING
		`,
		alert.ApplicationName,
	); err != nil {
		return fmt.Errorf("upsert application metadata %q: %w", alert.ApplicationName, err)
	}

	if _, err = tx.ExecContext(
		ctx,
		`
		INSERT INTO incidents (
			application_name,
			fingerprint,
			level,
			category,
			title,
			first_seen_at,
			last_seen_at,
			occurrence_count,
			suppressed_count,
			status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $6, 1, $7, 'OPEN')
		ON CONFLICT (fingerprint) DO UPDATE SET
			application_name = EXCLUDED.application_name,
			level = EXCLUDED.level,
			category = EXCLUDED.category,
			title = EXCLUDED.title,
			last_seen_at = GREATEST(incidents.last_seen_at, EXCLUDED.last_seen_at),
			occurrence_count = incidents.occurrence_count + 1,
			suppressed_count = incidents.suppressed_count + EXCLUDED.suppressed_count,
			status = CASE
				WHEN incidents.status = 'RESOLVED' THEN 'OPEN'
				ELSE incidents.status
			END,
			updated_at = now()
		`,
		alert.ApplicationName,
		alert.Fingerprint,
		alert.Level,
		category,
		buildIncidentTitle(alert, category),
		seenAt,
		suppressedIncrement,
	); err != nil {
		return fmt.Errorf("upsert incident fingerprint=%s app=%s: %w", alert.Fingerprint, alert.ApplicationName, err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit incident transaction: %w", err)
	}

	return nil
}

func buildIncidentTitle(alert sharedDomain.AlertEvent, category string) string {
	message := strings.Join(strings.Fields(alert.Message), " ")
	if message == "" {
		message = category
	}

	title := fmt.Sprintf("[%s] %s: %s", alert.Level, alert.ApplicationName, message)
	if len(title) <= maxTitleLength {
		return title
	}

	return title[:maxTitleLength-3] + "..."
}
