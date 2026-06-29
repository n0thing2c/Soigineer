package config

import (
	"log"
	"time"

	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv         string `env:"APP_ENV" envDefault:"local"`
	GatewayPort    string `env:"GATEWAY_PORT" envDefault:"8080"`
	AuthPort       string `env:"AUTH_PORT" envDefault:"8070"`
	MonitoringPort string `env:"MONITORING_PORT" envDefault:"8090"`
	KafkaPort      string `env:"REDPANDA_EXTERNAL_PORT" envDefault:"19092"`

	KafkaBrokers                  []string `env:"REDPANDA_BROKERS" envDefault:"localhost:19092"`
	KafkaRawLogsTopic             string   `env:"REDPANDA_RAW_LOGS_TOPIC" envDefault:"raw-logs"`
	KafkaAlertTopic               string   `env:"REDPANDA_ALERT_TOPIC" envDefault:"alert"`
	KafkaProcessedLogsTopic       string   `env:"REDPANDA_PROCESSED_LOGS_TOPIC" envDefault:"processed-logs"`
	KafkaWriterBatchSize          int      `env:"REDPANDA_WRITER_BATCH_SIZE" envDefault:"1000"`
	KafkaWriterBatchTimeoutMS     int      `env:"REDPANDA_WRITER_BATCH_TIMEOUT_MS" envDefault:"100"`
	AlertProducerTimeoutMS        int      `env:"PROCESSOR_ALERT_PRODUCER_TIMEOUT_MS" envDefault:"3000"`
	ProcessedLogProducerTimeoutMS int      `env:"PROCESSOR_PROCESSED_LOG_PRODUCER_TIMEOUT_MS" envDefault:"3000"`

	IngestionProducerTimeoutMS int `env:"INGESTION_PRODUCER_TIMEOUT_MS" envDefault:"3000"`

	ProcessorConsumerGroup   string `env:"PROCESSOR_CONSUMER_GROUP" envDefault:"process-raw-log"`
	ProcessorBatchRows       int    `env:"PROCESSOR_BATCH_ROWS" envDefault:"1000"`
	ProcessorBatchBytes      int    `env:"PROCESSOR_BATCH_BYTES" envDefault:"4194304"`
	ProcessorFlushIntervalMS int    `env:"PROCESSOR_FLUSH_INTERVAL_MS" envDefault:"500"`
	ProcessorMessageBuffer   int    `env:"PROCESSOR_MESSAGE_BUFFER" envDefault:"2000"`
	ProcessorSaveTimeoutMS   int    `env:"PROCESSOR_SAVE_TIMEOUT_MS" envDefault:"5000"`
	ProcessorShutdownMS      int    `env:"PROCESSOR_SHUTDOWN_TIMEOUT_MS" envDefault:"5000"`

	ClickHouseHost                   string `env:"CLICKHOUSE_HOST" envDefault:"localhost"`
	ClickHousePort                   string `env:"CLICKHOUSE_PORT" envDefault:"9000"`
	ClickHouseDatabase               string `env:"CLICKHOUSE_DB" envDefault:"logs_db"`
	ClickHouseUser                   string `env:"CLICKHOUSE_USER" envDefault:"admin"`
	ClickHousePassword               string `env:"CLICKHOUSE_PASSWORD" envDefault:"secret123"`
	ClickHouseMaxOpenConns           int    `env:"CLICKHOUSE_MAX_OPEN_CONNS" envDefault:"10"`
	ClickHouseMaxIdleConns           int    `env:"CLICKHOUSE_MAX_IDLE_CONNS" envDefault:"10"`
	ClickHouseConnMaxLifetimeMinutes int    `env:"CLICKHOUSE_CONN_MAX_LIFETIME_MINUTES" envDefault:"60"`

	PostgresHost                   string `env:"POSTGRES_HOST" envDefault:"localhost"`
	PostgresPort                   string `env:"POSTGRES_PORT" envDefault:"5432"`
	PostgresDatabase               string `env:"POSTGRES_DB" envDefault:"soigineer_metadata"`
	PostgresUser                   string `env:"POSTGRES_USER" envDefault:"soigineer"`
	PostgresPassword               string `env:"POSTGRES_PASSWORD" envDefault:"soigineer_secret"`
	PostgresSSLMode                string `env:"POSTGRES_SSLMODE" envDefault:"disable"`
	PostgresMaxOpenConns           int    `env:"POSTGRES_MAX_OPEN_CONNS" envDefault:"10"`
	PostgresMaxIdleConns           int    `env:"POSTGRES_MAX_IDLE_CONNS" envDefault:"5"`
	PostgresConnMaxLifetimeMinutes int    `env:"POSTGRES_CONN_MAX_LIFETIME_MINUTES" envDefault:"30"`

	AuthTokenSecret           string `env:"AUTH_TOKEN_SECRET" envDefault:"dev-auth-token-secret-change-me"`
	AuthTokenTTLMinutes       int    `env:"AUTH_TOKEN_TTL_MINUTES" envDefault:"480"`
	BootstrapAdminPassword    string `env:"BOOTSTRAP_ADMIN_PASSWORD" envDefault:"admin123"`
	BootstrapEngineerPassword string `env:"BOOTSTRAP_ENGINEER_PASSWORD" envDefault:"engineer123"`

	TelegramBotToken  string `env:"TELEGRAM_BOT_TOKEN"`
	TelegramChatID    string `env:"TELEGRAM_CHAT_ID"`
	TelegramTimeoutMS int    `env:"TELEGRAM_TIMEOUT_MS" envDefault:"5000"`

	AlertConsumerGroup       string `env:"ALERT_CONSUMER_GROUP" envDefault:"alert-dispatcher"`
	RealtimeLogConsumerGroup string `env:"REALTIME_LOG_CONSUMER_GROUP" envDefault:"realtime-log-viewer"`
	RedisAddress             string `env:"REDIS_ADDRESS" envDefault:"localhost:6379"`
	AlertDedupPeriod         int    `env:"ALERT_DEDUP_PERIOD" envDefault:"60"`
}

func LoadConfig() *Config {
	cfg := &Config{}

	_ = godotenv.Load()
	if err := env.Parse(cfg); err != nil {
		log.Fatalf("Fail to parse system configuration: %v", err)
	}
	return cfg
}

func (c *Config) KafkaWriterBatchTimeout() time.Duration {
	return time.Duration(c.KafkaWriterBatchTimeoutMS) * time.Millisecond
}

func (c *Config) IngestionProducerTimeout() time.Duration {
	return time.Duration(c.IngestionProducerTimeoutMS) * time.Millisecond
}

func (c *Config) AlertProducerTimeout() time.Duration {
	return time.Duration(c.AlertProducerTimeoutMS) * time.Millisecond
}

func (c *Config) ProcessedLogProducerTimeout() time.Duration {
	return time.Duration(c.ProcessedLogProducerTimeoutMS) * time.Millisecond
}

func (c *Config) ProcessorFlushInterval() time.Duration {
	return time.Duration(c.ProcessorFlushIntervalMS) * time.Millisecond
}

func (c *Config) ProcessorSaveTimeout() time.Duration {
	return time.Duration(c.ProcessorSaveTimeoutMS) * time.Millisecond
}

func (c *Config) ProcessorShutdownTimeout() time.Duration {
	return time.Duration(c.ProcessorShutdownMS) * time.Millisecond
}

func (c *Config) ClickHouseConnMaxLifetime() time.Duration {
	return time.Duration(c.ClickHouseConnMaxLifetimeMinutes) * time.Minute
}

func (c *Config) PostgresConnMaxLifetime() time.Duration {
	return time.Duration(c.PostgresConnMaxLifetimeMinutes) * time.Minute
}

func (c *Config) AuthTokenTTL() time.Duration {
	return time.Duration(c.AuthTokenTTLMinutes) * time.Minute
}

func (c *Config) TelegramTimeout() time.Duration {
	return time.Duration(c.TelegramTimeoutMS) * time.Millisecond
}
