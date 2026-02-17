package domain

import (
	"context"
	"errors"
	"net"
	"net/url"
	"syscall"
	"time"
)

// RunErrorKind is a high-level classification of runtime errors.
type RunErrorKind string

const (
	RunErrorUnknown  RunErrorKind = "unknown"
	RunErrorCanceled RunErrorKind = "canceled"
	RunErrorTimeout  RunErrorKind = "timeout"
	RunErrorDNS      RunErrorKind = "dns"
	RunErrorConn     RunErrorKind = "connection"
	RunErrorHTTP     RunErrorKind = "http"
)

// ExtractResult is the output of a single extraction rule.
type ExtractResult struct {
	Name    string
	Success bool
	Message string
}

// RunError represents a structured error produced by a runner.
type RunError struct {
	Kind    RunErrorKind
	Message string
}

// AssertionResult is the output of a single assertion.
type AssertionResult struct {
	Name    string
	Passed  bool
	Message string
}

// ResponseSnapshot stores a bounded view of the response.
// Keep it generic so the domain does not depend on net/http types.
type ResponseSnapshot struct {
	Headers   map[string][]string
	Body      []byte
	Truncated bool
}

// RequestResult represents the result of executing a single request.
type RequestResult struct {
	Name   string
	Method HTTPMethod
	URL    string

	StatusCode int
	LatencyMS  int64

	Assertions []AssertionResult

	Extracts  []ExtractResult
	Extracted Vars

	Response ResponseSnapshot
	Error    *RunError
}

// RunResult is a collection-level execution result suitable for UI and artifacts.
type RunResult struct {
	CollectionName string
	CollectionPath string

	EnvironmentName string

	StartedAt time.Time
	EndedAt   time.Time

	Results []RequestResult
}

// RunArtifact is the persisted representation for a run (MVP: same as RunResult).
type RunArtifact = RunResult

// NewRunError builds a RunError from an error using domain classification.
func NewRunError(err error) *RunError {
	if err == nil {
		return nil
	}
	return &RunError{
		Kind:    ClassifyRunError(err),
		Message: err.Error(),
	}
}

// ClassifyRunError tries to map an error to a stable UI-friendly kind.
// It avoids string parsing and relies on stdlib error types / sentinel errors.
func ClassifyRunError(err error) RunErrorKind {
	if err == nil {
		return RunErrorUnknown
	}

	// User cancellation (or explicit cancellation from callers).
	if errors.Is(err, context.Canceled) {
		return RunErrorCanceled
	}

	// Context deadline (timeouts) are common when the request has a timeout.
	if errors.Is(err, context.DeadlineExceeded) {
		return RunErrorTimeout
	}

	// url.Error wraps many network failures.
	var uerr *url.Error
	if errors.As(err, &uerr) {
		if uerr.Timeout() {
			return RunErrorTimeout
		}
		// Keep classifying the wrapped error.
		return ClassifyRunError(uerr.Unwrap())
	}

	// net.Error allows reliable timeout detection.
	var nerr net.Error
	if errors.As(err, &nerr) && nerr.Timeout() {
		return RunErrorTimeout
	}

	// DNS failures.
	var dnserr *net.DNSError
	if errors.As(err, &dnserr) {
		return RunErrorDNS
	}

	// Connection-ish syscall errors.
	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ETIMEDOUT) {
		return RunErrorConn
	}

	// net.OpError often wraps syscall errors (dial/read/write).
	var operr *net.OpError
	if errors.As(err, &operr) {
		if errors.Is(operr.Err, syscall.ECONNREFUSED) ||
			errors.Is(operr.Err, syscall.ECONNRESET) ||
			errors.Is(operr.Err, syscall.EPIPE) ||
			errors.Is(operr.Err, syscall.ETIMEDOUT) {
			return RunErrorConn
		}
	}

	return RunErrorUnknown
}
