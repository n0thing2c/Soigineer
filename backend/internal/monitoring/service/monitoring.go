package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/n0thing2c/Soigineer/internal/monitoring/access"
	"github.com/n0thing2c/Soigineer/internal/monitoring/repository"
	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
)

var (
	ErrMissingIdentity        = errors.New("X-User-ID is required")
	ErrUnauthorized           = errors.New("unauthorized")
	ErrForbidden              = errors.New("admin role is required")
	ErrInvalidIncidentStatus  = errors.New("status must be OPEN, ACKED, or RESOLVED")
	ErrInvalidAlertRuleUpdate = errors.New("dedupWindowSeconds must be greater than 0")
	ErrNotFound               = errors.New("resource not found")
)

type Credentials struct {
	BearerToken string
	Identity    string
}

type PrincipalStore interface {
	Load(ctx context.Context, identity string) (access.Principal, error)
	LoadToken(ctx context.Context, rawToken string) (access.Principal, error)
}

type LogReader interface {
	ListLogs(context.Context, access.Principal, repository.LogFilters) ([]sharedDomain.ProcessedLogEvent, error)
	Health(context.Context, access.Principal, repository.LogFilters) ([]repository.HealthRow, error)
}

type MetadataReader interface {
	ListApplications(context.Context, access.Principal) ([]string, error)
	ListIncidents(context.Context, access.Principal, repository.IncidentFilters) ([]repository.Incident, error)
	UpdateIncidentStatus(context.Context, string, string) error
	ListAlertRules(context.Context) ([]repository.AlertRule, error)
	UpdateAlertRule(context.Context, string, repository.AlertRuleUpdate) error
}

type MonitoringService struct {
	principals PrincipalStore
	logs       LogReader
	metadata   MetadataReader
}

func NewMonitoringService(
	principals PrincipalStore,
	logs LogReader,
	metadata MetadataReader,
) *MonitoringService {
	return &MonitoringService{
		principals: principals,
		logs:       logs,
		metadata:   metadata,
	}
}

func (s *MonitoringService) Me(
	ctx context.Context,
	credentials Credentials,
) (access.Principal, error) {
	return s.resolvePrincipal(ctx, credentials)
}

func (s *MonitoringService) ListApplications(
	ctx context.Context,
	credentials Credentials,
) ([]string, error) {
	principal, err := s.resolvePrincipal(ctx, credentials)
	if err != nil {
		return nil, err
	}
	return s.metadata.ListApplications(ctx, principal)
}

func (s *MonitoringService) ListLogs(
	ctx context.Context,
	credentials Credentials,
	filters repository.LogFilters,
) ([]sharedDomain.ProcessedLogEvent, error) {
	principal, err := s.resolvePrincipal(ctx, credentials)
	if err != nil {
		return nil, err
	}
	return s.logs.ListLogs(ctx, principal, filters)
}

func (s *MonitoringService) ListIncidents(
	ctx context.Context,
	credentials Credentials,
	filters repository.IncidentFilters,
) ([]repository.Incident, error) {
	principal, err := s.resolvePrincipal(ctx, credentials)
	if err != nil {
		return nil, err
	}
	return s.metadata.ListIncidents(ctx, principal, filters)
}

func (s *MonitoringService) UpdateIncidentStatus(
	ctx context.Context,
	credentials Credentials,
	id string,
	status string,
) error {
	if _, err := s.requireAdmin(ctx, credentials); err != nil {
		return err
	}

	status = normalizeIncidentStatus(status)
	if !validIncidentStatus(status) {
		return ErrInvalidIncidentStatus
	}

	if err := s.metadata.UpdateIncidentStatus(ctx, id, status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (s *MonitoringService) Health(
	ctx context.Context,
	credentials Credentials,
	filters repository.LogFilters,
) ([]repository.HealthRow, error) {
	principal, err := s.resolvePrincipal(ctx, credentials)
	if err != nil {
		return nil, err
	}
	return s.logs.Health(ctx, principal, filters)
}

func (s *MonitoringService) ListAlertRules(
	ctx context.Context,
	credentials Credentials,
) ([]repository.AlertRule, error) {
	if _, err := s.requireAdmin(ctx, credentials); err != nil {
		return nil, err
	}
	return s.metadata.ListAlertRules(ctx)
}

func (s *MonitoringService) UpdateAlertRule(
	ctx context.Context,
	credentials Credentials,
	id string,
	update repository.AlertRuleUpdate,
) error {
	if _, err := s.requireAdmin(ctx, credentials); err != nil {
		return err
	}
	if update.DedupWindowSeconds <= 0 {
		return ErrInvalidAlertRuleUpdate
	}

	if err := s.metadata.UpdateAlertRule(ctx, id, update); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (s *MonitoringService) resolvePrincipal(
	ctx context.Context,
	credentials Credentials,
) (access.Principal, error) {
	if strings.TrimSpace(credentials.BearerToken) != "" {
		principal, err := s.principals.LoadToken(ctx, credentials.BearerToken)
		if err != nil {
			return access.Principal{}, fmt.Errorf("%w: %v", ErrUnauthorized, err)
		}
		return principal, nil
	}

	identity := strings.TrimSpace(credentials.Identity)
	if identity == "" {
		return access.Principal{}, ErrMissingIdentity
	}

	principal, err := s.principals.Load(ctx, identity)
	if errors.Is(err, access.ErrPrincipalNotFound) {
		return access.Principal{}, fmt.Errorf("%w: %v", ErrUnauthorized, err)
	}
	if err != nil {
		return access.Principal{}, err
	}
	return principal, nil
}

func (s *MonitoringService) requireAdmin(
	ctx context.Context,
	credentials Credentials,
) (access.Principal, error) {
	principal, err := s.resolvePrincipal(ctx, credentials)
	if err != nil {
		return access.Principal{}, err
	}
	if !principal.IsAdmin() {
		return access.Principal{}, ErrForbidden
	}
	return principal, nil
}

func normalizeIncidentStatus(status string) string {
	return strings.ToUpper(strings.TrimSpace(status))
}

func validIncidentStatus(status string) bool {
	switch status {
	case "OPEN", "ACKED", "RESOLVED":
		return true
	default:
		return false
	}
}
