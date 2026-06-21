package domain

type RawLogEvent struct {
	EventID    string    `json:"eventId"`
	ReceivedAt string    `json:"receivedAt"`
	Source     string    `json:"source"`
	Payload    LogRecord `json:"payload"`
}
