package domain

import "time"

type LogModel struct {
	ApplicationName   string
	Level             string
	Message           string
	NormalizedMessage string
	Timestamp         time.Time
	ReceivedAt        time.Time
	TraceID           string
	Fingerprint       string
}
