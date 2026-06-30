package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidToken = errors.New("invalid auth token")
	ErrExpiredToken = errors.New("auth token expired")
)

type Claims struct {
	Subject  string `json:"sub"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Expires  int64  `json:"exp"`
}

type Manager struct {
	secret []byte
	ttl    time.Duration
}

func NewManager(secret string, ttl time.Duration) *Manager {
	if ttl <= 0 {
		ttl = 8 * time.Hour
	}
	return &Manager{
		secret: []byte(strings.TrimSpace(secret)),
		ttl:    ttl,
	}
}

func (m *Manager) Issue(userID, username, role string, now time.Time) (string, error) {
	if len(m.secret) == 0 {
		return "", errors.New("auth token secret is required")
	}
	claims := Claims{
		Subject:  userID,
		Username: username,
		Role:     role,
		Expires:  now.Add(m.ttl).Unix(),
	}

	header := encodeJSON(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	payload := encodeJSON(claims)
	body := header + "." + payload
	signature := sign(body, m.secret)

	return body + "." + signature, nil
}

func (m *Manager) Verify(raw string, now time.Time) (Claims, error) {
	if len(m.secret) == 0 {
		return Claims{}, errors.New("auth token secret is required")
	}

	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalidToken
	}

	body := parts[0] + "." + parts[1]
	expected := sign(body, m.secret)
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return Claims{}, ErrInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, fmt.Errorf("%w: decode payload", ErrInvalidToken)
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return Claims{}, fmt.Errorf("%w: decode claims", ErrInvalidToken)
	}
	if claims.Subject == "" || claims.Username == "" || claims.Role == "" {
		return Claims{}, ErrInvalidToken
	}
	if now.Unix() >= claims.Expires {
		return Claims{}, ErrExpiredToken
	}

	return claims, nil
}

func encodeJSON(value any) string {
	data, _ := json.Marshal(value)
	return base64.RawURLEncoding.EncodeToString(data)
}

func sign(body string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
