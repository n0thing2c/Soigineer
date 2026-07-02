package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	alertService "github.com/n0thing2c/Soigineer/internal/alerting/service"
	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
)

type AlertRuleResolver struct {
	db *sql.DB
}

func NewAlertRuleResolver(db *sql.DB) *AlertRuleResolver {
	return &AlertRuleResolver{db: db}
}

func (r *AlertRuleResolver) Resolve(
	ctx context.Context,
	alert sharedDomain.AlertEvent,
) (alertService.AlertRule, error) {
	rule := alertService.AlertRule{
		Enabled:         true,
		TelegramEnabled: true,
	}
	if r == nil || r.db == nil {
		return rule, nil
	}

	var dedupWindowSeconds int
	err := r.db.QueryRowContext(
		ctx,
		`
		SELECT ar.enabled, ar.dedup_window_seconds, ar.telegram_enabled
		FROM alert_rules ar
		JOIN applications a ON a.id = ar.application_id
		WHERE a.name = $1
			AND ar.level = $2
		`,
		alert.ApplicationName,
		alert.Level,
	).Scan(&rule.Enabled, &dedupWindowSeconds, &rule.TelegramEnabled)
	if errors.Is(err, sql.ErrNoRows) {
		return rule, nil
	}
	if err != nil {
		return alertService.AlertRule{}, fmt.Errorf("resolve alert rule app=%s level=%s: %w", alert.ApplicationName, alert.Level, err)
	}

	if dedupWindowSeconds > 0 {
		rule.DedupWindow = time.Duration(dedupWindowSeconds) * time.Second
	}
	return rule, nil
}
