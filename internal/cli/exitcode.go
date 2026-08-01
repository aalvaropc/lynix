package cli

import (
	"context"
	"errors"

	"github.com/aalvaropc/lynix/internal/domain"
)

// Exit codes, so pipelines can distinguish failure classes:
//
//	0 — everything passed
//	1 — assertion failures (the API misbehaved)
//	2 — usage or configuration error (bad flags, YAML, missing files/vars)
//	3 — execution error (network failures, timeouts, cancellation)
const (
	exitOK           = 0
	exitAssertFailed = 1
	exitUsage        = 2
	exitExecution    = 3
)

// codedError attaches an explicit exit code to an error.
type codedError struct {
	code int
	err  error
}

func (e *codedError) Error() string { return e.err.Error() }
func (e *codedError) Unwrap() error { return e.err }

// exitCodeFor maps an error to the exit code convention above.
func exitCodeFor(err error) int {
	if err == nil {
		return exitOK
	}

	var coded *codedError
	if errors.As(err, &coded) {
		return coded.code
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return exitExecution
	}

	var oe *domain.OpError
	if errors.As(err, &oe) {
		switch oe.Kind {
		case domain.KindExecution:
			return exitExecution
		default:
			// not-found, invalid config, missing var, ...
			return exitUsage
		}
	}

	// Flag parse errors, load errors, and anything unclassified: the run
	// never executed, so treat it as a usage/config problem.
	return exitUsage
}
