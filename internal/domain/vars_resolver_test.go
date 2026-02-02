package domain

import (
	"errors"
	"testing"
	"time"
)

func TestResolveString_MissingVar(t *testing.T) {
	r := NewVarResolver(
		WithNow(func() time.Time { return time.Unix(100, 0) }),
		WithUUID(func() (string, error) { return "00000000-0000-0000-0000-000000000000", nil }),
	)

	rt, err := r.NewRuntime(Vars{"base_url": "http://x"})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}

	_, err = rt.ResolveString("{{token}}")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !IsKind(err, KindMissingVar) {
		t.Fatalf("expected KindMissingVar, got: %v", err)
	}
	if !contains(err.Error(), "missing variable: token") {
		t.Fatalf("expected message to contain 'missing variable: token', got: %v", err)
	}
}

func TestResolveString_Builtins(t *testing.T) {
	r := NewVarResolver(
		WithNow(func() time.Time { return time.Unix(1700000000, 0) }),
		WithUUID(func() (string, error) { return "11111111-1111-1111-1111-111111111111", nil }),
	)

	rt, err := r.NewRuntime(Vars{})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}

	got, err := rt.ResolveString("ts={{$timestamp}} uuid={{$uuid}}")
	if err != nil {
		t.Fatalf("ResolveString: %v", err)
	}
	want := "ts=1700000000 uuid=11111111-1111-1111-1111-111111111111"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveJSON_Nested(t *testing.T) {
	r := NewVarResolver(
		WithNow(func() time.Time { return time.Unix(170, 0) }),
		WithUUID(func() (string, error) { return "22222222-2222-2222-2222-222222222222", nil }),
	)
	rt, err := r.NewRuntime(Vars{"base_url": "http://example"})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}

	in := map[string]any{
		"a": "{{base_url}}",
		"b": []any{
			map[string]any{"c": "{{$uuid}}"},
			"nope",
			123,
		},
	}

	out, err := rt.ResolveJSONValue(in)
	if err != nil {
		t.Fatalf("ResolveJSONValue: %v", err)
	}

	m := out.(map[string]any)
	if m["a"].(string) != "http://example" {
		t.Fatalf("expected a=http://example, got=%v", m["a"])
	}

	arr := m["b"].([]any)
	obj := arr[0].(map[string]any)
	if obj["c"].(string) != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("expected uuid, got=%v", obj["c"])
	}
}

func TestResolveString_UnclosedPlaceholder(t *testing.T) {
	r := NewVarResolver()
	rt, err := r.NewRuntime(Vars{"x": "y"})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}

	_, err = rt.ResolveString("{{x")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !IsKind(err, KindInvalidConfig) {
		t.Fatalf("expected KindInvalidConfig, got: %v", err)
	}
	var oe *OpError
	if !errors.As(err, &oe) {
		t.Fatalf("expected OpError")
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (stringIndex(s, sub) >= 0))
}

func stringIndex(s, sub string) int {
	// Avoid importing strings in this test file unnecessarily; keep it minimal.
	// (strings.Index would also be fine.)
outer:
	for i := 0; i+len(sub) <= len(s); i++ {
		for j := 0; j < len(sub); j++ {
			if s[i+j] != sub[j] {
				continue outer
			}
		}
		return i
	}
	return -1
}
