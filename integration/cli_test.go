package integration

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
)

type testConfig struct {
	Runtime struct {
		ClientBinary string `json:"client_binary"`
	} `json:"runtime"`
	SourceOrder []string `json:"source_order"`
	Sources     struct {
		Local struct {
			Enabled      bool   `json:"enabled"`
			CookbookPath string `json:"cookbook_path"`
		} `json:"local"`
		Remote struct {
			Enabled         bool   `json:"enabled"`
			URL             string `json:"url"`
			ChecksumURL     string `json:"checksum_url"`
			AllowInsecure   bool   `json:"allow_insecure"`
			RequireChecksum bool   `json:"require_checksum"`
			RefreshInterval string `json:"refresh_interval"`
			CacheDir        string `json:"cache_dir"`
			MaxCacheAge     string `json:"max_cache_age"`
			FailIfStale     bool   `json:"fail_if_stale"`
			AllowFallback   bool   `json:"allow_cached_fallback"`
		} `json:"remote"`
		ChefServer struct {
			Enabled     bool   `json:"enabled"`
			ClientRB    string `json:"client_rb"`
			Healthcheck struct {
				Endpoint string `json:"endpoint"`
				Timeout  string `json:"timeout"`
			} `json:"healthcheck"`
		} `json:"chef_server"`
	} `json:"sources"`
	Execution struct {
		RunListFile string `json:"run_list_file"`
		LockFile    string `json:"lock_file"`
	} `json:"execution"`
}

type cacheMetadata struct {
	Digest    string    `json:"digest"`
	FetchedAt time.Time `json:"fetched_at"`
	SourceURL string    `json:"source_url"`
	ExpiresAt time.Time `json:"expires_at"`
	ETag      string    `json:"etag"`
}

func TestIntegration(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	fakeClient := buildFakeClient(t, repoRoot)
	capturePath := filepath.Join(t.TempDir(), "client-args.txt")

	t.Run("local", func(t *testing.T) {
		cfgPath := writeLocalConfig(t, fakeClient)
		for _, command := range []string{"print-plan", "doctor", "run"} {
			out, err := runSushi(t, repoRoot, command, cfgPath, capturePath)
			if err != nil {
				t.Fatalf("%s failed: %v\n%s", command, err, out)
			}
		}
	})

	t.Run("run retries next source on retryable converge failure", func(t *testing.T) {
		clientRB := filepath.Join(t.TempDir(), "client.rb")
		if err := os.WriteFile(clientRB, []byte("chef_server_url 'https://chef.example.com'\n"), 0o644); err != nil {
			t.Fatalf("write client.rb: %v", err)
		}
		cfg := testConfig{}
		cfg.Runtime.ClientBinary = fakeClient
		cfg.SourceOrder = []string{"chef_server", "local"}
		cfg.Sources.ChefServer.Enabled = true
		cfg.Sources.ChefServer.ClientRB = clientRB
		cfg.Sources.Local.Enabled = true
		cfg.Sources.Local.CookbookPath = filepath.Join(repoRoot, "integration", "testdata", "local-cookbooks")
		cfgPath := writeConfig(t, cfg)

		marker := filepath.Join(t.TempDir(), "fail-once", "marker")
		out, err := runSushiWithEnv(t, repoRoot, "run", cfgPath, capturePath, []string{"SUSHI_FAKE_CLIENT_FAIL_ONCE=" + marker})
		if err != nil {
			t.Fatalf("expected fallback run success: %v\n%s", err, out)
		}
		if !strings.Contains(out, "attempting next source after retryable converge failure") {
			t.Fatalf("expected retryable fallback log, got\n%s", out)
		}
		args, readErr := os.ReadFile(capturePath)
		if readErr != nil {
			t.Fatalf("read capture args: %v", readErr)
		}
		if !strings.Contains(string(args), "-z") {
			t.Fatalf("expected local fallback to use -z, args=%s", args)
		}
	})

	t.Run("remote matrix", func(t *testing.T) {
		for _, tc := range remoteCases(t) {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				cacheDir := filepath.Join(t.TempDir(), "cache")
				cfgPath := writeRemoteConfig(t, fakeClient, tc.sourceURL, tc.checksumURL, cacheDir, tc.allowInsecure, tc.requireChecksum, "")
				for _, command := range []string{"print-plan", "doctor", "run"} {
					out, err := runSushi(t, repoRoot, command, cfgPath, capturePath)
					if tc.wantErr {
						if err == nil {
							t.Fatalf("%s expected failure, got success\n%s", command, out)
						}
						if tc.wantErrContains != "" && !strings.Contains(out, tc.wantErrContains) {
							t.Fatalf("%s output missing %q\n%s", command, tc.wantErrContains, out)
						}
						continue
					}
					if err != nil {
						t.Fatalf("%s unexpected failure: %v\n%s", command, err, out)
					}
					if command == "doctor" {
						if !strings.Contains(out, "source resolution: OK (selected remote)") {
							t.Fatalf("%s output missing remote doctor status\n%s", command, out)
						}
					} else if !strings.Contains(out, "selected source: remote") {
						t.Fatalf("%s output missing remote selection\n%s", command, out)
					}
					if command == "print-plan" {
						for _, want := range tc.wantSubstrs {
							if !strings.Contains(out, want) {
								t.Fatalf("%s output missing %q\n%s", command, want, out)
							}
						}
					}
				}
			})
		}
	})

	t.Run("fetch command", func(t *testing.T) {
		caseItem := remoteCases(t)[0]
		cacheDir := filepath.Join(t.TempDir(), "cache")
		cfgPath := writeRemoteConfig(t, fakeClient, caseItem.sourceURL, caseItem.checksumURL, cacheDir, true, true, "")
		out, err := runSushi(t, repoRoot, "fetch", cfgPath, capturePath)
		if err != nil {
			t.Fatalf("fetch failed: %v\n%s", err, out)
		}
		if !strings.Contains(out, "fetch result:") || !strings.Contains(out, "bundle digest:") {
			t.Fatalf("fetch output missing fields\n%s", out)
		}
	})

	t.Run("fetch command refreshes metadata on HTTP 304", func(t *testing.T) {
		bundle := buildRemoteBundleGzip(t)
		checksum := sha256Hex(bundle)
		cacheDir := filepath.Join(t.TempDir(), "cache")

		var ifNoneMatch string
		var requests int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/bundle.tar.gz":
				ifNoneMatch = r.Header.Get("If-None-Match")
				if atomic.AddInt32(&requests, 1) == 1 {
					w.Header().Set("ETag", `"etag-v1"`)
					w.Header().Set("Cache-Control", "max-age=1")
					_, _ = w.Write(bundle)
					return
				}
				w.Header().Set("ETag", `"etag-v1"`)
				w.Header().Set("Cache-Control", "max-age=120")
				w.WriteHeader(http.StatusNotModified)
			case "/checksum.good":
				_, _ = w.Write([]byte(checksum + "\n"))
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		cfgPath := writeRemoteConfig(t, fakeClient, server.URL+"/bundle.tar.gz", server.URL+"/checksum.good", cacheDir, true, true, "")

		if out, err := runSushi(t, repoRoot, "fetch", cfgPath, capturePath); err != nil {
			t.Fatalf("initial fetch failed: %v\n%s", err, out)
		}
		metaBefore := readCurrentMetadata(t, cacheDir)
		time.Sleep(1200 * time.Millisecond)

		out, err := runSushi(t, repoRoot, "fetch", cfgPath, capturePath)
		if err != nil {
			t.Fatalf("second fetch failed: %v\n%s", err, out)
		}
		if !strings.Contains(out, "HTTP 304") {
			t.Fatalf("expected 304 refresh reason, got\n%s", out)
		}
		if ifNoneMatch != `"etag-v1"` {
			t.Fatalf("expected If-None-Match header, got %q", ifNoneMatch)
		}

		metaAfter := readCurrentMetadata(t, cacheDir)
		if !metaAfter.FetchedAt.After(metaBefore.FetchedAt) {
			t.Fatalf("expected fetched_at to refresh, before=%s after=%s", metaBefore.FetchedAt, metaAfter.FetchedAt)
		}
		if !metaAfter.ExpiresAt.After(metaBefore.ExpiresAt) {
			t.Fatalf("expected expires_at to refresh, before=%s after=%s", metaBefore.ExpiresAt, metaAfter.ExpiresAt)
		}
	})

	t.Run("fetch command refreshes metadata on Last-Modified-only HTTP 304", func(t *testing.T) {
		bundle := buildRemoteBundleGzip(t)
		cacheDir := filepath.Join(t.TempDir(), "cache")

		var ifModifiedSince string
		var requests int32
		lastModified := time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/bundle.tar.gz":
				ifModifiedSince = r.Header.Get("If-Modified-Since")
				if atomic.AddInt32(&requests, 1) == 1 {
					w.Header().Set("Last-Modified", lastModified)
					w.Header().Set("Cache-Control", "max-age=1")
					_, _ = w.Write(bundle)
					return
				}
				w.Header().Set("Last-Modified", lastModified)
				w.Header().Set("Cache-Control", "max-age=90")
				w.WriteHeader(http.StatusNotModified)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		cfgPath := writeRemoteConfig(t, fakeClient, server.URL+"/bundle.tar.gz", "", cacheDir, true, false, "")
		if out, err := runSushi(t, repoRoot, "fetch", cfgPath, capturePath); err != nil {
			t.Fatalf("initial fetch failed: %v\n%s", err, out)
		}
		metaBefore := readCurrentMetadata(t, cacheDir)
		time.Sleep(1200 * time.Millisecond)

		out, err := runSushi(t, repoRoot, "fetch", cfgPath, capturePath)
		if err != nil {
			t.Fatalf("second fetch failed: %v\n%s", err, out)
		}
		if !strings.Contains(out, "HTTP 304") {
			t.Fatalf("expected 304 refresh reason, got\n%s", out)
		}
		if ifModifiedSince == "" {
			t.Fatal("expected If-Modified-Since header to be sent")
		}

		metaAfter := readCurrentMetadata(t, cacheDir)
		if !metaAfter.ExpiresAt.After(metaBefore.ExpiresAt) {
			t.Fatalf("expected expires_at to refresh, before=%s after=%s", metaBefore.ExpiresAt, metaAfter.ExpiresAt)
		}
	})

	t.Run("fetch command downloads new bundle when cache validator changes", func(t *testing.T) {
		bundleV1 := buildRemoteBundleGzip(t)
		cacheDir := filepath.Join(t.TempDir(), "cache")

		var ifNoneMatch string
		var requests int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/bundle.tar.gz":
				ifNoneMatch = r.Header.Get("If-None-Match")
				if atomic.AddInt32(&requests, 1) == 1 {
					w.Header().Set("ETag", `"etag-v1"`)
					_, _ = w.Write(bundleV1)
					return
				}
				w.Header().Set("ETag", `"etag-v2"`)
				_, _ = w.Write(bundleV1)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		cfgPath := writeRemoteConfig(t, fakeClient, server.URL+"/bundle.tar.gz", "", cacheDir, true, false, "")
		if out, err := runSushi(t, repoRoot, "fetch", cfgPath, capturePath); err != nil {
			t.Fatalf("initial fetch failed: %v\n%s", err, out)
		}
		metaBefore := readCurrentMetadata(t, cacheDir)

		out, err := runSushi(t, repoRoot, "fetch", cfgPath, capturePath)
		if err != nil {
			t.Fatalf("second fetch failed: %v\n%s", err, out)
		}
		if strings.Contains(out, "HTTP 304") {
			t.Fatalf("expected full fetch on validator change, got\n%s", out)
		}
		if ifNoneMatch != `"etag-v1"` {
			t.Fatalf("expected If-None-Match header from prior metadata, got %q", ifNoneMatch)
		}
		metaAfter := readCurrentMetadata(t, cacheDir)
		if metaAfter.ETag != `"etag-v2"` {
			t.Fatalf("expected updated etag, got %q", metaAfter.ETag)
		}
		if metaAfter.Digest != metaBefore.Digest {
			t.Fatalf("expected digest to remain same for same content, before=%s after=%s", metaBefore.Digest, metaAfter.Digest)
		}
	})

	t.Run("exit codes", func(t *testing.T) {
		t.Run("config invalid", func(t *testing.T) {
			badCfg := filepath.Join(t.TempDir(), "bad.json")
			if err := os.WriteFile(badCfg, []byte("{not-json"), 0o644); err != nil {
				t.Fatalf("write bad config: %v", err)
			}
			_, exitCode := runSushiExitCode(t, repoRoot, "print-plan", badCfg, capturePath, nil)
			if exitCode != 10 {
				t.Fatalf("expected exit code 10, got %d", exitCode)
			}
		})

		t.Run("dependency missing", func(t *testing.T) {
			cfgPath := writeLocalConfigWithClient(t, "missing-client-binary")
			_, exitCode := runSushiExitCode(t, repoRoot, "doctor", cfgPath, capturePath, nil)
			if exitCode != 11 {
				t.Fatalf("expected exit code 11, got %d", exitCode)
			}
		})

		t.Run("source unavailable", func(t *testing.T) {
			cfgPath := writeRemoteConfig(t, fakeClient, "http://127.0.0.1:1/bundle.tar.gz", "", filepath.Join(t.TempDir(), "cache"), true, false, "")
			_, exitCode := runSushiExitCode(t, repoRoot, "print-plan", cfgPath, capturePath, nil)
			if exitCode != 12 {
				t.Fatalf("expected exit code 12, got %d", exitCode)
			}
		})

		t.Run("converge failure", func(t *testing.T) {
			cfgPath := writeLocalConfig(t, fakeClient)
			_, exitCode := runSushiExitCode(t, repoRoot, "run", cfgPath, capturePath, []string{"SUSHI_FAKE_CLIENT_EXIT_CODE=5"})
			if exitCode != 14 {
				t.Fatalf("expected exit code 14, got %d", exitCode)
			}
		})

		t.Run("stale cache policy violation", func(t *testing.T) {
			cacheDir := filepath.Join(t.TempDir(), "cache")
			seedCacheDir := filepath.Join(cacheDir, "bundles", "stale-digest", "cookbooks")
			if err := os.MkdirAll(seedCacheDir, 0o755); err != nil {
				t.Fatalf("seed cache bundle dir: %v", err)
			}
			staleMeta := cacheMetadata{
				Digest:    "stale-digest",
				FetchedAt: time.Now().Add(-2 * time.Hour).UTC(),
				SourceURL: "http://127.0.0.1:1/bundle.tar.gz",
				ExpiresAt: time.Now().Add(-time.Hour).UTC(),
			}
			writeCurrentMetadata(t, cacheDir, staleMeta)

			cfg := testConfig{}
			cfg.Runtime.ClientBinary = fakeClient
			cfg.SourceOrder = []string{"remote"}
			cfg.Sources.Remote.Enabled = true
			cfg.Sources.Remote.URL = "http://127.0.0.1:1/bundle.tar.gz"
			cfg.Sources.Remote.AllowInsecure = true
			cfg.Sources.Remote.AllowFallback = true
			cfg.Sources.Remote.FailIfStale = true
			cfg.Sources.Remote.MaxCacheAge = "1m"
			cfg.Sources.Remote.RefreshInterval = "0s"
			cfg.Sources.Remote.CacheDir = cacheDir
			cfgPath := writeConfig(t, cfg)

			_, exitCode := runSushiExitCode(t, repoRoot, "print-plan", cfgPath, capturePath, nil)
			if exitCode != 13 {
				t.Fatalf("expected exit code 13, got %d", exitCode)
			}
		})
	})

	t.Run("chef_server", func(t *testing.T) {
		clientRB := filepath.Join(t.TempDir(), "client.rb")
		if err := os.WriteFile(clientRB, []byte("chef_server_url 'https://chef.example.com'\n"), 0o644); err != nil {
			t.Fatalf("write client.rb: %v", err)
		}
		healthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer healthServer.Close()

		cfgPath := writeChefServerConfig(t, fakeClient, clientRB, healthServer.URL, "500ms")
		for _, command := range []string{"print-plan", "doctor", "run"} {
			out, err := runSushi(t, repoRoot, command, cfgPath, capturePath)
			if err != nil {
				t.Fatalf("%s failed: %v\n%s", command, err, out)
			}
			if !strings.Contains(out, "selected source: chef_server") && command != "doctor" {
				t.Fatalf("%s output missing chef_server selection\n%s", command, out)
			}
			if command == "doctor" && !strings.Contains(out, "source resolution: OK (selected chef_server)") {
				t.Fatalf("doctor output missing chef_server status\n%s", out)
			}
			if command == "run" {
				args, readErr := os.ReadFile(capturePath)
				if readErr != nil {
					t.Fatalf("read capture args: %v", readErr)
				}
				if strings.Contains(string(args), "-z") {
					t.Fatalf("chef_server run should not include -z args: %s", args)
				}
			}
		}
	})

	t.Run("chef_server falls back to local", func(t *testing.T) {
		repo := repoRoot
		clientRB := filepath.Join(t.TempDir(), "client.rb")
		if err := os.WriteFile(clientRB, []byte("chef_server_url 'https://chef.example.com'\n"), 0o644); err != nil {
			t.Fatalf("write client.rb: %v", err)
		}
		cfg := testConfig{}
		cfg.Runtime.ClientBinary = fakeClient
		cfg.SourceOrder = []string{"chef_server", "local", "remote"}
		cfg.Sources.ChefServer.Enabled = true
		cfg.Sources.ChefServer.ClientRB = clientRB
		cfg.Sources.ChefServer.Healthcheck.Endpoint = "http://127.0.0.1:1"
		cfg.Sources.ChefServer.Healthcheck.Timeout = "200ms"
		cfg.Sources.Local.Enabled = true
		cfg.Sources.Local.CookbookPath = filepath.Join(repo, "integration", "testdata", "local-cookbooks")
		cfgPath := writeConfig(t, cfg)

		out, err := runSushi(t, repoRoot, "print-plan", cfgPath, capturePath)
		if err != nil {
			t.Fatalf("print-plan failed: %v\n%s", err, out)
		}
		if !strings.Contains(out, "selected source: local") {
			t.Fatalf("expected local fallback\n%s", out)
		}
		if !strings.Contains(out, "- chef_server: healthcheck failed") {
			t.Fatalf("expected chef_server failure reason\n%s", out)
		}
	})

	t.Run("lock file", func(t *testing.T) {
		lockPath := filepath.Join(t.TempDir(), "run", "sushi.lock")
		cfgPath := writeLocalConfigWithLock(t, fakeClient, lockPath)

		if _, err := runSushi(t, repoRoot, "run", cfgPath, capturePath); err != nil {
			t.Fatalf("run with no lock should succeed: %v", err)
		}
		if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
			t.Fatalf("lock file should be cleaned up after successful run")
		}

		if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
			t.Fatalf("mkdir lock dir: %v", err)
		}
		if err := os.WriteFile(lockPath, []byte("busy"), 0o644); err != nil {
			t.Fatalf("seed lock file: %v", err)
		}
		out, err := runSushi(t, repoRoot, "run", cfgPath, capturePath)
		if err == nil {
			t.Fatalf("run should fail when lock exists\n%s", out)
		}
		if !strings.Contains(out, "lock file already exists") {
			t.Fatalf("expected lock failure output\n%s", out)
		}
	})

	t.Run("windows service lifecycle", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("windows-only service integration test")
		}

		cfgPath := writeLocalConfig(t, fakeClient)
		_, _ = runSushiService(t, repoRoot, "uninstall", cfgPath)

		out, err := runSushiService(t, repoRoot, "status", cfgPath)
		if err == nil {
			t.Fatalf("status should fail before install\n%s", out)
		}
		if !containsAny(out, "FAILED 1060", "does not exist") {
			t.Fatalf("expected missing service status error, got\n%s", out)
		}

		installOut, installErr := runSushiService(t, repoRoot, "install", cfgPath)
		if installErr != nil {
			if !containsAny(installOut, "Access is denied", "FAILED 5", "requires elevation") {
				t.Fatalf("unexpected install error\n%s", installOut)
			}
			for _, cmd := range []string{"start", "stop", "status", "uninstall"} {
				out, err := runSushiService(t, repoRoot, cmd, cfgPath)
				if err == nil {
					t.Fatalf("%s should fail when service install is denied\n%s", cmd, out)
				}
			}
			return
		}
		t.Cleanup(func() {
			_, _ = runSushiService(t, repoRoot, "stop", cfgPath)
			_, _ = runSushiService(t, repoRoot, "uninstall", cfgPath)
		})

		out, err = runSushiService(t, repoRoot, "status", cfgPath)
		if err != nil {
			t.Fatalf("status after install failed: %v\n%s", err, out)
		}

		out, err = runSushiService(t, repoRoot, "start", cfgPath)
		if err != nil && !containsAny(out, "FAILED 1056", "already running") {
			t.Fatalf("start failed: %v\n%s", err, out)
		}

		running := false
		for i := 0; i < 10; i++ {
			out, err = runSushiService(t, repoRoot, "status", cfgPath)
			if err == nil && containsAny(out, "RUNNING", "START_PENDING") {
				running = true
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		if !running {
			t.Fatalf("service did not reach running state\n%s", out)
		}

		out, err = runSushiService(t, repoRoot, "stop", cfgPath)
		if err != nil && !containsAny(out, "FAILED 1062", "not been started") {
			t.Fatalf("stop failed: %v\n%s", err, out)
		}

		stopped := false
		for i := 0; i < 10; i++ {
			out, err = runSushiService(t, repoRoot, "status", cfgPath)
			if err == nil && containsAny(out, "STOPPED") {
				stopped = true
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		if !stopped {
			t.Fatalf("service did not reach stopped state\n%s", out)
		}

		out, err = runSushiService(t, repoRoot, "uninstall", cfgPath)
		if err != nil {
			t.Fatalf("uninstall failed: %v\n%s", err, out)
		}

		out, err = runSushiService(t, repoRoot, "status", cfgPath)
		if err == nil {
			t.Fatalf("status should fail after uninstall\n%s", out)
		}
		if !containsAny(out, "FAILED 1060", "does not exist") {
			t.Fatalf("expected missing service status after uninstall, got\n%s", out)
		}
	})

	assertCapturedArgs(t, capturePath)
}

func readCurrentMetadata(t *testing.T, cacheDir string) cacheMetadata {
	t.Helper()
	bytes, err := os.ReadFile(filepath.Join(cacheDir, "metadata", "current.json"))
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	meta := cacheMetadata{}
	if err := json.Unmarshal(bytes, &meta); err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	return meta
}

func writeCurrentMetadata(t *testing.T, cacheDir string, meta cacheMetadata) {
	t.Helper()
	metadataDir := filepath.Join(cacheDir, "metadata")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata dir: %v", err)
	}
	bytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(metadataDir, "current.json"), bytes, 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
}

type remoteCase struct {
	name            string
	sourceURL       string
	checksumURL     string
	allowInsecure   bool
	requireChecksum bool
	wantErr         bool
	wantErrContains string
	wantSubstrs     []string
}

func remoteCases(t *testing.T) []remoteCase {
	t.Helper()

	gzipBundle := buildRemoteBundleGzip(t)
	tgzBundle := gzipBundle
	zstdBundle := buildRemoteBundleZstd(t)
	rstBundle := zstdBundle
	checksumGood := sha256Hex(gzipBundle)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bundle.tar.gz":
			_, _ = w.Write(gzipBundle)
		case "/bundle.tgz":
			_, _ = w.Write(tgzBundle)
		case "/bundle.tar.zst":
			_, _ = w.Write(zstdBundle)
		case "/bundle.tar.rst":
			_, _ = w.Write(rstBundle)
		case "/checksum.good":
			_, _ = w.Write([]byte(checksumGood + "  bundle.tar.gz\n"))
		case "/checksum.bad":
			_, _ = w.Write([]byte("deadbeef\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	return []remoteCase{
		{name: "good checksum", sourceURL: server.URL + "/bundle.tar.gz", checksumURL: server.URL + "/checksum.good", allowInsecure: true, requireChecksum: true, wantSubstrs: []string{"bundle digest:"}},
		{name: "bad checksum", sourceURL: server.URL + "/bundle.tar.gz", checksumURL: server.URL + "/checksum.bad", allowInsecure: true, requireChecksum: true, wantErr: true, wantErrContains: "no usable source"},
		{name: "good url", sourceURL: server.URL + "/bundle.tar.gz", checksumURL: "", allowInsecure: true, requireChecksum: false, wantSubstrs: nil},
		{name: "bad url", sourceURL: "http://", checksumURL: "", allowInsecure: true, requireChecksum: false, wantErr: true, wantErrContains: "invalid config field \"sources.remote.url\""},
		{name: "allow_insecure false blocks http", sourceURL: server.URL + "/bundle.tar.gz", checksumURL: "", allowInsecure: false, requireChecksum: false, wantErr: true, wantErrContains: "allow_insecure"},
		{name: "require_checksum true requires checksum", sourceURL: server.URL + "/bundle.tar.gz", checksumURL: "", allowInsecure: true, requireChecksum: true, wantErr: true, wantErrContains: "required when require_checksum is true"},
		{name: "supports tgz", sourceURL: server.URL + "/bundle.tgz", checksumURL: "", allowInsecure: true, requireChecksum: false, wantSubstrs: nil},
		{name: "supports zst", sourceURL: server.URL + "/bundle.tar.zst", checksumURL: "", allowInsecure: true, requireChecksum: false, wantSubstrs: nil},
		{name: "supports rst", sourceURL: server.URL + "/bundle.tar.rst", checksumURL: "", allowInsecure: true, requireChecksum: false, wantSubstrs: nil},
	}
}

func runSushi(t *testing.T, root, command, cfgPath, capturePath string) (string, error) {
	t.Helper()
	return runSushiWithEnv(t, root, command, cfgPath, capturePath, nil)
}

func runSushiWithEnv(t *testing.T, root, command, cfgPath, capturePath string, extraEnv []string) (string, error) {
	t.Helper()
	cmd := exec.Command("go", "run", "./cmd/sushi", command, "-config", cfgPath)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), append([]string{"SUSHI_FAKE_CLIENT_CAPTURE=" + capturePath}, extraEnv...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func runSushiService(t *testing.T, root, subcommand, cfgPath string) (string, error) {
	t.Helper()
	cmd := exec.Command("go", "run", "./cmd/sushi", "service", subcommand, "-config", cfgPath)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

func runSushiExitCode(t *testing.T, root, command, cfgPath, capturePath string, extraEnv []string) (string, int) {
	t.Helper()
	out, err := runSushiWithEnv(t, root, command, cfgPath, capturePath, extraEnv)
	if err == nil {
		return out, 0
	}
	if idx := strings.LastIndex(out, "exit status "); idx >= 0 {
		value := strings.TrimSpace(out[idx+len("exit status "):])
		if fields := strings.Fields(value); len(fields) > 0 {
			if code, convErr := strconv.Atoi(fields[0]); convErr == nil {
				return out, code
			}
		}
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return out, exitErr.ExitCode()
	}
	t.Fatalf("unexpected command error type: %v", err)
	return out, -1
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve caller path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), ".."))
}

func buildFakeClient(t *testing.T, root string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "fake-client")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", out, "./integration/testdata/fakeclient")
	cmd.Dir = root
	if bytes, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake client: %v\n%s", err, bytes)
	}
	return out
}

func writeLocalConfig(t *testing.T, client string) string {
	t.Helper()
	return writeLocalConfigWithLock(t, client, "")
}

func writeLocalConfigWithLock(t *testing.T, client, lockPath string) string {
	t.Helper()

	repo := repoRoot(t)
	cfg := testConfig{}
	cfg.Runtime.ClientBinary = client
	cfg.SourceOrder = []string{"local", "remote", "chef_server"}
	cfg.Sources.Local.Enabled = true
	cfg.Sources.Local.CookbookPath = filepath.Join(repo, "integration", "testdata", "local-cookbooks")
	cfg.Sources.Remote.Enabled = false
	cfg.Sources.ChefServer.Enabled = false
	cfg.Execution.LockFile = lockPath

	return writeConfig(t, cfg)
}

func writeLocalConfigWithClient(t *testing.T, client string) string {
	t.Helper()

	repo := repoRoot(t)
	cfg := testConfig{}
	cfg.Runtime.ClientBinary = client
	cfg.SourceOrder = []string{"local"}
	cfg.Sources.Local.Enabled = true
	cfg.Sources.Local.CookbookPath = filepath.Join(repo, "integration", "testdata", "local-cookbooks")

	return writeConfig(t, cfg)
}

func writeChefServerConfig(t *testing.T, client, clientRB, endpoint, timeout string) string {
	t.Helper()

	cfg := testConfig{}
	cfg.Runtime.ClientBinary = client
	cfg.SourceOrder = []string{"chef_server", "local", "remote"}
	cfg.Sources.Local.Enabled = false
	cfg.Sources.Remote.Enabled = false
	cfg.Sources.ChefServer.Enabled = true
	cfg.Sources.ChefServer.ClientRB = clientRB
	cfg.Sources.ChefServer.Healthcheck.Endpoint = endpoint
	cfg.Sources.ChefServer.Healthcheck.Timeout = timeout

	return writeConfig(t, cfg)
}

func writeRemoteConfig(t *testing.T, client, bundleURL, checksumURL, cacheDir string, allowInsecure, requireChecksum bool, lockPath string) string {
	t.Helper()

	cfg := testConfig{}
	cfg.Runtime.ClientBinary = client
	cfg.SourceOrder = []string{"local", "remote", "chef_server"}
	cfg.Sources.Local.Enabled = false
	cfg.Sources.Remote.Enabled = true
	cfg.Sources.Remote.URL = bundleURL
	cfg.Sources.Remote.ChecksumURL = checksumURL
	cfg.Sources.Remote.AllowInsecure = allowInsecure
	cfg.Sources.Remote.RequireChecksum = requireChecksum
	cfg.Sources.Remote.AllowFallback = true
	cfg.Sources.Remote.CacheDir = cacheDir
	cfg.Sources.ChefServer.Enabled = false
	cfg.Execution.LockFile = lockPath

	return writeConfig(t, cfg)
}

func writeConfig(t *testing.T, cfg testConfig) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	bytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(path, bytes, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func assertCapturedArgs(t *testing.T, capturePath string) {
	t.Helper()
	args, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatalf("expected fake client capture file: %v", err)
	}
	for _, want := range []string{"-z", "-c"} {
		if !strings.Contains(string(args), want) {
			t.Fatalf("fake client args missing %q: %s", want, args)
		}
	}
}

func buildRemoteBundleGzip(t *testing.T) []byte {
	t.Helper()
	tarBytes := buildRemoteTar(t)
	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	if _, err := gz.Write(tarBytes); err != nil {
		t.Fatalf("write gzip bundle: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	return buf.Bytes()
}

func buildRemoteBundleZstd(t *testing.T) []byte {
	t.Helper()
	tarBytes := buildRemoteTar(t)
	buf := &bytes.Buffer{}
	zw, err := zstd.NewWriter(buf)
	if err != nil {
		t.Fatalf("create zstd writer: %v", err)
	}
	if _, err := zw.Write(tarBytes); err != nil {
		t.Fatalf("write zstd bundle: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zstd writer: %v", err)
	}
	return buf.Bytes()
}

func buildRemoteTar(t *testing.T) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	tarWriter := tar.NewWriter(buf)

	entries := []struct {
		name    string
		content string
		mode    int64
		typeFlg byte
	}{
		{name: "cookbooks", mode: 0o755, typeFlg: tar.TypeDir},
		{name: "cookbooks/example", mode: 0o755, typeFlg: tar.TypeDir},
		{name: "cookbooks/example/recipes", mode: 0o755, typeFlg: tar.TypeDir},
		{name: "cookbooks/example/recipes/default.rb", content: "file '/tmp/sushi-remote' do\n  content 'remote-mode'\nend\n", mode: 0o644, typeFlg: tar.TypeReg},
	}

	for _, entry := range entries {
		hdr := &tar.Header{Name: entry.name, Mode: entry.mode, Typeflag: entry.typeFlg}
		if entry.typeFlg == tar.TypeReg {
			hdr.Size = int64(len(entry.content))
		}
		if err := tarWriter.WriteHeader(hdr); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if entry.typeFlg == tar.TypeReg {
			if _, err := tarWriter.Write([]byte(entry.content)); err != nil {
				t.Fatalf("write tar body: %v", err)
			}
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}

	return buf.Bytes()
}

func sha256Hex(bytes []byte) string {
	digest := sha256.Sum256(bytes)
	return hex.EncodeToString(digest[:])
}
