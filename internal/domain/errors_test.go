package domain

import (
	"errors"
	"testing"
)

func TestDomainErrorWrapUnwrap(t *testing.T) {
	root := errors.New("root")
	err := &DomainError{
		Kind:  KindInvalidRequest,
		Msg:   "bad request spec",
		Cause: root,
	}

	if !errors.Is(err, root) {
		t.Fatalf("expected errors.Is to match cause")
	}

	var got *DomainError
	if !errors.As(err, &got) {
		t.Fatalf("expected errors.As to match DomainError")
	}
	if got.Kind != KindInvalidRequest {
		t.Fatalf("expected kind %s", KindInvalidRequest)
	}
}

func TestIsKindForDomainError(t *testing.T) {
	err := &DomainError{
		Kind: KindInvalidConfig,
		Msg:  "invalid",
	}

	if !IsKind(err, KindInvalidConfig) {
		t.Fatalf("expected IsKind to match domain error")
	}
}
