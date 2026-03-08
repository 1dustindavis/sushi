package runtime

import (
	"errors"
	"strings"
)

var DefaultRetryableExceptions = []string{
	"connection refused",
	"connection reset by peer",
	"timeout",
	"temporarily unavailable",
	"503",
}

func IsRetryableConvergeFailure(err error, exceptions []string) bool {
	if err == nil {
		return false
	}
	var convErr *ConvergeError
	if !errors.As(err, &convErr) {
		return false
	}
	output := strings.ToLower(convErr.Output)
	for _, candidate := range exceptions {
		if candidate == "" {
			continue
		}
		if strings.Contains(output, strings.ToLower(candidate)) {
			return true
		}
	}
	return false
}
