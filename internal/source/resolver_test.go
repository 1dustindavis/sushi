package source

import (
	"os"
	"path/filepath"
	"testing"

	"sushi/internal/config"
)

func TestResolvePrefersFirstUsableSource(t *testing.T) {
	tmp := t.TempDir()
	cookbooks := filepath.Join(tmp, "cookbooks")
	if err := os.MkdirAll(cookbooks, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		SourceOrder: []string{"local", "remote"},
		Sources: config.SourcesConfig{
			Local:  config.LocalSource{Enabled: true, CookbookPath: cookbooks},
			Remote: config.RemoteSource{Enabled: true, URL: "https://example.org/cb.tar.zst", CacheDir: tmp},
		},
	}

	plan, err := Resolve(cfg)
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if plan.Selected != "local" {
		t.Fatalf("expected local, got %s", plan.Selected)
	}
}

func TestResolveFallsBackToRemoteWhenLocalUnavailable(t *testing.T) {
	tmp := t.TempDir()

	cfg := &config.Config{
		SourceOrder: []string{"local", "remote"},
		Sources: config.SourcesConfig{
			Local:  config.LocalSource{Enabled: true, CookbookPath: filepath.Join(tmp, "missing")},
			Remote: config.RemoteSource{Enabled: true, URL: "https://example.org/cb.tar.zst", CacheDir: tmp},
		},
	}

	plan, err := Resolve(cfg)
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if plan.Selected != "remote" {
		t.Fatalf("expected remote fallback, got %s", plan.Selected)
	}
}

func TestResolveNoUsableSources(t *testing.T) {
	cfg := &config.Config{
		SourceOrder: []string{"local", "remote"},
		Sources: config.SourcesConfig{
			Local:  config.LocalSource{Enabled: false},
			Remote: config.RemoteSource{Enabled: true, URL: ":bad-url:"},
		},
	}

	if _, err := Resolve(cfg); err == nil {
		t.Fatal("expected no usable source error")
	}
}
