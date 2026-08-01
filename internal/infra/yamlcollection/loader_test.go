package yamlcollection

import (
	"os"
	"path/filepath"
	"strings"
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
	if nameA.Exists == nil || !*nameA.Exists {
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

func TestLoadCollection_WithTags(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "demo.yaml")

	content := []byte(`
name: Tagged API
requests:
  - name: smoke
    method: GET
    url: "http://x/health"
    tags: [smoke, auth]
    assert:
      status: 200
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
	tags := c.Requests[0].Tags
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got=%d", len(tags))
	}
	if tags[0] != "smoke" || tags[1] != "auth" {
		t.Fatalf("expected tags=[smoke, auth], got=%v", tags)
	}
}

func TestLoadCollection_WithoutTags(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "demo.yaml")

	content := []byte(`
name: No Tags API
requests:
  - name: health
    method: GET
    url: "http://x/health"
`)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l := NewLoader()
	c, err := l.LoadCollection(p)
	if err != nil {
		t.Fatalf("LoadCollection error: %v", err)
	}

	if c.Requests[0].Tags != nil {
		t.Fatalf("expected nil tags, got=%v", c.Requests[0].Tags)
	}
}

func TestLoadCollection_SchemaVersion_Present(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "demo.yaml")

	content := []byte(`
schema_version: 1
name: Versioned API
requests:
  - name: health
    method: GET
    url: "http://x/health"
`)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l := NewLoader()
	c, err := l.LoadCollection(p)
	if err != nil {
		t.Fatalf("LoadCollection error: %v", err)
	}

	if c.SchemaVersion != 1 {
		t.Fatalf("expected schema_version=1, got=%d", c.SchemaVersion)
	}
}

func TestLoadCollection_SchemaVersion_Missing_DefaultsToOne(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "demo.yaml")

	content := []byte(`
name: No Version API
requests:
  - name: health
    method: GET
    url: "http://x/health"
`)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l := NewLoader()
	c, err := l.LoadCollection(p)
	if err != nil {
		t.Fatalf("LoadCollection error: %v", err)
	}

	if c.SchemaVersion != 1 {
		t.Fatalf("expected schema_version default=1, got=%d", c.SchemaVersion)
	}
}

func TestLoadCollection_SchemaVersion_Invalid_ReturnsError(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "bad.yaml")

	content := []byte(`
schema_version: 0
name: Bad Version API
requests:
  - name: health
    method: GET
    url: "http://x/health"
`)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l := NewLoader()
	_, err := l.LoadCollection(p)
	if err == nil {
		t.Fatal("expected error for schema_version=0")
	}
	if !domain.IsKind(err, domain.KindInvalidConfig) {
		t.Fatalf("expected KindInvalidConfig, got: %v", err)
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

func TestLoadCollection_HeaderAssertions(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "demo.yaml")

	content := []byte(`
name: Header API
requests:
  - name: check
    method: GET
    url: "http://x/api"
    assert:
      status: 200
      headers:
        "Content-Type":
          eq: "application/json"
          contains: "json"
        "Cache-Control":
          exists: true
          not_contains: "no-store"
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

	hdr := c.Requests[0].Assert.Headers

	ct, ok := hdr["Content-Type"]
	if !ok {
		t.Fatal("expected Content-Type assertion")
	}
	if ct.Eq == nil || *ct.Eq != "application/json" {
		t.Errorf("expected Content-Type eq=application/json, got=%v", ct.Eq)
	}
	if ct.Contains == nil || *ct.Contains != "json" {
		t.Errorf("expected Content-Type contains=json, got=%v", ct.Contains)
	}

	cc, ok := hdr["Cache-Control"]
	if !ok {
		t.Fatal("expected Cache-Control assertion")
	}
	if cc.Exists == nil || !*cc.Exists {
		t.Error("expected Cache-Control exists=true")
	}
	if cc.NotContains == nil || *cc.NotContains != "no-store" {
		t.Errorf("expected Cache-Control not_contains=no-store, got=%v", cc.NotContains)
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

func TestLoadCollection_ExistsFalse(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "demo.yaml")

	content := []byte(`
name: Exists API
requests:
  - name: check
    method: GET
    url: "http://x/api"
    assert:
      jsonpath:
        "$.error":
          exists: false
`)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l := NewLoader()
	c, err := l.LoadCollection(p)
	if err != nil {
		t.Fatalf("LoadCollection error: %v", err)
	}

	a, ok := c.Requests[0].Assert.JSONPath["$.error"]
	if !ok {
		t.Fatal("expected $.error assertion")
	}
	if a.Exists == nil || *a.Exists {
		t.Errorf("expected exists=false to be preserved, got=%v", a.Exists)
	}
}

func TestLoadCollection_EmptyAssertionRejected(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "demo.yaml")

	content := []byte(`
name: Empty Assert API
requests:
  - name: check
    method: GET
    url: "http://x/api"
    assert:
      jsonpath:
        "$.foo": {}
`)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l := NewLoader()
	_, err := l.LoadCollection(p)
	if err == nil {
		t.Fatal("expected error for assertion without operators")
	}
	if !strings.Contains(err.Error(), "no operators") {
		t.Errorf("expected 'no operators' in error, got: %v", err)
	}
}

func TestLoadCollection_EmptyHeaderAssertionRejected(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "demo.yaml")

	content := []byte(`
name: Empty Header Assert API
requests:
  - name: check
    method: GET
    url: "http://x/api"
    assert:
      headers:
        "X-Foo": {}
`)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l := NewLoader()
	_, err := l.LoadCollection(p)
	if err == nil {
		t.Fatal("expected error for header assertion without operators")
	}
}

func TestLoadCollection_UnknownFieldRejected(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "typo.yaml")

	content := []byte(`
name: Typo API
requests:
  - name: check
    method: GET
    url: "http://x/api"
    assrt:
      status: 200
`)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l := NewLoader()
	_, err := l.LoadCollection(p)
	if err == nil {
		t.Fatal("expected error for unknown field 'assrt'")
	}
	if !domain.IsKind(err, domain.KindInvalidConfig) {
		t.Fatalf("expected KindInvalidConfig, got: %v", err)
	}
}

func TestLoadCollection_DuplicateRequestNamesRejected(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "dup.yaml")

	content := []byte(`
name: Dup API
requests:
  - name: check
    method: GET
    url: "http://x/a"
  - name: check
    method: GET
    url: "http://x/b"
`)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l := NewLoader()
	_, err := l.LoadCollection(p)
	if err == nil {
		t.Fatal("expected error for duplicate request names")
	}
	if !strings.Contains(err.Error(), "duplicate request name") {
		t.Errorf("expected duplicate name error, got: %v", err)
	}
}

func TestLoadCollection_StatusList(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "s.yaml")

	content := []byte(`
name: Status List
requests:
  - name: create
    method: POST
    url: "http://x"
    assert:
      status: [200, 201]
`)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	c, err := NewLoader().LoadCollection(p)
	if err != nil {
		t.Fatalf("LoadCollection error: %v", err)
	}
	a := c.Requests[0].Assert
	if a.Status != nil {
		t.Errorf("expected nil single status, got %v", *a.Status)
	}
	if len(a.StatusIn) != 2 || a.StatusIn[0] != 200 || a.StatusIn[1] != 201 {
		t.Errorf("expected StatusIn [200 201], got %v", a.StatusIn)
	}
}

func TestLoadCollection_BodyAssertion(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "b.yaml")

	content := []byte(`
name: Body Assert
requests:
  - name: page
    method: GET
    url: "http://x"
    assert:
      body:
        contains: "ok"
        not_matches: "error"
`)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	c, err := NewLoader().LoadCollection(p)
	if err != nil {
		t.Fatalf("LoadCollection error: %v", err)
	}
	b := c.Requests[0].Assert.Body
	if b == nil || b.Contains == nil || *b.Contains != "ok" || b.NotMatches == nil {
		t.Fatalf("body assertion not mapped: %+v", b)
	}
}

func TestLoadCollection_EmptyBodyAssertionRejected(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "b.yaml")

	content := []byte(`
name: Body Assert
requests:
  - name: page
    method: GET
    url: "http://x"
    assert:
      body: {}
`)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := NewLoader().LoadCollection(p); err == nil {
		t.Fatal("expected error for body assertion without operators")
	}
}
