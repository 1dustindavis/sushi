//go:build windows

package main

import "testing"

func TestBuildWindowsServiceCreateArgsWithConfig(t *testing.T) {
	exePath := `C:\\Program Files\\sushi\\sushi.exe`
	configPath := `C:\\ProgramData\\sushi\\config.json`
	args := buildWindowsServiceCreateArgs(exePath, configPath, true)

	want := []string{
		"create",
		windowsServiceName,
		"binPath=",
		`"C:\\Program Files\\sushi\\sushi.exe" service run -config "C:\\ProgramData\\sushi\\config.json"`,
		"start=",
		"auto",
		"DisplayName=",
		"sushi",
	}
	assertServiceArgs(t, args, want)
}

func TestBuildWindowsServiceCreateArgsWithoutConfig(t *testing.T) {
	exePath := `C:\\Program Files\\sushi\\sushi.exe`
	args := buildWindowsServiceCreateArgs(exePath, "", false)

	want := []string{
		"create",
		windowsServiceName,
		"binPath=",
		`"C:\\Program Files\\sushi\\sushi.exe" service run`,
		"start=",
		"auto",
		"DisplayName=",
		"sushi",
	}
	assertServiceArgs(t, args, want)
}

func assertServiceArgs(t *testing.T, args, want []string) {
	t.Helper()
	if len(args) != len(want) {
		t.Fatalf("arg length mismatch: got %d want %d", len(args), len(want))
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg[%d] mismatch: got %q want %q", i, args[i], want[i])
		}
	}
}
