package assert

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/PaesslerAG/jsonpath"
	"github.com/aalvaropc/lynix/internal/domain"
)

// regexCache avoids recompiling the same pattern on every evaluation.
var regexCache sync.Map // pattern string -> *regexp.Regexp | error

func compilePattern(pattern string) (*regexp.Regexp, error) {
	if v, ok := regexCache.Load(pattern); ok {
		if re, isRe := v.(*regexp.Regexp); isRe {
			return re, nil
		}
		return nil, v.(error)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		regexCache.Store(pattern, err)
		return nil, err
	}
	regexCache.Store(pattern, re)
	return re, nil
}

// checkContext carries the assertion target kind (e.g. "jsonpath", "header") and key
// (e.g. JSONPath expression or header name) so the 8 check functions can produce
// correctly-labelled results without hard-coding a single target.
type checkContext struct {
	kind string // "jsonpath" or "header"
	key  string // JSONPath expression or header name
}

// StatusIn passes when the observed status is one of the accepted codes.
func StatusIn(expected []int, got int) domain.AssertionResult {
	for _, e := range expected {
		if got == e {
			return domain.AssertionResult{
				Name:    "status",
				Passed:  true,
				Message: fmt.Sprintf("status %d in %v", got, expected),
			}
		}
	}
	return domain.AssertionResult{
		Name:    "status",
		Passed:  false,
		Message: fmt.Sprintf("expected status in %v, got %d", expected, got),
	}
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
	if len(spec.StatusIn) > 0 {
		out = append(out, StatusIn(spec.StatusIn, status))
	}
	if spec.MaxLatencyMS != nil {
		out = append(out, MaxLatency(*spec.MaxLatencyMS, latencyMs))
	}

	if spec.Body != nil {
		out = append(out, bodyChecks(*spec.Body, body, truncated)...)
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
		out = append(out, checkMatches(ctx, val, getErr, *a.Matches, false))
	}
	if a.NotMatches != nil {
		out = append(out, checkMatches(ctx, val, getErr, *a.NotMatches, true))
	}
	if a.Gt != nil {
		out = append(out, checkNumeric(ctx, val, getErr, *a.Gt, "gt"))
	}
	if a.Lt != nil {
		out = append(out, checkNumeric(ctx, val, getErr, *a.Lt, "lt"))
	}
	if a.Gte != nil {
		out = append(out, checkNumeric(ctx, val, getErr, *a.Gte, "gte"))
	}
	if a.Lte != nil {
		out = append(out, checkNumeric(ctx, val, getErr, *a.Lte, "lte"))
	}
	if a.NotEq != nil {
		out = append(out, checkNotEq(ctx, val, getErr, *a.NotEq))
	}
	if a.NotContains != nil {
		out = append(out, checkNotContains(ctx, val, getErr, *a.NotContains))
	}
	if a.Len != nil {
		out = append(out, checkLen(ctx, val, getErr, *a.Len))
	}
	return out
}

// bodyChecks evaluates assertions against the raw response body, whatever its
// content type. A truncated body cannot be asserted reliably, so every check
// fails loudly instead of comparing against partial data.
func bodyChecks(a domain.BodyAssertion, body []byte, truncated bool) []domain.AssertionResult {
	var out []domain.AssertionResult
	add := func(op string, passed bool, msg string) {
		out = append(out, domain.AssertionResult{Name: "body." + op, Passed: passed, Message: msg})
	}

	type op struct {
		name string
		set  bool
	}
	ops := []op{
		{"eq", a.Eq != nil},
		{"contains", a.Contains != nil},
		{"not_contains", a.NotContains != nil},
		{"matches", a.Matches != nil},
		{"not_matches", a.NotMatches != nil},
	}

	if truncated {
		for _, o := range ops {
			if o.set {
				add(o.name, false, "body: response body was truncated (>256KB), cannot assert reliably")
			}
		}
		return out
	}

	s := string(body)
	excerpt := func() string { return truncateForMessage(s, 120) }

	if a.Eq != nil {
		if s == *a.Eq {
			add("eq", true, "body eq expected value")
		} else {
			add("eq", false, fmt.Sprintf("body: expected %q, got %q", truncateForMessage(*a.Eq, 120), excerpt()))
		}
	}
	if a.Contains != nil {
		if strings.Contains(s, *a.Contains) {
			add("contains", true, fmt.Sprintf("body contains %q", *a.Contains))
		} else {
			add("contains", false, fmt.Sprintf("body does not contain %q (body: %q)", *a.Contains, excerpt()))
		}
	}
	if a.NotContains != nil {
		if !strings.Contains(s, *a.NotContains) {
			add("not_contains", true, fmt.Sprintf("body does not contain %q", *a.NotContains))
		} else {
			add("not_contains", false, fmt.Sprintf("body contains %q", *a.NotContains))
		}
	}
	if a.Matches != nil {
		re, err := compilePattern(*a.Matches)
		switch {
		case err != nil:
			add("matches", false, fmt.Sprintf("body: invalid regex %q: %v", *a.Matches, err))
		case re.MatchString(s):
			add("matches", true, fmt.Sprintf("body matches %q", *a.Matches))
		default:
			add("matches", false, fmt.Sprintf("body does not match %q (body: %q)", *a.Matches, excerpt()))
		}
	}
	if a.NotMatches != nil {
		re, err := compilePattern(*a.NotMatches)
		switch {
		case err != nil:
			add("not_matches", false, fmt.Sprintf("body: invalid regex %q: %v", *a.NotMatches, err))
		case !re.MatchString(s):
			add("not_matches", true, fmt.Sprintf("body does not match %q", *a.NotMatches))
		default:
			add("not_matches", false, fmt.Sprintf("body matches %q", *a.NotMatches))
		}
	}
	return out
}

func truncateForMessage(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes]) + "…"
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

func checkMatches(ctx checkContext, val any, getErr error, pattern string, negate bool) domain.AssertionResult {
	op := "matches"
	if negate {
		op = "not_matches"
	}
	name := ctx.kind + "." + op
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
	re, err := compilePattern(pattern)
	if err != nil {
		return domain.AssertionResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("%s %q: invalid regex %q: %v", ctx.kind, ctx.key, pattern, err),
		}
	}
	matched := re.MatchString(s)
	if matched != negate {
		verb := "matches"
		if negate {
			verb = "does not match"
		}
		return domain.AssertionResult{
			Name:    name,
			Passed:  true,
			Message: fmt.Sprintf("%s %q %s %q", ctx.kind, ctx.key, verb, pattern),
		}
	}
	if negate {
		return domain.AssertionResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("%s %q: %q matches %q", ctx.kind, ctx.key, s, pattern),
		}
	}
	return domain.AssertionResult{
		Name:    name,
		Passed:  false,
		Message: fmt.Sprintf("%s %q: %q does not match %q", ctx.kind, ctx.key, s, pattern),
	}
}

var numericOps = map[string]func(v, t float64) bool{
	"gt":  func(v, t float64) bool { return v > t },
	"lt":  func(v, t float64) bool { return v < t },
	"gte": func(v, t float64) bool { return v >= t },
	"lte": func(v, t float64) bool { return v <= t },
}

var numericSymbols = map[string]string{"gt": ">", "lt": "<", "gte": ">=", "lte": "<="}

func checkNumeric(ctx checkContext, val any, getErr error, threshold float64, op string) domain.AssertionResult {
	name := ctx.kind + "." + op
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
	sym := numericSymbols[op]
	if numericOps[op](f, threshold) {
		return domain.AssertionResult{
			Name:    name,
			Passed:  true,
			Message: fmt.Sprintf("%s %q: %v %s %v", ctx.kind, ctx.key, f, sym, threshold),
		}
	}
	return domain.AssertionResult{
		Name:    name,
		Passed:  false,
		Message: fmt.Sprintf("%s %q: expected %s %v, got %v", ctx.kind, ctx.key, sym, threshold, f),
	}
}

// checkLen asserts the length of an array, object, or string value. This is
// the reliable way to assert on arrays (exists passes for empty ones).
func checkLen(ctx checkContext, val any, getErr error, expected int) domain.AssertionResult {
	name := ctx.kind + ".len"
	if getErr != nil {
		return domain.AssertionResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("%s %q: %v", ctx.kind, ctx.key, getErr),
		}
	}
	var length int
	switch v := val.(type) {
	case string:
		length = utf8.RuneCountInString(v)
	case []any:
		length = len(v)
	case map[string]any:
		length = len(v)
	default:
		return domain.AssertionResult{
			Name:    name,
			Passed:  false,
			Message: fmt.Sprintf("%s %q: value of type %T has no length", ctx.kind, ctx.key, val),
		}
	}
	if length == expected {
		return domain.AssertionResult{
			Name:    name,
			Passed:  true,
			Message: fmt.Sprintf("%s %q has length %d", ctx.kind, ctx.key, length),
		}
	}
	return domain.AssertionResult{
		Name:    name,
		Passed:  false,
		Message: fmt.Sprintf("%s %q: expected length %d, got %d", ctx.kind, ctx.key, expected, length),
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
	default:
		return 0, fmt.Errorf("value of type %T is not numeric", val)
	}
}

// parseJSON decodes with UseNumber so large integers (e.g. int64 IDs) keep
// their exact representation instead of losing precision as float64.
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
	return doc, nil
}

func isEmptyValue(v any) bool {
	return v == nil
}
