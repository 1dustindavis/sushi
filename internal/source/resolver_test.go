package source

import (
	"archive/tar"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

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
			Remote: config.RemoteSource{Enabled: true, URL: "https://example.org/cb.tar", CacheDir: tmp},
		},
	}

	plan, err := Resolve(cfg)
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if plan.Selected != "local" {
		t.Fatalf("expected local, got %s", plan.Selected)
	}
	if plan.SelectedCookbook == "" {
		t.Fatal("expected selected cookbook path to be set")
	}
}

func TestResolveFallsBackToRemoteWhenLocalUnavailable(t *testing.T) {
	tmp := t.TempDir()
	bundle := makeTarBundle(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(bundle)
	}))
	defer server.Close()

	cfg := &config.Config{
		SourceOrder: []string{"local", "remote"},
		Sources: config.SourcesConfig{
			Local:  config.LocalSource{Enabled: true, CookbookPath: filepath.Join(tmp, "missing")},
			Remote: config.RemoteSource{Enabled: true, URL: server.URL + "/cookbooks.tar", CacheDir: filepath.Join(tmp, "cache")},
		},
	}

	plan, err := Resolve(cfg)
	if err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if plan.Selected != "remote" {
		t.Fatalf("expected remote fallback, got %s", plan.Selected)
	}
	if _, err := os.Stat(plan.SelectedCookbook); err != nil {
		t.Fatalf("expected remote cookbook path to exist: %v", err)
	}
}

func TestResolveRemoteUsesCachedFallbackWhenFetchFails(t *testing.T) {
	tmp := t.TempDir()
	cacheDir := filepath.Join(tmp, "cache")
	meta := cacheMetadata{Digest: "abc123", FetchedAt: time.Now().Add(-1 * time.Hour), SourceURL: "https://example.org/cookbooks.tar"}
	bundlePath := filepath.Join(cacheDir, "bundles", meta.Digest, "cookbooks")
	if err := os.MkdirAll(bundlePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := writeCurrentMetadata(cacheDir, meta); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		SourceOrder: []string{"remote"},
		Sources: config.SourcesConfig{
			Remote: config.RemoteSource{
				Enabled:             true,
				URL:                 "http://127.0.0.1:1/unreachable.tar",
				CacheDir:            cacheDir,
				RefreshInterval:     "0s",
				MaxCacheAge:         "24h",
				AllowCachedFallback: true,
				FailIfStale:         true,
			},
		},
	}

	plan, err := Resolve(cfg)
	if err != nil {
		t.Fatalf("expected cache fallback to succeed, got: %v", err)
	}
	if plan.Selected != "remote" {
		t.Fatalf("expected remote, got %q", plan.Selected)
	}
}

func TestResolveRemoteFailsWhenCacheIsStaleAndPolicyRequiresFresh(t *testing.T) {
	tmp := t.TempDir()
	cacheDir := filepath.Join(tmp, "cache")
	meta := cacheMetadata{Digest: "abc123", FetchedAt: time.Now().Add(-48 * time.Hour), SourceURL: "https://example.org/cookbooks.tar"}
	bundlePath := filepath.Join(cacheDir, "bundles", meta.Digest, "cookbooks")
	if err := os.MkdirAll(bundlePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := writeCurrentMetadata(cacheDir, meta); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		SourceOrder: []string{"remote"},
		Sources: config.SourcesConfig{
			Remote: config.RemoteSource{
				Enabled:             true,
				URL:                 "http://127.0.0.1:1/unreachable.tar",
				CacheDir:            cacheDir,
				RefreshInterval:     "0s",
				MaxCacheAge:         "24h",
				AllowCachedFallback: true,
				FailIfStale:         true,
			},
		},
	}

	if _, err := Resolve(cfg); err == nil {
		t.Fatal("expected stale cache policy failure")
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

func makeTarBundle(t *testing.T) []byte {
	t.Helper()
	buf := bytes.NewBuffer(nil)
	tw := tar.NewWriter(buf)

	entries := []struct {
		name string
		data string
		mode int64
	}{
		{name: "cookbooks/", mode: 0o755},
		{name: "cookbooks/base/", mode: 0o755},
		{name: "cookbooks/base/metadata.rb", data: "name 'base'", mode: 0o644},
	}

	for _, entry := range entries {
		hdr := &tar.Header{Name: entry.name, Mode: entry.mode}
		if entry.data != "" {
			hdr.Size = int64(len(entry.data))
			hdr.Typeflag = tar.TypeReg
		} else {
			hdr.Typeflag = tar.TypeDir
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write tar header %s: %v", entry.name, err)
		}
		if entry.data != "" {
			if _, err := tw.Write([]byte(entry.data)); err != nil {
				t.Fatalf("write tar data %s: %v", entry.name, err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	return buf.Bytes()
}

func ExampleResolve() {
	fmt.Println("resolver")
	// Output: resolver
}
