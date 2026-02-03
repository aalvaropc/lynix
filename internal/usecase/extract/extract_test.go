package extract

import (
	"testing"

	"github.com/aalvaropc/lynix/internal/domain"
)

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
