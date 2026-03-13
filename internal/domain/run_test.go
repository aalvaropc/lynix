package domain

import (
	"context"
	"net"
	"net/url"
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
		// ETIMEDOUT might be classified as connection; both are acceptable for MVP.
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
