package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/n0thing2c/Soigineer/internal/monitoring/access"
)

type IncidentFilters struct {
	Applications []string
	Levels       []string
	Status       string
	Limit        int
}

type Incident struct {
	ID              string    `json:"id"`
	ApplicationName string    `json:"applicationName"`
	Fingerprint     string    `json:"fingerprint"`
	Level           string    `json:"level"`
	Category        string    `json:"category"`
	Title           string    `json:"title"`
	FirstSeenAt     time.Time `json:"firstSeenAt"`
	LastSeenAt      time.Time `json:"lastSeenAt"`
	OccurrenceCount int64     `json:"occurrenceCount"`
	SuppressedCount int64     `json:"suppressedCount"`
	Status          string    `json:"status"`
}

type AlertRule struct {
	ID                 string `json:"id"`
	ApplicationName    string `json:"applicationName"`
	Level              string `json:"level"`
	Enabled            bool   `json:"enabled"`
	DedupWindowSeconds int    `json:"dedupWindowSeconds"`
	TelegramEnabled    bool   `json:"telegramEnabled"`
}

type AlertRuleUpdate struct {
	Enabled            bool
	DedupWindowSeconds int
	TelegramEnabled    bool
}

type PostgresReader struct {
	db *sql.DB
}

func NewPostgresReader(db *sql.DB) *PostgresReader {
	return &PostgresReader{db: db}
}

func (r *PostgresReader) ListApplications(
	ctx context.Context,
	principal access.Principal,
) ([]string, error) {
	if principal.IsAdmin() {
		rows, err := r.db.QueryContext(ctx, "SELECT name FROM applications ORDER BY name")
		if err != nil {
			return nil, fmt.Errorf("list applications: %w", err)
		}
		defer rows.Close()

		apps := make([]string, 0)
		for rows.Next() {
			var app string
			if err := rows.Scan(&app); err != nil {
				return nil, fmt.Errorf("scan application: %w", err)
			}
			apps = append(apps, app)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate applications: %w", err)
		}
		return apps, nil
	}

	return principal.Applications, nil
}

func (r *PostgresReader) ListIncidents(
	ctx context.Context,
	principal access.Principal,
	filters IncidentFilters,
) ([]Incident, error) {
	where, args, ok := buildIncidentWhere(principal, filters)
	if !ok {
		return []Incident{}, nil
	}

	limit := normalizeLimit(filters.Limit)
	args = append(args, limit)

	query := fmt.Sprintf(
		`
		SELECT
			id::text,
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
		FROM incidents
		%s
		ORDER BY last_seen_at DESC
		LIMIT $%d
		`,
		where,
		len(args),
	)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	defer rows.Close()

	incidents := make([]Incident, 0, limit)
	for rows.Next() {
		var incident Incident
		if err := rows.Scan(
			&incident.ID,
			&incident.ApplicationName,
			&incident.Fingerprint,
			&incident.Level,
			&incident.Category,
			&incident.Title,
			&incident.FirstSeenAt,
			&incident.LastSeenAt,
			&incident.OccurrenceCount,
			&incident.SuppressedCount,
			&incident.Status,
		); err != nil {
			return nil, fmt.Errorf("scan incident: %w", err)
		}
		incidents = append(incidents, incident)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate incidents: %w", err)
	}

	return incidents, nil
}

func (r *PostgresReader) UpdateIncidentStatus(
	ctx context.Context,
	id string,
	status string,
) error {
	result, err := r.db.ExecContext(
		ctx,
		`
		UPDATE incidents
		SET status = $2, updated_at = now()
		WHERE id = $1
		`,
		id,
		status,
	)
	if err != nil {
		return fmt.Errorf("update incident status: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read incident update result: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *PostgresReader) ListAlertRules(ctx context.Context) ([]AlertRule, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`
		SELECT
			ar.id::text,
			a.name,
			ar.level,
			ar.enabled,
			ar.dedup_window_seconds,
			ar.telegram_enabled
		FROM alert_rules ar
		JOIN applications a ON a.id = ar.application_id
		ORDER BY a.name, ar.level
		`,
	)
	if err != nil {
		return nil, fmt.Errorf("list alert rules: %w", err)
	}
	defer rows.Close()

	rules := make([]AlertRule, 0)
	for rows.Next() {
		var rule AlertRule
		if err := rows.Scan(
			&rule.ID,
			&rule.ApplicationName,
			&rule.Level,
			&rule.Enabled,
			&rule.DedupWindowSeconds,
			&rule.TelegramEnabled,
		); err != nil {
			return nil, fmt.Errorf("scan alert rule: %w", err)
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate alert rules: %w", err)
	}

	return rules, nil
}

func (r *PostgresReader) UpdateAlertRule(
	ctx context.Context,
	id string,
	update AlertRuleUpdate,
) error {
	result, err := r.db.ExecContext(
		ctx,
		`
		UPDATE alert_rules
		SET enabled = $2,
			dedup_window_seconds = $3,
			telegram_enabled = $4,
			updated_at = now()
		WHERE id = $1
		`,
		id,
		update.Enabled,
		update.DedupWindowSeconds,
		update.TelegramEnabled,
	)
	if err != nil {
		return fmt.Errorf("update alert rule: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read alert rule update result: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func buildIncidentWhere(
	principal access.Principal,
	filters IncidentFilters,
) (string, []any, bool) {
	clauses := make([]string, 0, 4)
	args := make([]any, 0)

	apps, ok := allowedApplications(principal, filters.Applications)
	if !ok {
		return "", nil, false
	}
	if len(apps) > 0 {
		clauses = append(clauses, "application_name IN ("+postgresPlaceholders(len(args)+1, len(apps))+")")
		for _, app := range apps {
			args = append(args, app)
		}
	}

	if len(filters.Levels) > 0 {
		clauses = append(clauses, "level IN ("+postgresPlaceholders(len(args)+1, len(filters.Levels))+")")
		for _, level := range filters.Levels {
			args = append(args, level)
		}
	}

	if filters.Status != "" {
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)+1))
		args = append(args, filters.Status)
	}

	if len(clauses) == 0 {
		return "", args, true
	}
	return "WHERE " + strings.Join(clauses, " AND "), args, true
}

func postgresPlaceholders(start, count int) string {
	values := make([]string, count)
	for i := range values {
		values[i] = fmt.Sprintf("$%d", start+i)
	}
	return strings.Join(values, ",")
}
