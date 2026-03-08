package source

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"sushi/internal/config"
)

type Decision struct {
	Source string
	Reason string
}

type Plan struct {
	Selected         string
	SelectedCookbook string
	BundleDigest     string
	ChefServerClient string
	Decisions        []Decision
}

func Resolve(cfg *config.Config) (*Plan, error) {
	plan := &Plan{}
	for _, sourceName := range cfg.SourceOrder {
		switch sourceName {
		case "local":
			if !cfg.Sources.Local.Enabled {
				plan.Decisions = append(plan.Decisions, Decision{Source: "local", Reason: "disabled"})
				continue
			}
			info, err := os.Stat(cfg.Sources.Local.CookbookPath)
			if err != nil {
				plan.Decisions = append(plan.Decisions, Decision{Source: "local", Reason: fmt.Sprintf("path unavailable: %v", err)})
				continue
			}
			if !info.IsDir() {
				plan.Decisions = append(plan.Decisions, Decision{Source: "local", Reason: "cookbook_path is not a directory"})
				continue
			}
			path, err := filepath.Abs(cfg.Sources.Local.CookbookPath)
			if err != nil {
				path = cfg.Sources.Local.CookbookPath
			}
			plan.Decisions = append(plan.Decisions, Decision{Source: "local", Reason: "usable"})
			plan.Selected = "local"
			plan.SelectedCookbook = path
			return plan, nil
		case "remote":
			if !cfg.Sources.Remote.Enabled {
				plan.Decisions = append(plan.Decisions, Decision{Source: "remote", Reason: "disabled"})
				continue
			}
			if _, err := url.ParseRequestURI(cfg.Sources.Remote.URL); err != nil {
				plan.Decisions = append(plan.Decisions, Decision{Source: "remote", Reason: "invalid URL"})
				continue
			}

			remote, err := ResolveRemote(cfg.Sources.Remote)
			if err != nil {
				plan.Decisions = append(plan.Decisions, Decision{Source: "remote", Reason: err.Error()})
				continue
			}
			plan.Decisions = append(plan.Decisions, Decision{Source: "remote", Reason: remote.Reason})
			plan.Selected = "remote"
			plan.SelectedCookbook = remote.CookbookPath
			plan.BundleDigest = remote.Digest
			return plan, nil
		case "chef_server":
			if !cfg.Sources.ChefServer.Enabled {
				plan.Decisions = append(plan.Decisions, Decision{Source: "chef_server", Reason: "disabled"})
				continue
			}
			reason, err := validateChefServerSource(cfg.Sources.ChefServer)
			if err != nil {
				plan.Decisions = append(plan.Decisions, Decision{Source: "chef_server", Reason: err.Error()})
				continue
			}
			plan.Decisions = append(plan.Decisions, Decision{Source: "chef_server", Reason: reason})
			plan.Selected = "chef_server"
			plan.ChefServerClient = cfg.Sources.ChefServer.ClientRB
			return plan, nil
		default:
			plan.Decisions = append(plan.Decisions, Decision{Source: sourceName, Reason: "unsupported source"})
		}
	}

	if len(plan.Decisions) == 0 {
		return nil, errors.New("no sources configured")
	}
	return nil, fmt.Errorf("no usable source from configured source_order")
}

func validateChefServerSource(cfg config.ChefServerSource) (string, error) {
	if _, err := os.Stat(cfg.ClientRB); err != nil {
		return "", fmt.Errorf("client_rb unavailable: %v", err)
	}
	if cfg.Healthcheck.Endpoint == "" {
		return "usable (healthcheck skipped)", nil
	}

	timeout := 2 * time.Second
	if cfg.Healthcheck.Timeout != "" {
		parsed, err := time.ParseDuration(cfg.Healthcheck.Timeout)
		if err != nil {
			return "", fmt.Errorf("invalid healthcheck timeout: %v", err)
		}
		timeout = parsed
	}
	if timeout <= 0 {
		return "", fmt.Errorf("invalid healthcheck timeout: must be > 0")
	}

	client := http.Client{Timeout: timeout}
	resp, err := client.Get(cfg.Healthcheck.Endpoint)
	if err != nil {
		return "", fmt.Errorf("healthcheck failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("healthcheck returned status %d", resp.StatusCode)
	}
	return fmt.Sprintf("usable (healthcheck %s)", resp.Status), nil
}
