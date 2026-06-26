package config

import (
	"testing"
	"time"
)

func TestConfigDurationHelpers(t *testing.T) {
	cfg := Config{
		KafkaWriterBatchTimeoutMS:        100,
		IngestionProducerTimeoutMS:       3000,
		ProcessedLogProducerTimeoutMS:    4000,
		ProcessorFlushIntervalMS:         500,
		ProcessorSaveTimeoutMS:           5000,
		ProcessorShutdownMS:              7000,
		ClickHouseConnMaxLifetimeMinutes: 60,
	}

	if cfg.KafkaWriterBatchTimeout() != 100*time.Millisecond {
		t.Fatalf("KafkaWriterBatchTimeout() = %s", cfg.KafkaWriterBatchTimeout())
	}
	if cfg.IngestionProducerTimeout() != 3*time.Second {
		t.Fatalf("IngestionProducerTimeout() = %s", cfg.IngestionProducerTimeout())
	}
	if cfg.ProcessedLogProducerTimeout() != 4*time.Second {
		t.Fatalf("ProcessedLogProducerTimeout() = %s", cfg.ProcessedLogProducerTimeout())
	}
	if cfg.ProcessorFlushInterval() != 500*time.Millisecond {
		t.Fatalf("ProcessorFlushInterval() = %s", cfg.ProcessorFlushInterval())
	}
	if cfg.ProcessorSaveTimeout() != 5*time.Second {
		t.Fatalf("ProcessorSaveTimeout() = %s", cfg.ProcessorSaveTimeout())
	}
	if cfg.ProcessorShutdownTimeout() != 7*time.Second {
		t.Fatalf("ProcessorShutdownTimeout() = %s", cfg.ProcessorShutdownTimeout())
	}
	if cfg.ClickHouseConnMaxLifetime() != time.Hour {
		t.Fatalf("ClickHouseConnMaxLifetime() = %s", cfg.ClickHouseConnMaxLifetime())
	}
}
