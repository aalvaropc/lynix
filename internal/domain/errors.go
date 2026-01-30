package domain

import (
	"errors"
	"fmt"
)

// Sentinel errors for broad classification.
var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidConfig = errors.New("invalid config")
	ErrMissingVar    = errors.New("missing variable")
	ErrExecution     = errors.New("execution error")
)

// ErrorKind is a coarse-grained categorization for errors.
type ErrorKind string

const (
	KindNotFound      ErrorKind = "not_found"
	KindInvalidConfig ErrorKind = "invalid_config"
	KindMissingVar    ErrorKind = "missing_variable"
	KindExecution     ErrorKind = "execution"
)

// OpError wraps an underlying error with operation context and a kind.
type OpError struct {
	Op   string
	Kind ErrorKind
	Path string // Optional: relevant file path
	Err  error
}

func (e *OpError) Error() string {
	if e == nil {
		return "<nil>"
	}

	base := fmt.Sprintf("%s: %s", e.Op, e.Kind)
	if e.Path != "" {
		base += fmt.Sprintf(" (path=%s)", e.Path)
	}
	if e.Err != nil {
		base += fmt.Sprintf(": %v", e.Err)
	}
	return base
}

func (e *OpError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// IsKind helps callers classify errors without depending on infra packages.
func IsKind(err error, kind ErrorKind) bool {
	var oe *OpError
	if errors.As(err, &oe) {
		return oe.Kind == kind
	}
	return false
}
