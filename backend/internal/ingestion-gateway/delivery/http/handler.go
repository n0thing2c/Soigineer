package delivery

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/n0thing2c/Soigineer/internal/shared/domain"
)

	type LogHandler struct {
		ingestor LogIngestor
	}

	type LogIngestor interface {
		IngestSingleLog(ctx context.Context, log domain.LogRecord) error
		IngestBatchLog(ctx context.Context, logs []domain.LogRecord) error
	}

	func NewLogHandler(i LogIngestor) *LogHandler {
		return &LogHandler{ingestor: i}
	}

func (h *LogHandler) SingleLogHandle(ctx *gin.Context) {
	var req LogRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		if errs, ok := err.(validator.ValidationErrors); ok {
			firstErr := errs[0]

			var errorMsg string
			switch firstErr.Tag() {
			case "required":
				errorMsg = "Field '" + firstErr.Field() + "' must not empty."
			case "oneof":
				if firstErr.Field() == "Level" {
					errorMsg = "Field 'Level' must in: INFO, WARN, ERROR, CRITICAL."
				} else {
					errorMsg = "Field '" + firstErr.Field() + "' has invalid value."
				}
			default:
				errorMsg = "Field '" + firstErr.Field() + "' not pass validation."
			}

			RespondValidationError(ctx, "VALIDATION_FAILED", errorMsg)
			return
		}

		RespondValidationError(ctx, "BAD_REQUEST", "Invalid JSON.")
		return
	}

	if err := validateLogRequest(req); err != nil {
		RespondValidationError(ctx, "VALIDATION_FAILED", err.Error())
		return
	}

	logRecord := domain.LogRecord(req)
	if err := h.ingestor.IngestSingleLog(ctx, logRecord); err != nil {
		RespondSystemError(ctx, "QUEUE_UNAVAILABLE", "failed to publish log event")
		return
	}

	RespondAccepted(ctx, logRecord.TraceID)
}

func (h *LogHandler) BatchLogHandle(ctx *gin.Context) {
	var req BatchLogRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		RespondValidationError(ctx, "VALIDATION_FAILED", "One or more logs is invalid")
		return
	}

	logs := make([]domain.LogRecord, len(req.Logs))
	for idx, logReq := range req.Logs {
		if err := validateLogRequest(logReq); err != nil {
			RespondValidationError(ctx, "VALIDATION_FAILED", fmt.Sprintf("logs[%d]: %s", idx, err.Error()))
			return
		}
		logs[idx] = domain.LogRecord(logReq)
	}

	if err := h.ingestor.IngestBatchLog(ctx, logs); err != nil {
		RespondSystemError(ctx, "QUEUE_UNAVAILABLE", "failed to publish log event")
		return
	}
	RespondBatchAccepted(ctx, len(req.Logs))
}

func validateLogRequest(req LogRequest) error {
	if _, err := time.Parse(time.RFC3339Nano, req.Timestamp); err != nil {
		return fmt.Errorf("Field 'Timestamp' must be a valid RFC3339 timestamp")
	}
	return nil
}
