package access

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	authToken "github.com/n0thing2c/Soigineer/internal/auth/token"
	realtimeService "github.com/n0thing2c/Soigineer/internal/realtime/service"
)

const (
	RoleAdmin    = "admin"
	RoleEngineer = "engineer"
)

var ErrPrincipalNotFound = errors.New("principal not found")

type Principal struct {
	UserID       string   `json:"userId"`
	Username     string   `json:"username"`
	Role         string   `json:"role"`
	Applications []string `json:"applications"`
	appSet       map[string]bool
}

type PrincipalStore struct {
	db     *sql.DB
	tokens *authToken.Manager
}

func NewPrincipalStore(db *sql.DB) *PrincipalStore {
	return &PrincipalStore{db: db}
}

func NewPrincipalStoreWithToken(db *sql.DB, tokens *authToken.Manager) *PrincipalStore {
	return &PrincipalStore{
		db:     db,
		tokens: tokens,
	}
}

func (s *PrincipalStore) Load(ctx context.Context, identity string) (Principal, error) {
	identity = strings.TrimSpace(identity)
	if identity == "" {
		return Principal{}, ErrPrincipalNotFound
	}

	var principal Principal
	err := s.db.QueryRowContext(
		ctx,
		`
		SELECT id::text, username, role
		FROM users
		WHERE username = $1 OR id::text = $1
		`,
		identity,
	).Scan(&principal.UserID, &principal.Username, &principal.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return Principal{}, ErrPrincipalNotFound
	}
	if err != nil {
		return Principal{}, fmt.Errorf("load principal %q: %w", identity, err)
	}

	apps, err := s.loadApplications(ctx, principal)
	if err != nil {
		return Principal{}, err
	}
	principal.Applications = apps
	principal.appSet = toSet(apps)

	return principal, nil
}

func (s *PrincipalStore) LoadToken(ctx context.Context, rawToken string) (Principal, error) {
	if s.tokens == nil {
		return Principal{}, ErrPrincipalNotFound
	}

	claims, err := s.tokens.Verify(strings.TrimSpace(rawToken), time.Now().UTC())
	if err != nil {
		return Principal{}, err
	}
	return s.Load(ctx, claims.Subject)
}

func (s *PrincipalStore) LoadRealtimePrincipal(
	ctx context.Context,
	identity string,
) (realtimeService.Principal, error) {
	principal, err := s.LoadToken(ctx, identity)
	if err != nil && strings.Count(identity, ".") != 2 {
		principal, err = s.Load(ctx, identity)
	}
	if err != nil {
		return realtimeService.Principal{}, err
	}
	return principal.ToRealtimePrincipal(), nil
}

func (s *PrincipalStore) loadApplications(ctx context.Context, principal Principal) ([]string, error) {
	query := `
		SELECT a.name
		FROM applications a
		ORDER BY a.name
	`
	args := []any{}

	if principal.Role != RoleAdmin {
		query = `
			SELECT a.name
			FROM applications a
			JOIN user_applications ua ON ua.application_id = a.id
			WHERE ua.user_id = $1
			ORDER BY a.name
		`
		args = append(args, principal.UserID)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("load applications for principal %q: %w", principal.Username, err)
	}
	defer rows.Close()

	apps := make([]string, 0)
	for rows.Next() {
		var app string
		if err := rows.Scan(&app); err != nil {
			return nil, fmt.Errorf("scan principal application: %w", err)
		}
		apps = append(apps, app)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate principal applications: %w", err)
	}

	return apps, nil
}

func (p Principal) IsAdmin() bool {
	return p.Role == RoleAdmin
}

func (p Principal) CanAccessApplication(app string) bool {
	if p.IsAdmin() {
		return true
	}
	if p.appSet == nil {
		return toSet(p.Applications)[app]
	}
	return p.appSet[app]
}

func (p Principal) ToRealtimePrincipal() realtimeService.Principal {
	return realtimeService.Principal{
		UserID:   p.UserID,
		Username: p.Username,
		Role:     p.Role,
		Apps:     toSet(p.Applications),
	}
}

func toSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			set[value] = true
		}
	}
	return set
}
