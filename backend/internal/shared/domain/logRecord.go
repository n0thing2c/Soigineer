package domain

type LogRecord struct {
	ApplicationName string `json:"applicationName"`
	Level           string `json:"level"`
	Message         string `json:"message"`
	Timestamp       string `json:"timestamp"`
	TraceID         string `json:"traceId"`
}
