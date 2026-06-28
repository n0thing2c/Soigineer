package domain

import "time"

type LogModel struct {
	EventID           string
	ApplicationName   string
	Level             string
	Category          string
	Message           string
	NormalizedMessage string
	Timestamp         time.Time
	ReceivedAt        time.Time
	TraceID           string
	Fingerprint       string
}
