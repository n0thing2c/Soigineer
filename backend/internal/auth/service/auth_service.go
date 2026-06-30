package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/n0thing2c/Soigineer/internal/auth/repository"
	"github.com/n0thing2c/Soigineer/internal/auth/token"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrForbidden          = errors.New("admin role is required")
	ErrInvalidRole        = errors.New("role must be admin or engineer")
)

type AuthService struct {
	users  *repository.UserRepository
	tokens *token.Manager
}

type LoginResult struct {
	Token string          `json:"token"`
	User  repository.User `json:"user"`
}

func NewAuthService(users *repository.UserRepository, tokens *token.Manager) *AuthService {
	return &AuthService{
		users:  users,
		tokens: tokens,
	}
}

func (s *AuthService) BootstrapDefaults(
	ctx context.Context,
	adminPassword string,
	engineerPassword string,
) error {
	adminHash, err := hashPassword(adminPassword)
	if err != nil {
		return err
	}
	engineerHash, err := hashPassword(engineerPassword)
	if err != nil {
		return err
	}
	return s.users.BootstrapDefaults(ctx, adminHash, engineerHash)
}

func (s *AuthService) Login(
	ctx context.Context,
	username string,
	password string,
) (LoginResult, error) {
	user, err := s.users.FindByUsername(ctx, strings.TrimSpace(username))
	if errors.Is(err, repository.ErrUserNotFound) {
		return LoginResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginResult{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}

	rawToken, err := s.tokens.Issue(user.ID, user.Username, user.Role, time.Now().UTC())
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{Token: rawToken, User: user}, nil
}

func (s *AuthService) Verify(rawToken string) (token.Claims, error) {
	return s.tokens.Verify(rawToken, time.Now().UTC())
}

func (s *AuthService) RequireAdmin(ctx context.Context, rawToken string) (repository.User, error) {
	claims, err := s.Verify(rawToken)
	if err != nil {
		return repository.User{}, err
	}
	user, err := s.users.FindByID(ctx, claims.Subject)
	if err != nil {
		return repository.User{}, err
	}
	if user.Role != "admin" {
		return repository.User{}, ErrForbidden
	}
	return user, nil
}

func (s *AuthService) Me(ctx context.Context, rawToken string) (repository.User, error) {
	claims, err := s.Verify(rawToken)
	if err != nil {
		return repository.User{}, err
	}
	return s.users.FindByID(ctx, claims.Subject)
}

func (s *AuthService) ListUsers(ctx context.Context) ([]repository.User, error) {
	return s.users.List(ctx)
}

func (s *AuthService) ListApplications(ctx context.Context) ([]string, error) {
	return s.users.ListApplications(ctx)
}

func (s *AuthService) CreateUser(
	ctx context.Context,
	username string,
	password string,
	role string,
	applications []string,
) (repository.User, error) {
	role = strings.TrimSpace(role)
	if role != "admin" && role != "engineer" {
		return repository.User{}, ErrInvalidRole
	}

	passwordHash, err := hashPassword(password)
	if err != nil {
		return repository.User{}, err
	}

	user, err := s.users.Create(ctx, repository.CreateUserInput{
		Username:     strings.TrimSpace(username),
		Role:         role,
		PasswordHash: passwordHash,
		Applications: applications,
	})
	if err != nil {
		return repository.User{}, err
	}

	return user, nil
}

func (s *AuthService) ReplaceApplications(
	ctx context.Context,
	userID string,
	applications []string,
) (repository.User, error) {
	return s.users.ReplaceApplications(ctx, userID, applications)
}

func hashPassword(password string) (string, error) {
	password = strings.TrimSpace(password)
	if len(password) < 6 {
		return "", fmt.Errorf("password must be at least 6 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}
