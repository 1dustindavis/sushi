package config

import "fmt"

type ResolvedConfig struct {
	Source string
	Path   string
}

var platformConfigResolver = resolvePlatformConfig

func LoadResolvedConfig(configArg string, configArgProvided bool) (*Config, ResolvedConfig, error) {
	if configArgProvided {
		cfg, err := Load(configArg)
		if err != nil {
			return nil, ResolvedConfig{}, err
		}
		return cfg, ResolvedConfig{Source: "config argument", Path: configArg}, nil
	}

	if cfg, source, err := platformConfigResolver(); err != nil {
		return nil, ResolvedConfig{}, err
	} else if cfg != nil {
		return cfg, source, nil
	}

	defaultPath := DefaultConfigPath()
	cfg, err := Load(defaultPath)
	if err != nil {
		return nil, ResolvedConfig{}, err
	}
	return cfg, ResolvedConfig{Source: "default config path", Path: defaultPath}, nil
}

func loadConfigFromJSON(raw string, source string) (*Config, ResolvedConfig, error) {
	cfg, err := parseConfigJSON([]byte(raw))
	if err != nil {
		return nil, ResolvedConfig{}, fmt.Errorf("parse %s JSON: %w", source, err)
	}
	return cfg, ResolvedConfig{Source: source}, nil
}
