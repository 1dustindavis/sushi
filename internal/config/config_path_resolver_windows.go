//go:build windows

package config

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

const windowsRegistryConfigPath = `SOFTWARE\com.github.1dustindavis.sushi`

func resolvePlatformConfig() (*Config, ResolvedConfig, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, windowsRegistryConfigPath, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		key, err = registry.OpenKey(registry.LOCAL_MACHINE, windowsRegistryConfigPath, registry.QUERY_VALUE|registry.WOW64_32KEY)
		if err != nil {
			return nil, ResolvedConfig{}, nil
		}
	}
	defer key.Close()

	value, _, err := key.GetStringValue("Config")
	if err != nil {
		return nil, ResolvedConfig{}, nil
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, ResolvedConfig{}, nil
	}

	return loadConfigFromJSON(value, "Windows registry")
}
