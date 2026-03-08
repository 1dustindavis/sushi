package config

import "testing"

func TestDefaultPathsNonEmpty(t *testing.T) {
	if DefaultConfigPath() == "" {
		t.Fatal("expected default config path")
	}
	if DefaultLogPath() == "" {
		t.Fatal("expected default log path")
	}
}
