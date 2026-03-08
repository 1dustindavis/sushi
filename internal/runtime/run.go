package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type RunRequest struct {
	ClientBinary       string
	CookbookPath       string
	ClientRBPath       string
	RunListFile        string
	JSONAttributesFile string
	LockFile           string
	LockWaitTimeout    time.Duration
	LockPollInterval   time.Duration
	LockStaleAge       time.Duration
	ConvergeTimeout    time.Duration
}

type ConvergeError struct {
	Err      error
	Output   string
	ExitCode int
}

func (e *ConvergeError) Error() string {
	return e.Err.Error()
}

func (e *ConvergeError) Unwrap() error {
	return e.Err
}

func ExecuteLocalMode(req RunRequest) error {
	if req.ClientBinary == "" {
		return fmt.Errorf("client binary is required")
	}
	if req.CookbookPath == "" {
		return fmt.Errorf("cookbook path is required")
	}

	releaseLock, err := acquireRequestedLock(req)
	if err != nil {
		return err
	}
	defer releaseLock()

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
	return executeConverge(req, args)
}

func ExecuteChefServerMode(req RunRequest) error {
	if req.ClientBinary == "" {
		return fmt.Errorf("client binary is required")
	}
	if req.ClientRBPath == "" {
		return fmt.Errorf("client.rb path is required")
	}
	if _, err := os.Stat(req.ClientRBPath); err != nil {
		return fmt.Errorf("client.rb unavailable: %w", err)
	}

	releaseLock, err := acquireRequestedLock(req)
	if err != nil {
		return err
	}
	defer releaseLock()

	args := []string{"-c", req.ClientRBPath}
	return executeConverge(req, args)
}

func executeConverge(req RunRequest, args []string) error {
	jsonInput := req.JSONAttributesFile
	if jsonInput == "" {
		jsonInput = req.RunListFile
	}
	if jsonInput != "" {
		args = append(args, "-j", jsonInput)
	}

	ctx := context.Background()
	if req.ConvergeTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.ConvergeTimeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, req.ClientBinary, args...)
	var combined bytes.Buffer
	stdout := io.MultiWriter(os.Stdout, &combined)
	stderr := io.MultiWriter(os.Stderr, &combined)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return &ConvergeError{Err: fmt.Errorf("execute converge: timed out after %s", req.ConvergeTimeout), Output: combined.String()}
		}
		exitCode := 0
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		return &ConvergeError{Err: fmt.Errorf("execute converge: %w", err), Output: combined.String(), ExitCode: exitCode}
	}
	return nil
}

func acquireRequestedLock(req RunRequest) (func(), error) {
	releaseLock := func() {}
	if req.LockFile == "" {
		return releaseLock, nil
	}
	release, err := acquireLock(req.LockFile, req.LockWaitTimeout, req.LockPollInterval, req.LockStaleAge)
	if err != nil {
		return nil, err
	}
	return release, nil
}

func AcquireLock(path string, waitTimeout time.Duration, pollInterval time.Duration, staleAge time.Duration) (func(), error) {
	return acquireLock(path, waitTimeout, pollInterval, staleAge)
}

func acquireLock(path string, waitTimeout time.Duration, pollInterval time.Duration, staleAge time.Duration) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("prepare lock directory: %w", err)
	}
	if pollInterval <= 0 {
		pollInterval = 250 * time.Millisecond
	}

	deadline := time.Now().Add(waitTimeout)
	for {
		lock, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			lockID := fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UTC().UnixNano())
			lockContents := fmt.Sprintf("pid=%d\nlock_id=%s\nacquired_at=%s\n", os.Getpid(), lockID, time.Now().UTC().Format(time.RFC3339Nano))
			if _, writeErr := lock.WriteString(lockContents); writeErr != nil {
				_ = lock.Close()
				_ = os.Remove(path)
				return nil, fmt.Errorf("initialize lock file: %w", writeErr)
			}
			_ = lock.Close()

			var wg sync.WaitGroup
			stopHeartbeat := make(chan struct{})
			if staleAge > 0 {
				heartbeatInterval := staleAge / 3
				if heartbeatInterval <= 0 {
					heartbeatInterval = 1 * time.Second
				}
				if heartbeatInterval > 5*time.Second {
					heartbeatInterval = 5 * time.Second
				}
				wg.Add(1)
				go func() {
					defer wg.Done()
					ticker := time.NewTicker(heartbeatInterval)
					defer ticker.Stop()
					for {
						select {
						case <-stopHeartbeat:
							return
						case now := <-ticker.C:
							_ = os.Chtimes(path, now, now)
						}
					}
				}()
			}

			return func() {
				close(stopHeartbeat)
				wg.Wait()
				bytes, readErr := os.ReadFile(path)
				if readErr != nil {
					return
				}
				if !strings.Contains(string(bytes), "lock_id="+lockID) {
					return
				}
				_ = os.Remove(path)
			}, nil
		}

		if !os.IsExist(err) {
			return nil, fmt.Errorf("create lock file: %w", err)
		}

		if staleAge > 0 {
			if info, statErr := os.Stat(path); statErr == nil {
				if time.Since(info.ModTime()) > staleAge {
					if removeErr := os.Remove(path); removeErr == nil || os.IsNotExist(removeErr) {
						continue
					}
				}
			}
		}

		if waitTimeout <= 0 || time.Now().After(deadline) {
			return nil, fmt.Errorf("lock file already exists: %s", path)
		}
		time.Sleep(pollInterval)
	}
}
