package domain

import "time"

type AlertEvent struct {
	EventID         string `json:"eventId"`
	ApplicationName string `json:"applicationName"`
	Level           string `json:"level"`
	Message         string `json:"message"`
	Fingerprint     string `json:"fingerprint"`
	TraceID         string `json:"traceId"`
	Timestamp       time.Time `json:"timestamp"`
}
