package config

import "testing"

func validConfig() *Config {
	return &Config{
		Runtime:     RuntimeConfig{ClientBinary: "auto"},
		SourceOrder: []string{"local", "remote"},
		Sources: SourcesConfig{
			Local: LocalSource{Enabled: true, CookbookPath: "/tmp"},
			Remote: RemoteSource{Enabled: true, URL: "https://example.org/cookbooks.tar.zst", CacheDir: "/tmp/cache",
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
