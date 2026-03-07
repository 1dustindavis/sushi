package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/evanphx/sushi/internal/config"
	"github.com/evanphx/sushi/internal/runtime"
	"github.com/evanphx/sushi/internal/source"
)

const defaultConfigPath = "./config.json"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	command := os.Args[1]
	switch command {
	case "run":
		exitOnErr(run(os.Args[2:]))
	case "doctor":
		exitOnErr(doctor(os.Args[2:]))
	case "print-plan":
		exitOnErr(printPlan(os.Args[2:]))
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

	fmt.Printf("selected source: %s\n", plan.Selected)
	fmt.Printf("client binary: %s\n", client)
	fmt.Println("run command execution placeholder")
	return nil
}

func doctor(args []string) error {
	cfg, err := loadConfig(args)
	if err != nil {
		return err
	}

	client, clientErr := runtime.DiscoverClientBinary(cfg.Runtime.ClientBinary)
	if clientErr != nil {
		fmt.Printf("client discovery: FAIL (%v)\n", clientErr)
	} else {
		fmt.Printf("client discovery: OK (%s)\n", client)
	}

	plan, planErr := source.Resolve(cfg)
	if planErr != nil {
		fmt.Printf("source resolution: FAIL (%v)\n", planErr)
	} else {
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

	fmt.Printf("selected source: %s\n", plan.Selected)
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

func exitOnErr(err error) {
	if err == nil {
		return
	}
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
}
