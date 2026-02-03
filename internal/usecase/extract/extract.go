package extract

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/PaesslerAG/jsonpath"
	"github.com/aalvaropc/lynix/internal/domain"
)

// Apply extracts variables from a JSON response body using JSONPath rules.
// rules: map[varName]jsonPathExpr
//
// Policy (MVP):
// - If body is not JSON -> every extract rule fails (no vars extracted).
// - If a rule fails -> it's reported in ExtractResult; other rules still run.
func Apply(body []byte, rules domain.ExtractSpec) (domain.Vars, []domain.ExtractResult) {
	if len(rules) == 0 {
		return domain.Vars{}, []domain.ExtractResult{}
	}

	keys := make([]string, 0, len(rules))
	for k := range rules {
		keys = append(keys, k)
	}
	sort.Strings(keys) // stable output for tests/UI

	doc, err := parseJSON(body)
	if err != nil {
		out := make([]domain.ExtractResult, 0, len(keys))
		for _, name := range keys {
			expr := strings.TrimSpace(rules[name])
			out = append(out, domain.ExtractResult{
				Name:    name,
				Success: false,
				Message: fmt.Sprintf("extract %q (%s): response body is not valid JSON", name, expr),
			})
		}
		return domain.Vars{}, out
	}

	extracted := domain.Vars{}
	results := make([]domain.ExtractResult, 0, len(keys))

	for _, name := range keys {
		expr := strings.TrimSpace(rules[name])
		if expr == "" {
			results = append(results, domain.ExtractResult{
				Name:    name,
				Success: false,
				Message: fmt.Sprintf("extract %q: empty jsonpath expression", name),
			})
			continue
		}

		val, getErr := jsonpath.Get(expr, doc)
		if getErr != nil {
			results = append(results, domain.ExtractResult{
				Name:    name,
				Success: false,
				Message: fmt.Sprintf("extract %q (%s): jsonpath error: %v", name, expr, getErr),
			})
			continue
		}

		if isEmptyValue(val) {
			results = append(results, domain.ExtractResult{
				Name:    name,
				Success: false,
				Message: fmt.Sprintf("extract %q (%s): no value found", name, expr),
			})
			continue
		}

		s, convErr := toString(val)
		if convErr != nil {
			results = append(results, domain.ExtractResult{
				Name:    name,
				Success: false,
				Message: fmt.Sprintf("extract %q (%s): cannot convert value to string: %v", name, expr, convErr),
			})
			continue
		}

		extracted[name] = s
		results = append(results, domain.ExtractResult{
			Name:    name,
			Success: true,
			Message: fmt.Sprintf("extracted %q", name),
		})
	}

	return extracted, results
}

func parseJSON(body []byte) (any, error) {
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func isEmptyValue(v any) bool {
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

func toString(v any) (string, error) {
	// Common case: jsonpath returns a slice with 1 element
	if arr, ok := v.([]any); ok {
		if len(arr) == 0 {
			return "", fmt.Errorf("empty array")
		}
		if len(arr) == 1 {
			return toString(arr[0])
		}
		b, err := json.Marshal(arr)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}

	switch t := v.(type) {
	case string:
		return t, nil
	case float64, bool, int, int64, uint64:
		return fmt.Sprint(t), nil
	case map[string]any:
		b, err := json.Marshal(t)
		if err != nil {
			return "", err
		}
		return string(b), nil
	default:
		// fallback: do not fail silently, but still allow MVP use
		return fmt.Sprint(t), nil
	}
}
