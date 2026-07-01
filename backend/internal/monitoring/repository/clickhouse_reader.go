package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/n0thing2c/Soigineer/internal/monitoring/access"
	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
)

const maxQueryLimit = 500

type LogFilters struct {
	Applications []string
	Levels       []string
	From         time.Time
	To           time.Time
	Limit        int
}

type HealthRow struct {
	ApplicationName string    `json:"applicationName"`
	TotalCount      uint64    `json:"totalCount"`
	WarnCount       uint64    `json:"warnCount"`
	ErrorCount      uint64    `json:"errorCount"`
	CriticalCount   uint64    `json:"criticalCount"`
	LastSeenAt      time.Time `json:"lastSeenAt"`
}

type ClickHouseReader struct {
	conn clickhouse.Conn
}

func NewClickHouseReader(conn clickhouse.Conn) *ClickHouseReader {
	return &ClickHouseReader{conn: conn}
}

func (r *ClickHouseReader) ListLogs(
	ctx context.Context,
	principal access.Principal,
	filters LogFilters,
) ([]sharedDomain.ProcessedLogEvent, error) {
	where, args, ok := buildLogWhere(principal, filters)
	if !ok {
		return []sharedDomain.ProcessedLogEvent{}, nil
	}

	limit := normalizeLimit(filters.Limit)
	args = append(args, limit)

	query := fmt.Sprintf(
		`
		SELECT
			EventID,
			ApplicationName,
			Level,
			Message,
			NormalizedMessage,
			Timestamp,
			ReceivedAt,
			TraceID,
			Fingerprint
		FROM logs_table
		%s
		ORDER BY Timestamp DESC
		LIMIT ?
		`,
		where,
	)

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query logs: %w", err)
	}
	defer rows.Close()

	logs := make([]sharedDomain.ProcessedLogEvent, 0, limit)
	for rows.Next() {
		var log sharedDomain.ProcessedLogEvent
		if err := rows.Scan(
			&log.EventID,
			&log.ApplicationName,
			&log.Level,
			&log.Message,
			&log.NormalizedMessage,
			&log.Timestamp,
			&log.ReceivedAt,
			&log.TraceID,
			&log.Fingerprint,
		); err != nil {
			return nil, fmt.Errorf("scan log row: %w", err)
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate log rows: %w", err)
	}

	return logs, nil
}

func (r *ClickHouseReader) Health(
	ctx context.Context,
	principal access.Principal,
	filters LogFilters,
) ([]HealthRow, error) {
	where, args, ok := buildLogWhere(principal, filters)
	if !ok {
		return []HealthRow{}, nil
	}

	limit := normalizeLimit(filters.Limit)
	args = append(args, limit)

	query := fmt.Sprintf(
		`
		SELECT
			ApplicationName,
			count() AS total_count,
			countIf(Level = 'WARN') AS warn_count,
			countIf(Level = 'ERROR') AS error_count,
			countIf(Level = 'CRITICAL') AS critical_count,
			max(Timestamp) AS last_seen_at
		FROM logs_table
		%s
		GROUP BY ApplicationName
		ORDER BY critical_count DESC, error_count DESC, warn_count DESC, total_count DESC
		LIMIT ?
		`,
		where,
	)

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query health analytics: %w", err)
	}
	defer rows.Close()

	result := make([]HealthRow, 0, limit)
	for rows.Next() {
		var row HealthRow
		if err := rows.Scan(
			&row.ApplicationName,
			&row.TotalCount,
			&row.WarnCount,
			&row.ErrorCount,
			&row.CriticalCount,
			&row.LastSeenAt,
		); err != nil {
			return nil, fmt.Errorf("scan health row: %w", err)
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate health rows: %w", err)
	}

	return result, nil
}

func buildLogWhere(
	principal access.Principal,
	filters LogFilters,
) (string, []any, bool) {
	clauses := make([]string, 0, 4)
	args := make([]any, 0)

	apps, ok := allowedApplications(principal, filters.Applications)
	if !ok {
		return "", nil, false
	}
	if len(apps) > 0 {
		clauses = append(clauses, "ApplicationName IN ("+placeholders(len(apps))+")")
		for _, app := range apps {
			args = append(args, app)
		}
	}

	if len(filters.Levels) > 0 {
		clauses = append(clauses, "Level IN ("+placeholders(len(filters.Levels))+")")
		for _, level := range filters.Levels {
			args = append(args, level)
		}
	}

	if !filters.From.IsZero() {
		clauses = append(clauses, "Timestamp >= ?")
		args = append(args, filters.From)
	}
	if !filters.To.IsZero() {
		clauses = append(clauses, "Timestamp <= ?")
		args = append(args, filters.To)
	}

	if len(clauses) == 0 {
		return "", args, true
	}
	return "WHERE " + strings.Join(clauses, " AND "), args, true
}

func allowedApplications(
	principal access.Principal,
	requested []string,
) ([]string, bool) {
	if principal.IsAdmin() {
		return requested, true
	}

	if len(requested) == 0 {
		if len(principal.Applications) == 0 {
			return nil, false
		}
		return principal.Applications, true
	}

	apps := make([]string, 0, len(requested))
	for _, app := range requested {
		if principal.CanAccessApplication(app) {
			apps = append(apps, app)
		}
	}
	return apps, len(apps) > 0
}

func placeholders(count int) string {
	values := make([]string, count)
	for i := range values {
		values[i] = "?"
	}
	return strings.Join(values, ",")
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > maxQueryLimit {
		return maxQueryLimit
	}
	return limit
}
