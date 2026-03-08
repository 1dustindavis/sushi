package config

import (
	"os"
	"path/filepath"
	"runtime"
)

func DefaultConfigPath() string {
	if override := os.Getenv("SUSHI_CONFIG_PATH"); override != "" {
		return override
	}
	switch runtime.GOOS {
	case "windows":
		if base := os.Getenv("ProgramData"); base != "" {
			return filepath.Join(base, "sushi", "config.json")
		}
		return filepath.Join("C:\\", "ProgramData", "sushi", "config.json")
	case "darwin":
		return filepath.Join("/Library", "Application Support", "sushi", "config.json")
	default:
		return filepath.Join("/etc", "sushi", "config.json")
	}
}

func DefaultLogPath() string {
	if override := os.Getenv("SUSHI_LOG_PATH"); override != "" {
		return override
	}
	switch runtime.GOOS {
	case "windows":
		if base := os.Getenv("ProgramData"); base != "" {
			return filepath.Join(base, "sushi", "logs", "sushi.log")
		}
		return filepath.Join("C:\\", "ProgramData", "sushi", "logs", "sushi.log")
	case "darwin":
		return filepath.Join("/Library", "Logs", "sushi", "sushi.log")
	default:
		return filepath.Join("/var", "log", "sushi", "sushi.log")
	}
}
