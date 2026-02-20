package assert

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PaesslerAG/jsonpath"
	"github.com/aalvaropc/lynix/internal/domain"
)

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
func Evaluate(spec domain.AssertionsSpec, status int, latencyMs int64, body []byte) []domain.AssertionResult {
	var out []domain.AssertionResult

	if spec.Status != nil {
		out = append(out, Status(*spec.Status, status))
	}
	if spec.MaxLatencyMS != nil {
		out = append(out, MaxLatency(*spec.MaxLatencyMS, latencyMs))
	}

	if len(spec.JSONPath) == 0 {
		return out
	}

	doc, err := parseJSON(body)
	if err != nil {
		for expr, a := range spec.JSONPath {
			out = append(out, jsonPathChecks(expr, a, nil,
				fmt.Errorf("response body is not valid JSON"))...)
		}
		return out
	}

	for expr, a := range spec.JSONPath {
		val, getErr := jsonpath.Get(expr, doc)
		out = append(out, jsonPathChecks(expr, a, val, getErr)...)
	}

	return out
}

func jsonPathChecks(expr string, a domain.JSONPathAssertion, val any, getErr error) []domain.AssertionResult {
	var out []domain.AssertionResult
	if a.Exists {
		out = append(out, checkExists(expr, val, getErr))
	}
	if a.Eq != nil {
		out = append(out, checkEq(expr, val, getErr, *a.Eq))
	}
	if a.Contains != nil {
		out = append(out, checkContains(expr, val, getErr, *a.Contains))
	}
	if a.Matches != nil {
		out = append(out, checkMatches(expr, val, getErr, *a.Matches))
	}
	if a.Gt != nil {
		out = append(out, checkGt(expr, val, getErr, *a.Gt))
	}
	if a.Lt != nil {
		out = append(out, checkLt(expr, val, getErr, *a.Lt))
	}
	return out
}

func checkExists(expr string, val any, getErr error) domain.AssertionResult {
	if getErr != nil {
		return domain.AssertionResult{
			Name:    "jsonpath.exists",
			Passed:  false,
			Message: fmt.Sprintf("invalid jsonpath %q: %v", expr, getErr),
		}
	}
	if isEmptyJSONPathValue(val) {
		return domain.AssertionResult{
			Name:    "jsonpath.exists",
			Passed:  false,
			Message: fmt.Sprintf("jsonpath %q: expected value to exist, got empty", expr),
		}
	}
	return domain.AssertionResult{
		Name:    "jsonpath.exists",
		Passed:  true,
		Message: fmt.Sprintf("jsonpath %q exists", expr),
	}
}

func checkEq(expr string, val any, getErr error, expected string) domain.AssertionResult {
	if getErr != nil {
		return domain.AssertionResult{
			Name:    "jsonpath.eq",
			Passed:  false,
			Message: fmt.Sprintf("jsonpath %q: %v", expr, getErr),
		}
	}
	s, err := jsonPathToString(val)
	if err != nil {
		return domain.AssertionResult{
			Name:    "jsonpath.eq",
			Passed:  false,
			Message: fmt.Sprintf("jsonpath %q: %v", expr, err),
		}
	}
	if s == expected {
		return domain.AssertionResult{
			Name:    "jsonpath.eq",
			Passed:  true,
			Message: fmt.Sprintf("jsonpath %q eq %q", expr, expected),
		}
	}
	return domain.AssertionResult{
		Name:    "jsonpath.eq",
		Passed:  false,
		Message: fmt.Sprintf("jsonpath %q: expected %q, got %q", expr, expected, s),
	}
}

func checkContains(expr string, val any, getErr error, sub string) domain.AssertionResult {
	if getErr != nil {
		return domain.AssertionResult{
			Name:    "jsonpath.contains",
			Passed:  false,
			Message: fmt.Sprintf("jsonpath %q: %v", expr, getErr),
		}
	}
	s, err := jsonPathToString(val)
	if err != nil {
		return domain.AssertionResult{
			Name:    "jsonpath.contains",
			Passed:  false,
			Message: fmt.Sprintf("jsonpath %q: %v", expr, err),
		}
	}
	if strings.Contains(s, sub) {
		return domain.AssertionResult{
			Name:    "jsonpath.contains",
			Passed:  true,
			Message: fmt.Sprintf("jsonpath %q contains %q", expr, sub),
		}
	}
	return domain.AssertionResult{
		Name:    "jsonpath.contains",
		Passed:  false,
		Message: fmt.Sprintf("jsonpath %q: %q does not contain %q", expr, s, sub),
	}
}

func checkMatches(expr string, val any, getErr error, pattern string) domain.AssertionResult {
	if getErr != nil {
		return domain.AssertionResult{
			Name:    "jsonpath.matches",
			Passed:  false,
			Message: fmt.Sprintf("jsonpath %q: %v", expr, getErr),
		}
	}
	s, err := jsonPathToString(val)
	if err != nil {
		return domain.AssertionResult{
			Name:    "jsonpath.matches",
			Passed:  false,
			Message: fmt.Sprintf("jsonpath %q: %v", expr, err),
		}
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return domain.AssertionResult{
			Name:    "jsonpath.matches",
			Passed:  false,
			Message: fmt.Sprintf("jsonpath %q: invalid regex %q: %v", expr, pattern, err),
		}
	}
	if re.MatchString(s) {
		return domain.AssertionResult{
			Name:    "jsonpath.matches",
			Passed:  true,
			Message: fmt.Sprintf("jsonpath %q matches %q", expr, pattern),
		}
	}
	return domain.AssertionResult{
		Name:    "jsonpath.matches",
		Passed:  false,
		Message: fmt.Sprintf("jsonpath %q: %q does not match %q", expr, s, pattern),
	}
}

func checkGt(expr string, val any, getErr error, threshold float64) domain.AssertionResult {
	if getErr != nil {
		return domain.AssertionResult{
			Name:    "jsonpath.gt",
			Passed:  false,
			Message: fmt.Sprintf("jsonpath %q: %v", expr, getErr),
		}
	}
	f, err := jsonPathToFloat64(val)
	if err != nil {
		return domain.AssertionResult{
			Name:    "jsonpath.gt",
			Passed:  false,
			Message: fmt.Sprintf("jsonpath %q: %v", expr, err),
		}
	}
	if f > threshold {
		return domain.AssertionResult{
			Name:    "jsonpath.gt",
			Passed:  true,
			Message: fmt.Sprintf("jsonpath %q: %v > %v", expr, f, threshold),
		}
	}
	return domain.AssertionResult{
		Name:    "jsonpath.gt",
		Passed:  false,
		Message: fmt.Sprintf("jsonpath %q: expected > %v, got %v", expr, threshold, f),
	}
}

func checkLt(expr string, val any, getErr error, threshold float64) domain.AssertionResult {
	if getErr != nil {
		return domain.AssertionResult{
			Name:    "jsonpath.lt",
			Passed:  false,
			Message: fmt.Sprintf("jsonpath %q: %v", expr, getErr),
		}
	}
	f, err := jsonPathToFloat64(val)
	if err != nil {
		return domain.AssertionResult{
			Name:    "jsonpath.lt",
			Passed:  false,
			Message: fmt.Sprintf("jsonpath %q: %v", expr, err),
		}
	}
	if f < threshold {
		return domain.AssertionResult{
			Name:    "jsonpath.lt",
			Passed:  true,
			Message: fmt.Sprintf("jsonpath %q: %v < %v", expr, f, threshold),
		}
	}
	return domain.AssertionResult{
		Name:    "jsonpath.lt",
		Passed:  false,
		Message: fmt.Sprintf("jsonpath %q: expected < %v, got %v", expr, threshold, f),
	}
}

func jsonPathToString(val any) (string, error) {
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

func jsonPathToFloat64(val any) (float64, error) {
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

func isEmptyJSONPathValue(v any) bool {
	if v == nil {
		return true
	}

	switch t := v.(type) {
	case string:
		return t == ""
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	default:
		return false
	}
}
