package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrUserNotFound = errors.New("user not found")

type User struct {
	ID           string   `json:"id"`
	Username     string   `json:"username"`
	Role         string   `json:"role"`
	Applications []string `json:"applications"`
	PasswordHash string   `json:"-"`
}

type CreateUserInput struct {
	Username     string
	Role         string
	PasswordHash string
	Applications []string
}

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) BootstrapDefaults(
	ctx context.Context,
	adminHash string,
	engineerHash string,
) error {
	if _, err := r.db.ExecContext(
		ctx,
		"ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT NOT NULL DEFAULT ''",
	); err != nil {
		return fmt.Errorf("ensure users.password_hash column: %w", err)
	}

	updates := []struct {
		username string
		hash     string
	}{
		{username: "admin", hash: adminHash},
		{username: "engineer-payment", hash: engineerHash},
	}

	for _, update := range updates {
		if strings.TrimSpace(update.hash) == "" {
			continue
		}
		if _, err := r.db.ExecContext(
			ctx,
			`
			UPDATE users
			SET password_hash = $2
			WHERE username = $1
				AND (password_hash IS NULL OR password_hash = '')
			`,
			update.username,
			update.hash,
		); err != nil {
			return fmt.Errorf("bootstrap password for %s: %w", update.username, err)
		}
	}

	return nil
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (User, error) {
	user, err := r.find(ctx, "username = $1", username)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	return user, err
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (User, error) {
	user, err := r.find(ctx, "id::text = $1", id)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	return user, err
}

func (r *UserRepository) List(ctx context.Context) ([]User, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`
		SELECT id::text, username, role, COALESCE(password_hash, '')
		FROM users
		ORDER BY username
		`,
	)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username, &user.Role, &user.PasswordHash); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		apps, err := r.loadApplications(ctx, user.ID)
		if err != nil {
			return nil, err
		}
		user.Applications = apps
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}

	return users, nil
}

func (r *UserRepository) Create(ctx context.Context, input CreateUserInput) (User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("begin create user transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var user User
	err = tx.QueryRowContext(
		ctx,
		`
		INSERT INTO users (username, role, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id::text, username, role, password_hash
		`,
		input.Username,
		input.Role,
		input.PasswordHash,
	).Scan(&user.ID, &user.Username, &user.Role, &user.PasswordHash)
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}

	if err = replaceApplications(ctx, tx, user.ID, input.Applications); err != nil {
		return User{}, err
	}
	if err = tx.Commit(); err != nil {
		return User{}, fmt.Errorf("commit create user transaction: %w", err)
	}

	return r.FindByID(ctx, user.ID)
}

func (r *UserRepository) ReplaceApplications(
	ctx context.Context,
	userID string,
	applications []string,
) (User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("begin replace user applications transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = replaceApplications(ctx, tx, userID, applications); err != nil {
		return User{}, err
	}
	if err = tx.Commit(); err != nil {
		return User{}, fmt.Errorf("commit replace user applications transaction: %w", err)
	}

	return r.FindByID(ctx, userID)
}

func (r *UserRepository) ListApplications(ctx context.Context) ([]string, error) {
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

func (r *UserRepository) find(ctx context.Context, condition string, arg string) (User, error) {
	var user User
	err := r.db.QueryRowContext(
		ctx,
		fmt.Sprintf(
			`
			SELECT id::text, username, role, COALESCE(password_hash, '')
			FROM users
			WHERE %s
			`,
			condition,
		),
		arg,
	).Scan(&user.ID, &user.Username, &user.Role, &user.PasswordHash)
	if err != nil {
		return User{}, err
	}

	apps, err := r.loadApplications(ctx, user.ID)
	if err != nil {
		return User{}, err
	}
	user.Applications = apps

	return user, nil
}

func (r *UserRepository) loadApplications(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`
		SELECT a.name
		FROM applications a
		JOIN user_applications ua ON ua.application_id = a.id
		WHERE ua.user_id = $1
		ORDER BY a.name
		`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("load user applications: %w", err)
	}
	defer rows.Close()

	apps := make([]string, 0)
	for rows.Next() {
		var app string
		if err := rows.Scan(&app); err != nil {
			return nil, fmt.Errorf("scan user application: %w", err)
		}
		apps = append(apps, app)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user applications: %w", err)
	}

	return apps, nil
}

func replaceApplications(ctx context.Context, tx *sql.Tx, userID string, applications []string) error {
	if _, err := tx.ExecContext(
		ctx,
		"DELETE FROM user_applications WHERE user_id = $1",
		userID,
	); err != nil {
		return fmt.Errorf("delete user applications: %w", err)
	}

	for _, app := range applications {
		app = strings.TrimSpace(app)
		if app == "" {
			continue
		}
		if _, err := tx.ExecContext(
			ctx,
			`
			INSERT INTO user_applications (user_id, application_id)
			SELECT $1, id
			FROM applications
			WHERE name = $2
			ON CONFLICT DO NOTHING
			`,
			userID,
			app,
		); err != nil {
			return fmt.Errorf("assign application %q to user: %w", app, err)
		}
	}

	return nil
}
