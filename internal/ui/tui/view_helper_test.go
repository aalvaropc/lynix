package tui

import (
	"strings"
	"testing"

	"github.com/aalvaropc/lynix/internal/domain"
)

func TestClampString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"empty", "", 5, ""},
		{"shorter", "abc", 5, "abc"},
		{"exact", "abcde", 5, "abcde"},
		{"longer", "abcdef", 5, "abcde…"},
		{"zero max", "abc", 0, ""},
		{"negative max", "abc", -1, ""},
		{"unicode", "héllo wörld", 5, "héllo…"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := clampString(tc.input, tc.maxLen)
			if got != tc.want {
				t.Errorf("clampString(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.want)
			}
		})
	}
}

func TestPrettyBody(t *testing.T) {
	tests := []struct {
		name string
		body []byte
		want string
	}{
		{"empty", nil, "(empty)"},
		{"empty slice", []byte{}, "(empty)"},
		{"valid json", []byte(`{"a":1}`), "{\n  \"a\": 1\n}"},
		{"plain text", []byte("hello world"), "hello world"},
		{"invalid json", []byte(`{bad`), "{bad"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := prettyBody(tc.body)
			if got != tc.want {
				t.Errorf("prettyBody(%q) = %q, want %q", tc.body, got, tc.want)
			}
		})
	}
}

func TestRenderResultDetails(t *testing.T) {
	rr := domain.RequestResult{
		StatusCode: 200,
		LatencyMS:  42,
		Assertions: []domain.AssertionResult{
			{Name: "status", Passed: true, Message: "status 200"},
			{Name: "max_ms", Passed: false, Message: "too slow"},
		},
		Extracts: []domain.ExtractResult{
			{Name: "token", Success: true, Message: "extracted"},
		},
		Extracted: domain.Vars{"token": "abc123"},
	}

	out := renderResultDetails(rr)

	if !strings.Contains(out, "Status: 200") {
		t.Error("expected Status: 200 in output")
	}
	if !strings.Contains(out, "Latency: 42ms") {
		t.Error("expected Latency: 42ms in output")
	}
	if !strings.Contains(out, "[PASS]") {
		t.Error("expected [PASS] in assertions")
	}
	if !strings.Contains(out, "[FAIL]") {
		t.Error("expected [FAIL] in assertions")
	}
	if !strings.Contains(out, "token = abc123") {
		t.Error("expected extracted var in output")
	}
}

func TestRenderResultDetails_WithError(t *testing.T) {
	rr := domain.RequestResult{
		Error: &domain.RunError{
			Kind:    domain.RunErrorConn,
			Message: "connection refused",
		},
	}

	out := renderResultDetails(rr)

	if !strings.Contains(out, "Error:") {
		t.Error("expected Error section")
	}
	if !strings.Contains(out, "connection refused") {
		t.Error("expected error message")
	}
}

func TestRenderResultResponse(t *testing.T) {
	rr := domain.RequestResult{
		Response: domain.ResponseSnapshot{
			Headers: map[string][]string{
				"Content-Type": {"application/json"},
			},
			Body:      []byte(`{"ok":true}`),
			Truncated: false,
		},
	}

	out := renderResultResponse(rr)

	if !strings.Contains(out, "Content-Type: application/json") {
		t.Error("expected header in output")
	}
	if !strings.Contains(out, `"ok": true`) {
		t.Error("expected formatted JSON body")
	}
}

func TestRenderResultResponse_Truncated(t *testing.T) {
	rr := domain.RequestResult{
		Response: domain.ResponseSnapshot{
			Headers:   map[string][]string{},
			Body:      []byte("some body"),
			Truncated: true,
		},
	}

	out := renderResultResponse(rr)

	if !strings.Contains(out, "(truncated)") {
		t.Error("expected truncated marker")
	}
}

func TestRenderResultResponse_NoHeaders(t *testing.T) {
	rr := domain.RequestResult{
		Response: domain.ResponseSnapshot{
			Headers: map[string][]string{},
		},
	}

	out := renderResultResponse(rr)

	if !strings.Contains(out, "(none)") {
		t.Error("expected (none) for empty headers")
	}
}
