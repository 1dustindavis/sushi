package logging

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

const (
	FormatText = "text"
	FormatJSON = "json"
)

func New(format string, output io.Writer) (*slog.Logger, error) {
	normalized := strings.ToLower(strings.TrimSpace(format))
	if normalized == "" {
		normalized = FormatText
	}

	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	switch normalized {
	case FormatText:
		return slog.New(slog.NewTextHandler(output, opts)), nil
	case FormatJSON:
		return slog.New(slog.NewJSONHandler(output, opts)), nil
	default:
		return nil, fmt.Errorf("unsupported log format %q", format)
	}
}

func MustNewDefault(output io.Writer) *slog.Logger {
	logger, err := New(FormatText, output)
	if err != nil {
		panic(err)
	}
	slog.SetDefault(logger)
	return logger
}
