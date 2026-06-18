package delivery

type LogRequest struct {
	ApplicationName string `json:"applicationName" binding:"required"`
	Level           string `json:"level" binding:"required,oneof=INFO WARN ERROR CRITICAL"`
	Message         string `json:"message" binding:"required"`
	Timestamp       string `json:"timestamp" binding:"required"`
	TraceID         string `json:"traceId" binding:"required"`
}
type BatchLogRequest struct {
	Logs []LogRequest `json:"logs" binding:"required,min=1,max=500,dive"`
}
