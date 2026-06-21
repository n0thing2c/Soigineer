package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

type LogRequest struct {
	ApplicationName string `json:"applicationName"`
	Level           string `json:"level"`
	Message         string `json:"message"`
	Timestamp       string `json:"timestamp"`
	TraceID         string `json:"traceId"`
}

type BatchLogRequest struct {
	Logs []LogRequest `json:"logs"`
}

type weightedLevel struct {
	Name   string
	Weight int
}

type Generator struct {
	rng      *rand.Rand
	messages []string
	levels   []weightedLevel
	total    int
	runID    string
}

func NewGenerator(seed int64, runID string) *Generator {
	levels := []weightedLevel{
		{Name: "INFO", Weight: 55},
		{Name: "WARN", Weight: 25},
		{Name: "ERROR", Weight: 15},
		{Name: "CRITICAL", Weight: 5},
	}

	total := 0
	for _, level := range levels {
		total += level.Weight
	}

	return &Generator{
		rng: rand.New(rand.NewSource(seed)),
		messages: []string{
			"Request completed successfully",
			"Cache miss for session lookup",
			"Database connection timeout after 3000ms",
			"Payment failed due to insufficient funds",
			"External API returned 503",
			"Worker restarted after panic recovery",
			"Memory usage above warning threshold",
			"User login completed",
		},
		levels: levels,
		total:  total,
		runID:  runID,
	}
}

func (g *Generator) NextLog(appName string) LogRequest {
	return LogRequest{
		ApplicationName: appName,
		Level:           g.nextLevel(),
		Message:         g.nextMessage(),
		Timestamp:       time.Now().UTC().Format(time.RFC3339Nano),
		TraceID:         g.runID + "-" + uuid.NewString(),
	}
}

func (g *Generator) nextLevel() string {
	pick := g.rng.Intn(g.total)
	acc := 0
	for _, level := range g.levels {
		acc += level.Weight
		if pick < acc {
			return level.Name
		}
	}
	return "INFO"
}

func (g *Generator) nextMessage() string {
	message := g.messages[g.rng.Intn(len(g.messages))]
	return fmt.Sprintf("%s [sample=%d]", message, g.rng.Intn(1000))
}
