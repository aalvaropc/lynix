package assert

import (
	"encoding/json"
	"fmt"

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
		for expr := range spec.JSONPath {
			out = append(out, domain.AssertionResult{
				Name:    "jsonpath.exists",
				Passed:  false,
				Message: fmt.Sprintf("jsonpath %q: response body is not valid JSON", expr),
			})
		}
		return out
	}

	for expr, a := range spec.JSONPath {
		if !a.Exists {
			continue
		}
		out = append(out, assertJSONPathExists(expr, doc))
	}

	return out
}

func assertJSONPathExists(expr string, doc any) domain.AssertionResult {
	val, err := jsonpath.Get(expr, doc)
	if err != nil {
		return domain.AssertionResult{
			Name:    "jsonpath.exists",
			Passed:  false,
			Message: fmt.Sprintf("invalid jsonpath %q: %v", expr, err),
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
