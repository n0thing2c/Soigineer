package main

import (
	"flag"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type Mode string

const (
	ModeSingle Mode = "single"
	ModeBatch  Mode = "batch"
	ModeMixed  Mode = "mixed"
)

type Config struct {
	BaseURL        string
	ServerCount    int
	LogsPerSec     int
	Mode           Mode
	BatchSize      int
	SingleRatio    float64
	Duration       time.Duration
	RunForever     bool
	Timeout        time.Duration
	RetryCount     int
	RetryBackoff   time.Duration
	Seed           int64
	ProgressEvery  time.Duration
	WorkerCount    int
	DispatchBuffer int
	ReportFile     string
	ReportWait     time.Duration
	KafkaBrokers   []string
	KafkaTopic     string
	ClickHouseHost string
	ClickHousePort string
	ClickHouseDB   string
	ClickHouseUser string
	ClickHousePass string
}

func ParseConfig(args []string) (Config, error) {
	cfg := Config{}
	fs := flag.NewFlagSet("simulator", flag.ContinueOnError)
	var kafkaBrokers string

	mode := fs.String("mode", string(ModeSingle), "Load mode: single|batch|mixed")
	fs.StringVar(&cfg.BaseURL, "base-url", "http://localhost:8080", "Gateway base URL")
	fs.IntVar(&cfg.ServerCount, "server-count", 10, "Number of simulated servers")
	fs.IntVar(&cfg.LogsPerSec, "logs-per-sec", 10, "Logs per second per server")
	fs.IntVar(&cfg.BatchSize, "batch-size", 50, "Batch size for batch or mixed mode")
	fs.Float64Var(&cfg.SingleRatio, "single-ratio", 0.5, "Single log ratio for mixed mode")
	fs.DurationVar(&cfg.Duration, "duration", 30*time.Second, "Run duration, example: 30s or 2m")
	fs.BoolVar(&cfg.RunForever, "run-forever", false, "Run until interrupted")
	fs.DurationVar(&cfg.Timeout, "timeout", 5*time.Second, "HTTP client timeout")
	fs.IntVar(&cfg.RetryCount, "retry-count", 3, "Number of retries for failed HTTP requests")
	fs.DurationVar(&cfg.RetryBackoff, "retry-backoff", 200*time.Millisecond, "Initial backoff between retries")
	fs.Int64Var(&cfg.Seed, "seed", time.Now().UnixNano(), "Seed for reproducible workload")
	fs.DurationVar(&cfg.ProgressEvery, "progress-every", 5*time.Second, "Progress output interval")
	fs.IntVar(&cfg.WorkerCount, "worker-count", 0, "Number of HTTP workers; 0 means auto")
	fs.IntVar(&cfg.DispatchBuffer, "dispatch-buffer", 0, "Job queue buffer; 0 means auto")
	fs.StringVar(&cfg.ReportFile, "report-file", "", "Markdown report output path; empty means auto")
	fs.DurationVar(&cfg.ReportWait, "report-wait", 15*time.Second, "Wait time after load to observe queue-to-DB progress")
	fs.StringVar(&kafkaBrokers, "kafka-brokers", "localhost:19092", "Comma-separated Redpanda/Kafka brokers")
	fs.StringVar(&cfg.KafkaTopic, "kafka-topic", "raw-logs", "Redpanda/Kafka topic for raw logs")
	fs.StringVar(&cfg.ClickHouseHost, "clickhouse-host", "localhost", "ClickHouse host for report verification")
	fs.StringVar(&cfg.ClickHousePort, "clickhouse-port", "9000", "ClickHouse port for report verification")
	fs.StringVar(&cfg.ClickHouseDB, "clickhouse-db", "logs_db", "ClickHouse database for report verification")
	fs.StringVar(&cfg.ClickHouseUser, "clickhouse-user", "admin", "ClickHouse user for report verification")
	fs.StringVar(&cfg.ClickHousePass, "clickhouse-password", "secret123", "ClickHouse password for report verification")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	cfg.Mode = Mode(strings.ToLower(strings.TrimSpace(*mode)))
	cfg.KafkaBrokers = splitAndTrim(kafkaBrokers)
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	if cfg.WorkerCount == 0 {
		cfg.WorkerCount = maxInt(8, cfg.ServerCount*2)
	}
	if cfg.DispatchBuffer == 0 {
		cfg.DispatchBuffer = maxInt(1024, cfg.ServerCount*cfg.BatchSize*2)
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if c.ServerCount <= 0 {
		return fmt.Errorf("server-count must be greater than 0")
	}
	if c.LogsPerSec <= 0 {
		return fmt.Errorf("logs-per-sec must be greater than 0")
	}
	if c.BatchSize <= 0 || c.BatchSize > 500 {
		return fmt.Errorf("batch-size must be between 1 and 500")
	}
	if c.Mode != ModeSingle && c.Mode != ModeBatch && c.Mode != ModeMixed {
		return fmt.Errorf("mode must be one of: single, batch, mixed")
	}
	if c.Mode == ModeMixed && (c.SingleRatio < 0 || c.SingleRatio > 1) {
		return fmt.Errorf("single-ratio must be between 0 and 1")
	}
	if c.Duration <= 0 && !c.RunForever {
		return fmt.Errorf("duration must be greater than 0 unless run-forever is enabled")
	}
	if c.Duration > 0 && c.RunForever {
		return fmt.Errorf("duration and run-forever cannot be used together")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than 0")
	}
	if c.RetryCount < 0 {
		return fmt.Errorf("retry-count must be greater than or equal to 0")
	}
	if c.RetryBackoff < 0 {
		return fmt.Errorf("retry-backoff must be greater than or equal to 0")
	}
	if c.ProgressEvery <= 0 {
		return fmt.Errorf("progress-every must be greater than 0")
	}
	if c.ReportWait < 0 {
		return fmt.Errorf("report-wait must be greater than or equal to 0")
	}
	if len(c.KafkaBrokers) == 0 {
		return fmt.Errorf("kafka-brokers must contain at least one broker")
	}
	if strings.TrimSpace(c.KafkaTopic) == "" {
		return fmt.Errorf("kafka-topic must not be empty")
	}
	if _, err := url.ParseRequestURI(strings.TrimRight(c.BaseURL, "/")); err != nil {
		return fmt.Errorf("base-url must be a valid URL: %w", err)
	}
	return nil
}

func (c Config) SingleEndpoint() string {
	return strings.TrimRight(c.BaseURL, "/") + "/v1/ingest/logs"
}

func (c Config) BatchEndpoint() string {
	return strings.TrimRight(c.BaseURL, "/") + "/v1/ingest/logs/batch"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func splitAndTrim(input string) []string {
	parts := strings.Split(input, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}
