package domain

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/url"
	"strings"
	"syscall"
	"testing"
)

func TestClassifyRunError_Timeout_ContextDeadline(t *testing.T) {
	if got := ClassifyRunError(context.DeadlineExceeded); got != RunErrorTimeout {
		t.Fatalf("expected timeout, got=%s", got)
	}
}

func TestClassifyRunError_Canceled_ContextCanceled(t *testing.T) {
	if got := ClassifyRunError(context.Canceled); got != RunErrorCanceled {
		t.Fatalf("expected canceled, got=%s", got)
	}
}

func TestClassifyRunError_Timeout_NetError(t *testing.T) {
	// net.OpError wrapping ETIMEDOUT
	err := &net.OpError{Op: "read", Net: "tcp", Err: syscall.ETIMEDOUT}
	if got := ClassifyRunError(err); got != RunErrorConn && got != RunErrorTimeout {
		// ETIMEDOUT might be classified as connection; both are acceptable.
		t.Fatalf("expected conn/timeout, got=%s", got)
	}
}

func TestClassifyRunError_DNS(t *testing.T) {
	err := &net.DNSError{Err: "no such host", Name: "example.invalid"}
	if got := ClassifyRunError(err); got != RunErrorDNS {
		t.Fatalf("expected dns, got=%s", got)
	}
}

func TestClassifyRunError_ConnReset(t *testing.T) {
	err := &net.OpError{Op: "read", Net: "tcp", Err: syscall.ECONNRESET}
	if got := ClassifyRunError(err); got != RunErrorConn {
		t.Fatalf("expected conn, got=%s", got)
	}
}

func TestClassifyRunError_URLWraps(t *testing.T) {
	inner := &net.DNSError{Err: "no such host", Name: "x.invalid"}
	err := &url.Error{Op: "Get", URL: "http://x.invalid", Err: inner}

	if got := ClassifyRunError(err); got != RunErrorDNS {
		t.Fatalf("expected dns, got=%s", got)
	}
}

// --- IsRetryable ---

func TestIsRetryable_TransientKinds(t *testing.T) {
	for _, kind := range []RunErrorKind{RunErrorTimeout, RunErrorDNS, RunErrorConn} {
		if !IsRetryable(kind) {
			t.Errorf("expected IsRetryable(%s)=true", kind)
		}
	}
}

func TestIsRetryable_NonTransientKinds(t *testing.T) {
	for _, kind := range []RunErrorKind{RunErrorCanceled, RunErrorHTTP, RunErrorUnknown} {
		if IsRetryable(kind) {
			t.Errorf("expected IsRetryable(%s)=false", kind)
		}
	}
}

// --- RequestResult.Failed ---

func TestRequestResult_Failed_ErrorSet(t *testing.T) {
	r := RequestResult{Error: &RunError{Kind: RunErrorConn, Message: "refused"}}
	if !r.Failed() {
		t.Error("expected Failed()=true when Error is set")
	}
}

func TestRequestResult_Failed_AssertionFail(t *testing.T) {
	r := RequestResult{
		Assertions: []AssertionResult{{Passed: false}},
	}
	if !r.Failed() {
		t.Error("expected Failed()=true when assertion fails")
	}
}

func TestRequestResult_Failed_ExtractFail(t *testing.T) {
	r := RequestResult{
		Extracts: []ExtractResult{{Success: false}},
	}
	if !r.Failed() {
		t.Error("expected Failed()=true when extract fails")
	}
}

func TestRequestResult_Failed_AllPass(t *testing.T) {
	r := RequestResult{
		Assertions: []AssertionResult{{Passed: true}},
		Extracts:   []ExtractResult{{Success: true}},
	}
	if r.Failed() {
		t.Error("expected Failed()=false when all pass")
	}
}

func TestBodyBytes_TextRoundtrip(t *testing.T) {
	in := BodyBytes(`{"user":"alice"}`)
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != `"{\"user\":\"alice\"}"` {
		t.Fatalf("expected plain-text serialization, got %s", b)
	}
	var out BodyBytes
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(out) != string(in) {
		t.Fatalf("roundtrip mismatch: %q != %q", out, in)
	}
}

func TestBodyBytes_BinaryRoundtrip(t *testing.T) {
	in := BodyBytes{0xff, 0xfe, 0x00, 0x01}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), "base64:") {
		t.Fatalf("expected base64 serialization for binary, got %s", b)
	}
	var out BodyBytes
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !bytes.Equal(out, in) {
		t.Fatalf("roundtrip mismatch: %v != %v", out, in)
	}
}

func TestRunResult_SnakeCaseArtifact(t *testing.T) {
	run := RunResult{
		SchemaVersion:  ArtifactSchemaVersion,
		CollectionName: "col",
		Results: []RequestResult{
			{Name: "r", Method: MethodGet, URL: "http://x", StatusCode: 200, LatencyMS: 5},
		},
	}
	b, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(b)
	for _, key := range []string{`"schema_version"`, `"collection_name"`, `"status_code"`, `"latency_ms"`, `"started_at"`} {
		if !strings.Contains(s, key) {
			t.Errorf("expected key %s in artifact JSON, got: %s", key, s)
		}
	}
	for _, bad := range []string{`"CollectionName"`, `"StatusCode"`, `"LatencyMS"`} {
		if strings.Contains(s, bad) {
			t.Errorf("unexpected PascalCase key %s in artifact JSON", bad)
		}
	}
}

func TestBodySpec_Serialize_FormURLEncoded(t *testing.T) {
	b := BodySpec{Type: BodyForm, Form: map[string]string{"b key": "v&1", "a": "2"}}
	got := string(b.Serialize())
	if got != "a=2&b+key=v%261" {
		t.Fatalf("expected sorted URL-encoded form, got %q", got)
	}
}
