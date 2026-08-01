package assert

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"regexp"
	"strconv"
	"strings"

	"github.com/PaesslerAG/jsonpath"
	"github.com/aalvaropc/lynix/internal/domain"
)

// checkContext carries the assertion target kind (e.g. "jsonpath", "header") and key
// (e.g. JSONPath expression or header name) so the 8 check functions can produce
// correctly-labelled results without hard-coding a single target.
type checkContext struct {
	kind string // "jsonpath" or "header"
	key  string // JSONPath expression or header name
}

func Status(expected int, got int) domain.AssertionResult {
	if got == expected {
		return domain.AssertionResult{
			Name:    "status",
			Passed:  true,
			Message: fmt.Sprintf("status %d", got),
		}
	}

	return domain.AssertionResult{
		Name:    "status",
		Passed:  false,
		Message: fmt.Sprintf("expected status %d, got %d", expected, got),
	}
}

func MaxLatency(maxMs int, latencyMs int64) domain.AssertionResult {
	if latencyMs <= int64(maxMs) {
		return domain.AssertionResult{
			Name:    "max_ms",
			Passed:  true,
			Message: fmt.Sprintf("latency %dms <= %dms", latencyMs, maxMs),
		}
	}

	return domain.AssertionResult{
		Name:    "max_ms",
		Passed:  false,
		Message: fmt.Sprintf("expected latency <= %dms, got %dms", maxMs, latencyMs),
	}
}

// Evaluate applies the assertions spec against the observed response data.
// It parses JSON only if JSONPath assertions are present.
// schemaBytes is the pre-loaded JSON Schema content (nil if no schema assertion).
// truncated indicates the response body was cut off (>256KB) and may not be valid JSON.
func Evaluate(spec domain.AssertionsSpec, status int, latencyMs int64, body []byte, schemaBytes []byte, headers map[string][]string, truncated bool) []domain.AssertionResult {
	var out []domain.AssertionResult

	if spec.Status != nil {
		out = append(out, Status(*spec.Status, status))
	}
	if spec.MaxLatencyMS != nil {
		out = append(out, MaxLatency(*spec.MaxLatencyMS, latencyMs))
	}

	if len(schemaBytes) > 0 {
		out = append(out, SchemaValidate(schemaBytes, body, truncated))
	}

	if len(spec.JSONPath) > 0 {
		doc, err := parseJSON(body)
		if err != nil {
			jsonErrMsg := "response body is not valid JSON"
			if truncated {
				jsonErrMsg = "response body was truncated (>256KB) and is not valid JSON"
			}
			for expr, a := range spec.JSONPath {
				ctx := checkContext{kind: "jsonpath", key: expr}
				out = append(out, valueChecks(ctx, a, nil,
					&bodyError{msg: jsonErrMsg})...)
			}
		} else {
			for expr, a := range spec.JSONPath {
				ctx := checkContext{kind: "jsonpath", key: expr}
				// Compile first: a syntactically invalid expression must be a
				// hard failure for every operator. Treating it as "absent"
				// would let exists:false pass forever on a typo'd path.
				eval, compileErr := jsonpath.New(expr)
				if compileErr != nil {
					out = append(out, valueChecks(ctx, a, nil,
						&bodyError{msg: fmt.Sprintf("invalid jsonpath expression: %v", compileErr)})...)
					continue
				}
				val, getErr := eval(context.Background(), doc)
				out = append(out, valueChecks(ctx, a, val, getErr)...)
			}
		}
	}

	for name, a := range spec.Headers {
		ctx := checkContext{kind: "header", key: name}
		val, found := lookupHeader(headers, name)
		var getErr error
		if !found {
			getErr = fmt.Errorf("header %q not present in response", name)
		}
		out = append(out, valueChecks(ctx, a, val, getErr)...)
	}

	return out
}

// lookupHeader performs a case-insensitive header lookup.
// Multi-value headers are joined with ", ".
func lookupHeader(headers map[string][]string, name string) (string, bool) {
	canonical := textproto.CanonicalMIMEHeaderKey(name)
	values, ok := headers[canonical]
	if !ok || len(values) == 0 {
		return "", false
	}
	return strings.Join(values, ", "), true
}

// bodyError marks a failure to parse the response body itself (as opposed to a
// path or header lookup miss), so exists:false does not treat it as absence.
type bodyError struct{ msg string }

func (e *bodyError) Error() string { return e.msg }

func valueChecks(ctx checkContext, a domain.ValueAssertion, val any, getErr error) []domain.AssertionResult {
	var out []domain.AssertionResult
	if a.Exists != nil {
		out = append(out, checkExists(ctx, val, getErr, *a.Exists))
	}
	if a.Eq != nil {
		out = append(out, checkEq(ctx, val, getErr, *a.Eq))
	}
	if a.Contains != nil {
		out = append(out, checkContains(ctx, val, getErr, *a.Contains))
	}
	if a.Matches != nil {
		out = append(out, checkMatches(ctx, val, getErr, *a.Matches))
	}
	if a.Gt != nil {
		out = append(out, checkGt(ctx, val, getErr, *a.Gt))
	}
	if a.Lt != nil {
		out = append(out, checkLt(ctx, val, getErr, *a.Lt))
	}
	if a.NotEq != nil {
		out = append(out, checkNotEq(ctx, val, getErr, *a.NotEq))
	}
	if a.NotContains != nil {
		out = append(out, checkNotContains(ctx, val, getErr, *a.NotContains))
	}
	return out
}

func checkExists(ctx checkContext, val any, getErr error, expected bool) domain.AssertionResult {
	name := ctx.kind + ".exists"

	var be *bodyError
	if errors.As(getErr, &be) {
		return domain.AssertionResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("%s %q: %v", ctx.kind, ctx.key, getErr),
		}
	}

	absent := getErr != nil || isEmptyValue(val)

	if expected {
		if absent {
			msg := fmt.Sprintf("%s %q: expected value to exist, got empty", ctx.kind, ctx.key)
			if getErr != nil {
				msg = fmt.Sprintf("%s %q: %v", ctx.kind, ctx.key, getErr)
			}
			return domain.AssertionResult{Name: name, Passed: false, Message: msg}
		}
		return domain.AssertionResult{
			Name:    name,
			Passed:  true,
			Message: fmt.Sprintf("%s %q exists", ctx.kind, ctx.key),
		}
	}

	if absent {
		return domain.AssertionResult{
			Name:    name,
			Passed:  true,
			Message: fmt.Sprintf("%s %q does not exist", ctx.kind, ctx.key),
		}
	}
	return domain.AssertionResult{
		Name:    name,
		Passed:  false,
		Message: fmt.Sprintf("%s %q: expected value to not exist, but it does", ctx.kind, ctx.key),
	}
}

func checkEq(ctx checkContext, val any, getErr error, expected string) domain.AssertionResult {
	name := ctx.kind + ".eq"
	if getErr != nil {
		return domain.AssertionResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("%s %q: %v", ctx.kind, ctx.key, getErr),
		}
	}
	s, err := valueToString(val)
	if err != nil {
		return domain.AssertionResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("%s %q: %v", ctx.kind, ctx.key, err),
		}
	}
	if s == expected {
		return domain.AssertionResult{
			Name:    name,
			Passed:  true,
			Message: fmt.Sprintf("%s %q eq %q", ctx.kind, ctx.key, expected),
		}
	}
	return domain.AssertionResult{
		Name:    name,
		Passed:  false,
		Message: fmt.Sprintf("%s %q: expected %q, got %q", ctx.kind, ctx.key, expected, s),
	}
}

func checkContains(ctx checkContext, val any, getErr error, sub string) domain.AssertionResult {
	name := ctx.kind + ".contains"
	if getErr != nil {
		return domain.AssertionResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("%s %q: %v", ctx.kind, ctx.key, getErr),
		}
	}
	s, err := valueToString(val)
	if err != nil {
		return domain.AssertionResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("%s %q: %v", ctx.kind, ctx.key, err),
		}
	}
	if strings.Contains(s, sub) {
		return domain.AssertionResult{
			Name:    name,
			Passed:  true,
			Message: fmt.Sprintf("%s %q contains %q", ctx.kind, ctx.key, sub),
		}
	}
	return domain.AssertionResult{
		Name:    name,
		Passed:  false,
		Message: fmt.Sprintf("%s %q: %q does not contain %q", ctx.kind, ctx.key, s, sub),
	}
}

func checkMatches(ctx checkContext, val any, getErr error, pattern string) domain.AssertionResult {
	name := ctx.kind + ".matches"
	if getErr != nil {
		return domain.AssertionResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("%s %q: %v", ctx.kind, ctx.key, getErr),
		}
	}
	s, err := valueToString(val)
	if err != nil {
		return domain.AssertionResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("%s %q: %v", ctx.kind, ctx.key, err),
		}
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return domain.AssertionResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("%s %q: invalid regex %q: %v", ctx.kind, ctx.key, pattern, err),
		}
	}
	if re.MatchString(s) {
		return domain.AssertionResult{
			Name:    name,
			Passed:  true,
			Message: fmt.Sprintf("%s %q matches %q", ctx.kind, ctx.key, pattern),
		}
	}
	return domain.AssertionResult{
		Name:    name,
		Passed:  false,
		Message: fmt.Sprintf("%s %q: %q does not match %q", ctx.kind, ctx.key, s, pattern),
	}
}

func checkGt(ctx checkContext, val any, getErr error, threshold float64) domain.AssertionResult {
	name := ctx.kind + ".gt"
	if getErr != nil {
		return domain.AssertionResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("%s %q: %v", ctx.kind, ctx.key, getErr),
		}
	}
	f, err := valueToFloat64(val)
	if err != nil {
		return domain.AssertionResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("%s %q: %v", ctx.kind, ctx.key, err),
		}
	}
	if f > threshold {
		return domain.AssertionResult{
			Name:    name,
			Passed:  true,
			Message: fmt.Sprintf("%s %q: %v > %v", ctx.kind, ctx.key, f, threshold),
		}
	}
	return domain.AssertionResult{
		Name:    name,
		Passed:  false,
		Message: fmt.Sprintf("%s %q: expected > %v, got %v", ctx.kind, ctx.key, threshold, f),
	}
}

func checkLt(ctx checkContext, val any, getErr error, threshold float64) domain.AssertionResult {
	name := ctx.kind + ".lt"
	if getErr != nil {
		return domain.AssertionResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("%s %q: %v", ctx.kind, ctx.key, getErr),
		}
	}
	f, err := valueToFloat64(val)
	if err != nil {
		return domain.AssertionResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("%s %q: %v", ctx.kind, ctx.key, err),
		}
	}
	if f < threshold {
		return domain.AssertionResult{
			Name:    name,
			Passed:  true,
			Message: fmt.Sprintf("%s %q: %v < %v", ctx.kind, ctx.key, f, threshold),
		}
	}
	return domain.AssertionResult{
		Name:    name,
		Passed:  false,
		Message: fmt.Sprintf("%s %q: expected < %v, got %v", ctx.kind, ctx.key, threshold, f),
	}
}

func checkNotEq(ctx checkContext, val any, getErr error, expected string) domain.AssertionResult {
	name := ctx.kind + ".not_eq"
	if getErr != nil {
		return domain.AssertionResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("%s %q: %v", ctx.kind, ctx.key, getErr),
		}
	}
	s, err := valueToString(val)
	if err != nil {
		return domain.AssertionResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("%s %q: %v", ctx.kind, ctx.key, err),
		}
	}
	if s != expected {
		return domain.AssertionResult{
			Name:    name,
			Passed:  true,
			Message: fmt.Sprintf("%s %q not eq %q (got %q)", ctx.kind, ctx.key, expected, s),
		}
	}
	return domain.AssertionResult{
		Name:    name,
		Passed:  false,
		Message: fmt.Sprintf("%s %q: expected not %q, but got %q", ctx.kind, ctx.key, expected, s),
	}
}

func checkNotContains(ctx checkContext, val any, getErr error, sub string) domain.AssertionResult {
	name := ctx.kind + ".not_contains"
	if getErr != nil {
		return domain.AssertionResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("%s %q: %v", ctx.kind, ctx.key, getErr),
		}
	}
	s, err := valueToString(val)
	if err != nil {
		return domain.AssertionResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("%s %q: %v", ctx.kind, ctx.key, err),
		}
	}
	if !strings.Contains(s, sub) {
		return domain.AssertionResult{
			Name:    name,
			Passed:  true,
			Message: fmt.Sprintf("%s %q does not contain %q", ctx.kind, ctx.key, sub),
		}
	}
	return domain.AssertionResult{
		Name:    name,
		Passed:  false,
		Message: fmt.Sprintf("%s %q: %q contains %q", ctx.kind, ctx.key, s, sub),
	}
}

func valueToString(val any) (string, error) {
	switch v := val.(type) {
	case string:
		return v, nil
	case json.Number:
		return v.String(), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	case nil:
		return "", fmt.Errorf("value is null")
	case []any:
		// JSONPath filters/wildcards return slices; unwrap single matches
		// (same semantics as extract) so eq works on filter results.
		if len(v) == 1 {
			return valueToString(v[0])
		}
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	default:
		return fmt.Sprint(v), nil
	}
}

func valueToFloat64(val any) (float64, error) {
	switch v := val.(type) {
	case json.Number:
		f, err := v.Float64()
		if err != nil {
			return 0, fmt.Errorf("value %q is not numeric", v.String())
		}
		return f, nil
	case float64:
		return v, nil
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, fmt.Errorf("value %q is not numeric", v)
		}
		return f, nil
	case []any:
		if len(v) == 1 {
			return valueToFloat64(v[0])
		}
		return 0, fmt.Errorf("value is a %d-element array, not a number", len(v))
	default:
		return 0, fmt.Errorf("value of type %T is not numeric", val)
	}
}

// parseJSON decodes with UseNumber so large integers (e.g. int64 IDs) keep
// their exact representation, then normalizes the document: json.Number stays
// ONLY for integers float64 cannot represent exactly. Everything else becomes
// float64 again, because gval-based JSONPath filters ($[?(@.id == 4)]) compare
// float64 and would silently stop matching against json.Number.
func parseJSON(body []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("unexpected data after JSON value")
	}
	return normalizeJSONNumbers(doc), nil
}

// float64Exact is the largest integer magnitude float64 represents exactly.
const float64Exact = int64(1) << 53

func normalizeJSONNumbers(v any) any {
	switch t := v.(type) {
	case json.Number:
		s := t.String()
		if !strings.ContainsAny(s, ".eE") {
			if i, err := t.Int64(); err == nil && i >= -float64Exact && i <= float64Exact {
				return float64(i)
			}
			return t // huge integer: keep the exact literal
		}
		if f, err := t.Float64(); err == nil {
			return f
		}
		return t
	case map[string]any:
		for k, val := range t {
			t[k] = normalizeJSONNumbers(val)
		}
	case []any:
		for i, val := range t {
			t[i] = normalizeJSONNumbers(val)
		}
	}
	return v
}

func isEmptyValue(v any) bool {
	return v == nil
}
