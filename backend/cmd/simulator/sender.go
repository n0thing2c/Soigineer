package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type jobType string

const (
	jobSingle jobType = "single"
	jobBatch  jobType = "batch"
)

type Job struct {
	Kind jobType
	Logs []LogRequest
}

type Result struct {
	Kind       jobType
	Logs       int
	StatusCode int
	Latency    time.Duration
	Err        error
}

type Sender struct {
	client        *http.Client
	singleURL     string
	batchURL      string
	requestHeader http.Header
}

func NewSender(cfg Config) *Sender {
	transport := &http.Transport{
		MaxIdleConns:        cfg.WorkerCount * 4,
		MaxIdleConnsPerHost: cfg.WorkerCount * 4,
		MaxConnsPerHost:     cfg.WorkerCount * 8,
		IdleConnTimeout:     90 * time.Second,
	}

	return &Sender{
		client: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: transport,
		},
		singleURL: cfg.SingleEndpoint(),
		batchURL:  cfg.BatchEndpoint(),
		requestHeader: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}
}

func (s *Sender) Send(ctx context.Context, job Job) Result {
	start := time.Now()
	url := s.batchURL

	var body []byte
	var err error
	if job.Kind == jobSingle {
		url = s.singleURL
		body, err = marshalSingleLog(job.Logs[0])
	} else {
		body, err = marshalBatchLogs(job.Logs)
	}
	if err != nil {
		return Result{Kind: job.Kind, Logs: len(job.Logs), Err: err}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Result{Kind: job.Kind, Logs: len(job.Logs), Err: err}
	}
	req.Header = s.requestHeader.Clone()

	resp, err := s.client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return Result{Kind: job.Kind, Logs: len(job.Logs), Latency: latency, Err: err}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= http.StatusMultipleChoices {
		return Result{
			Kind:       job.Kind,
			Logs:       len(job.Logs),
			StatusCode: resp.StatusCode,
			Latency:    latency,
			Err:        fmt.Errorf("unexpected status code %d", resp.StatusCode),
		}
	}

	return Result{
		Kind:       job.Kind,
		Logs:       len(job.Logs),
		StatusCode: resp.StatusCode,
		Latency:    latency,
	}
}

func marshalSingleLog(log LogRequest) ([]byte, error) {
	return json.Marshal(log)
}

func marshalBatchLogs(logs []LogRequest) ([]byte, error) {
	payload := BatchLogRequest{Logs: logs}
	return json.Marshal(payload)
}
