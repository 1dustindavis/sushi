package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	if out := os.Getenv("SUSHI_FAKE_CLIENT_CAPTURE"); out != "" {
		_ = os.WriteFile(out, []byte(strings.Join(os.Args[1:], "\n")), 0o644)
	}
	fmt.Println("fake converge ok")
}
