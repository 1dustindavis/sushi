package source

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

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
			plan.Decisions = append(plan.Decisions, Decision{Source: "chef_server", Reason: "chef_server is not available in phase 1"})
			continue
		default:
			plan.Decisions = append(plan.Decisions, Decision{Source: sourceName, Reason: "unsupported source"})
		}
	}

	if len(plan.Decisions) == 0 {
		return nil, errors.New("no sources configured")
	}
	return nil, fmt.Errorf("no usable source from configured source_order")
}
