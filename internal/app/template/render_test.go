package template

import "testing"

func TestRenderStringSingleVar(t *testing.T) {
	out, err := RenderString("Hello {{name}}", map[string]string{"name": "Ada"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "Hello Ada" {
		t.Fatalf("expected replaced string, got %q", out)
	}
}

func TestRenderStringMultipleVars(t *testing.T) {
	out, err := RenderString("{{greet}}, {{name}}!", map[string]string{
		"greet": "Hi",
		"name":  "Sam",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "Hi, Sam!" {
		t.Fatalf("expected replaced string, got %q", out)
	}
}

func TestRenderStringMissingVar(t *testing.T) {
	_, err := RenderString("Hello {{name}}", map[string]string{})
	if err == nil {
		t.Fatalf("expected error")
	}
}
