package postgres

import (
	"strings"
	"testing"
	"time"

	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
)

func TestBuildIncidentTitleCompactsWhitespaceAndTruncates(t *testing.T) {
	alert := sharedDomain.AlertEvent{
		ApplicationName: "payment-service",
		Level:           "ERROR",
		Category:        "DATABASE_ERROR",
		Message:         strings.Repeat("database timeout ", 20),
		Timestamp:       time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC),
	}

	title := buildIncidentTitle(alert, alert.Category)
	if len(title) > maxTitleLength {
		t.Fatalf("title length = %d, want <= %d", len(title), maxTitleLength)
	}
	if strings.Contains(title, "  ") {
		t.Fatalf("title contains repeated spaces: %q", title)
	}
	if !strings.HasPrefix(title, "[ERROR] payment-service: database timeout") {
		t.Fatalf("title = %q", title)
	}
}
