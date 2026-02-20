package extract

import (
	"testing"

	"github.com/aalvaropc/lynix/internal/domain"
)

func TestApply_EmptyRules(t *testing.T) {
	vars, results := Apply([]byte(`{"name":"alice"}`), domain.ExtractSpec{})
	if len(vars) != 0 {
		t.Fatalf("expected empty vars, got %v", vars)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty results, got %v", results)
	}
}

func TestApply_Success(t *testing.T) {
	body := []byte(`{"token":"abc123","user":{"id":7}}`)
	rules := domain.ExtractSpec{
		"auth.token": "$.token",
		"user.id":    "$.user.id",
	}

	vars, res := Apply(body, rules)

	if vars["auth.token"] != "abc123" {
		t.Fatalf("expected token=abc123, got=%q", vars["auth.token"])
	}
	if vars["user.id"] != "7" {
		t.Fatalf("expected user.id=7, got=%q", vars["user.id"])
	}

	if len(res) != 2 {
		t.Fatalf("expected 2 results, got=%d", len(res))
	}
	for _, r := range res {
		if !r.Success {
			t.Fatalf("expected all success, got fail: %+v", r)
		}
	}
}

func TestApply_NonJSONBody_FailsAll(t *testing.T) {
	body := []byte("hello")
	rules := domain.ExtractSpec{
		"auth.token": "$.token",
	}

	vars, res := Apply(body, rules)
	if len(vars) != 0 {
		t.Fatalf("expected no vars, got=%v", vars)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got=%d", len(res))
	}
	if res[0].Success {
		t.Fatalf("expected failure")
	}
}

func TestApply_ExtractBool(t *testing.T) {
	vars, results := Apply([]byte(`{"active":true}`), domain.ExtractSpec{"active": "$.active"})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Success {
		t.Fatalf("expected Success=true, got: %s", results[0].Message)
	}
	if vars["active"] != "true" {
		t.Fatalf("expected active=true, got %q", vars["active"])
	}
}

func TestApply_ExtractObject(t *testing.T) {
	vars, results := Apply([]byte(`{"meta":{"key":"val"}}`), domain.ExtractSpec{"meta": "$.meta"})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Success {
		t.Fatalf("expected Success=true, got: %s", results[0].Message)
	}
	if vars["meta"] == "" {
		t.Fatalf("expected non-empty meta value")
	}
}

func TestApply_InvalidJSONPath_FailsRule(t *testing.T) {
	body := []byte(`{"token":"abc"}`)
	rules := domain.ExtractSpec{
		"auth.token": "$.token[",
	}

	vars, res := Apply(body, rules)

	if len(vars) != 0 {
		t.Fatalf("expected no vars, got=%v", vars)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got=%d", len(res))
	}
	if res[0].Success {
		t.Fatalf("expected failure")
	}
}

func TestApply_EmptyExpression(t *testing.T) {
	_, results := Apply([]byte(`{"name":"alice"}`), domain.ExtractSpec{"username": ""})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Success {
		t.Fatalf("expected Success=false for empty expression")
	}
}

func TestApply_MissingValue_FailsRule(t *testing.T) {
	body := []byte(`{"x":1}`)
	rules := domain.ExtractSpec{
		"auth.token": "$.token",
	}

	vars, res := Apply(body, rules)

	if len(vars) != 0 {
		t.Fatalf("expected no vars, got=%v", vars)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got=%d", len(res))
	}
	if res[0].Success {
		t.Fatalf("expected failure")
	}
}

func TestApply_NullValue_FailsRule(t *testing.T) {
	// null is considered empty by isEmptyValue, triggering "no value found".
	_, results := Apply([]byte(`{"name":null}`), domain.ExtractSpec{"username": "$.name"})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Success {
		t.Fatalf("expected Success=false for null value")
	}
}

func TestApply_MixedResults_StableOrder(t *testing.T) {
	rules := domain.ExtractSpec{
		"aaa": "$.name",
		"bbb": "",
	}
	vars, results := Apply([]byte(`{"name":"alice"}`), rules)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Results must be in sorted key order: aaa before bbb.
	if results[0].Name != "aaa" {
		t.Fatalf("expected results[0].Name=aaa (sorted), got %q", results[0].Name)
	}
	if results[1].Name != "bbb" {
		t.Fatalf("expected results[1].Name=bbb (sorted), got %q", results[1].Name)
	}
	if !results[0].Success {
		t.Fatalf("expected aaa to succeed")
	}
	if results[1].Success {
		t.Fatalf("expected bbb to fail (empty expression)")
	}
	if vars["aaa"] != "alice" {
		t.Fatalf("expected aaa=alice, got %q", vars["aaa"])
	}
}

func TestApply_SingleElementArrayUnwrapped(t *testing.T) {
	// jsonpath returns a slice for index access; single-element arrays are unwrapped.
	vars, results := Apply([]byte(`{"items":["single"]}`), domain.ExtractSpec{"item": "$.items[0]"})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Success {
		t.Fatalf("expected Success=true, got: %s", results[0].Message)
	}
	if vars["item"] != "single" {
		t.Fatalf("expected item=single, got %q", vars["item"])
	}
}
