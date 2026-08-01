package domain

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
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
	Name    string `json:"name"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// RunError represents a structured error produced by a runner.
type RunError struct {
	Kind    RunErrorKind `json:"kind"`
	Message string       `json:"message"`
}

// AssertionResult is the output of a single assertion.
type AssertionResult struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

// BodyBytes is a byte slice that serializes as readable text when it is valid
// UTF-8 (keeping artifacts greppable and diffable) and as a "base64:"-prefixed
// string otherwise.
type BodyBytes []byte

const base64Prefix = "base64:"

func (b BodyBytes) MarshalJSON() ([]byte, error) {
	if len(b) == 0 {
		return []byte(`""`), nil
	}
	if utf8.Valid(b) && !strings.HasPrefix(string(b), base64Prefix) {
		return json.Marshal(string(b))
	}
	return json.Marshal(base64Prefix + base64.StdEncoding.EncodeToString(b))
}

func (b *BodyBytes) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if encoded, ok := strings.CutPrefix(s, base64Prefix); ok {
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return err
		}
		*b = raw
		return nil
	}
	if s == "" {
		*b = nil
		return nil
	}
	*b = []byte(s)
	return nil
}

// ResponseSnapshot stores a bounded view of the response.
// Keep it generic so the domain does not depend on net/http types.
type ResponseSnapshot struct {
	Headers   map[string][]string `json:"headers,omitempty"`
	Body      BodyBytes           `json:"body,omitempty"`
	Truncated bool                `json:"truncated,omitempty"`
}

// RequestResult represents the result of executing a single request.
type RequestResult struct {
	Name   string     `json:"name"`
	Method HTTPMethod `json:"method"`
	URL    string     `json:"url"`

	// ResolvedURL is the final URL after variable substitution (may contain query params).
	ResolvedURL string `json:"resolved_url,omitempty"`

	// RequestHeaders are the headers sent in the request (after variable substitution).
	RequestHeaders map[string]string `json:"request_headers,omitempty"`

	// RequestBody is the resolved request body (captured for artifact storage).
	RequestBody BodyBytes `json:"request_body,omitempty"`

	StatusCode int   `json:"status_code"`
	LatencyMS  int64 `json:"latency_ms"`

	Assertions []AssertionResult `json:"assertions"`

	Extracts  []ExtractResult `json:"extracts,omitempty"`
	Extracted Vars            `json:"extracted,omitempty"`

	Response ResponseSnapshot `json:"response"`
	Error    *RunError        `json:"error,omitempty"`
	Attempts int              `json:"attempts,omitempty"`
}

// Failed reports whether this request should be considered failed:
// runner error, any assertion failure, or any extract failure.
func (r RequestResult) Failed() bool {
	if r.Error != nil {
		return true
	}
	for _, a := range r.Assertions {
		if !a.Passed {
			return true
		}
	}
	for _, e := range r.Extracts {
		if !e.Success {
			return true
		}
	}
	return false
}

// ArtifactSchemaVersion identifies the persisted artifact format so it can
// evolve without breaking consumers.
const ArtifactSchemaVersion = 1

// RunResult is a collection-level execution result suitable for UI and artifacts.
type RunResult struct {
	SchemaVersion int `json:"schema_version,omitempty"`

	CollectionName string `json:"collection_name"`
	CollectionPath string `json:"collection_path,omitempty"`

	EnvironmentName string `json:"environment_name,omitempty"`

	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`

	Results []RequestResult `json:"results"`
}

// RunArtifact is the persisted representation for a run.
type RunArtifact = RunResult

// IsRetryable reports whether a run error kind is transient and worth retrying.
func IsRetryable(kind RunErrorKind) bool {
	switch kind {
	case RunErrorTimeout, RunErrorDNS, RunErrorConn:
		return true
	default:
		return false
	}
}

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
