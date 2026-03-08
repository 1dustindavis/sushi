package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteLocalModeRequiresClientBinary(t *testing.T) {
	err := ExecuteLocalMode(RunRequest{CookbookPath: "/tmp/cookbooks"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExecuteLocalModeRequiresCookbookPath(t *testing.T) {
	err := ExecuteLocalMode(RunRequest{ClientBinary: "chef-client"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAcquireLockFailsWhenAlreadyPresent(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "sushi.lock")
	if err := os.WriteFile(lockPath, []byte("busy"), 0o644); err != nil {
		t.Fatalf("write lock file: %v", err)
	}

	_, err := acquireLock(lockPath)
	if err == nil || !strings.Contains(err.Error(), "lock file already exists") {
		t.Fatalf("expected lock-file error, got %v", err)
	}
}

func TestAcquireLockCreatesAndRemovesLockFile(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "nested", "sushi.lock")
	release, err := acquireLock(lockPath)
	if err != nil {
		t.Fatalf("acquire lock: %v", err)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("expected lock file to exist: %v", err)
	}
	release()
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("expected lock file to be removed, got %v", err)
	}
}
