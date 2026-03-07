package source

import (
	"fmt"
	"net/url"
	"os"

	"github.com/evanphx/sushi/internal/config"
)

type Decision struct {
	Source string
	Reason string
}

type Plan struct {
	Selected  string
	Decisions []Decision
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
			if _, err := os.Stat(cfg.Sources.Local.CookbookPath); err != nil {
				plan.Decisions = append(plan.Decisions, Decision{Source: "local", Reason: fmt.Sprintf("path unavailable: %v", err)})
				continue
			}
			plan.Decisions = append(plan.Decisions, Decision{Source: "local", Reason: "usable"})
			plan.Selected = "local"
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
			plan.Decisions = append(plan.Decisions, Decision{Source: "remote", Reason: "usable"})
			plan.Selected = "remote"
			return plan, nil
		case "chef_server":
			if !cfg.Sources.ChefServer.Enabled {
				plan.Decisions = append(plan.Decisions, Decision{Source: "chef_server", Reason: "disabled"})
				continue
			}
			if cfg.Sources.ChefServer.Healthcheck.Endpoint == "" {
				plan.Decisions = append(plan.Decisions, Decision{Source: "chef_server", Reason: "missing healthcheck endpoint"})
				continue
			}
			plan.Decisions = append(plan.Decisions, Decision{Source: "chef_server", Reason: "usable"})
			plan.Selected = "chef_server"
			return plan, nil
		default:
			plan.Decisions = append(plan.Decisions, Decision{Source: sourceName, Reason: "unsupported source"})
		}
	}

	return nil, fmt.Errorf("no usable source from configured source_order")
}
