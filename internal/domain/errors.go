package domain

import (
	"errors"
	"fmt"
)

// Sentinel errors for broad classification.
var (
	ErrNotFound       = errors.New("not found")
	ErrInvalidConfig  = errors.New("invalid config")
	ErrInvalidRequest = errors.New("invalid request")
	ErrMissingVar     = errors.New("missing variable")
	ErrExecution      = errors.New("execution error")
)

// ErrorKind is a coarse-grained categorization for errors.
type ErrorKind string

const (
	KindNotFound       ErrorKind = "not_found"
	KindInvalidConfig  ErrorKind = "invalid_config"
	KindInvalidRequest ErrorKind = "invalid_request"
	KindMissingVar     ErrorKind = "missing_variable"
	KindExecution      ErrorKind = "execution"
)

// Error provides consistent error metadata for domain logic.
type Error struct {
	Kind  ErrorKind
	Msg   string
	Cause error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Kind, e.Msg, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Msg)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

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
	var de *Error
	if errors.As(err, &de) {
		return de.Kind == kind
	}
	return false
}
