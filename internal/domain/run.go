package domain

import "time"

// RunErrorKind is a high-level classification of runtime errors.
type RunErrorKind string

const (
	RunErrorUnknown RunErrorKind = "unknown"
	RunErrorTimeout RunErrorKind = "timeout"
	RunErrorDNS     RunErrorKind = "dns"
	RunErrorConn    RunErrorKind = "connection"
	RunErrorHTTP    RunErrorKind = "http"
)

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

// RunResult represents the result of executing a single request.
type RunResult struct {
	RequestName string
	Method      HTTPMethod
	URL         string

	StatusCode int
	LatencyMS  int64

	Assertions []AssertionResult
	Extracted  Vars

	Response ResponseSnapshot
	Error    *RunError
}

// RunArtifact represents a persisted run for reproducibility.
type RunArtifact struct {
	ID string

	CollectionPath  string
	EnvironmentName string

	StartedAt  time.Time
	FinishedAt time.Time

	Results []RunResult
}
