package assert

import (
	"testing"

	"github.com/aalvaropc/lynix/internal/domain"
)

// --- Status ---

func TestStatus_Equal(t *testing.T) {
	r := Status(200, 200)
	if !r.Passed {
		t.Fatalf("expected Passed=true for equal status")
	}
	if r.Name != "status" {
		t.Fatalf("expected Name=status, got %q", r.Name)
	}
}

func TestStatus_FailMessage(t *testing.T) {
	r := Status(200, 500)
	if r.Passed {
		t.Fatalf("expected fail")
	}
	if r.Message != "expected status 200, got 500" {
		t.Fatalf("unexpected message: %q", r.Message)
	}
}

func TestStatus_Lesser(t *testing.T) {
	r := Status(404, 200)
	if r.Passed {
		t.Fatalf("expected Passed=false when got < expected")
	}
}

// --- MaxLatency ---

func TestMaxLatency_WithinThreshold(t *testing.T) {
	r := MaxLatency(500, 100)
	if !r.Passed {
		t.Fatalf("expected Passed=true when latency within threshold")
	}
	if r.Name != "max_ms" {
		t.Fatalf("expected Name=max_ms, got %q", r.Name)
	}
}

func TestMaxLatency_ExactlyEqual(t *testing.T) {
	r := MaxLatency(500, 500)
	if !r.Passed {
		t.Fatalf("expected Passed=true when latency exactly equals threshold")
	}
}

func TestMaxLatency_FailMessage(t *testing.T) {
	r := MaxLatency(100, 250)
	if r.Passed {
		t.Fatalf("expected fail")
	}
	if r.Message != "expected latency <= 100ms, got 250ms" {
		t.Fatalf("unexpected message: %q", r.Message)
	}
}

// --- Evaluate ---

func TestEvaluate_NoAssertions(t *testing.T) {
	results := Evaluate(domain.AssertionsSpec{}, 200, 50, []byte(`{}`), nil, nil)
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestEvaluate_OnlyStatus(t *testing.T) {
	s := 200
	spec := domain.AssertionsSpec{Status: &s}
	results := Evaluate(spec, 200, 50, nil, nil, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Passed {
		t.Fatalf("expected status assertion to pass")
	}
}

func TestEvaluate_OnlyMaxLatency(t *testing.T) {
	ms := 1000
	spec := domain.AssertionsSpec{MaxLatencyMS: &ms}
	results := Evaluate(spec, 200, 500, nil, nil, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Passed {
		t.Fatalf("expected max_ms assertion to pass")
	}
}

func TestEvaluate_JSONPathExists_True(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.data.id": {Exists: true},
		},
	}

	body := []byte(`{"data":{"id":123}}`)
	out := Evaluate(spec, 200, 10, body, nil, nil)

	if len(out) != 1 {
		t.Fatalf("expected 1 result, got=%d", len(out))
	}
	if !out[0].Passed {
		t.Fatalf("expected pass, got fail: %s", out[0].Message)
	}
}

func TestEvaluate_JSONPathExists_False(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.data.missing": {Exists: true},
		},
	}

	body := []byte(`{"data":{"id":123}}`)
	out := Evaluate(spec, 200, 10, body, nil, nil)

	if len(out) != 1 {
		t.Fatalf("expected 1 result, got=%d", len(out))
	}
	if out[0].Passed {
		t.Fatalf("expected fail")
	}
}

func TestEvaluate_JSONPathExists_EmptyString_Pass(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.msg": {Exists: true},
		},
	}
	body := []byte(`{"msg":""}`)
	out := Evaluate(spec, 200, 10, body, nil, nil)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got=%d", len(out))
	}
	if !out[0].Passed {
		t.Fatalf("empty string should exist, got: %s", out[0].Message)
	}
}

func TestEvaluate_JSONPathExists_Null_Fail(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.field": {Exists: true},
		},
	}
	body := []byte(`{"field":null}`)
	out := Evaluate(spec, 200, 10, body, nil, nil)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got=%d", len(out))
	}
	if out[0].Passed {
		t.Fatalf("null value should not pass exists check")
	}
}

func TestEvaluate_JSONPathExists_EmptyArray_Pass(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.items": {Exists: true},
		},
	}
	body := []byte(`{"items":[]}`)
	out := Evaluate(spec, 200, 10, body, nil, nil)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got=%d", len(out))
	}
	if !out[0].Passed {
		t.Fatalf("empty array should exist, got: %s", out[0].Message)
	}
}

func TestEvaluate_JSONPathExists_EmptyObject_Pass(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.meta": {Exists: true},
		},
	}
	body := []byte(`{"meta":{}}`)
	out := Evaluate(spec, 200, 10, body, nil, nil)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got=%d", len(out))
	}
	if !out[0].Passed {
		t.Fatalf("empty object should exist, got: %s", out[0].Message)
	}
}

func TestEvaluate_HeaderExists_EmptyValue_Pass(t *testing.T) {
	spec := domain.AssertionsSpec{
		Headers: map[string]domain.ValueAssertion{
			"X-Empty": {Exists: true},
		},
	}
	headers := map[string][]string{"X-Empty": {""}}
	out := Evaluate(spec, 200, 10, nil, nil, headers)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got=%d", len(out))
	}
	if !out[0].Passed {
		t.Fatalf("header with empty value should exist, got: %s", out[0].Message)
	}
}

func TestEvaluate_JSONPath_NonJSONBody(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.data.id": {Exists: true},
		},
	}

	out := Evaluate(spec, 200, 10, []byte("hello"), nil, nil)

	if len(out) != 1 {
		t.Fatalf("expected 1 result, got=%d", len(out))
	}
	if out[0].Passed {
		t.Fatalf("expected fail")
	}
}

func TestEvaluate_JSONPath_InvalidExpr(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.data[": {Exists: true},
		},
	}

	body := []byte(`{"data":{"id":123}}`)
	out := Evaluate(spec, 200, 10, body, nil, nil)

	if len(out) != 1 {
		t.Fatalf("expected 1 result, got=%d", len(out))
	}
	if out[0].Passed {
		t.Fatalf("expected fail")
	}
}

func TestEvaluate_InvalidBodyMarksAllJSONPathFailed(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.name": {Exists: true},
			"$.age":  {Exists: true},
		},
	}
	out := Evaluate(spec, 200, 50, []byte("not json"), nil, nil)
	if len(out) != 2 {
		t.Fatalf("expected 2 results, got %d", len(out))
	}
	for _, r := range out {
		if r.Name != "jsonpath.exists" {
			t.Errorf("expected Name=jsonpath.exists, got %q", r.Name)
		}
		if r.Passed {
			t.Errorf("expected Passed=false for invalid JSON body")
		}
	}
}

func TestEvaluate_MultipleAssertionsCombined(t *testing.T) {
	s := 200
	ms := 500
	spec := domain.AssertionsSpec{
		Status:       &s,
		MaxLatencyMS: &ms,
		JSONPath: map[string]domain.ValueAssertion{
			"$.id": {Exists: true},
		},
	}
	results := Evaluate(spec, 200, 100, []byte(`{"id":42}`), nil, nil)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// Status is always first, max_ms second.
	if results[0].Name != "status" || !results[0].Passed {
		t.Errorf("expected status assertion to pass, got %+v", results[0])
	}
	if results[1].Name != "max_ms" || !results[1].Passed {
		t.Errorf("expected max_ms assertion to pass, got %+v", results[1])
	}
	if results[2].Name != "jsonpath.exists" || !results[2].Passed {
		t.Errorf("expected jsonpath assertion to pass, got %+v", results[2])
	}
}

func TestEvaluate_JSONPathExistsFalseSkipped(t *testing.T) {
	// Exists: false entries produce no assertion result.
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.name": {Exists: false},
		},
	}
	results := Evaluate(spec, 200, 50, []byte(`{"name":"alice"}`), nil, nil)
	if len(results) != 0 {
		t.Fatalf("expected 0 results for Exists=false, got %d", len(results))
	}
}

// --- JSONPath eq ---

func strPtr(s string) *string { return &s }

func TestEvaluate_JSONPathEq_Pass(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.name": {Eq: strPtr("alice")},
		},
	}
	out := Evaluate(spec, 200, 10, []byte(`{"name":"alice"}`), nil, nil)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	if !out[0].Passed {
		t.Fatalf("expected pass, got: %s", out[0].Message)
	}
}

func TestEvaluate_JSONPathEq_Fail(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.name": {Eq: strPtr("alice")},
		},
	}
	out := Evaluate(spec, 200, 10, []byte(`{"name":"bob"}`), nil, nil)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	if out[0].Passed {
		t.Fatalf("expected fail")
	}
}

func TestEvaluate_JSONPathEq_Number(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.count": {Eq: strPtr("42")},
		},
	}
	out := Evaluate(spec, 200, 10, []byte(`{"count":42}`), nil, nil)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	if !out[0].Passed {
		t.Fatalf("expected pass, got: %s", out[0].Message)
	}
}

// --- JSONPath contains ---

func TestEvaluate_JSONPathContains_Pass(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.name": {Contains: strPtr("ali")},
		},
	}
	out := Evaluate(spec, 200, 10, []byte(`{"name":"alice"}`), nil, nil)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	if !out[0].Passed {
		t.Fatalf("expected pass, got: %s", out[0].Message)
	}
}

func TestEvaluate_JSONPathContains_Fail(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.name": {Contains: strPtr("ali")},
		},
	}
	out := Evaluate(spec, 200, 10, []byte(`{"name":"bob"}`), nil, nil)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	if out[0].Passed {
		t.Fatalf("expected fail")
	}
}

// --- JSONPath matches ---

func TestEvaluate_JSONPathMatches_Pass(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.email": {Matches: strPtr("^.+@.+")},
		},
	}
	out := Evaluate(spec, 200, 10, []byte(`{"email":"a@b.com"}`), nil, nil)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	if !out[0].Passed {
		t.Fatalf("expected pass, got: %s", out[0].Message)
	}
}

func TestEvaluate_JSONPathMatches_Fail(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.email": {Matches: strPtr("^.+@.+")},
		},
	}
	out := Evaluate(spec, 200, 10, []byte(`{"email":"notanemail"}`), nil, nil)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	if out[0].Passed {
		t.Fatalf("expected fail")
	}
}

func TestEvaluate_JSONPathMatches_InvalidRegex(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.x": {Matches: strPtr("[invalid")},
		},
	}
	out := Evaluate(spec, 200, 10, []byte(`{"x":"y"}`), nil, nil)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	if out[0].Passed {
		t.Fatalf("expected fail for invalid regex")
	}
}

// --- JSONPath gt ---

func float64Ptr(f float64) *float64 { return &f }

func TestEvaluate_JSONPathGt_Pass(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.count": {Gt: float64Ptr(0)},
		},
	}
	out := Evaluate(spec, 200, 10, []byte(`{"count":5}`), nil, nil)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	if !out[0].Passed {
		t.Fatalf("expected pass, got: %s", out[0].Message)
	}
}

func TestEvaluate_JSONPathGt_Fail(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.count": {Gt: float64Ptr(5)},
		},
	}
	out := Evaluate(spec, 200, 10, []byte(`{"count":0}`), nil, nil)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	if out[0].Passed {
		t.Fatalf("expected fail")
	}
}

// --- JSONPath lt ---

func TestEvaluate_JSONPathLt_Pass(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.count": {Lt: float64Ptr(10)},
		},
	}
	out := Evaluate(spec, 200, 10, []byte(`{"count":5}`), nil, nil)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	if !out[0].Passed {
		t.Fatalf("expected pass, got: %s", out[0].Message)
	}
}

func TestEvaluate_JSONPathLt_Fail(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.count": {Lt: float64Ptr(5)},
		},
	}
	out := Evaluate(spec, 200, 10, []byte(`{"count":10}`), nil, nil)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	if out[0].Passed {
		t.Fatalf("expected fail")
	}
}

// --- Multiple checks on same path ---

func TestEvaluate_JSONPathMultipleChecksOnSamePath(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.name": {Eq: strPtr("alice"), Contains: strPtr("ali")},
		},
	}
	out := Evaluate(spec, 200, 10, []byte(`{"name":"alice"}`), nil, nil)
	if len(out) != 2 {
		t.Fatalf("expected 2 results, got %d", len(out))
	}
	for _, r := range out {
		if !r.Passed {
			t.Errorf("expected pass for %q, got: %s", r.Name, r.Message)
		}
	}
}

// --- Missing path ---

func TestEvaluate_JSONPathEq_MissingPath(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.name": {Eq: strPtr("alice")},
		},
	}
	out := Evaluate(spec, 200, 10, []byte(`{"x":1}`), nil, nil)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	if out[0].Passed {
		t.Fatalf("expected fail for missing path")
	}
}

// --- JSONPath not_eq ---

func TestEvaluate_JSONPathNotEq_Pass(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.name": {NotEq: strPtr("bob")},
		},
	}
	out := Evaluate(spec, 200, 10, []byte(`{"name":"alice"}`), nil, nil)
	if len(out) != 1 || !out[0].Passed {
		t.Fatalf("expected pass, got %+v", out)
	}
}

func TestEvaluate_JSONPathNotEq_Fail(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.name": {NotEq: strPtr("alice")},
		},
	}
	out := Evaluate(spec, 200, 10, []byte(`{"name":"alice"}`), nil, nil)
	if len(out) != 1 || out[0].Passed {
		t.Fatalf("expected fail, got %+v", out)
	}
}

func TestEvaluate_JSONPathNotEq_MissingPath(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.missing": {NotEq: strPtr("x")},
		},
	}
	out := Evaluate(spec, 200, 10, []byte(`{"name":"alice"}`), nil, nil)
	if len(out) != 1 || out[0].Passed {
		t.Fatalf("expected fail for missing path")
	}
}

// --- JSONPath not_contains ---

func TestEvaluate_JSONPathNotContains_Pass(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.msg": {NotContains: strPtr("error")},
		},
	}
	out := Evaluate(spec, 200, 10, []byte(`{"msg":"all good"}`), nil, nil)
	if len(out) != 1 || !out[0].Passed {
		t.Fatalf("expected pass, got %+v", out)
	}
}

func TestEvaluate_JSONPathNotContains_Fail(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.msg": {NotContains: strPtr("error")},
		},
	}
	out := Evaluate(spec, 200, 10, []byte(`{"msg":"internal error occurred"}`), nil, nil)
	if len(out) != 1 || out[0].Passed {
		t.Fatalf("expected fail, got %+v", out)
	}
}

func TestEvaluate_JSONPathNotContains_MissingPath(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.ValueAssertion{
			"$.missing": {NotContains: strPtr("x")},
		},
	}
	out := Evaluate(spec, 200, 10, []byte(`{"msg":"ok"}`), nil, nil)
	if len(out) != 1 || out[0].Passed {
		t.Fatalf("expected fail for missing path")
	}
}

// --- Header assertions ---

func TestEvaluate_HeaderExists_Pass(t *testing.T) {
	spec := domain.AssertionsSpec{
		Headers: map[string]domain.ValueAssertion{
			"Content-Type": {Exists: true},
		},
	}
	headers := map[string][]string{"Content-Type": {"application/json"}}
	out := Evaluate(spec, 200, 10, nil, nil, headers)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	if !out[0].Passed {
		t.Fatalf("expected pass, got: %s", out[0].Message)
	}
	if out[0].Name != "header.exists" {
		t.Fatalf("expected Name=header.exists, got %q", out[0].Name)
	}
}

func TestEvaluate_HeaderExists_Fail(t *testing.T) {
	spec := domain.AssertionsSpec{
		Headers: map[string]domain.ValueAssertion{
			"X-Custom": {Exists: true},
		},
	}
	headers := map[string][]string{"Content-Type": {"text/html"}}
	out := Evaluate(spec, 200, 10, nil, nil, headers)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	if out[0].Passed {
		t.Fatalf("expected fail")
	}
}

func TestEvaluate_HeaderEq_Pass(t *testing.T) {
	spec := domain.AssertionsSpec{
		Headers: map[string]domain.ValueAssertion{
			"Content-Type": {Eq: strPtr("application/json")},
		},
	}
	headers := map[string][]string{"Content-Type": {"application/json"}}
	out := Evaluate(spec, 200, 10, nil, nil, headers)
	if len(out) != 1 || !out[0].Passed {
		t.Fatalf("expected pass, got %+v", out)
	}
}

func TestEvaluate_HeaderEq_Fail(t *testing.T) {
	spec := domain.AssertionsSpec{
		Headers: map[string]domain.ValueAssertion{
			"Content-Type": {Eq: strPtr("application/json")},
		},
	}
	headers := map[string][]string{"Content-Type": {"text/html"}}
	out := Evaluate(spec, 200, 10, nil, nil, headers)
	if len(out) != 1 || out[0].Passed {
		t.Fatalf("expected fail, got %+v", out)
	}
}

func TestEvaluate_HeaderEq_CaseInsensitive(t *testing.T) {
	spec := domain.AssertionsSpec{
		Headers: map[string]domain.ValueAssertion{
			"content-type": {Eq: strPtr("application/json")},
		},
	}
	headers := map[string][]string{"Content-Type": {"application/json"}}
	out := Evaluate(spec, 200, 10, nil, nil, headers)
	if len(out) != 1 || !out[0].Passed {
		t.Fatalf("expected pass (case-insensitive), got %+v", out)
	}
}

func TestEvaluate_HeaderContains_Pass(t *testing.T) {
	spec := domain.AssertionsSpec{
		Headers: map[string]domain.ValueAssertion{
			"Content-Type": {Contains: strPtr("json")},
		},
	}
	headers := map[string][]string{"Content-Type": {"application/json; charset=utf-8"}}
	out := Evaluate(spec, 200, 10, nil, nil, headers)
	if len(out) != 1 || !out[0].Passed {
		t.Fatalf("expected pass, got %+v", out)
	}
}

func TestEvaluate_HeaderContains_Fail(t *testing.T) {
	spec := domain.AssertionsSpec{
		Headers: map[string]domain.ValueAssertion{
			"Content-Type": {Contains: strPtr("xml")},
		},
	}
	headers := map[string][]string{"Content-Type": {"application/json"}}
	out := Evaluate(spec, 200, 10, nil, nil, headers)
	if len(out) != 1 || out[0].Passed {
		t.Fatalf("expected fail, got %+v", out)
	}
}

func TestEvaluate_HeaderNotEq_Pass(t *testing.T) {
	spec := domain.AssertionsSpec{
		Headers: map[string]domain.ValueAssertion{
			"Content-Type": {NotEq: strPtr("text/html")},
		},
	}
	headers := map[string][]string{"Content-Type": {"application/json"}}
	out := Evaluate(spec, 200, 10, nil, nil, headers)
	if len(out) != 1 || !out[0].Passed {
		t.Fatalf("expected pass, got %+v", out)
	}
}

func TestEvaluate_HeaderNotEq_Fail(t *testing.T) {
	spec := domain.AssertionsSpec{
		Headers: map[string]domain.ValueAssertion{
			"Content-Type": {NotEq: strPtr("application/json")},
		},
	}
	headers := map[string][]string{"Content-Type": {"application/json"}}
	out := Evaluate(spec, 200, 10, nil, nil, headers)
	if len(out) != 1 || out[0].Passed {
		t.Fatalf("expected fail, got %+v", out)
	}
}

func TestEvaluate_HeaderNotContains_Pass(t *testing.T) {
	spec := domain.AssertionsSpec{
		Headers: map[string]domain.ValueAssertion{
			"Content-Type": {NotContains: strPtr("xml")},
		},
	}
	headers := map[string][]string{"Content-Type": {"application/json"}}
	out := Evaluate(spec, 200, 10, nil, nil, headers)
	if len(out) != 1 || !out[0].Passed {
		t.Fatalf("expected pass, got %+v", out)
	}
}

func TestEvaluate_HeaderNotContains_Fail(t *testing.T) {
	spec := domain.AssertionsSpec{
		Headers: map[string]domain.ValueAssertion{
			"Content-Type": {NotContains: strPtr("json")},
		},
	}
	headers := map[string][]string{"Content-Type": {"application/json"}}
	out := Evaluate(spec, 200, 10, nil, nil, headers)
	if len(out) != 1 || out[0].Passed {
		t.Fatalf("expected fail, got %+v", out)
	}
}

func TestEvaluate_HeaderMultiValue(t *testing.T) {
	spec := domain.AssertionsSpec{
		Headers: map[string]domain.ValueAssertion{
			"Set-Cookie": {Contains: strPtr("session")},
		},
	}
	headers := map[string][]string{"Set-Cookie": {"session=abc123", "theme=dark"}}
	out := Evaluate(spec, 200, 10, nil, nil, headers)
	if len(out) != 1 || !out[0].Passed {
		t.Fatalf("expected pass (multi-value joined), got %+v", out)
	}
}

func TestEvaluate_HeaderMissing_AllFail(t *testing.T) {
	spec := domain.AssertionsSpec{
		Headers: map[string]domain.ValueAssertion{
			"X-Missing": {
				Exists:      true,
				Eq:          strPtr("val"),
				Contains:    strPtr("v"),
				NotEq:       strPtr("other"),
				NotContains: strPtr("x"),
			},
		},
	}
	headers := map[string][]string{"Content-Type": {"text/html"}}
	out := Evaluate(spec, 200, 10, nil, nil, headers)
	if len(out) != 5 {
		t.Fatalf("expected 5 results for missing header, got %d", len(out))
	}
	for _, r := range out {
		if r.Passed {
			t.Errorf("expected %q to fail for missing header, got pass", r.Name)
		}
	}
}
