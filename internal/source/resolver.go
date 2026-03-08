package source

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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

type Candidate struct {
	Source         string
	CookbookPath   string
	BundleDigest   string
	ChefServerPath string
}

type Resolution struct {
	Plan       *Plan
	Candidates []Candidate
}

type ResolveOptions struct {
	ReadOnly bool
}

type ResolutionError struct {
	Err                 error
	StaleCacheViolation bool
	Decisions           []Decision
}

func (e *ResolutionError) Error() string {
	return e.Err.Error()
}

func (e *ResolutionError) Unwrap() error {
	return e.Err
}

func Resolve(cfg *config.Config) (*Plan, error) {
	resolution, err := ResolveWithCandidates(cfg)
	if err != nil {
		return nil, err
	}
	return resolution.Plan, nil
}

func ResolveForInspection(cfg *config.Config) (*Plan, error) {
	resolution, err := ResolveWithCandidatesOptions(cfg, ResolveOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return resolution.Plan, nil
}

func ResolveWithCandidates(cfg *config.Config) (*Resolution, error) {
	return ResolveWithCandidatesOptions(cfg, ResolveOptions{})
}

func ResolveWithCandidatesOptions(cfg *config.Config, opts ResolveOptions) (*Resolution, error) {
	plan := &Plan{}
	var candidates []Candidate
	staleCacheViolation := false
	for _, sourceName := range cfg.SourceOrder {
		switch sourceName {
		case "local":
			candidate, reason, err := evaluateLocal(cfg)
			if err != nil {
				plan.Decisions = append(plan.Decisions, Decision{Source: "local", Reason: reason})
				continue
			}
			plan.Decisions = append(plan.Decisions, Decision{Source: "local", Reason: reason})
			candidates = append(candidates, *candidate)
		case "remote":
			candidate, reason, err := evaluateRemote(cfg, opts)
			if err != nil {
				var remoteUnavailable *RemoteUnavailableError
				if errors.As(err, &remoteUnavailable) && remoteUnavailable.StaleCacheViolation {
					staleCacheViolation = true
				}
				plan.Decisions = append(plan.Decisions, Decision{Source: "remote", Reason: reason})
				continue
			}
			plan.Decisions = append(plan.Decisions, Decision{Source: "remote", Reason: reason})
			candidates = append(candidates, *candidate)
		case "chef_server":
			candidate, reason, err := evaluateChefServer(cfg)
			if err != nil {
				plan.Decisions = append(plan.Decisions, Decision{Source: "chef_server", Reason: reason})
				continue
			}
			plan.Decisions = append(plan.Decisions, Decision{Source: "chef_server", Reason: reason})
			candidates = append(candidates, *candidate)
		default:
			plan.Decisions = append(plan.Decisions, Decision{Source: sourceName, Reason: "unsupported source"})
		}
	}

	if len(candidates) == 0 {
		if len(plan.Decisions) == 0 {
			return nil, &ResolutionError{Err: errors.New("no sources configured"), Decisions: plan.Decisions}
		}
		detail := formatDecisions(plan.Decisions)
		return nil, &ResolutionError{Err: fmt.Errorf("no usable source from configured source_order (%s)", detail), StaleCacheViolation: staleCacheViolation, Decisions: plan.Decisions}
	}

	selected := candidates[0]
	plan.Selected = selected.Source
	plan.SelectedCookbook = selected.CookbookPath
	plan.BundleDigest = selected.BundleDigest
	plan.ChefServerClient = selected.ChefServerPath
	return &Resolution{Plan: plan, Candidates: candidates}, nil
}

func formatDecisions(decisions []Decision) string {
	parts := make([]string, 0, len(decisions))
	for _, decision := range decisions {
		parts = append(parts, fmt.Sprintf("%s=%s", decision.Source, decision.Reason))
	}
	return strings.Join(parts, "; ")
}

func evaluateLocal(cfg *config.Config) (*Candidate, string, error) {
	if !cfg.Sources.Local.Enabled {
		return nil, "disabled", errors.New("local disabled")
	}
	info, err := os.Stat(cfg.Sources.Local.CookbookPath)
	if err != nil {
		return nil, fmt.Sprintf("path unavailable: %v", err), err
	}
	if !info.IsDir() {
		return nil, "cookbook_path is not a directory", errors.New("local cookbook path not dir")
	}
	path, err := filepath.Abs(cfg.Sources.Local.CookbookPath)
	if err != nil {
		path = cfg.Sources.Local.CookbookPath
	}
	return &Candidate{Source: "local", CookbookPath: path}, "usable", nil
}

func evaluateRemote(cfg *config.Config, opts ResolveOptions) (*Candidate, string, error) {
	if !cfg.Sources.Remote.Enabled {
		return nil, "disabled", errors.New("remote disabled")
	}
	if _, err := url.ParseRequestURI(cfg.Sources.Remote.URL); err != nil {
		return nil, "invalid URL", err
	}
	var (
		remote *RemoteResult
		err    error
	)
	if opts.ReadOnly {
		remote, err = ResolveRemoteReadOnly(cfg.Sources.Remote)
	} else {
		remote, err = ResolveRemote(cfg.Sources.Remote)
	}
	if err != nil {
		return nil, err.Error(), err
	}
	return &Candidate{Source: "remote", CookbookPath: remote.CookbookPath, BundleDigest: remote.Digest}, remote.Reason, nil
}

func evaluateChefServer(cfg *config.Config) (*Candidate, string, error) {
	if !cfg.Sources.ChefServer.Enabled {
		return nil, "disabled", errors.New("chef_server disabled")
	}
	reason, err := validateChefServerSource(cfg.Sources.ChefServer)
	if err != nil {
		return nil, err.Error(), err
	}
	return &Candidate{Source: "chef_server", ChefServerPath: cfg.Sources.ChefServer.ClientRB}, reason, nil
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
