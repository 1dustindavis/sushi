//go:build darwin

package config

import (
	"os/exec"
	"strings"
)

const macOSPreferenceDomain = "com.github.1dustindavis.sushi"

func resolvePlatformConfig() (*Config, ResolvedConfig, error) {
	if raw, ok := readMacOSDefaultsValue("/Library/Managed Preferences/"+macOSPreferenceDomain, "Config"); ok {
		return loadConfigFromJSON(raw, "macOS configuration profile")
	}
	if raw, ok := readMacOSDefaultsValue(macOSPreferenceDomain, "Config"); ok {
		return loadConfigFromJSON(raw, "macOS defaults")
	}
	return nil, ResolvedConfig{}, nil
}

func readMacOSDefaultsValue(domainOrPath, key string) (string, bool) {
	out, err := exec.Command("/usr/bin/defaults", "read", domainOrPath, key).Output()
	if err != nil {
		return "", false
	}
	value := strings.TrimSpace(string(out))
	if value == "" {
		return "", false
	}
	return value, true
}
