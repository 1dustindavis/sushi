package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	if out := os.Getenv("SUSHI_FAKE_CLIENT_CAPTURE"); out != "" {
		_ = os.WriteFile(out, []byte(strings.Join(os.Args[1:], "\n")), 0o644)
	}
	if marker := os.Getenv("SUSHI_FAKE_CLIENT_FAIL_ONCE"); marker != "" {
		if _, err := os.Stat(marker); os.IsNotExist(err) {
			_ = os.MkdirAll(filepath.Dir(marker), 0o755)
			_ = os.WriteFile(marker, []byte("failed"), 0o644)
			fmt.Fprintln(os.Stderr, "connection refused during cookbook sync")
			os.Exit(1)
		}
	}
	if exitCode := os.Getenv("SUSHI_FAKE_CLIENT_EXIT_CODE"); exitCode != "" {
		code, err := strconv.Atoi(exitCode)
		if err == nil && code > 0 {
			fmt.Fprintln(os.Stderr, "forced fake-client failure")
			os.Exit(code)
		}
	}
	fmt.Println("fake converge ok")
}
