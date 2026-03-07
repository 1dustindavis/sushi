package integration

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
			Enabled       bool   `json:"enabled"`
			URL           string `json:"url"`
			ChecksumURL   string `json:"checksum_url"`
			AllowInsecure bool   `json:"allow_insecure"`
			SkipChecksum  bool   `json:"skip_checksum"`
			CacheDir      string `json:"cache_dir"`
		} `json:"remote"`
		ChefServer struct {
			Enabled  bool   `json:"enabled"`
			ClientRB string `json:"client_rb"`
		} `json:"chef_server"`
	} `json:"sources"`
	Execution struct {
		RunListFile string `json:"run_list_file"`
	} `json:"execution"`
}

func TestIntegrationLocal(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	fakeClient := buildFakeClient(t, repoRoot)
	cfgPath := writeLocalConfig(t, fakeClient)
	capturePath := filepath.Join(t.TempDir(), "client-args.txt")

	tests := []struct {
		name        string
		command     string
		wantSubstrs []string
	}{
		{name: "print-plan", command: "print-plan", wantSubstrs: []string{"selected source: local", "- local: usable"}},
		{name: "doctor", command: "doctor", wantSubstrs: []string{"client discovery: OK", "source resolution: OK (selected local)", "doctor checks passed"}},
		{name: "run", command: "run", wantSubstrs: []string{"selected source: local", "client binary: " + fakeClient}},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("go", "run", "./cmd/sushi", tc.command, "-config", cfgPath)
			cmd.Dir = repoRoot
			cmd.Env = append(os.Environ(), "SUSHI_FAKE_CLIENT_CAPTURE="+capturePath)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("command failed: %v\noutput:\n%s", err, out)
			}
			output := string(out)
			for _, want := range tc.wantSubstrs {
				if !strings.Contains(output, want) {
					t.Fatalf("output missing %q\nfull output:\n%s", want, output)
				}
			}
		})
	}

	assertCapturedArgs(t, capturePath)
}

func TestIntegrationRemote(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	fakeClient := buildFakeClient(t, repoRoot)
	capturePath := filepath.Join(t.TempDir(), "client-args.txt")
	cacheDir := filepath.Join(t.TempDir(), "cache")

	bundleBytes := buildRemoteBundle(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bundleBytes)
	}))
	defer server.Close()

	cfgPath := writeRemoteConfig(t, fakeClient, server.URL+"/bundle.tar.gz", cacheDir)

	tests := []struct {
		name        string
		command     string
		wantSubstrs []string
	}{
		{name: "print-plan", command: "print-plan", wantSubstrs: []string{"selected source: remote", "- remote: fetched and activated remote bundle", "bundle digest:"}},
		{name: "doctor", command: "doctor", wantSubstrs: []string{"client discovery: OK", "source resolution: OK (selected remote)", "doctor checks passed"}},
		{name: "run", command: "run", wantSubstrs: []string{"selected source: remote", "client binary: " + fakeClient}},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("go", "run", "./cmd/sushi", tc.command, "-config", cfgPath)
			cmd.Dir = repoRoot
			cmd.Env = append(os.Environ(), "SUSHI_FAKE_CLIENT_CAPTURE="+capturePath)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("command failed: %v\noutput:\n%s", err, out)
			}
			output := string(out)
			for _, want := range tc.wantSubstrs {
				if !strings.Contains(output, want) {
					t.Fatalf("output missing %q\nfull output:\n%s", want, output)
				}
			}
		})
	}

	assertCapturedArgs(t, capturePath)
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

	repo := repoRoot(t)
	cfg := testConfig{}
	cfg.Runtime.ClientBinary = client
	cfg.SourceOrder = []string{"local", "remote", "chef_server"}
	cfg.Sources.Local.Enabled = true
	cfg.Sources.Local.CookbookPath = filepath.Join(repo, "integration", "testdata", "local-cookbooks")
	cfg.Sources.Remote.Enabled = false
	cfg.Sources.ChefServer.Enabled = false

	return writeConfig(t, cfg)
}

func writeRemoteConfig(t *testing.T, client, bundleURL, cacheDir string) string {
	t.Helper()

	cfg := testConfig{}
	cfg.Runtime.ClientBinary = client
	cfg.SourceOrder = []string{"local", "remote", "chef_server"}
	cfg.Sources.Local.Enabled = false
	cfg.Sources.Remote.Enabled = true
	cfg.Sources.Remote.URL = bundleURL
	cfg.Sources.Remote.AllowInsecure = true
	cfg.Sources.Remote.SkipChecksum = true
	cfg.Sources.Remote.CacheDir = cacheDir
	cfg.Sources.ChefServer.Enabled = false

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

func buildRemoteBundle(t *testing.T) []byte {
	t.Helper()

	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	tarWriter := tar.NewWriter(gz)

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
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	return buf.Bytes()
}
