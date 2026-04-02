package assert

import (
	"encoding/json"
	"fmt"
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
func Evaluate(spec domain.AssertionsSpec, status int, latencyMs int64, body []byte, schemaBytes []byte, headers map[string][]string) []domain.AssertionResult {
	var out []domain.AssertionResult

	if spec.Status != nil {
		out = append(out, Status(*spec.Status, status))
	}
	if spec.MaxLatencyMS != nil {
		out = append(out, MaxLatency(*spec.MaxLatencyMS, latencyMs))
	}

	if len(schemaBytes) > 0 {
		out = append(out, SchemaValidate(schemaBytes, body))
	}

	if len(spec.JSONPath) > 0 {
		doc, err := parseJSON(body)
		if err != nil {
			for expr, a := range spec.JSONPath {
				ctx := checkContext{kind: "jsonpath", key: expr}
				out = append(out, valueChecks(ctx, a, nil,
					fmt.Errorf("response body is not valid JSON"))...)
			}
		} else {
			for expr, a := range spec.JSONPath {
				ctx := checkContext{kind: "jsonpath", key: expr}
				val, getErr := jsonpath.Get(expr, doc)
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

func valueChecks(ctx checkContext, a domain.ValueAssertion, val any, getErr error) []domain.AssertionResult {
	var out []domain.AssertionResult
	if a.Exists {
		out = append(out, checkExists(ctx, val, getErr))
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

func checkExists(ctx checkContext, val any, getErr error) domain.AssertionResult {
	name := ctx.kind + ".exists"
	if getErr != nil {
		return domain.AssertionResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("%s %q: %v", ctx.kind, ctx.key, getErr),
		}
	}
	if isEmptyValue(val) {
		return domain.AssertionResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("%s %q: expected value to exist, got empty", ctx.kind, ctx.key),
		}
	}
	return domain.AssertionResult{
		Name:    name,
		Passed:  true,
		Message: fmt.Sprintf("%s %q exists", ctx.kind, ctx.key),
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
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	case nil:
		return "", fmt.Errorf("value is null")
	default:
		return fmt.Sprint(v), nil
	}
}

func valueToFloat64(val any) (float64, error) {
	switch v := val.(type) {
	case float64:
		return v, nil
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, fmt.Errorf("value %q is not numeric", v)
		}
		return f, nil
	default:
		return 0, fmt.Errorf("value of type %T is not numeric", val)
	}
}

func parseJSON(body []byte) (any, error) {
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func isEmptyValue(v any) bool {
	return v == nil
}
