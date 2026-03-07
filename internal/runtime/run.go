package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type RunRequest struct {
	ClientBinary       string
	CookbookPath       string
	RunListFile        string
	JSONAttributesFile string
}

func ExecuteLocalMode(req RunRequest) error {
	if req.ClientBinary == "" {
		return fmt.Errorf("client binary is required")
	}
	if req.CookbookPath == "" {
		return fmt.Errorf("cookbook path is required")
	}

	tmpDir, err := os.MkdirTemp("", "sushi-run-")
	if err != nil {
		return fmt.Errorf("create temp runtime dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	clientRB := filepath.Join(tmpDir, "client.rb")
	content := fmt.Sprintf("local_mode true\nchef_zero.enabled true\ncookbook_path [%q]\n", filepath.Clean(req.CookbookPath))
	if err := os.WriteFile(clientRB, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write generated client.rb: %w", err)
	}

	args := []string{"-z", "-c", clientRB}
	jsonInput := req.JSONAttributesFile
	if jsonInput == "" {
		jsonInput = req.RunListFile
	}
	if jsonInput != "" {
		args = append(args, "-j", jsonInput)
	}

	cmd := exec.Command(req.ClientBinary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("execute converge: %w", err)
	}
	return nil
}
