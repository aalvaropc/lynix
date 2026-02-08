package domain

import "testing"

func TestGetSetVars(t *testing.T) {
	vars := Vars{}

	vars = Set(vars, "token", "abc123")
	got, ok := Get(vars, "token")
	if !ok {
		t.Fatalf("expected key to exist")
	}
	if got != "abc123" {
		t.Fatalf("expected value %q, got %q", "abc123", got)
	}

	if _, ok := Get(vars, "missing"); ok {
		t.Fatalf("expected missing key to be absent")
	}
}

func TestMergeVars(t *testing.T) {
	base := Vars{
		"region": "us-east-1",
		"token":  "base",
	}
	override := Vars{
		"token": "override",
		"user":  "alice",
	}

	merged := Merge(base, override)

	if merged["region"] != "us-east-1" {
		t.Fatalf("expected base value to remain")
	}
	if merged["token"] != "override" {
		t.Fatalf("expected override value to win")
	}
	if merged["user"] != "alice" {
		t.Fatalf("expected new override key to be present")
	}

	if base["token"] != "base" {
		t.Fatalf("expected base to remain unchanged")
	}
}
