package extract

import (
	"testing"

	"github.com/aalvaropc/lynix/internal/domain"
)

func TestApply_EmptyRules(t *testing.T) {
	vars, results := Apply([]byte(`{"name":"alice"}`), domain.ExtractSpec{}, false)
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

	vars, res := Apply(body, rules, false)

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

	vars, res := Apply(body, rules, false)
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
	vars, results := Apply([]byte(`{"active":true}`), domain.ExtractSpec{"active": "$.active"}, false)
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
	vars, results := Apply([]byte(`{"meta":{"key":"val"}}`), domain.ExtractSpec{"meta": "$.meta"}, false)
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

	vars, res := Apply(body, rules, false)

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
	_, results := Apply([]byte(`{"name":"alice"}`), domain.ExtractSpec{"username": ""}, false)
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

	vars, res := Apply(body, rules, false)

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
	_, results := Apply([]byte(`{"name":null}`), domain.ExtractSpec{"username": "$.name"}, false)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Success {
		t.Fatalf("expected Success=false for null value")
	}
}

func TestApply_EmptyString_Succeeds(t *testing.T) {
	vars, results := Apply([]byte(`{"msg":""}`), domain.ExtractSpec{"msg": "$.msg"}, false)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Success {
		t.Fatalf("expected success for empty string, got: %s", results[0].Message)
	}
	if vars["msg"] != "" {
		t.Fatalf("expected empty string, got %q", vars["msg"])
	}
}

func TestApply_EmptyArray_Succeeds(t *testing.T) {
	vars, results := Apply([]byte(`{"items":[]}`), domain.ExtractSpec{"items": "$.items"}, false)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Success {
		t.Fatalf("expected success for empty array, got: %s", results[0].Message)
	}
	if vars["items"] != "[]" {
		t.Fatalf("expected '[]', got %q", vars["items"])
	}
}

func TestApplyHeaders_EmptyValue_Succeeds(t *testing.T) {
	headers := map[string][]string{"X-Empty": {""}}
	vars, results := ApplyHeaders(headers, domain.ExtractHeaderSpec{"empty": "X-Empty"})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Success {
		t.Fatalf("expected success for empty header value, got: %s", results[0].Message)
	}
	if vars["empty"] != "" {
		t.Fatalf("expected empty string, got %q", vars["empty"])
	}
}

func TestApplyHeaders_NotFound_Fails(t *testing.T) {
	headers := map[string][]string{"X-Other": {"val"}}
	_, results := ApplyHeaders(headers, domain.ExtractHeaderSpec{"missing": "X-Gone"})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Success {
		t.Fatalf("expected failure for missing header")
	}
}

func TestApply_MixedResults_StableOrder(t *testing.T) {
	rules := domain.ExtractSpec{
		"aaa": "$.name",
		"bbb": "",
	}
	vars, results := Apply([]byte(`{"name":"alice"}`), rules, false)
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

// --- ApplyHeaders tests ---

func TestApplyHeaders_EmptyRules(t *testing.T) {
	vars, results := ApplyHeaders(map[string][]string{"X-Test": {"val"}}, domain.ExtractHeaderSpec{})
	if len(vars) != 0 || len(results) != 0 {
		t.Fatalf("expected empty, got vars=%v results=%v", vars, results)
	}
}

func TestApplyHeaders_Success(t *testing.T) {
	headers := map[string][]string{
		"X-Request-Id":          {"abc-123"},
		"Content-Type":          {"application/json"},
		"X-Ratelimit-Remaining": {"42"},
	}
	rules := domain.ExtractHeaderSpec{
		"req_id":     "X-Request-Id",
		"rate_limit": "X-Ratelimit-Remaining",
	}

	vars, results := ApplyHeaders(headers, rules)
	if vars["req_id"] != "abc-123" {
		t.Errorf("expected req_id=abc-123, got %q", vars["req_id"])
	}
	if vars["rate_limit"] != "42" {
		t.Errorf("expected rate_limit=42, got %q", vars["rate_limit"])
	}
	for _, r := range results {
		if !r.Success {
			t.Errorf("expected all success, got fail: %+v", r)
		}
	}
}

func TestApplyHeaders_CaseInsensitive(t *testing.T) {
	headers := map[string][]string{
		"Content-Type": {"text/html"},
	}
	rules := domain.ExtractHeaderSpec{
		"ct": "content-type",
	}

	vars, results := ApplyHeaders(headers, rules)
	if vars["ct"] != "text/html" {
		t.Errorf("expected ct=text/html, got %q", vars["ct"])
	}
	if !results[0].Success {
		t.Errorf("expected success, got: %s", results[0].Message)
	}
}

func TestApplyHeaders_MissingHeader(t *testing.T) {
	headers := map[string][]string{
		"Content-Type": {"application/json"},
	}
	rules := domain.ExtractHeaderSpec{
		"token": "X-Auth-Token",
	}

	vars, results := ApplyHeaders(headers, rules)
	if len(vars) != 0 {
		t.Errorf("expected no vars, got %v", vars)
	}
	if results[0].Success {
		t.Error("expected failure for missing header")
	}
}

func TestApplyHeaders_EmptyHeaderName(t *testing.T) {
	headers := map[string][]string{"X-Test": {"val"}}
	rules := domain.ExtractHeaderSpec{"token": ""}

	_, results := ApplyHeaders(headers, rules)
	if results[0].Success {
		t.Error("expected failure for empty header name")
	}
}

func TestApplyHeaders_MultipleValues_TakesFirst(t *testing.T) {
	headers := map[string][]string{
		"Set-Cookie": {"session=abc; Path=/", "lang=en; Path=/"},
	}
	rules := domain.ExtractHeaderSpec{
		"cookie": "Set-Cookie",
	}

	vars, results := ApplyHeaders(headers, rules)
	if vars["cookie"] != "session=abc; Path=/" {
		t.Errorf("expected first cookie value, got %q", vars["cookie"])
	}
	if !results[0].Success {
		t.Errorf("expected success: %s", results[0].Message)
	}
}

func TestApplyHeaders_StableOrder(t *testing.T) {
	headers := map[string][]string{
		"X-A": {"1"},
		"X-B": {"2"},
	}
	rules := domain.ExtractHeaderSpec{
		"bbb": "X-B",
		"aaa": "X-A",
	}

	_, results := ApplyHeaders(headers, rules)
	if results[0].Name != "aaa" {
		t.Errorf("expected sorted order, got %q first", results[0].Name)
	}
	if results[1].Name != "bbb" {
		t.Errorf("expected sorted order, got %q second", results[1].Name)
	}
}

func TestApply_SingleElementArrayUnwrapped(t *testing.T) {
	// jsonpath returns a slice for index access; single-element arrays are unwrapped.
	vars, results := Apply([]byte(`{"items":["single"]}`), domain.ExtractSpec{"item": "$.items[0]"}, false)
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
