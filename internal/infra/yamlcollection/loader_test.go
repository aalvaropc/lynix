package yamlcollection

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCollection_Valid(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "demo.yaml")

	content := []byte(`
name: Demo API
vars:
  base_url: "https://api.example.com"
requests:
  - name: health
    method: GET
    url: "{{base_url}}/health"
    assert:
      status: 200
      max_ms: 1500
`)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l := NewLoader()
	c, err := l.LoadCollection(p)
	if err != nil {
		t.Fatalf("LoadCollection error: %v", err)
	}

	if c.Name != "Demo API" {
		t.Fatalf("expected name=Demo API, got=%s", c.Name)
	}
	if len(c.Requests) != 1 {
		t.Fatalf("expected 1 request, got=%d", len(c.Requests))
	}
	if c.Requests[0].Name != "health" {
		t.Fatalf("expected request name=health, got=%s", c.Requests[0].Name)
	}
}

func TestLoadCollection_InvalidMethod(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "bad.yaml")

	content := []byte(`
name: Demo API
requests:
  - name: health
    method: FETCH
    url: "http://x"
`)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l := NewLoader()
	_, err := l.LoadCollection(p)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadCollection_MissingRequestName(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "bad.yaml")

	content := []byte(`
name: Demo API
requests:
  - method: GET
    url: "http://x"
`)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l := NewLoader()
	_, err := l.LoadCollection(p)
	if err == nil {
		t.Fatalf("expected error")
	}
}
