package delivery

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.RouterGroup, h *LogHandler) {
	IngestRoutes := r.Group("/ingest")
	{
		IngestRoutes.POST("/logs", h.SingleLogHandle)
		IngestRoutes.POST("/logs/batch", h.BatchLogHandle)
	}
}
