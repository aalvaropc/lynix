package domain

import "strings"

// DepGraph represents a dependency DAG for a set of requests.
// Each level contains request indices that can run in parallel.
// Levels are ordered: all requests in level N must complete before level N+1 starts.
type DepGraph struct {
	Levels [][]int
}

// BuildDepGraph computes execution levels from variable dependencies.
// seedVars are variables available before any request runs (collection + env vars).
func BuildDepGraph(requests []RequestSpec, seedVars Vars) DepGraph {
	if len(requests) == 0 {
		return DepGraph{}
	}

	consumed := make([]map[string]bool, len(requests))
	produced := make([]map[string]bool, len(requests))
	for i, req := range requests {
		consumed[i] = requestConsumedVars(req)
		produced[i] = requestProducedVars(req)
	}

	available := make(map[string]bool, len(seedVars))
	for k := range seedVars {
		available[k] = true
	}

	remaining := make(map[int]bool, len(requests))
	for i := range requests {
		remaining[i] = true
	}

	var levels [][]int
	for len(remaining) > 0 {
		var level []int
		for i := range remaining {
			if allSatisfied(consumed[i], available) {
				level = append(level, i)
			}
		}

		if len(level) == 0 {
			// Unresolvable dependencies — append remaining in original order.
			for i := range requests {
				if remaining[i] {
					level = append(level, i)
				}
			}
			levels = append(levels, level)
			break
		}

		// Sort level by original index for deterministic ordering.
		sortInts(level)
		levels = append(levels, level)

		for _, i := range level {
			delete(remaining, i)
			for k := range produced[i] {
				available[k] = true
			}
		}
	}

	return DepGraph{Levels: levels}
}

func requestConsumedVars(req RequestSpec) map[string]bool {
	refs := make(map[string]bool)
	for _, v := range extractVarRefs(req.URL) {
		refs[v] = true
	}
	for _, val := range req.Headers {
		for _, v := range extractVarRefs(val) {
			refs[v] = true
		}
	}
	for _, v := range bodyVarRefs(req.Body) {
		refs[v] = true
	}
	return refs
}

func requestProducedVars(req RequestSpec) map[string]bool {
	vars := make(map[string]bool)
	for k := range req.Extract {
		vars[k] = true
	}
	for k := range req.ExtractHeaders {
		vars[k] = true
	}
	return vars
}

// extractVarRefs scans a string for {{name}} placeholders and returns
// referenced variable names, excluding $-prefixed builtins.
func extractVarRefs(s string) []string {
	if !strings.Contains(s, "{{") {
		return nil
	}

	var refs []string
	for i := 0; i < len(s); {
		if i+1 < len(s) && s[i] == '{' && s[i+1] == '{' {
			start := i + 2
			end := strings.Index(s[start:], "}}")
			if end < 0 {
				break
			}
			end = start + end
			name := strings.TrimSpace(s[start:end])
			if name != "" && !strings.HasPrefix(name, "$") {
				refs = append(refs, name)
			}
			i = end + 2
			continue
		}
		i++
	}
	return refs
}

func extractJSONVarRefs(v any) []string {
	switch t := v.(type) {
	case string:
		return extractVarRefs(t)
	case map[string]any:
		var refs []string
		for _, val := range t {
			refs = append(refs, extractJSONVarRefs(val)...)
		}
		return refs
	case []any:
		var refs []string
		for _, item := range t {
			refs = append(refs, extractJSONVarRefs(item)...)
		}
		return refs
	default:
		return nil
	}
}

func bodyVarRefs(body BodySpec) []string {
	switch body.Type {
	case BodyJSON:
		if body.JSON != nil {
			return extractJSONVarRefs(body.JSON)
		}
	case BodyForm:
		var refs []string
		for _, v := range body.Form {
			refs = append(refs, extractVarRefs(v)...)
		}
		return refs
	case BodyRaw:
		return extractVarRefs(body.Raw)
	}
	return nil
}

func allSatisfied(consumed, available map[string]bool) bool {
	for k := range consumed {
		if !available[k] {
			return false
		}
	}
	return true
}

func sortInts(s []int) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
