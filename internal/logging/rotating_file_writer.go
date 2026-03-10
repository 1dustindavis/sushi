package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// RotatingFileWriter writes logs to a file and rotates when size exceeds MaxBytes.
// Rotated files use a numeric suffix (.1 newest, increasing with age), capped by MaxBackups.
type RotatingFileWriter struct {
	path       string
	maxBytes   int64
	maxBackups int

	mu   sync.Mutex
	file *os.File
}

func NewRotatingFileWriter(path string, maxBytes int64, maxBackups int) (io.Writer, error) {
	if path == "" {
		return nil, fmt.Errorf("log path must not be empty")
	}
	if maxBytes <= 0 {
		return nil, fmt.Errorf("maxBytes must be > 0")
	}
	if maxBackups < 1 {
		return nil, fmt.Errorf("maxBackups must be >= 1")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return &RotatingFileWriter{path: path, maxBytes: maxBytes, maxBackups: maxBackups, file: f}, nil
}

func (w *RotatingFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.rotateIfNeeded(int64(len(p))); err != nil {
		return 0, err
	}
	return w.file.Write(p)
}

func (w *RotatingFileWriter) rotateIfNeeded(incoming int64) error {
	info, err := w.file.Stat()
	if err != nil {
		return fmt.Errorf("stat log file: %w", err)
	}
	if info.Size()+incoming <= w.maxBytes {
		return nil
	}

	if err := w.file.Close(); err != nil {
		return fmt.Errorf("close log file before rotate: %w", err)
	}

	oldest := fmt.Sprintf("%s.%d", w.path, w.maxBackups)
	_ = os.Remove(oldest)

	for i := w.maxBackups - 1; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", w.path, i)
		dst := fmt.Sprintf("%s.%d", w.path, i+1)
		if _, err := os.Stat(src); err == nil {
			if err := os.Rename(src, dst); err != nil {
				return fmt.Errorf("rotate log %s -> %s: %w", src, dst, err)
			}
		}
	}

	if _, err := os.Stat(w.path); err == nil {
		if err := os.Rename(w.path, fmt.Sprintf("%s.1", w.path)); err != nil {
			return fmt.Errorf("rotate active log: %w", err)
		}
	}

	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open new active log file: %w", err)
	}
	w.file = f
	return nil
}
