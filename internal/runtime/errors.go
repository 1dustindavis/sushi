package runtime

import "errors"

var (
	ErrConfigInvalid     = errors.New("config invalid")
	ErrDependencyMissing = errors.New("dependency missing")
	ErrSourceUnavailable = errors.New("source unavailable")
	ErrStaleCachePolicy  = errors.New("stale cache policy violation")
	ErrConvergeFailure   = errors.New("converge failure")
)
