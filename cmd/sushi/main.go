package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"sushi/internal/config"
	"sushi/internal/logging"
	"sushi/internal/runtime"
	"sushi/internal/source"
)

var logger = newLogger()
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		exitOnErr(run(os.Args[1:]))
		return
	}

	command := os.Args[1]
	logger.Info("command invoked", "command", command)
	switch command {
	case "run":
		exitOnErr(run(os.Args[2:]))
	case "doctor":
		exitOnErr(doctor(os.Args[2:]))
	case "print-plan":
		exitOnErr(printPlan(os.Args[2:]))
	case "version":
		printVersion()
	case "service":
		exitOnErr(serviceCommand(os.Args[2:]))
	case "help", "-h", "--help":
		printUsage()
	default:
		if isFlag(command) {
			exitOnErr(run(os.Args[1:]))
			return
		}
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", command)
		printUsage()
		os.Exit(2)
	}
}

func run(args []string) error {
	cfg, err := loadConfig(args)
	if err != nil {
		return err
	}
	return runWithConfig(cfg)
}

func runWithConfig(cfg *config.Config) error {
	client, err := runtime.DiscoverClientBinary(cfg.Runtime.ClientBinary)
	if err != nil {
		return err
	}

	resolution, err := source.ResolveWithCandidates(cfg)
	if err != nil {
		return err
	}
	plan := resolution.Plan

	logger.Info("run plan resolved", "selected_mode", plan.Selected, "client_binary", client, "candidate_count", len(resolution.Candidates), "cookbook_path", plan.SelectedCookbook, "bundle_digest", plan.BundleDigest)

	lockWaitTimeout, err := parseOptionalDuration(cfg.Execution.LockWaitTimeout)
	if err != nil {
		return fmt.Errorf("parse execution.lock_wait_timeout: %w", err)
	}
	lockPollInterval, err := parseOptionalDuration(cfg.Execution.LockPollInterval)
	if err != nil {
		return fmt.Errorf("parse execution.lock_poll_interval: %w", err)
	}
	lockStaleAge, err := parseOptionalDuration(cfg.Execution.LockStaleAge)
	if err != nil {
		return fmt.Errorf("parse execution.lock_stale_age: %w", err)
	}
	convergeTimeout, err := parseOptionalDuration(cfg.Execution.ConvergeTimeout)
	if err != nil {
		return fmt.Errorf("parse execution.converge_timeout: %w", err)
	}

	for idx, candidate := range resolution.Candidates {
		fmt.Printf("selected source: %s\n", candidate.Source)
		if candidate.CookbookPath != "" {
			fmt.Printf("cookbook path: %s\n", candidate.CookbookPath)
		}
		if candidate.ChefServerPath != "" {
			fmt.Printf("chef client.rb: %s\n", candidate.ChefServerPath)
		}
		fmt.Printf("client binary: %s\n", client)

		req := runtime.RunRequest{
			ClientBinary:       client,
			CookbookPath:       candidate.CookbookPath,
			ClientRBPath:       candidate.ChefServerPath,
			RunListFile:        cfg.Execution.RunListFile,
			JSONAttributesFile: cfg.Execution.JSONAttributesFile,
			LockFile:           cfg.Execution.LockFile,
			LockWaitTimeout:    lockWaitTimeout,
			LockPollInterval:   lockPollInterval,
			LockStaleAge:       lockStaleAge,
			ConvergeTimeout:    convergeTimeout,
		}

		started := time.Now()
		attemptErr := executeCandidate(candidate.Source, req)
		latency := time.Since(started)
		if attemptErr == nil {
			logger.Info("converge completed", "selected_mode", candidate.Source, "attempt_index", idx, "converge_latency_ms", latency.Milliseconds())
			return nil
		}

		retryable := runtime.IsRetryableConvergeFailure(attemptErr, runtime.DefaultRetryableExceptions)
		logger.Error("converge failed", "selected_mode", candidate.Source, "attempt_index", idx, "converge_latency_ms", latency.Milliseconds(), "retryable_failure", retryable, "error", attemptErr)
		if !retryable || idx == len(resolution.Candidates)-1 {
			return attemptErr
		}
		logger.Warn("attempting next source after retryable converge failure", "failed_mode", candidate.Source, "next_mode", resolution.Candidates[idx+1].Source)
	}

	return errors.New("run failed without any source candidates")
}
func executeCandidate(selected string, req runtime.RunRequest) error {
	switch selected {
	case "chef_server":
		return runtime.ExecuteChefServerMode(req)
	default:
		return runtime.ExecuteLocalMode(req)
	}
}

func doctor(args []string) error {
	cfg, err := loadConfig(args)
	if err != nil {
		return err
	}

	client, clientErr := runtime.DiscoverClientBinary(cfg.Runtime.ClientBinary)
	if clientErr != nil {
		logger.Warn("doctor client discovery failed", "error", clientErr)
		fmt.Printf("client discovery: FAIL (%v)\n", clientErr)
	} else {
		logger.Info("doctor client discovery ok", "client_binary", client)
		fmt.Printf("client discovery: OK (%s)\n", client)
	}

	plan, planErr := source.Resolve(cfg)
	if planErr != nil {
		logger.Warn("doctor source resolution failed", "error", planErr)
		fmt.Printf("source resolution: FAIL (%v)\n", planErr)
	} else {
		logger.Info("doctor source resolution ok", "selected_mode", plan.Selected)
		fmt.Printf("source resolution: OK (selected %s)\n", plan.Selected)
	}

	if clientErr != nil || planErr != nil {
		return errors.New("doctor checks failed")
	}

	fmt.Println("doctor checks passed")
	return nil
}

func printPlan(args []string) error {
	cfg, err := loadConfig(args)
	if err != nil {
		return err
	}

	plan, err := source.Resolve(cfg)
	if err != nil {
		return err
	}

	logger.Info("print-plan resolved", "selected_mode", plan.Selected, "decision_count", len(plan.Decisions), "cookbook_path", plan.SelectedCookbook, "bundle_digest", plan.BundleDigest)
	fmt.Printf("selected source: %s\n", plan.Selected)
	if plan.SelectedCookbook != "" {
		fmt.Printf("cookbook path: %s\n", plan.SelectedCookbook)
	}
	if plan.ChefServerClient != "" {
		fmt.Printf("chef client.rb: %s\n", plan.ChefServerClient)
	}
	if plan.BundleDigest != "" {
		fmt.Printf("bundle digest: %s\n", plan.BundleDigest)
	}
	for _, decision := range plan.Decisions {
		fmt.Printf("- %s: %s\n", decision.Source, decision.Reason)
	}
	return nil
}

func loadConfig(args []string) (*config.Config, error) {
	fs := flag.NewFlagSet("sushi", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", config.DefaultConfigPath(), "path to sushi JSON config")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		return nil, err
	}
	if err := config.Validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func parseOptionalDuration(value string) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}
	return time.ParseDuration(value)
}

func exitOnErr(err error) {
	if err == nil {
		return
	}
	if errors.Is(err, context.Canceled) {
		logger.Info("command canceled", "error", err)
		return
	}
	logger.Error("command failed", "error", err)
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

func printUsage() {
	fmt.Println("sushi <command> [flags]")
	fmt.Println("sushi [flags]  # alias for: sushi run [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  run         resolve source and run converge")
	fmt.Println("  doctor      validate environment and config")
	fmt.Println("  print-plan  print source resolution decisions")
	fmt.Println("  version     print build version")
	fmt.Println("  service     manage native Windows service operations (Windows only)")
	fmt.Println("  help        print this usage information")
}

func printVersion() {
	fmt.Println(version)
}

func isFlag(value string) bool {
	return len(value) > 1 && value[0] == '-'
}

func newLogger() *slog.Logger {
	writers := []io.Writer{os.Stderr}
	logPath := config.DefaultLogPath()
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err == nil {
		if file, openErr := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); openErr == nil {
			writers = append(writers, file)
		}
	}
	return logging.MustNewDefault(io.MultiWriter(writers...))
}
