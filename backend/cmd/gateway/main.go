package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/n0thing2c/Soigineer/internal/infrastructure/queue"
	delivery "github.com/n0thing2c/Soigineer/internal/ingestion-gateway/delivery/http"
	"github.com/n0thing2c/Soigineer/internal/ingestion-gateway/service"
	"github.com/n0thing2c/Soigineer/internal/shared/config"
)

func main() {

	cfg := config.LoadConfig()

	producer := queue.NewRedpandaLogProducer(cfg.KafkaBrokers)
	defer producer.Close()

	// Tiêm mockProducer vào Service như bình thường
	ingestionService := service.NewIngestionService(producer, "raw-logs", 3*time.Second)
	logHandler := delivery.NewLogHandler(ingestionService)

	engine := gin.Default()

	//middleware limit stream size -> max 2MB
	engine.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
		c.Next()
	})

	v1 := engine.Group("/v1")
	delivery.RegisterRoutes(v1, logHandler)

	log.Println("Ingestion Gateway is running at http://localhost:8080")
	if err := engine.Run(":8080"); err != nil {
		log.Fatalf("Fail to start server: %v", err)
	}
}
