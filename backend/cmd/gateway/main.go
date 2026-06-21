package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	delivery "github.com/n0thing2c/Soigineer/internal/ingestion-gateway/delivery/http"
	"github.com/n0thing2c/Soigineer/internal/ingestion-gateway/infrastructure/queue"
	"github.com/n0thing2c/Soigineer/internal/ingestion-gateway/service"
	"github.com/n0thing2c/Soigineer/internal/shared/config"
)

func main() {

	cfg := config.LoadConfig()

	producer := queue.NewRedpandaLogProducer(
		cfg.KafkaBrokers,
		cfg.KafkaWriterBatchSize,
		cfg.KafkaWriterBatchTimeout(),
	)
	defer producer.Close()

	ingestionService := service.NewIngestionService(
		producer,
		cfg.KafkaRawLogsTopic,
		cfg.IngestionProducerTimeout(),
	)
	logHandler := delivery.NewLogHandler(ingestionService)

	engine := gin.Default()

	engine.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
		c.Next()
	})

	v1 := engine.Group("/v1")
	delivery.RegisterRoutes(v1, logHandler)

	address := ":" + cfg.GatewayPort
	log.Printf("Ingestion Gateway is running at http://localhost%s", address)
	if err := engine.Run(address); err != nil {
		log.Fatalf("Fail to start server: %v", err)
	}
}
