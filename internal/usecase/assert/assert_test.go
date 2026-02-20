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
	results := Evaluate(domain.AssertionsSpec{}, 200, 50, []byte(`{}`))
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestEvaluate_OnlyStatus(t *testing.T) {
	s := 200
	spec := domain.AssertionsSpec{Status: &s}
	results := Evaluate(spec, 200, 50, nil)
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
	results := Evaluate(spec, 200, 500, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Passed {
		t.Fatalf("expected max_ms assertion to pass")
	}
}

func TestEvaluate_JSONPathExists_True(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.JSONPathAssertion{
			"$.data.id": {Exists: true},
		},
	}

	body := []byte(`{"data":{"id":123}}`)
	out := Evaluate(spec, 200, 10, body)

	if len(out) != 1 {
		t.Fatalf("expected 1 result, got=%d", len(out))
	}
	if !out[0].Passed {
		t.Fatalf("expected pass, got fail: %s", out[0].Message)
	}
}

func TestEvaluate_JSONPathExists_False(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.JSONPathAssertion{
			"$.data.missing": {Exists: true},
		},
	}

	body := []byte(`{"data":{"id":123}}`)
	out := Evaluate(spec, 200, 10, body)

	if len(out) != 1 {
		t.Fatalf("expected 1 result, got=%d", len(out))
	}
	if out[0].Passed {
		t.Fatalf("expected fail")
	}
}

func TestEvaluate_JSONPath_NonJSONBody(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.JSONPathAssertion{
			"$.data.id": {Exists: true},
		},
	}

	out := Evaluate(spec, 200, 10, []byte("hello"))

	if len(out) != 1 {
		t.Fatalf("expected 1 result, got=%d", len(out))
	}
	if out[0].Passed {
		t.Fatalf("expected fail")
	}
}

func TestEvaluate_JSONPath_InvalidExpr(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.JSONPathAssertion{
			"$.data[": {Exists: true},
		},
	}

	body := []byte(`{"data":{"id":123}}`)
	out := Evaluate(spec, 200, 10, body)

	if len(out) != 1 {
		t.Fatalf("expected 1 result, got=%d", len(out))
	}
	if out[0].Passed {
		t.Fatalf("expected fail")
	}
}

func TestEvaluate_InvalidBodyMarksAllJSONPathFailed(t *testing.T) {
	spec := domain.AssertionsSpec{
		JSONPath: map[string]domain.JSONPathAssertion{
			"$.name": {Exists: true},
			"$.age":  {Exists: true},
		},
	}
	out := Evaluate(spec, 200, 50, []byte("not json"))
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
		JSONPath: map[string]domain.JSONPathAssertion{
			"$.id": {Exists: true},
		},
	}
	results := Evaluate(spec, 200, 100, []byte(`{"id":42}`))
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
		JSONPath: map[string]domain.JSONPathAssertion{
			"$.name": {Exists: false},
		},
	}
	results := Evaluate(spec, 200, 50, []byte(`{"name":"alice"}`))
	if len(results) != 0 {
		t.Fatalf("expected 0 results for Exists=false, got %d", len(results))
	}
}
