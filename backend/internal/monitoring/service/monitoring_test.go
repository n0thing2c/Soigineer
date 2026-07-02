package service

import (
	"context"
	"errors"
	"testing"

	"github.com/n0thing2c/Soigineer/internal/monitoring/access"
	"github.com/n0thing2c/Soigineer/internal/monitoring/repository"
	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
)

func TestCreateAlertRuleRequiresAdmin(t *testing.T) {
	service := NewMonitoringService(
		fakePrincipals{principal: access.Principal{Role: access.RoleEngineer}},
		fakeLogs{},
		&fakeMetadata{},
	)

	_, err := service.CreateAlertRule(context.Background(), Credentials{Identity: "engineer"}, repository.AlertRuleCreate{
		ApplicationName:    "payment-service",
		Level:              "ERROR",
		DedupWindowSeconds: 60,
		Enabled:            true,
		TelegramEnabled:    true,
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestCreateAlertRuleValidatesPayload(t *testing.T) {
	service := newCreateRuleTestService(&fakeMetadata{})

	tests := []repository.AlertRuleCreate{
		{ApplicationName: "", Level: "ERROR", DedupWindowSeconds: 60},
		{ApplicationName: "payment-service", Level: "WARN", DedupWindowSeconds: 60},
		{ApplicationName: "payment-service", Level: "ERROR", DedupWindowSeconds: 0},
	}

	for _, input := range tests {
		_, err := service.CreateAlertRule(context.Background(), Credentials{Identity: "admin"}, input)
		if !errors.Is(err, ErrInvalidAlertRuleCreate) {
			t.Fatalf("expected ErrInvalidAlertRuleCreate for %+v, got %v", input, err)
		}
	}
}

func TestCreateAlertRuleMapsDuplicateToConflict(t *testing.T) {
	service := newCreateRuleTestService(&fakeMetadata{createErr: repository.ErrAlertRuleExists})

	_, err := service.CreateAlertRule(context.Background(), Credentials{Identity: "admin"}, repository.AlertRuleCreate{
		ApplicationName:    "payment-service",
		Level:              "ERROR",
		DedupWindowSeconds: 60,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestCreateAlertRuleCreatesNormalizedRule(t *testing.T) {
	metadata := &fakeMetadata{
		rule: repository.AlertRule{
			ID:                 "rule-1",
			ApplicationName:    "payment-service",
			Level:              "CRITICAL",
			DedupWindowSeconds: 60,
			Enabled:            true,
			TelegramEnabled:    true,
		},
	}
	service := newCreateRuleTestService(metadata)

	rule, err := service.CreateAlertRule(context.Background(), Credentials{Identity: "admin"}, repository.AlertRuleCreate{
		ApplicationName:    " payment-service ",
		Level:              "critical",
		DedupWindowSeconds: 60,
		Enabled:            true,
		TelegramEnabled:    true,
	})
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}
	if rule.ID != "rule-1" {
		t.Fatalf("unexpected rule: %+v", rule)
	}
	if metadata.create.ApplicationName != "payment-service" || metadata.create.Level != "CRITICAL" {
		t.Fatalf("create payload was not normalized: %+v", metadata.create)
	}
}

func newCreateRuleTestService(metadata *fakeMetadata) *MonitoringService {
	return NewMonitoringService(
		fakePrincipals{principal: access.Principal{Role: access.RoleAdmin}},
		fakeLogs{},
		metadata,
	)
}

type fakePrincipals struct {
	principal access.Principal
	err       error
}

func (f fakePrincipals) Load(context.Context, string) (access.Principal, error) {
	return f.principal, f.err
}

func (f fakePrincipals) LoadToken(context.Context, string) (access.Principal, error) {
	return f.principal, f.err
}

type fakeLogs struct{}

func (fakeLogs) ListLogs(context.Context, access.Principal, repository.LogFilters) ([]sharedDomain.ProcessedLogEvent, error) {
	return nil, nil
}

func (fakeLogs) Health(context.Context, access.Principal, repository.LogFilters) ([]repository.HealthRow, error) {
	return nil, nil
}

type fakeMetadata struct {
	create    repository.AlertRuleCreate
	rule      repository.AlertRule
	createErr error
}

func (f *fakeMetadata) ListApplications(context.Context, access.Principal) ([]string, error) {
	return nil, nil
}

func (f *fakeMetadata) ListIncidents(context.Context, access.Principal, repository.IncidentFilters) ([]repository.Incident, error) {
	return nil, nil
}

func (f *fakeMetadata) UpdateIncidentStatus(context.Context, string, string) error {
	return nil
}

func (f *fakeMetadata) ListAlertRules(context.Context) ([]repository.AlertRule, error) {
	return nil, nil
}

func (f *fakeMetadata) UpdateAlertRule(context.Context, string, repository.AlertRuleUpdate) error {
	return nil
}

func (f *fakeMetadata) CreateAlertRule(_ context.Context, create repository.AlertRuleCreate) (repository.AlertRule, error) {
	f.create = create
	return f.rule, f.createErr
}
