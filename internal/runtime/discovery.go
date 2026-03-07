package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func DiscoverClientBinary(configured string) (string, error) {
	requested := strings.TrimSpace(configured)
	if requested == "" || requested == "auto" {
		return discoverAuto()
	}
	return lookupBinary(requested)
}

func discoverAuto() (string, error) {
	if path, err := lookupBinary("cinc-client"); err == nil {
		return path, nil
	}
	if path, err := lookupBinary("chef-client"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("no supported client binary found in PATH (looked for cinc-client, chef-client)")
}

func lookupBinary(name string) (string, error) {
	candidate := name
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(candidate), ".exe") {
		candidate = candidate + ".exe"
	}

	path, err := exec.LookPath(candidate)
	if err == nil {
		return path, nil
	}

	if runtime.GOOS == "windows" {
		alt, altErr := exec.LookPath(name)
		if altErr == nil {
			return alt, nil
		}
	}

	if strings.ContainsRune(name, os.PathSeparator) {
		if _, statErr := os.Stat(name); statErr == nil {
			return name, nil
		}
	}

	return "", fmt.Errorf("unable to locate %q: %w", name, err)
}
