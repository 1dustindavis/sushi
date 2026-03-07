package runtime

import (
	"os"
	"path/filepath"
	runtimepkg "runtime"
	"testing"
)

func TestDiscoverClientBinaryAutoPrefersCinc(t *testing.T) {
	tmp := t.TempDir()
	createFakeBinary(t, tmp, "cinc-client")
	createFakeBinary(t, tmp, "chef-client")

	oldPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", oldPath) })
	if err := os.Setenv("PATH", tmp+string(os.PathListSeparator)+oldPath); err != nil {
		t.Fatal(err)
	}

	got, err := DiscoverClientBinary("auto")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(got) != withExe("cinc-client") {
		t.Fatalf("expected cinc-client, got %s", got)
	}
}

func createFakeBinary(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, withExe(name))
	if err := os.WriteFile(path, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func withExe(name string) string {
	if runtimepkg.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
