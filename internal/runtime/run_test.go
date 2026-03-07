package runtime

import "testing"

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
