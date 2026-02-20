package domain

import (
	"errors"
	"testing"
	"time"
)

// --- helpers ---

func testRuntime(t *testing.T, vars Vars, now func() time.Time, uuidFn func() (string, error)) *RuntimeResolver {
	t.Helper()
	if now == nil {
		now = func() time.Time { return time.Unix(1700000000, 0) }
	}
	if uuidFn == nil {
		uuidFn = func() (string, error) { return "00000000-0000-0000-0000-000000000000", nil }
	}
	vr := NewVarResolver(WithNow(now), WithUUID(uuidFn))
	rt, err := vr.NewRuntime(vars)
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	return rt
}

// --- ResolveString ---

func TestResolveString_NoPlaceholders(t *testing.T) {
	rt := testRuntime(t, Vars{}, nil, nil)
	got, err := rt.ResolveString("hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello world" {
		t.Fatalf("expected %q, got %q", "hello world", got)
	}
}

func TestResolveString_SimpleVar(t *testing.T) {
	rt := testRuntime(t, Vars{"base_url": "https://api.example.com"}, nil, nil)
	got, err := rt.ResolveString("{{base_url}}/users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://api.example.com/users"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

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

func TestResolveString_MultipleVars(t *testing.T) {
	rt := testRuntime(t, Vars{"host": "api.example.com", "version": "v2"}, nil, nil)
	got, err := rt.ResolveString("https://{{host}}/{{version}}/items")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://api.example.com/v2/items"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
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

func TestResolveString_EmptyPlaceholder(t *testing.T) {
	rt := testRuntime(t, Vars{}, nil, nil)
	_, err := rt.ResolveString("{{  }}")
	if err == nil {
		t.Fatalf("expected error for empty placeholder")
	}
	if !IsKind(err, KindInvalidConfig) {
		t.Fatalf("expected KindInvalidConfig, got %v", err)
	}
}

// --- ResolveHeaders ---

func TestResolveHeaders_Success(t *testing.T) {
	rt := testRuntime(t, Vars{"token": "abc123"}, nil, nil)
	h := Headers{"Authorization": "Bearer {{token}}", "Content-Type": "application/json"}
	got, err := rt.ResolveHeaders(h)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["Authorization"] != "Bearer abc123" {
		t.Fatalf("expected Authorization %q, got %q", "Bearer abc123", got["Authorization"])
	}
	if got["Content-Type"] != "application/json" {
		t.Fatalf("expected Content-Type unchanged")
	}
}

func TestResolveHeaders_PropagatesError(t *testing.T) {
	rt := testRuntime(t, Vars{}, nil, nil)
	_, err := rt.ResolveHeaders(Headers{"X-Token": "{{missing_var}}"})
	if err == nil {
		t.Fatal("expected error for missing variable in header")
	}
	if !IsKind(err, KindMissingVar) {
		t.Fatalf("expected KindMissingVar, got %v", err)
	}
}

// --- ResolveBodySpec ---

func TestResolveBodySpec_BodyJSON(t *testing.T) {
	rt := testRuntime(t, Vars{"name": "alice"}, nil, nil)
	b := BodySpec{
		Type: BodyJSON,
		JSON: map[string]any{"user": "{{name}}", "count": 42.0},
	}
	got, err := rt.ResolveBodySpec(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.JSON["user"] != "alice" {
		t.Fatalf("expected user=alice, got %v", got.JSON["user"])
	}
	if got.JSON["count"] != 42.0 {
		t.Fatalf("expected count=42.0, got %v", got.JSON["count"])
	}
}

func TestResolveBodySpec_BodyForm(t *testing.T) {
	rt := testRuntime(t, Vars{"key": "mykey"}, nil, nil)
	b := BodySpec{
		Type: BodyForm,
		Form: map[string]string{"apikey": "{{key}}", "static": "value"},
	}
	got, err := rt.ResolveBodySpec(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Form["apikey"] != "mykey" {
		t.Fatalf("expected apikey=mykey, got %q", got.Form["apikey"])
	}
	if got.Form["static"] != "value" {
		t.Fatalf("expected static=value unchanged")
	}
}

func TestResolveBodySpec_BodyRaw(t *testing.T) {
	rt := testRuntime(t, Vars{"data": "hello"}, nil, nil)
	b := BodySpec{Type: BodyRaw, Raw: "raw: {{data}}"}
	got, err := rt.ResolveBodySpec(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Raw != "raw: hello" {
		t.Fatalf("expected %q, got %q", "raw: hello", got.Raw)
	}
}

func TestResolveBodySpec_BodyNone(t *testing.T) {
	rt := testRuntime(t, Vars{}, nil, nil)
	b := BodySpec{Type: BodyNone}
	got, err := rt.ResolveBodySpec(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Type != BodyNone {
		t.Fatalf("expected BodyNone passthrough")
	}
}

// --- ResolveRequest ---

func TestResolveRequest_Integration(t *testing.T) {
	rt := testRuntime(t, Vars{"base": "https://api.example.com", "token": "tok"}, nil, nil)
	req := RequestSpec{
		Name:    "get-users",
		Method:  MethodGet,
		URL:     "{{base}}/users",
		Headers: Headers{"Authorization": "Bearer {{token}}"},
		Body:    BodySpec{Type: BodyNone},
	}
	got, err := rt.ResolveRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.URL != "https://api.example.com/users" {
		t.Fatalf("expected URL %q, got %q", "https://api.example.com/users", got.URL)
	}
	if got.Headers["Authorization"] != "Bearer tok" {
		t.Fatalf("expected Authorization header resolved, got %q", got.Headers["Authorization"])
	}
}

// --- ResolveJSONValue ---

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

func TestResolveJSONValue_String(t *testing.T) {
	rt := testRuntime(t, Vars{"k": "v"}, nil, nil)
	val, err := rt.ResolveJSONValue("{{k}}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "v" {
		t.Fatalf("expected v, got %v", val)
	}
}

func TestResolveJSONValue_NumberPassthrough(t *testing.T) {
	rt := testRuntime(t, Vars{}, nil, nil)
	val, err := rt.ResolveJSONValue(42.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 42.0 {
		t.Fatalf("expected 42.0, got %v", val)
	}
}

// --- WithNow / WithUUID options ---

func TestWithNow(t *testing.T) {
	fixed := time.Unix(999, 0)
	vr := NewVarResolver(
		WithNow(func() time.Time { return fixed }),
		WithUUID(func() (string, error) { return "x", nil }),
	)
	rt, err := vr.NewRuntime(Vars{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := rt.ResolveString("{{$timestamp}}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "999" {
		t.Fatalf("expected 999, got %q", got)
	}
}

func TestWithUUID_Error(t *testing.T) {
	uuidErr := errors.New("uuid failed")
	vr := NewVarResolver(
		WithNow(func() time.Time { return time.Unix(0, 0) }),
		WithUUID(func() (string, error) { return "", uuidErr }),
	)
	_, err := vr.NewRuntime(Vars{})
	if err == nil {
		t.Fatal("expected error from uuid generator")
	}
	if !IsKind(err, KindExecution) {
		t.Fatalf("expected KindExecution, got %v", err)
	}
}

// --- internal helpers ---

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
