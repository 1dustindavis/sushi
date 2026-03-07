package logging

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewDefaultsToTextFormat(t *testing.T) {
	var buf bytes.Buffer

	logger, err := New("", &buf)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	logger.Info("hello", "key", "value")
	output := buf.String()
	if !strings.Contains(output, "level=INFO") {
		t.Fatalf("expected text logger output to contain level field, got %q", output)
	}
	if !strings.Contains(output, "msg=hello") {
		t.Fatalf("expected text logger output to contain message field, got %q", output)
	}
}

func TestNewJSONFormat(t *testing.T) {
	var buf bytes.Buffer

	logger, err := New("json", &buf)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	logger.Info("hello", "key", "value")
	output := buf.String()
	if !strings.Contains(output, `"level":"INFO"`) {
		t.Fatalf("expected JSON logger output to contain level field, got %q", output)
	}
	if !strings.Contains(output, `"msg":"hello"`) {
		t.Fatalf("expected JSON logger output to contain message field, got %q", output)
	}
}

func TestNewRejectsUnsupportedFormat(t *testing.T) {
	if _, err := New("xml", &bytes.Buffer{}); err == nil {
		t.Fatal("expected unsupported format error")
	}
}
