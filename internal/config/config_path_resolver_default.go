//go:build !darwin && !windows

package config

func resolvePlatformConfig() (*Config, ResolvedConfig, error) {
	return nil, ResolvedConfig{}, nil
}
