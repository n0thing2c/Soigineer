package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/n0thing2c/Soigineer/internal/auth/repository"
	"github.com/n0thing2c/Soigineer/internal/auth/token"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials  = errors.New("invalid username or password")
	ErrForbidden           = errors.New("admin role is required")
	ErrInvalidRole         = errors.New("role must be admin or engineer")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
)

type AuthService struct {
	users           *repository.UserRepository
	refreshTokens   *repository.RefreshTokenRepository
	tokens          *token.Manager
	refreshTokenTTL time.Duration
}

type LoginResult struct {
	Token        string          `json:"token"`
	RefreshToken string          `json:"refreshToken"`
	User         repository.User `json:"user"`
}

func NewAuthService(
	users *repository.UserRepository,
	refreshTokens *repository.RefreshTokenRepository,
	tokens *token.Manager,
	refreshTokenTTL time.Duration,
) *AuthService {
	if refreshTokenTTL <= 0 {
		refreshTokenTTL = 7 * 24 * time.Hour
	}
	return &AuthService{
		users:           users,
		refreshTokens:   refreshTokens,
		tokens:          tokens,
		refreshTokenTTL: refreshTokenTTL,
	}
}

func (s *AuthService) BootstrapDefaults(
	ctx context.Context,
	adminPassword string,
	engineerPassword string,
) error {
	if err := s.refreshTokens.EnsureSchema(ctx); err != nil {
		return err
	}

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

	return s.issueTokenPair(ctx, user, time.Now().UTC())
}

func (s *AuthService) Refresh(
	ctx context.Context,
	rawRefreshToken string,
) (LoginResult, error) {
	rawRefreshToken = strings.TrimSpace(rawRefreshToken)
	if rawRefreshToken == "" {
		return LoginResult{}, ErrInvalidRefreshToken
	}

	now := time.Now().UTC()
	newRefreshToken, newRefreshTokenHash, err := generateRefreshToken()
	if err != nil {
		return LoginResult{}, err
	}

	userID, err := s.refreshTokens.Rotate(
		ctx,
		hashRefreshToken(rawRefreshToken),
		newRefreshTokenHash,
		now.Add(s.refreshTokenTTL),
		now,
	)
	if errors.Is(err, repository.ErrRefreshTokenNotFound) {
		return LoginResult{}, ErrInvalidRefreshToken
	}
	if err != nil {
		return LoginResult{}, err
	}

	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return LoginResult{}, err
	}

	accessToken, err := s.tokens.Issue(user.ID, user.Username, user.Role, now)
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		Token:        accessToken,
		RefreshToken: newRefreshToken,
		User:         user,
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, rawRefreshToken string) error {
	rawRefreshToken = strings.TrimSpace(rawRefreshToken)
	if rawRefreshToken == "" {
		return ErrInvalidRefreshToken
	}
	return s.refreshTokens.Revoke(ctx, hashRefreshToken(rawRefreshToken), time.Now().UTC())
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

func (s *AuthService) issueTokenPair(
	ctx context.Context,
	user repository.User,
	now time.Time,
) (LoginResult, error) {
	accessToken, err := s.tokens.Issue(user.ID, user.Username, user.Role, now)
	if err != nil {
		return LoginResult{}, err
	}

	refreshToken, refreshTokenHash, err := generateRefreshToken()
	if err != nil {
		return LoginResult{}, err
	}
	if err := s.refreshTokens.Save(
		ctx,
		user.ID,
		refreshTokenHash,
		now.Add(s.refreshTokenTTL),
	); err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		Token:        accessToken,
		RefreshToken: refreshToken,
		User:         user,
	}, nil
}

func generateRefreshToken() (string, string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	raw := base64.RawURLEncoding.EncodeToString(bytes)
	return raw, hashRefreshToken(raw), nil
}

func hashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}
