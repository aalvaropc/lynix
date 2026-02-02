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
