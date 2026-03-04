package yamlcollection

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aalvaropc/lynix/internal/domain"
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

func TestLoadCollection_JSONPathAssertions(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "demo.yaml")

	content := []byte(`
name: JSONPath API
requests:
  - name: check
    method: GET
    url: "http://x/api"
    assert:
      status: 200
      jsonpath:
        "$.name":
          exists: true
          eq: "alice"
          contains: "ali"
          matches: "^[a-z]+$"
        "$.age":
          gt: 18
          lt: 100
`)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l := NewLoader()
	c, err := l.LoadCollection(p)
	if err != nil {
		t.Fatalf("LoadCollection error: %v", err)
	}

	if len(c.Requests) != 1 {
		t.Fatalf("expected 1 request, got=%d", len(c.Requests))
	}

	jp := c.Requests[0].Assert.JSONPath

	nameA, ok := jp["$.name"]
	if !ok {
		t.Fatal("expected $.name assertion")
	}
	if !nameA.Exists {
		t.Error("expected $.name exists=true")
	}
	if nameA.Eq == nil || *nameA.Eq != "alice" {
		t.Errorf("expected $.name eq=alice, got=%v", nameA.Eq)
	}
	if nameA.Contains == nil || *nameA.Contains != "ali" {
		t.Errorf("expected $.name contains=ali, got=%v", nameA.Contains)
	}
	if nameA.Matches == nil || *nameA.Matches != "^[a-z]+$" {
		t.Errorf("expected $.name matches=^[a-z]+$, got=%v", nameA.Matches)
	}

	ageA, ok := jp["$.age"]
	if !ok {
		t.Fatal("expected $.age assertion")
	}
	if ageA.Gt == nil || *ageA.Gt != 18 {
		t.Errorf("expected $.age gt=18, got=%v", ageA.Gt)
	}
	if ageA.Lt == nil || *ageA.Lt != 100 {
		t.Errorf("expected $.age lt=100, got=%v", ageA.Lt)
	}
}

func TestLoadCollection_DuplicateBodyTypes(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "bad.yaml")

	content := []byte(`
name: Demo API
requests:
  - name: dup
    method: POST
    url: "http://x"
    json:
      key: value
    form:
      field: val
`)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l := NewLoader()
	_, err := l.LoadCollection(p)
	if err == nil {
		t.Fatal("expected error for duplicate body types")
	}

	if !domain.IsKind(err, domain.KindInvalidConfig) {
		t.Fatalf("expected KindInvalidConfig, got: %v", err)
	}
}

func TestLoadCollection_ContentType(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "demo.yaml")

	content := []byte(`
name: Demo API
requests:
  - name: post.raw
    method: POST
    url: "http://x"
    raw: "hello"
    content_type: "text/plain"
`)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l := NewLoader()
	c, err := l.LoadCollection(p)
	if err != nil {
		t.Fatalf("LoadCollection error: %v", err)
	}

	if len(c.Requests) != 1 {
		t.Fatalf("expected 1 request, got=%d", len(c.Requests))
	}
	if c.Requests[0].Body.Type != domain.BodyRaw {
		t.Fatalf("expected body type raw, got=%s", c.Requests[0].Body.Type)
	}
	if c.Requests[0].Body.ContentType != "text/plain" {
		t.Fatalf("expected content type text/plain, got=%q", c.Requests[0].Body.ContentType)
	}
}

func TestLoadCollection_SchemaFile(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "demo.yaml")

	content := []byte(`
name: Schema API
requests:
  - name: check
    method: GET
    url: "http://x/api"
    assert:
      schema: "schemas/user.json"
`)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l := NewLoader()
	c, err := l.LoadCollection(p)
	if err != nil {
		t.Fatalf("LoadCollection error: %v", err)
	}

	if c.Requests[0].Assert.Schema == nil {
		t.Fatal("expected schema to be set")
	}
	// Schema path should be resolved relative to the collection file directory.
	expected := filepath.Join(tmp, "schemas/user.json")
	if *c.Requests[0].Assert.Schema != expected {
		t.Errorf("expected schema=%q, got=%q", expected, *c.Requests[0].Assert.Schema)
	}
}

func TestLoadCollection_SchemaInline(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "demo.yaml")

	content := []byte(`
name: Schema API
requests:
  - name: check
    method: GET
    url: "http://x/api"
    assert:
      schema_inline:
        type: object
        required: ["id"]
        properties:
          id:
            type: integer
`)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l := NewLoader()
	c, err := l.LoadCollection(p)
	if err != nil {
		t.Fatalf("LoadCollection error: %v", err)
	}

	if c.Requests[0].Assert.SchemaInline == nil {
		t.Fatal("expected schema_inline to be set")
	}
	if c.Requests[0].Assert.SchemaInline["type"] != "object" {
		t.Errorf("expected type=object, got=%v", c.Requests[0].Assert.SchemaInline["type"])
	}
}

func TestLoadCollection_SchemaBothError(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "bad.yaml")

	content := []byte(`
name: Schema API
requests:
  - name: check
    method: GET
    url: "http://x/api"
    assert:
      schema: "schemas/user.json"
      schema_inline:
        type: object
`)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l := NewLoader()
	_, err := l.LoadCollection(p)
	if err == nil {
		t.Fatal("expected error when both schema and schema_inline are set")
	}
	if !domain.IsKind(err, domain.KindInvalidConfig) {
		t.Fatalf("expected KindInvalidConfig, got: %v", err)
	}
}
