package tui

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aalvaropc/lynix/internal/domain"
)

func TestUserMessage_Nil(t *testing.T) {
	if got := userMessage(nil); got != "" {
		t.Fatalf("expected empty string for nil, got %q", got)
	}
}

func TestUserMessage_ContextCanceled(t *testing.T) {
	got := userMessage(context.Canceled)
	if got != "Run cancelled" {
		t.Fatalf("expected 'Run cancelled', got %q", got)
	}
}

func TestUserMessage_ContextDeadlineExceeded(t *testing.T) {
	got := userMessage(context.DeadlineExceeded)
	if got != "Run timed out" {
		t.Fatalf("expected 'Run timed out', got %q", got)
	}
}

func TestUserMessage_OpError_NotFound_Collection(t *testing.T) {
	err := &domain.OpError{
		Op:   "yamlcollection.load",
		Kind: domain.KindNotFound,
		Path: "/tmp/demo.yaml",
		Err:  fmt.Errorf("%w: file not found", domain.ErrNotFound),
	}
	got := userMessage(err)
	if got != "Collection not found" {
		t.Fatalf("expected 'Collection not found', got %q", got)
	}
}

func TestUserMessage_OpError_NotFound_Env(t *testing.T) {
	err := &domain.OpError{
		Op:   "yamlenv.load",
		Kind: domain.KindNotFound,
		Path: "/tmp/dev.yaml",
		Err:  fmt.Errorf("%w: file not found", domain.ErrNotFound),
	}
	got := userMessage(err)
	if got != "Environment not found" {
		t.Fatalf("expected 'Environment not found', got %q", got)
	}
}

func TestUserMessage_OpError_NotFound_Workspace(t *testing.T) {
	err := &domain.OpError{
		Op:   "workspacefinder.findroot",
		Kind: domain.KindNotFound,
		Err:  domain.ErrNotFound,
	}
	got := userMessage(err)
	if got != "Workspace not found" {
		t.Fatalf("expected 'Workspace not found', got %q", got)
	}
}

func TestUserMessage_OpError_NotFound_Generic(t *testing.T) {
	err := &domain.OpError{
		Op:   "other.op",
		Kind: domain.KindNotFound,
		Err:  errors.New("not found"),
	}
	got := userMessage(err)
	if got != "Not found" {
		t.Fatalf("expected 'Not found', got %q", got)
	}
}

func TestUserMessage_OpError_MissingVar(t *testing.T) {
	err := &domain.OpError{
		Op:   "resolver.resolve",
		Kind: domain.KindMissingVar,
		Err:  errors.New("missing variable: api_key"),
	}
	got := userMessage(err)
	if got != "Missing variable api_key" {
		t.Fatalf("expected 'Missing variable api_key', got %q", got)
	}
}

func TestUserMessage_OpError_MissingVar_NoName(t *testing.T) {
	err := &domain.OpError{
		Op:   "resolver.resolve",
		Kind: domain.KindMissingVar,
		Err:  errors.New("some error"),
	}
	got := userMessage(err)
	if got != "Missing variable" {
		t.Fatalf("expected 'Missing variable', got %q", got)
	}
}

func TestUserMessage_OpError_InvalidConfig_WithPath(t *testing.T) {
	err := &domain.OpError{
		Op:   "yamlcollection.load",
		Kind: domain.KindInvalidConfig,
		Path: "/tmp/demo.yaml",
		Err:  errors.New("yaml: line 5: did not find expected key"),
	}
	got := userMessage(err)
	if got != "Invalid YAML at demo.yaml line 5" {
		t.Fatalf("expected 'Invalid YAML at demo.yaml line 5', got %q", got)
	}
}

func TestUserMessage_OpError_InvalidConfig_NoLine(t *testing.T) {
	err := &domain.OpError{
		Op:   "yamlcollection.load",
		Kind: domain.KindInvalidConfig,
		Path: "/tmp/demo.yaml",
		Err:  errors.New("yaml: cannot unmarshal string into Go value"),
	}
	got := userMessage(err)
	if got != "Invalid YAML at demo.yaml" {
		t.Fatalf("expected 'Invalid YAML at demo.yaml', got %q", got)
	}
}

func TestUserMessage_OpError_InvalidConfig_NoPath(t *testing.T) {
	err := &domain.OpError{
		Op:   "yamlcollection.validate",
		Kind: domain.KindInvalidConfig,
		Err:  errors.New("field name: required"),
	}
	got := userMessage(err)
	if got != "Invalid config" {
		t.Fatalf("expected 'Invalid config', got %q", got)
	}
}

func TestUserMessage_OpError_UnknownKind(t *testing.T) {
	err := &domain.OpError{
		Op:   "something",
		Kind: domain.KindExecution,
		Err:  errors.New("boom"),
	}
	got := userMessage(err)
	if got != "Unexpected error (see logs)" {
		t.Fatalf("expected 'Unexpected error (see logs)', got %q", got)
	}
}

func TestUserMessage_PlainError_YAMLLike(t *testing.T) {
	err := errors.New("yaml: line 3: did not find expected key")
	got := userMessage(err)
	if got != "Invalid YAML line 3" {
		t.Fatalf("expected 'Invalid YAML line 3', got %q", got)
	}
}

func TestUserMessage_PlainError_MissingVariable(t *testing.T) {
	err := errors.New("missing variable: token")
	got := userMessage(err)
	if got != "Missing variable token" {
		t.Fatalf("expected 'Missing variable token', got %q", got)
	}
}

func TestUserMessage_PlainError_Generic(t *testing.T) {
	err := errors.New("something went wrong")
	got := userMessage(err)
	if got != "Unexpected error (see logs)" {
		t.Fatalf("expected 'Unexpected error (see logs)', got %q", got)
	}
}

func TestExtractMissingVarName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"missing variable: api_key", "api_key"},
		{"missing variable api_key", "api_key"},
		{"missing variable: ", ""},
		{"no variable here", ""},
	}
	for _, tc := range tests {
		got := extractMissingVarName(tc.input)
		if got != tc.want {
			t.Errorf("extractMissingVarName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestExtractLine(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"yaml: line 5: did not find expected", "5"},
		{"error at Line 42: bad value", "42"},
		{"no line number", ""},
	}
	for _, tc := range tests {
		got := extractLine(tc.input)
		if got != tc.want {
			t.Errorf("extractLine(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
