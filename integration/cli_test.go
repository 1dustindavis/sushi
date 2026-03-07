package integration

import (
	"encoding/json"
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
			Enabled  bool   `json:"enabled"`
			URL      string `json:"url"`
			CacheDir string `json:"cache_dir"`
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

func TestCLICommandsLocalMode(t *testing.T) {
	t.Parallel()

	repoRoot := repoRoot(t)
	fakeClient := buildFakeClient(t, repoRoot)
	cfgPath := writeConfig(t, fakeClient)
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

func writeConfig(t *testing.T, client string) string {
	t.Helper()

	repo := repoRoot(t)
	cfg := testConfig{}
	cfg.Runtime.ClientBinary = client
	cfg.SourceOrder = []string{"local", "remote", "chef_server"}
	cfg.Sources.Local.Enabled = true
	cfg.Sources.Local.CookbookPath = filepath.Join(repo, "integration", "testdata", "local-cookbooks")
	cfg.Sources.Remote.Enabled = false
	cfg.Sources.ChefServer.Enabled = false

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
