package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg, err := ParseConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid simulator config: %v\n", err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Fprintf(
		os.Stdout,
		"starting simulator mode=%s servers=%d logs_per_sec=%d total_target=%d base_url=%s\n",
		cfg.Mode,
		cfg.ServerCount,
		cfg.LogsPerSec,
		cfg.ServerCount*cfg.LogsPerSec,
		cfg.BaseURL,
	)

	runner := NewRunner(cfg, os.Stdout)
	result, runErr := runner.Run(ctx)
	report := buildReportData(ctx, cfg, result)
	reportPath, reportErr := writeMarkdownReport(cfg, report)

	fmt.Fprintln(os.Stdout, "summary:")
	for _, line := range result.Snapshot.SummaryLines() {
		fmt.Fprintln(os.Stdout, line)
	}
	if reportErr == nil {
		fmt.Fprintf(os.Stdout, "report_file=%s\n", reportPath)
	} else {
		fmt.Fprintf(os.Stderr, "failed to write benchmark report: %v\n", reportErr)
	}

	if runErr != nil {
		fmt.Fprintf(os.Stderr, "simulator finished with errors: %v\n", runErr)
		os.Exit(1)
	}
	if reportErr != nil {
		os.Exit(1)
	}
}
