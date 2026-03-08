//go:build !windows

package main

import "fmt"

func serviceCommand(_ []string) error {
	return fmt.Errorf("service command is only available on Windows")
}
