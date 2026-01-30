package domain

// HTTPMethod represents an HTTP method (e.g., GET, POST).
type HTTPMethod string

const (
	MethodGet     HTTPMethod = "GET"
	MethodPost    HTTPMethod = "POST"
	MethodPut     HTTPMethod = "PUT"
	MethodPatch   HTTPMethod = "PATCH"
	MethodDelete  HTTPMethod = "DELETE"
	MethodHead    HTTPMethod = "HEAD"
	MethodOptions HTTPMethod = "OPTIONS"
)

// BodyType represents the type of payload for a request body.
type BodyType string

const (
	BodyNone BodyType = "none"
	BodyJSON BodyType = "json"
	BodyForm BodyType = "form"
	BodyRaw  BodyType = "raw"
)

// Header is a key/value representation of an HTTP header.
// In most cases you will use Headers (map) for convenience.
type Header struct {
	Name  string
	Value string
}

// Headers is a map representation of HTTP headers.
type Headers map[string]string

// BodySpec describes an HTTP request body.
// Only one of JSON/Form/Raw is typically used depending on Type.
type BodySpec struct {
	Type        BodyType
	JSON        map[string]any
	Form        map[string]string
	Raw         string
	ContentType string // Optional override (useful for raw payloads).
}

// JSONPathAssertion defines a JSONPath-based check.
// MVP supports Exists checks.
type JSONPathAssertion struct {
	Exists bool
}

// AssertionsSpec defines functional assertions for a request.
type AssertionsSpec struct {
	// Status is an expected HTTP status code (optional).
	Status *int

	// MaxLatencyMS is a maximum allowed latency in milliseconds (optional).
	MaxLatencyMS *int

	// JSONPath contains JSONPath assertions keyed by an identifier (optional).
	// Example key could be "$.data" or a friendly label.
	JSONPath map[string]JSONPathAssertion
}

// ExtractSpec defines variable extraction from responses.
// Map: variableName -> jsonpathExpression
type ExtractSpec map[string]string

// RequestSpec describes a single API request and its validation/extraction rules.
type RequestSpec struct {
	Name    string
	Method  HTTPMethod
	URL     string
	Headers Headers
	Body    BodySpec

	Assert  AssertionsSpec
	Extract ExtractSpec
}

// Collection groups multiple requests under one logical unit (Git-friendly).
type Collection struct {
	Name string

	// Vars are default variables available to all requests in the collection.
	// These can be overridden by environment vars and secrets.
	Vars Vars

	Requests []RequestSpec
}

// CollectionRef is a lightweight reference to a collection file on disk.
type CollectionRef struct {
	Name string
	Path string
}
