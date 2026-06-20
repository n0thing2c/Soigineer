package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	Endpoint = "http://localhost:8080/v1/ingest/logs"

	NumApps    = 100 // N apps
	LogsPerSec = 20  // L logs/s cho mỗi app
)

type LogRequest struct {
	ApplicationName string `json:"applicationName"`
	Level           string `json:"level"`
	Message         string `json:"message"`
	Timestamp       string `json:"timestamp"`
	TraceID         string `json:"traceId"`
}

var levels = []string{"INFO", "WARN", "ERROR", "CRITICAL"}

var messages = []string{
	"Request completed",
	"Database connection timeout",
	"Cache miss",
	"User login success",
	"Payment failed",
	"External API error",
	"Service started",
	"Memory usage high",
}

func randomLog(appName string) LogRequest {
	return LogRequest{
		ApplicationName: appName,
		Level:           levels[rand.Intn(len(levels))],
		Message:         messages[rand.Intn(len(messages))],
		Timestamp:       time.Now().UTC().Format(time.RFC3339Nano),
		TraceID:         uuid.NewString(),
	}
}

func sendLog(client *http.Client, logReq LogRequest) error {
	payload, err := json.Marshal(logReq)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		Endpoint,
		bytes.NewBuffer(payload),
	)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("status=%d", resp.StatusCode)
	}

	return nil
}

func runApp(appName string, logsPerSec int) {
	appClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	interval := time.Second / time.Duration(logsPerSec)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var sent uint64

	for range ticker.C {
		logReq := randomLog(appName)

		go func() {
			if err := sendLog(appClient, logReq); err != nil {
				log.Printf("[%s] send error: %v", appName, err)
			}
		}()

		sent++

		if sent%1000 == 0 {
			log.Printf("[%s] sent=%d", appName, sent)
		}
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	log.Printf(
		"Starting simulator: apps=%d, logs/app=%d/s, total=%d logs/s",
		NumApps,
		LogsPerSec,
		NumApps*LogsPerSec,
	)

	var wg sync.WaitGroup

	for i := 1; i <= NumApps; i++ {
		appName := fmt.Sprintf("app-%03d", i)

		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			runApp(name, LogsPerSec)
		}(appName)
	}

	wg.Wait()
}
