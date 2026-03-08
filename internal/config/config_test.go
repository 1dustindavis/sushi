package config

import "testing"

func validConfig() *Config {
	return &Config{
		Runtime:     RuntimeConfig{ClientBinary: "auto"},
		SourceOrder: []string{"local", "remote"},
		Sources: SourcesConfig{
			Local: LocalSource{Enabled: true, CookbookPath: "/tmp"},
			Remote: RemoteSource{Enabled: true, URL: "https://example.org/cookbooks.tar.zst", ChecksumURL: "https://example.org/cookbooks.sha256", CacheDir: "/tmp/cache",
				RefreshInterval: "1h", MaxCacheAge: "24h"},
		},
	}
}

func TestValidateSuccess(t *testing.T) {
	cfg := validConfig()
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected config to validate, got %v", err)
	}
}

func TestValidateUnknownSource(t *testing.T) {
	cfg := validConfig()
	cfg.SourceOrder = []string{"wat"}

	if err := Validate(cfg); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateRequiresEnabledSourceInOrder(t *testing.T) {
	cfg := validConfig()
	cfg.Sources.Local.Enabled = false
	cfg.Sources.Remote.Enabled = false

	if err := Validate(cfg); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateRemoteHTTPRequiresAllowInsecure(t *testing.T) {
	cfg := validConfig()
	cfg.Sources.Remote.URL = "http://example.org/cookbooks.tar"

	if err := Validate(cfg); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateRemoteChecksumRequiredNeedsURL(t *testing.T) {
	cfg := validConfig()
	cfg.Sources.Remote.ChecksumURL = ""
	cfg.Sources.Remote.RequireChecksum = true

	if err := Validate(cfg); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateRemoteAllowsExplicitInsecureAndOptionalChecksum(t *testing.T) {
	cfg := validConfig()
	cfg.Sources.Remote.URL = "http://example.org/cookbooks.tar"
	cfg.Sources.Remote.ChecksumURL = ""
	cfg.Sources.Remote.AllowInsecure = true
	cfg.Sources.Remote.RequireChecksum = false

	if err := Validate(cfg); err != nil {
		t.Fatalf("expected config to validate, got %v", err)
	}
}

func TestValidateExecutionDurations(t *testing.T) {
	cfg := validConfig()
	cfg.Execution.LockWaitTimeout = "5s"
	cfg.Execution.LockPollInterval = "250ms"
	cfg.Execution.LockStaleAge = "1h"
	cfg.Execution.ConvergeTimeout = "30m"

	if err := Validate(cfg); err != nil {
		t.Fatalf("expected config to validate, got %v", err)
	}
}
