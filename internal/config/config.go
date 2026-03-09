package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

var knownSources = map[string]struct{}{
	"local":       {},
	"remote":      {},
	"chef_server": {},
}

type Config struct {
	Runtime     RuntimeConfig   `json:"runtime"`
	SourceOrder []string        `json:"source_order"`
	Sources     SourcesConfig   `json:"sources"`
	Execution   ExecutionConfig `json:"execution"`
}

type RuntimeConfig struct {
	ClientBinary string `json:"client_binary"`
}

type SourcesConfig struct {
	Local      LocalSource      `json:"local"`
	Remote     RemoteSource     `json:"remote"`
	ChefServer ChefServerSource `json:"chef_server"`
}

type ExecutionConfig struct {
	RunListFile        string `json:"run_list_file"`
	JSONAttributesFile string `json:"json_attributes_file"`
	LockFile           string `json:"lock_file"`
	LockWaitTimeout    string `json:"lock_wait_timeout"`
	LockPollInterval   string `json:"lock_poll_interval"`
	LockStaleAge       string `json:"lock_stale_age"`
	ConvergeTimeout    string `json:"converge_timeout"`
}

type LocalSource struct {
	Enabled      bool   `json:"enabled"`
	CookbookPath string `json:"cookbook_path"`
}

type RemoteSource struct {
	Enabled             bool   `json:"enabled"`
	URL                 string `json:"url"`
	ChecksumURL         string `json:"checksum_url"`
	AllowInsecure       bool   `json:"allow_insecure"`
	RequireChecksum     bool   `json:"require_checksum"`
	RefreshInterval     string `json:"refresh_interval"`
	RequestTimeout      string `json:"request_timeout"`
	FetchRetries        int    `json:"fetch_retries"`
	RetryBackoff        string `json:"retry_backoff"`
	CacheDir            string `json:"cache_dir"`
	MaxCacheAge         string `json:"max_cache_age"`
	StaleWarningWindow  string `json:"stale_warning_window"`
	AllowCachedFallback bool   `json:"allow_cached_fallback"`
	FailIfStale         bool   `json:"fail_if_stale"`
}

type ChefServerSource struct {
	Enabled     bool   `json:"enabled"`
	ClientRB    string `json:"client_rb"`
	Healthcheck struct {
		Endpoint string `json:"endpoint"`
		Timeout  string `json:"timeout"`
	} `json:"healthcheck"`
}

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("invalid config field %q: %s", e.Field, e.Message)
}

func Load(path string) (*Config, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	return parseConfigJSON(bytes)
}

func parseConfigJSON(bytes []byte) (*Config, error) {
	var cfg Config
	if err := json.Unmarshal(bytes, &cfg); err != nil {
		return nil, fmt.Errorf("parse config JSON: %w", err)
	}
	return &cfg, nil
}

func Validate(cfg *Config) error {
	if cfg == nil {
		return ValidationError{Field: "", Message: "config must not be nil"}
	}

	if cfg.Runtime.ClientBinary == "" {
		cfg.Runtime.ClientBinary = "auto"
	}

	seen := map[string]struct{}{}
	enabledInOrder := 0

	for idx, sourceName := range cfg.SourceOrder {
		if _, ok := knownSources[sourceName]; !ok {
			return ValidationError{Field: fmt.Sprintf("source_order[%d]", idx), Message: "unknown source"}
		}
		if _, ok := seen[sourceName]; ok {
			return ValidationError{Field: "source_order", Message: "source names must be unique"}
		}
		seen[sourceName] = struct{}{}

		switch sourceName {
		case "local":
			if cfg.Sources.Local.Enabled {
				enabledInOrder++
				if cfg.Sources.Local.CookbookPath == "" {
					return ValidationError{Field: "sources.local.cookbook_path", Message: "required when local source is enabled"}
				}
			}
		case "remote":
			if cfg.Sources.Remote.Enabled {
				enabledInOrder++
				if cfg.Sources.Remote.URL == "" {
					return ValidationError{Field: "sources.remote.url", Message: "required when remote source is enabled"}
				}
				parsedURL, err := url.Parse(cfg.Sources.Remote.URL)
				if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
					return ValidationError{Field: "sources.remote.url", Message: "must be a valid absolute URL"}
				}
				if strings.EqualFold(parsedURL.Scheme, "http") && !cfg.Sources.Remote.AllowInsecure {
					return ValidationError{Field: "sources.remote.allow_insecure", Message: "must be true when using an http source URL"}
				}
				if cfg.Sources.Remote.RequireChecksum && cfg.Sources.Remote.ChecksumURL == "" {
					return ValidationError{Field: "sources.remote.checksum_url", Message: "required when require_checksum is true"}
				}
				if cfg.Sources.Remote.ChecksumURL != "" {
					parsedChecksumURL, err := url.Parse(cfg.Sources.Remote.ChecksumURL)
					if err != nil || parsedChecksumURL.Scheme == "" || parsedChecksumURL.Host == "" {
						return ValidationError{Field: "sources.remote.checksum_url", Message: "must be a valid absolute URL"}
					}
					if strings.EqualFold(parsedChecksumURL.Scheme, "http") && !cfg.Sources.Remote.AllowInsecure {
						return ValidationError{Field: "sources.remote.allow_insecure", Message: "must be true when using an http checksum URL"}
					}
				}
				if cfg.Sources.Remote.CacheDir == "" {
					return ValidationError{Field: "sources.remote.cache_dir", Message: "required when remote source is enabled"}
				}
				if cfg.Sources.Remote.RefreshInterval != "" {
					if _, err := time.ParseDuration(cfg.Sources.Remote.RefreshInterval); err != nil {
						return ValidationError{Field: "sources.remote.refresh_interval", Message: "must be a valid duration"}
					}
				}
				if cfg.Sources.Remote.MaxCacheAge != "" {
					if _, err := time.ParseDuration(cfg.Sources.Remote.MaxCacheAge); err != nil {
						return ValidationError{Field: "sources.remote.max_cache_age", Message: "must be a valid duration"}
					}
				}
				if cfg.Sources.Remote.RequestTimeout != "" {
					if _, err := time.ParseDuration(cfg.Sources.Remote.RequestTimeout); err != nil {
						return ValidationError{Field: "sources.remote.request_timeout", Message: "must be a valid duration"}
					}
				}
				if cfg.Sources.Remote.FetchRetries < 0 {
					return ValidationError{Field: "sources.remote.fetch_retries", Message: "must be >= 0"}
				}
				if cfg.Sources.Remote.RetryBackoff != "" {
					if _, err := time.ParseDuration(cfg.Sources.Remote.RetryBackoff); err != nil {
						return ValidationError{Field: "sources.remote.retry_backoff", Message: "must be a valid duration"}
					}
				}
				if cfg.Sources.Remote.StaleWarningWindow != "" {
					if _, err := time.ParseDuration(cfg.Sources.Remote.StaleWarningWindow); err != nil {
						return ValidationError{Field: "sources.remote.stale_warning_window", Message: "must be a valid duration"}
					}
				}
			}
		case "chef_server":
			if cfg.Sources.ChefServer.Enabled {
				enabledInOrder++
				if cfg.Sources.ChefServer.ClientRB == "" {
					return ValidationError{Field: "sources.chef_server.client_rb", Message: "required when chef_server source is enabled"}
				}
				if cfg.Sources.ChefServer.Healthcheck.Timeout != "" {
					if _, err := time.ParseDuration(cfg.Sources.ChefServer.Healthcheck.Timeout); err != nil {
						return ValidationError{Field: "sources.chef_server.healthcheck.timeout", Message: "must be a valid duration"}
					}
				}
			}
		}
	}

	if len(cfg.SourceOrder) == 0 {
		return ValidationError{Field: "source_order", Message: "must not be empty"}
	}
	if enabledInOrder == 0 {
		return ValidationError{Field: "source_order", Message: "must reference at least one enabled source"}
	}

	if cfg.Execution.LockWaitTimeout != "" {
		if _, err := time.ParseDuration(cfg.Execution.LockWaitTimeout); err != nil {
			return ValidationError{Field: "execution.lock_wait_timeout", Message: "must be a valid duration"}
		}
	}
	if cfg.Execution.LockPollInterval != "" {
		if _, err := time.ParseDuration(cfg.Execution.LockPollInterval); err != nil {
			return ValidationError{Field: "execution.lock_poll_interval", Message: "must be a valid duration"}
		}
	}
	if cfg.Execution.LockStaleAge != "" {
		if _, err := time.ParseDuration(cfg.Execution.LockStaleAge); err != nil {
			return ValidationError{Field: "execution.lock_stale_age", Message: "must be a valid duration"}
		}
	}
	if cfg.Execution.ConvergeTimeout != "" {
		if _, err := time.ParseDuration(cfg.Execution.ConvergeTimeout); err != nil {
			return ValidationError{Field: "execution.converge_timeout", Message: "must be a valid duration"}
		}
	}

	return nil
}
