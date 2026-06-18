package service

import (
	"regexp"
	"strings"
)

var (
	uuidRegex   = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\b`)
	emailRegex  = regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)
	ipRegex     = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	numberRegex = regexp.MustCompile(`\b\d+\b`)
	longIDRegex = regexp.MustCompile(`\b[a-zA-Z0-9]{16,}\b`)
)

func NormalizeMessage(msg string) (original, normalized string) {
	normalized = strings.TrimSpace(msg)
	normalized = strings.Join(strings.Fields(normalized), " ")
	normalized = strings.ToLower(normalized)

	original = normalized

	normalized = uuidRegex.ReplaceAllString(normalized, "{uuid}")
	normalized = emailRegex.ReplaceAllString(normalized, "{email}")
	normalized = ipRegex.ReplaceAllString(normalized, "{ip}")
	normalized = longIDRegex.ReplaceAllString(normalized, "{id}")
	normalized = numberRegex.ReplaceAllString(normalized, "{number}")

	return original, normalized
}
