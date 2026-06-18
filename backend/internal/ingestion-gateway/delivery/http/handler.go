	package delivery

	import (
		"context"

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
			// Validation
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

		log := domain.LogRecord(req)

		// Queue error
		if err := h.ingestor.IngestSingleLog(ctx, log); err != nil {
			RespondSystemError(ctx, "QUEUE_UNAVAILABLE", "failed to publish log event")
			return
		}

		RespondAccepted(ctx, log.TraceID)
	}

	func (h *LogHandler) BatchLogHandle(ctx *gin.Context) {
		var req BatchLogRequest

		if err := ctx.ShouldBindJSON(&req); err != nil {
			RespondValidationError(ctx, "VALIDATION_FAILED", "One or more logs is invalid")
			return
		}

		logs := make([]domain.LogRecord, len(req.Logs))
		for idx, log := range req.Logs {
			logs[idx] = domain.LogRecord(log)
		}

		if err := h.ingestor.IngestBatchLog(ctx, logs); err != nil {
			RespondSystemError(ctx, "QUEUE_UNAVAILABLE", "failed to publish log event")
			return
		}
		RespondBatchAccepted(ctx, len(req.Logs))
	}
