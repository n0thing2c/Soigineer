package config

import (
	"log"

	"github.com/caarlos0/env" // (hoặc phiên bản bạn đang dùng)
	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv      string `env:"APP_ENV" envDefault:"local"`
	GatewayPort string `env:"GATEWAY_PORT" envDefault:"8080"`
	KafkaPort   string `env:"REDPANDA_EXTERNAL_PORT" envDefault:"19092"`

	KafkaBrokers      []string `env:"REDPANDA_BROKERS" envDefault:"localhost:19092"`
	KafkaRawLogsTopic string `env:"REDPANDA_RAW_LOGS_TOPIC" envDefault:"raw-logs"`
}

func LoadConfig() *Config {
	cfg := &Config{}

	_ = godotenv.Load()
	if err := env.Parse(cfg); err != nil {
		log.Fatalf("Fail to parse system configuration: %v", err)
	}
	return cfg
}
