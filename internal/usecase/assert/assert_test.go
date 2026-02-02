package assert

import (
	"testing"

	"github.com/aalvaropc/lynix/internal/domain"
)

func TestAssertStatus_FailMessage(t *testing.T) {
	r := AssertStatus(200, 500)
	if r.Passed {
		t.Fatalf("expected fail")
	}
	if r.Message != "expected status 200, got 500" {
		t.Fatalf("unexpected message: %q", r.Message)
	}
}

func TestAssertMaxLatency_FailMessage(t *testing.T) {
	r := AssertMaxLatency(100, 250)
	if r.Passed {
		t.Fatalf("expected fail")
	}
	if r.Message != "expected latency <= 100ms, got 250ms" {
		t.Fatalf("unexpected message: %q", r.Message)
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
