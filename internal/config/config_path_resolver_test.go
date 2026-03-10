package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadResolvedConfigPrefersCLIArgument(t *testing.T) {
	previousResolver := platformConfigResolver
	platformConfigResolver = func() (*Config, ResolvedConfig, error) {
		return nil, ResolvedConfig{}, nil
	}
	t.Cleanup(func() {
		platformConfigResolver = previousResolver
	})

	path := writeTestConfig(t, filepath.Join(t.TempDir(), "cli.json"))
	cfg, resolved, err := LoadResolvedConfig(path, true)
	if err != nil {
		t.Fatalf("LoadResolvedConfig() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config")
	}
	if resolved.Source != "config argument" || resolved.Path != path {
		t.Fatalf("unexpected resolution: %+v", resolved)
	}
}

func TestLoadResolvedConfigUsesPlatformBeforeDefault(t *testing.T) {
	previousResolver := platformConfigResolver
	platformConfigResolver = func() (*Config, ResolvedConfig, error) {
		return &Config{Runtime: RuntimeConfig{ClientBinary: "auto"}}, ResolvedConfig{Source: "platform"}, nil
	}
	t.Cleanup(func() {
		platformConfigResolver = previousResolver
	})

	cfg, resolved, err := LoadResolvedConfig("", false)
	if err != nil {
		t.Fatalf("LoadResolvedConfig() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config")
	}
	if resolved.Source != "platform" {
		t.Fatalf("expected platform source, got %q", resolved.Source)
	}
}

func TestLoadResolvedConfigFallsBackToDefaultPath(t *testing.T) {
	previousResolver := platformConfigResolver
	platformConfigResolver = func() (*Config, ResolvedConfig, error) {
		return nil, ResolvedConfig{}, nil
	}
	t.Cleanup(func() {
		platformConfigResolver = previousResolver
	})

	defaultPath := writeTestConfig(t, filepath.Join(t.TempDir(), "default.json"))
	t.Setenv("SUSHI_CONFIG_PATH", defaultPath)

	_, resolved, err := LoadResolvedConfig("", false)
	if err != nil {
		t.Fatalf("LoadResolvedConfig() error = %v", err)
	}
	if resolved.Source != "default config path" || resolved.Path != defaultPath {
		t.Fatalf("unexpected resolution: %+v", resolved)
	}
}

func writeTestConfig(t *testing.T, path string) string {
	t.Helper()
	content := `{"runtime":{"client_binary":"auto"},"source_order":["local"],"sources":{"local":{"enabled":true,"cookbook_path":"/tmp/cookbooks"},"remote":{"enabled":false},"chef_server":{"enabled":false}},"execution":{}}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
