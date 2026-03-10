package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRotatingFileWriterRotatesAndPurges(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "sushi.log")
	writer, err := NewRotatingFileWriter(logPath, 40, 2)
	if err != nil {
		t.Fatalf("NewRotatingFileWriter() error = %v", err)
	}

	line := strings.Repeat("x", 30) + "\n"
	for i := 0; i < 6; i++ {
		if _, err := writer.Write([]byte(line)); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}

	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("expected active log file: %v", err)
	}
	if _, err := os.Stat(logPath + ".1"); err != nil {
		t.Fatalf("expected newest rotated log file: %v", err)
	}
	if _, err := os.Stat(logPath + ".2"); err != nil {
		t.Fatalf("expected oldest retained rotated log file: %v", err)
	}
	if _, err := os.Stat(logPath + ".3"); !os.IsNotExist(err) {
		t.Fatalf("expected older rotated log to be purged")
	}
}

func TestNewRotatingFileWriterRejectsInvalidSettings(t *testing.T) {
	if _, err := NewRotatingFileWriter("", 10, 1); err == nil {
		t.Fatal("expected empty path error")
	}
	if _, err := NewRotatingFileWriter(filepath.Join(t.TempDir(), "sushi.log"), 0, 1); err == nil {
		t.Fatal("expected maxBytes validation error")
	}
	if _, err := NewRotatingFileWriter(filepath.Join(t.TempDir(), "sushi.log"), 10, 0); err == nil {
		t.Fatal("expected maxBackups validation error")
	}
}
