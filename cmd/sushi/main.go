package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"sushi/internal/config"
	"sushi/internal/logging"
	"sushi/internal/runtime"
	"sushi/internal/source"
)

const defaultConfigPath = "./config.json"

var logger = logging.MustNewDefault(os.Stderr)
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
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
	case "help", "-h", "--help":
		printUsage()
	default:
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

	client, err := runtime.DiscoverClientBinary(cfg.Runtime.ClientBinary)
	if err != nil {
		return err
	}

	plan, err := source.Resolve(cfg)
	if err != nil {
		return err
	}

	logger.Info("run plan resolved", "selected_mode", plan.Selected, "client_binary", client, "cookbook_path", plan.SelectedCookbook, "bundle_digest", plan.BundleDigest)
	fmt.Printf("selected source: %s\n", plan.Selected)
	if plan.SelectedCookbook != "" {
		fmt.Printf("cookbook path: %s\n", plan.SelectedCookbook)
	}
	if plan.ChefServerClient != "" {
		fmt.Printf("chef client.rb: %s\n", plan.ChefServerClient)
	}
	fmt.Printf("client binary: %s\n", client)

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

	req := runtime.RunRequest{
		ClientBinary:       client,
		CookbookPath:       plan.SelectedCookbook,
		ClientRBPath:       plan.ChefServerClient,
		RunListFile:        cfg.Execution.RunListFile,
		JSONAttributesFile: cfg.Execution.JSONAttributesFile,
		LockFile:           cfg.Execution.LockFile,
		LockWaitTimeout:    lockWaitTimeout,
		LockPollInterval:   lockPollInterval,
		LockStaleAge:       lockStaleAge,
		ConvergeTimeout:    convergeTimeout,
	}

	switch plan.Selected {
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
	configPath := fs.String("config", defaultConfigPath, "path to sushi JSON config")
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
	logger.Error("command failed", "error", err)
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

func printUsage() {
	fmt.Println("sushi <command> [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  run         resolve source and run converge")
	fmt.Println("  doctor      validate environment and config")
	fmt.Println("  print-plan  print source resolution decisions")
	fmt.Println("  version     print build version")
	fmt.Println("  help        print this usage information")
}

func printVersion() {
	fmt.Println(version)
}
