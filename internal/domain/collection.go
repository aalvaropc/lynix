package domain

import (
	"encoding/json"
	"net/url"
)

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
	Type BodyType
	JSON any
	Form map[string]string
	Raw  string
}

// Validate checks that a BodySpec has at most one body type populated.
func (b BodySpec) Validate() error {
	count := 0
	if b.JSON != nil {
		count++
	}
	if b.Form != nil {
		count++
	}
	if b.Raw != "" {
		count++
	}
	if count > 1 {
		return &Error{
			Kind: KindInvalidConfig,
			Msg:  "only one body type allowed (json, form, or raw)",
		}
	}
	if b.Type == BodyNone && count > 0 {
		return &Error{
			Kind: KindInvalidConfig,
			Msg:  "body type is none but body data is present",
		}
	}
	return nil
}

// ValidateJSONBody checks that v is a valid JSON body: object or array.
func ValidateJSONBody(v any) error {
	if v == nil {
		return nil
	}
	switch v.(type) {
	case map[string]any, []any:
		return nil
	default:
		return &Error{
			Kind: KindInvalidConfig,
			Msg:  "json body must be an object or array",
		}
	}
}

// ValueAssertion defines a value-based check (used for JSONPath and header assertions).
type ValueAssertion struct {
	Exists      *bool    // true: value exists (not null); false: value is absent or null
	Eq          *string  // toStr(value) == *Eq
	Contains    *string  // toStr(value) contains substring
	Matches     *string  // toStr(value) matches regex pattern (stdlib regexp)
	NotMatches  *string  // toStr(value) does not match regex pattern
	Gt          *float64 // numeric value > threshold
	Lt          *float64 // numeric value < threshold
	Gte         *float64 // numeric value >= threshold
	Lte         *float64 // numeric value <= threshold
	NotEq       *string  // toStr(value) != *NotEq
	NotContains *string  // toStr(value) does not contain substring
	Len         *int     // length of array/object/string == *Len
}

// BodyAssertion defines checks against the raw response body, regardless of
// content type (JSON, HTML, plain text, CSV, ...).
type BodyAssertion struct {
	Eq          *string
	Contains    *string
	NotContains *string
	Matches     *string
	NotMatches  *string
}

// AssertionsSpec defines functional assertions for a request.
type AssertionsSpec struct {
	// Status is an expected HTTP status code (optional).
	Status *int

	// StatusIn is a set of accepted HTTP status codes (optional).
	// Mutually exclusive with Status at load time.
	StatusIn []int

	// MaxLatencyMS is a maximum allowed latency in milliseconds (optional).
	MaxLatencyMS *int

	// Body contains assertions on the raw response body (optional).
	Body *BodyAssertion

	// JSONPath contains JSONPath assertions keyed by a JSONPath expression (optional).
	// The key is passed directly to jsonpath.Get(), e.g. "$.data", "$.users[0].id".
	JSONPath map[string]ValueAssertion

	// Headers contains response header assertions keyed by header name (case-insensitive).
	Headers map[string]ValueAssertion

	// Schema is a file path to a JSON Schema file (relative to collection dir).
	// The response body is validated against this schema.
	Schema *string

	// SchemaInline is an inline JSON Schema definition.
	// Cannot be used together with Schema.
	SchemaInline map[string]any
}

// ExtractSpec defines variable extraction from responses.
// Map: variableName -> jsonpathExpression
type ExtractSpec map[string]string

// ExtractHeaderSpec defines variable extraction from response headers.
// Map: variableName -> headerName
type ExtractHeaderSpec map[string]string

// RequestSpec describes a single API request and its validation/extraction rules.
type RequestSpec struct {
	Name    string
	Method  HTTPMethod
	URL     string
	Headers Headers
	Body    BodySpec
	Tags    []string

	DelayMS         *int  // delay in ms before executing this request (nil = no delay)
	TimeoutMS       *int  // per-request timeout in ms (nil = use global client timeout)
	FollowRedirects *bool // nil = follow (Go default), false = stop at redirect

	Assert         AssertionsSpec
	Extract        ExtractSpec
	ExtractHeaders ExtractHeaderSpec
}

// Collection groups multiple requests under one logical unit (Git-friendly).
type Collection struct {
	SchemaVersion int

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

// Serialize renders the body exactly as the HTTP layer sends it, so dry-run
// output and stored artifacts always match the real request. Form bodies are
// URL-encoded with sorted keys (url.Values.Encode), matching the transport.
func (b BodySpec) Serialize() []byte {
	switch b.Type {
	case BodyJSON:
		if b.JSON != nil {
			out, err := json.Marshal(b.JSON)
			if err != nil {
				return nil
			}
			return out
		}
	case BodyForm:
		if b.Form != nil {
			vals := url.Values{}
			for k, v := range b.Form {
				vals.Set(k, v)
			}
			return []byte(vals.Encode())
		}
	case BodyRaw:
		if b.Raw != "" {
			return []byte(b.Raw)
		}
	}
	return nil
}
