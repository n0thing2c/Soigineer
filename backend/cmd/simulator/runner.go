package main

import (
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"sync"
	"time"
)

type Runner struct {
	cfg     Config
	sender  *Sender
	metrics *Metrics
	output  io.Writer
	runID   string
}

func NewRunner(cfg Config, output io.Writer) *Runner {
	start := time.Now()
	return &Runner{
		cfg:     cfg,
		sender:  NewSender(cfg),
		metrics: NewMetrics(start),
		output:  output,
		runID:   newRunID(),
	}
}

func (r *Runner) Run(ctx context.Context) (RunResult, error) {
	result := RunResult{
		RunID:     r.runID,
		StartedAt: r.metrics.start.UTC(),
	}

	result.TopicBefore = captureTopicOffsets(context.Background(), r.cfg.KafkaBrokers, r.cfg.KafkaTopic)

	jobs := make(chan Job, r.cfg.DispatchBuffer)
	var producerWG sync.WaitGroup
	var workerWG sync.WaitGroup

	for workerID := 0; workerID < r.cfg.WorkerCount; workerID++ {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			for job := range jobs {
				result := r.sendWithRetry(ctx, job)
				r.metrics.Record(result)
			}
		}()
	}

	progressCtx, stopProgress := context.WithCancel(context.Background())
	reportDone := make(chan struct{})
	go func() {
		defer close(reportDone)
		r.reportProgress(progressCtx)
	}()

	for serverID := 1; serverID <= r.cfg.ServerCount; serverID++ {
		producerWG.Add(1)
		go func(id int) {
			defer producerWG.Done()
			serverSeed := r.cfg.Seed + int64(id*1000)
			if r.cfg.RunForever {
				r.runServerForever(ctx, id, rand.New(rand.NewSource(serverSeed)), jobs)
				return
			}
			r.runServerFixed(ctx, id, rand.New(rand.NewSource(serverSeed)), jobs)
		}(serverID)
	}

	producerWG.Wait()
	close(jobs)
	workerWG.Wait()
	stopProgress()
	<-reportDone

	r.metrics.Finish(time.Now())
	result.FinishedAt = time.Now().UTC()
	result.Snapshot = r.metrics.Snapshot()
	result.TopicAfter = captureTopicOffsets(context.Background(), r.cfg.KafkaBrokers, r.cfg.KafkaTopic)
	result.TopicDelta = result.TopicAfter.Total - result.TopicBefore.Total

	if result.Snapshot.FailedReqs > 0 {
		result.BenchmarkErr = fmt.Errorf("benchmark completed with %d failed requests", result.Snapshot.FailedReqs)
		return result, result.BenchmarkErr
	}
	return result, nil
}

func (r *Runner) sendWithRetry(ctx context.Context, job Job) Result {
	var result Result
	for attempt := 0; attempt <= r.cfg.RetryCount; attempt++ {
		result = r.sender.Send(ctx, job)
		if result.Err == nil {
			return result
		}
		if attempt == r.cfg.RetryCount || ctx.Err() != nil || r.cfg.RetryBackoff == 0 {
			return result
		}

		backoff := r.cfg.RetryBackoff * time.Duration(attempt+1)
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return result
		case <-timer.C:
		}
	}
	return result
}

func (r *Runner) runServerFixed(ctx context.Context, serverID int, rng *rand.Rand, jobs chan<- Job) {
	appName := fmt.Sprintf("app-%03d", serverID)
	serverGenerator := NewGenerator(r.cfg.Seed+int64(serverID), r.runID)
	targetLogs := int(math.Round(r.cfg.Duration.Seconds() * float64(r.cfg.LogsPerSec)))
	if targetLogs <= 0 {
		return
	}

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	started := time.Now()
	generated := 0
	batchBuffer := make([]LogRequest, 0, r.cfg.BatchSize)

	flushBatch := func(force bool) bool {
		if len(batchBuffer) == 0 || (!force && len(batchBuffer) < r.cfg.BatchSize) {
			return true
		}
		batch := append([]LogRequest(nil), batchBuffer...)
		batchBuffer = batchBuffer[:0]
		return submitJob(ctx, jobs, Job{Kind: jobBatch, Logs: batch})
	}

	for generated < targetLogs {
		select {
		case <-ctx.Done():
			return
		case tickTime := <-ticker.C:
			expected := int(math.Floor(tickTime.Sub(started).Seconds() * float64(r.cfg.LogsPerSec)))
			if expected > targetLogs {
				expected = targetLogs
			}
			due := expected - generated
			if due <= 0 {
				continue
			}

			generatedThisTick := 0
			for i := 0; i < due; i++ {
				logRecord := serverGenerator.NextLog(appName)
				if !r.dispatchLog(ctx, rng, logRecord, jobs, &batchBuffer) {
					if generatedThisTick > 0 {
						r.metrics.AddPlanned(generatedThisTick)
					}
					return
				}
				generated++
				generatedThisTick++
			}
			if generatedThisTick > 0 {
				r.metrics.AddPlanned(generatedThisTick)
			}
		}
	}

	if r.cfg.Mode == ModeBatch {
		_ = flushBatch(true)
	}
}

func (r *Runner) runServerForever(ctx context.Context, serverID int, rng *rand.Rand, jobs chan<- Job) {
	appName := fmt.Sprintf("app-%03d", serverID)
	serverGenerator := NewGenerator(r.cfg.Seed+int64(serverID), r.runID)

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	started := time.Now()
	generated := 0

	for {
		select {
		case <-ctx.Done():
			return
		case tickTime := <-ticker.C:
			target := int(math.Floor(tickTime.Sub(started).Seconds() * float64(r.cfg.LogsPerSec)))
			due := target - generated
			if due <= 0 {
				continue
			}

			logs := make([]LogRequest, 0, due)
			for i := 0; i < due; i++ {
				logs = append(logs, serverGenerator.NextLog(appName))
			}
			generated += due
			r.metrics.AddPlanned(due)
			r.dispatchLogs(ctx, rng, logs, jobs)
		}
	}
}

func (r *Runner) dispatchLog(ctx context.Context, rng *rand.Rand, logRecord LogRequest, jobs chan<- Job, batchBuffer *[]LogRequest) bool {
	switch r.cfg.Mode {
	case ModeSingle:
		return submitJob(ctx, jobs, Job{Kind: jobSingle, Logs: []LogRequest{logRecord}})
	case ModeBatch:
		*batchBuffer = append(*batchBuffer, logRecord)
		if len(*batchBuffer) < r.cfg.BatchSize {
			return true
		}
		batch := append([]LogRequest(nil), (*batchBuffer)...)
		*batchBuffer = (*batchBuffer)[:0]
		return submitJob(ctx, jobs, Job{Kind: jobBatch, Logs: batch})
	case ModeMixed:
		if rng.Float64() < r.cfg.SingleRatio {
			return submitJob(ctx, jobs, Job{Kind: jobSingle, Logs: []LogRequest{logRecord}})
		}
		return submitJob(ctx, jobs, Job{Kind: jobBatch, Logs: []LogRequest{logRecord}})
	default:
		return false
	}
}

func (r *Runner) dispatchLogs(ctx context.Context, rng *rand.Rand, logs []LogRequest, jobs chan<- Job) {
	switch r.cfg.Mode {
	case ModeSingle:
		for _, logRecord := range logs {
			if !submitJob(ctx, jobs, Job{Kind: jobSingle, Logs: []LogRequest{logRecord}}) {
				return
			}
		}
	case ModeBatch:
		r.submitBatches(ctx, jobs, logs)
	case ModeMixed:
		singleLogs := make([]LogRequest, 0, len(logs))
		batchLogs := make([]LogRequest, 0, len(logs))
		for _, logRecord := range logs {
			if rng.Float64() < r.cfg.SingleRatio {
				singleLogs = append(singleLogs, logRecord)
			} else {
				batchLogs = append(batchLogs, logRecord)
			}
		}
		for _, logRecord := range singleLogs {
			if !submitJob(ctx, jobs, Job{Kind: jobSingle, Logs: []LogRequest{logRecord}}) {
				return
			}
		}
		r.submitBatches(ctx, jobs, batchLogs)
	}
}

func (r *Runner) submitBatches(ctx context.Context, jobs chan<- Job, logs []LogRequest) {
	for start := 0; start < len(logs); start += r.cfg.BatchSize {
		end := minInt(start+r.cfg.BatchSize, len(logs))
		batch := append([]LogRequest(nil), logs[start:end]...)
		if !submitJob(ctx, jobs, Job{Kind: jobBatch, Logs: batch}) {
			return
		}
	}
}

func (r *Runner) reportProgress(ctx context.Context) {
	ticker := time.NewTicker(r.cfg.ProgressEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snapshot := r.metrics.Snapshot()
			fmt.Fprintf(
				r.output,
				"[progress] duration=%s planned_logs=%d sent_logs=%d dropped_logs=%d success=%d failed=%d dropped=%d throughput=%.2f logs/s\n",
				snapshot.Duration.Round(time.Millisecond),
				snapshot.PlannedLogs,
				snapshot.SentLogs,
				snapshot.DroppedLogs,
				snapshot.SuccessReqs,
				snapshot.FailedReqs,
				snapshot.DroppedReqs,
				snapshot.ThroughputLPS,
			)
		}
	}
}

func submitJob(ctx context.Context, jobs chan<- Job, job Job) bool {
	select {
	case <-ctx.Done():
		return false
	case jobs <- job:
		return true
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
