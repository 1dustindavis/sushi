package main

import (
	"errors"
	"testing"

	"sushi/internal/runtime"
	"sushi/internal/source"
)

func TestMapExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "config invalid", err: errors.Join(runtime.ErrConfigInvalid, errors.New("bad")), want: exitCodeConfigInvalid},
		{name: "dependency missing", err: errors.Join(runtime.ErrDependencyMissing, errors.New("missing")), want: exitCodeDependencyMissing},
		{name: "source unavailable", err: errors.Join(runtime.ErrSourceUnavailable, errors.New("down")), want: exitCodeSourceUnavailable},
		{name: "stale cache", err: errors.Join(runtime.ErrStaleCachePolicy, errors.New("stale")), want: exitCodeStaleCachePolicy},
		{name: "converge", err: errors.Join(runtime.ErrConvergeFailure, errors.New("failed")), want: exitCodeConvergeFailed},
		{name: "default", err: errors.New("other"), want: exitCodeUnknownOperational},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := mapExitCode(tc.err); got != tc.want {
				t.Fatalf("mapExitCode()=%d, want %d", got, tc.want)
			}
		})
	}
}

func TestClassifySourceResolutionErr(t *testing.T) {
	staleErr := classifySourceResolutionErr(&source.ResolutionError{Err: errors.New("no usable source"), StaleCacheViolation: true})
	if !errors.Is(staleErr, runtime.ErrStaleCachePolicy) {
		t.Fatalf("expected stale cache classification, got %v", staleErr)
	}

	sourceErr := classifySourceResolutionErr(errors.New("no usable source"))
	if !errors.Is(sourceErr, runtime.ErrSourceUnavailable) {
		t.Fatalf("expected source unavailable classification, got %v", sourceErr)
	}
}
