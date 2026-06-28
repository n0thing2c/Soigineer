package domain

import "time"

type ProcessedLogEvent struct {
	EventID           string    `json:"eventId"`
	ApplicationName   string    `json:"applicationName"`
	Level             string    `json:"level"`
	Category          string    `json:"category"`
	Message           string    `json:"message"`
	NormalizedMessage string    `json:"normalizedMessage"`
	Timestamp         time.Time `json:"timestamp"`
	ReceivedAt        time.Time `json:"receivedAt"`
	TraceID           string    `json:"traceId"`
	Fingerprint       string    `json:"fingerprint"`
}
