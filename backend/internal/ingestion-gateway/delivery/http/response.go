package delivery

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type SuccessResponse struct {
	Status  string `json:"status"`
	TraceID string `json:"traceId,omitempty"`
	Count   int    `json:"count,omitempty"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// 202 Accepted
func RespondAccepted(c *gin.Context, traceID string) {
	c.JSON(http.StatusAccepted, SuccessResponse{
		Status:  "accepted",
		TraceID: traceID,
	})
}

func RespondBatchAccepted(c *gin.Context, count int) {
	c.JSON(http.StatusAccepted, SuccessResponse{
		Status: "accepted",
		Count:  count,
	})
}

// 400 Bad Request
func RespondValidationError(c *gin.Context, errCode string, message string) {
	c.JSON(http.StatusBadRequest, ErrorResponse{
		Error:   errCode,
		Message: message,
	})
}

// 503 Service Unavailable
func RespondSystemError(c *gin.Context, errCode string, message string) {
	c.JSON(http.StatusServiceUnavailable, ErrorResponse{
		Error:   errCode,
		Message: message,
	})
}
