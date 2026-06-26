package service

import (
	sharedDomain "github.com/n0thing2c/Soigineer/internal/shared/domain"
)

type StreamType string

const (
	StreamLogs   StreamType = "logs"
	StreamAlerts StreamType = "alerts"
)

type Principal struct {
	UserID string
	Role   string
	Apps   map[string]bool
}

type Subscription struct {
	Applications map[string]bool
	Levels       map[string]bool
}

func (s Subscription) MatchLog(log sharedDomain.ProcessedLogEvent) bool {
	if len(s.Applications) > 0 && !s.Applications[log.ApplicationName] {
		return false
	}

	if len(s.Levels) > 0 && !s.Levels[log.Level] {
		return false
	}

	return true
}

func (s Subscription) MatchAlert(alert sharedDomain.AlertEvent) bool {
	if len(s.Applications) > 0 && !s.Applications[alert.ApplicationName] {
		return false
	}

	if len(s.Levels) > 0 && !s.Levels[alert.Level] {
		return false
	}

	return true
}
