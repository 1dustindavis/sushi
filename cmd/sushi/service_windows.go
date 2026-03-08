//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/sys/windows/svc"

	"sushi/internal/config"
)

const windowsServiceName = "sushi"

func serviceCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: sushi service <install|uninstall|start|stop|status|run> [-config <path>]")
	}

	subcommand := args[0]
	configPath, err := serviceConfigPathFromArgs(args[1:])
	if err != nil {
		return err
	}

	switch subcommand {
	case "install":
		return installWindowsService(configPath)
	case "uninstall":
		return runSC("delete", windowsServiceName)
	case "start":
		return runSC("start", windowsServiceName)
	case "stop":
		return runSC("stop", windowsServiceName)
	case "status":
		out, queryErr := runSCOutput("query", windowsServiceName)
		if queryErr != nil {
			return queryErr
		}
		fmt.Print(out)
		return nil
	case "run":
		return svc.Run(windowsServiceName, &sushiService{configPath: configPath})
	default:
		return fmt.Errorf("unknown service command %q", subcommand)
	}
}

func serviceConfigPathFromArgs(args []string) (string, error) {
	configPath := config.DefaultConfigPath()
	for idx := 0; idx < len(args); idx++ {
		if args[idx] != "-config" {
			continue
		}
		if idx+1 >= len(args) {
			return "", fmt.Errorf("-config requires a value")
		}
		configPath = args[idx+1]
		idx++
	}
	return configPath, nil
}

type sushiService struct {
	configPath string
}

func (m *sushiService) Execute(_ []string, req <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	status <- svc.Status{State: svc.StartPending}
	status <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	interval := 10 * time.Minute
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	runOnce := func() {
		if err := runWithConfigPath(m.configPath); err != nil {
			logger.Error("windows service converge failed", "error", err)
		}
	}

	runOnce()
	for {
		select {
		case c := <-req:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
				return false, 0
			}
		case <-ticker.C:
			runOnce()
		}
	}
}

func runWithConfigPath(path string) error {
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	if err := config.Validate(cfg); err != nil {
		return err
	}
	return runWithConfig(cfg)
}

func installWindowsService(configPath string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	if err := runSC(buildWindowsServiceCreateArgs(exePath, configPath)...); err != nil {
		return err
	}
	_ = runSC("description", windowsServiceName, "sushi local-first converge service")
	return nil
}

func buildWindowsServiceCreateArgs(exePath, configPath string) []string {
	binPath := fmt.Sprintf("\"%s\" service run -config \"%s\"", exePath, configPath)
	return []string{"create", windowsServiceName, "binPath=", binPath, "start=", "auto", "DisplayName=", "sushi"}
}

func runSC(args ...string) error {
	_, err := runSCOutput(args...)
	return err
}

func runSCOutput(args ...string) (string, error) {
	cmd := exec.Command("sc.exe", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("sc.exe %s failed: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
