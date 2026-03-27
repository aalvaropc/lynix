package yamlcollection

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aalvaropc/lynix/internal/domain"
)

// --- MarshalCollection round-trip ---

func TestMarshalCollection_RoundTrip(t *testing.T) {
	status200 := 200
	col := domain.Collection{
		SchemaVersion: 1,
		Name:          "round-trip-test",
		Vars:          domain.Vars{"base_url": "https://api.example.com"},
		Requests: []domain.RequestSpec{
			{
				Name:    "get-users",
				Method:  domain.MethodGet,
				URL:     "{{base_url}}/users",
				Headers: domain.Headers{"Authorization": "Bearer {{token}}"},
				Body:    domain.BodySpec{Type: domain.BodyNone},
				Assert:  domain.AssertionsSpec{Status: &status200},
			},
			{
				Name:    "create-user",
				Method:  domain.MethodPost,
				URL:     "{{base_url}}/users",
				Headers: domain.Headers{"Content-Type": "application/json"},
				Body: domain.BodySpec{
					Type: domain.BodyJSON,
					JSON: map[string]any{"name": "test", "active": true},
				},
				Extract: domain.ExtractSpec{"user_id": "$.id"},
			},
		},
	}

	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatalf("MarshalCollection failed: %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "collection.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	loader := NewLoader()
	loaded, err := loader.LoadCollection(path)
	if err != nil {
		t.Fatalf("LoadCollection failed: %v\nYAML:\n%s", err, string(b))
	}

	if loaded.Name != col.Name {
		t.Errorf("name: got %q, want %q", loaded.Name, col.Name)
	}
	if loaded.Vars["base_url"] != col.Vars["base_url"] {
		t.Errorf("vars[base_url]: got %q, want %q", loaded.Vars["base_url"], col.Vars["base_url"])
	}
	if len(loaded.Requests) != 2 {
		t.Fatalf("requests count: got %d, want 2", len(loaded.Requests))
	}

	r0 := loaded.Requests[0]
	if r0.Name != "get-users" {
		t.Errorf("r0.Name: got %q, want %q", r0.Name, "get-users")
	}
	if r0.Method != domain.MethodGet {
		t.Errorf("r0.Method: got %q, want %q", r0.Method, domain.MethodGet)
	}
	if r0.Headers["Authorization"] != "Bearer {{token}}" {
		t.Errorf("r0.Headers[Authorization]: got %q", r0.Headers["Authorization"])
	}
	if r0.Assert.Status == nil || *r0.Assert.Status != 200 {
		t.Error("r0.Assert.Status: expected 200")
	}

	r1 := loaded.Requests[1]
	if r1.Name != "create-user" {
		t.Errorf("r1.Name: got %q, want %q", r1.Name, "create-user")
	}
	if r1.Method != domain.MethodPost {
		t.Errorf("r1.Method: got %q, want %q", r1.Method, domain.MethodPost)
	}
	if r1.Body.Type != domain.BodyJSON {
		t.Errorf("r1.Body.Type: got %q, want %q", r1.Body.Type, domain.BodyJSON)
	}
	if r1.Extract["user_id"] != "$.id" {
		t.Errorf("r1.Extract[user_id]: got %q, want %q", r1.Extract["user_id"], "$.id")
	}
}

// --- Empty / minimal collections ---

func TestMarshalCollection_EmptyRequests(t *testing.T) {
	col := domain.Collection{
		Name:     "empty",
		Requests: []domain.RequestSpec{},
	}
	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatalf("MarshalCollection failed: %v", err)
	}
	if len(b) == 0 {
		t.Error("expected non-empty YAML output")
	}
}

func TestMarshalCollection_NilRequests(t *testing.T) {
	col := domain.Collection{
		Name:     "nil-reqs",
		Requests: nil,
	}
	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatalf("MarshalCollection failed: %v", err)
	}
	if len(b) == 0 {
		t.Error("expected non-empty YAML output")
	}
}

func TestMarshalCollection_MinimalRequest(t *testing.T) {
	col := domain.Collection{
		Name: "minimal",
		Requests: []domain.RequestSpec{
			{
				Name:   "ping",
				Method: domain.MethodGet,
				URL:    "https://example.com/ping",
				Body:   domain.BodySpec{Type: domain.BodyNone},
			},
		},
	}
	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatalf("MarshalCollection failed: %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "minimal.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	loaded, err := loader.LoadCollection(path)
	if err != nil {
		t.Fatalf("LoadCollection: %v\nYAML:\n%s", err, string(b))
	}
	if loaded.Requests[0].Name != "ping" {
		t.Errorf("name: got %q", loaded.Requests[0].Name)
	}
	if loaded.Requests[0].Method != domain.MethodGet {
		t.Errorf("method: got %q", loaded.Requests[0].Method)
	}
}

// --- Body types ---

func TestMarshalCollection_FormBody(t *testing.T) {
	col := domain.Collection{
		Name: "form-test",
		Requests: []domain.RequestSpec{
			{
				Name:   "login",
				Method: domain.MethodPost,
				URL:    "https://example.com/login",
				Body: domain.BodySpec{
					Type: domain.BodyForm,
					Form: map[string]string{"user": "admin", "pass": "secret"},
				},
			},
		},
	}

	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatalf("MarshalCollection failed: %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "form.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	loaded, err := loader.LoadCollection(path)
	if err != nil {
		t.Fatalf("LoadCollection failed: %v\nYAML:\n%s", err, string(b))
	}

	if loaded.Requests[0].Body.Type != domain.BodyForm {
		t.Errorf("body type: got %q, want %q", loaded.Requests[0].Body.Type, domain.BodyForm)
	}
	if loaded.Requests[0].Body.Form["user"] != "admin" {
		t.Errorf("form[user]: got %q", loaded.Requests[0].Body.Form["user"])
	}
}

func TestMarshalCollection_RawBody(t *testing.T) {
	col := domain.Collection{
		Name: "raw-test",
		Requests: []domain.RequestSpec{
			{
				Name:   "send-xml",
				Method: domain.MethodPost,
				URL:    "https://example.com/xml",
				Body: domain.BodySpec{
					Type: domain.BodyRaw,
					Raw:  "<root><item>1</item></root>",
				},
			},
		},
	}

	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatalf("MarshalCollection failed: %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "raw.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	loaded, err := loader.LoadCollection(path)
	if err != nil {
		t.Fatalf("LoadCollection failed: %v\nYAML:\n%s", err, string(b))
	}

	if loaded.Requests[0].Body.Type != domain.BodyRaw {
		t.Errorf("body type: got %q, want %q", loaded.Requests[0].Body.Type, domain.BodyRaw)
	}
	if loaded.Requests[0].Body.Raw != "<root><item>1</item></root>" {
		t.Errorf("raw body: got %q", loaded.Requests[0].Body.Raw)
	}
}

func TestMarshalCollection_JSONBody_NestedObjects(t *testing.T) {
	col := domain.Collection{
		Name: "nested-json",
		Requests: []domain.RequestSpec{
			{
				Name:   "create",
				Method: domain.MethodPost,
				URL:    "https://example.com/api",
				Body: domain.BodySpec{
					Type: domain.BodyJSON,
					JSON: map[string]any{
						"user": map[string]any{
							"name":  "alice",
							"roles": []any{"admin", "editor"},
						},
						"active": true,
					},
				},
			},
		},
	}

	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatalf("MarshalCollection failed: %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "nested.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	loaded, err := loader.LoadCollection(path)
	if err != nil {
		t.Fatalf("LoadCollection: %v\nYAML:\n%s", err, string(b))
	}
	if loaded.Requests[0].Body.Type != domain.BodyJSON {
		t.Errorf("body type: got %q", loaded.Requests[0].Body.Type)
	}
	userMap, ok := loaded.Requests[0].Body.JSON.(map[string]any)["user"].(map[string]any)
	if !ok {
		t.Fatal("expected nested user object")
	}
	if userMap["name"] != "alice" {
		t.Errorf("user.name: got %v", userMap["name"])
	}
}

func TestMarshalCollection_JSONArrayBody_RoundTrip(t *testing.T) {
	col := domain.Collection{
		Name: "array-body",
		Requests: []domain.RequestSpec{
			{
				Name:   "batch",
				Method: domain.MethodPost,
				URL:    "https://example.com/batch",
				Body: domain.BodySpec{
					Type: domain.BodyJSON,
					JSON: []any{
						map[string]any{"id": 1, "action": "create"},
						map[string]any{"id": 2, "action": "update"},
					},
				},
			},
		},
	}

	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatalf("MarshalCollection failed: %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "array.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	loaded, err := loader.LoadCollection(path)
	if err != nil {
		t.Fatalf("LoadCollection: %v\nYAML:\n%s", err, string(b))
	}
	if loaded.Requests[0].Body.Type != domain.BodyJSON {
		t.Errorf("body type: got %q", loaded.Requests[0].Body.Type)
	}
	arr, ok := loaded.Requests[0].Body.JSON.([]any)
	if !ok {
		t.Fatal("expected []any body after round-trip")
	}
	if len(arr) != 2 {
		t.Errorf("expected 2 elements, got %d", len(arr))
	}
}

func TestMarshalCollection_BodyNone_NoBodyKeysInYAML(t *testing.T) {
	col := domain.Collection{
		Name: "no-body",
		Requests: []domain.RequestSpec{
			{
				Name:   "get",
				Method: domain.MethodGet,
				URL:    "https://example.com/",
				Body:   domain.BodySpec{Type: domain.BodyNone},
			},
		},
	}
	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatal(err)
	}
	yaml := string(b)
	if strings.Contains(yaml, "json:") || strings.Contains(yaml, "form:") || strings.Contains(yaml, "raw:") {
		t.Errorf("expected no body keys in YAML for BodyNone:\n%s", yaml)
	}
}

// --- Vars ---

func TestMarshalCollection_NilVars_OmittedFromYAML(t *testing.T) {
	col := domain.Collection{
		Name:     "no-vars",
		Vars:     nil,
		Requests: []domain.RequestSpec{{Name: "x", Method: domain.MethodGet, URL: "https://e.com/"}},
	}
	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "vars:") {
		t.Errorf("expected vars to be omitted:\n%s", string(b))
	}
}

func TestMarshalCollection_EmptyVars_OmittedFromYAML(t *testing.T) {
	col := domain.Collection{
		Name:     "empty-vars",
		Vars:     domain.Vars{},
		Requests: []domain.RequestSpec{{Name: "x", Method: domain.MethodGet, URL: "https://e.com/"}},
	}
	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "vars:") {
		t.Errorf("expected vars to be omitted when empty:\n%s", string(b))
	}
}

func TestMarshalCollection_MultipleVars(t *testing.T) {
	col := domain.Collection{
		Name: "multi-vars",
		Vars: domain.Vars{"base_url": "https://api.dev", "api_key": "abc", "timeout": "3000"},
		Requests: []domain.RequestSpec{
			{Name: "test", Method: domain.MethodGet, URL: "{{base_url}}/test"},
		},
	}
	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "vars.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	loaded, err := loader.LoadCollection(path)
	if err != nil {
		t.Fatalf("LoadCollection: %v", err)
	}
	if loaded.Vars["base_url"] != "https://api.dev" {
		t.Errorf("base_url: %q", loaded.Vars["base_url"])
	}
	if loaded.Vars["api_key"] != "abc" {
		t.Errorf("api_key: %q", loaded.Vars["api_key"])
	}
	if loaded.Vars["timeout"] != "3000" {
		t.Errorf("timeout: %q", loaded.Vars["timeout"])
	}
}

// --- Headers ---

func TestMarshalCollection_NilHeaders_OmittedFromYAML(t *testing.T) {
	col := domain.Collection{
		Name: "no-headers",
		Requests: []domain.RequestSpec{
			{Name: "x", Method: domain.MethodGet, URL: "https://e.com/", Headers: nil},
		},
	}
	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "headers:") {
		t.Errorf("expected headers to be omitted:\n%s", string(b))
	}
}

func TestMarshalCollection_EmptyHeaders_OmittedFromYAML(t *testing.T) {
	col := domain.Collection{
		Name: "empty-headers",
		Requests: []domain.RequestSpec{
			{Name: "x", Method: domain.MethodGet, URL: "https://e.com/", Headers: domain.Headers{}},
		},
	}
	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "headers:") {
		t.Errorf("expected headers to be omitted when empty:\n%s", string(b))
	}
}

func TestMarshalCollection_MultipleHeaders(t *testing.T) {
	col := domain.Collection{
		Name: "multi-headers",
		Requests: []domain.RequestSpec{
			{
				Name:   "req",
				Method: domain.MethodGet,
				URL:    "https://e.com/",
				Headers: domain.Headers{
					"Authorization": "Bearer tok",
					"Accept":        "application/json",
					"X-Custom":      "value",
				},
			},
		},
	}
	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "headers.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	loaded, err := loader.LoadCollection(path)
	if err != nil {
		t.Fatalf("LoadCollection: %v", err)
	}
	if loaded.Requests[0].Headers["Authorization"] != "Bearer tok" {
		t.Error("Authorization header mismatch")
	}
	if loaded.Requests[0].Headers["Accept"] != "application/json" {
		t.Error("Accept header mismatch")
	}
	if loaded.Requests[0].Headers["X-Custom"] != "value" {
		t.Error("X-Custom header mismatch")
	}
}

// --- Tags ---

func TestMarshalCollection_Tags(t *testing.T) {
	col := domain.Collection{
		Name: "tagged",
		Requests: []domain.RequestSpec{
			{
				Name:   "req",
				Method: domain.MethodGet,
				URL:    "https://e.com/",
				Tags:   []string{"smoke", "auth", "critical"},
			},
		},
	}
	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "tags.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	loaded, err := loader.LoadCollection(path)
	if err != nil {
		t.Fatalf("LoadCollection: %v", err)
	}
	if len(loaded.Requests[0].Tags) != 3 {
		t.Fatalf("tags count: got %d, want 3", len(loaded.Requests[0].Tags))
	}
	want := map[string]bool{"smoke": true, "auth": true, "critical": true}
	for _, tag := range loaded.Requests[0].Tags {
		if !want[tag] {
			t.Errorf("unexpected tag: %q", tag)
		}
	}
}

func TestMarshalCollection_EmptyTags_OmittedFromYAML(t *testing.T) {
	col := domain.Collection{
		Name: "no-tags",
		Requests: []domain.RequestSpec{
			{Name: "x", Method: domain.MethodGet, URL: "https://e.com/", Tags: nil},
		},
	}
	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "tags:") {
		t.Errorf("expected tags to be omitted:\n%s", string(b))
	}
}

// --- Assert ---

func TestMarshalCollection_NoAssert_OmittedFromYAML(t *testing.T) {
	col := domain.Collection{
		Name: "no-assert",
		Requests: []domain.RequestSpec{
			{Name: "x", Method: domain.MethodGet, URL: "https://e.com/"},
		},
	}
	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "assert:") {
		t.Errorf("expected assert to be omitted:\n%s", string(b))
	}
}

func TestMarshalCollection_AssertStatus(t *testing.T) {
	status := 201
	col := domain.Collection{
		Name: "assert-status",
		Requests: []domain.RequestSpec{
			{
				Name:   "create",
				Method: domain.MethodPost,
				URL:    "https://e.com/",
				Assert: domain.AssertionsSpec{Status: &status},
			},
		},
	}
	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "assert.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	loaded, err := loader.LoadCollection(path)
	if err != nil {
		t.Fatalf("LoadCollection: %v", err)
	}
	if loaded.Requests[0].Assert.Status == nil {
		t.Fatal("expected status assertion")
	}
	if *loaded.Requests[0].Assert.Status != 201 {
		t.Errorf("status: got %d, want 201", *loaded.Requests[0].Assert.Status)
	}
}

// --- Extract ---

func TestMarshalCollection_NoExtract_OmittedFromYAML(t *testing.T) {
	col := domain.Collection{
		Name: "no-extract",
		Requests: []domain.RequestSpec{
			{Name: "x", Method: domain.MethodGet, URL: "https://e.com/"},
		},
	}
	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "extract:") {
		t.Errorf("expected extract to be omitted:\n%s", string(b))
	}
}

func TestMarshalCollection_MultipleExtracts(t *testing.T) {
	col := domain.Collection{
		Name: "multi-extract",
		Requests: []domain.RequestSpec{
			{
				Name:   "login",
				Method: domain.MethodPost,
				URL:    "https://e.com/login",
				Extract: domain.ExtractSpec{
					"token":      "$.access_token",
					"user_id":    "$.user.id",
					"expires_in": "$.expires_in",
				},
			},
		},
	}
	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "extract.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	loaded, err := loader.LoadCollection(path)
	if err != nil {
		t.Fatalf("LoadCollection: %v", err)
	}
	if loaded.Requests[0].Extract["token"] != "$.access_token" {
		t.Error("token extract mismatch")
	}
	if loaded.Requests[0].Extract["user_id"] != "$.user.id" {
		t.Error("user_id extract mismatch")
	}
	if loaded.Requests[0].Extract["expires_in"] != "$.expires_in" {
		t.Error("expires_in extract mismatch")
	}
}

// --- SchemaVersion ---

func TestMarshalCollection_SchemaVersion_AlwaysOne(t *testing.T) {
	col := domain.Collection{
		SchemaVersion: 42,
		Name:          "version-test",
		Requests:      []domain.RequestSpec{{Name: "x", Method: domain.MethodGet, URL: "https://e.com/"}},
	}
	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "schema_version: 1") {
		t.Errorf("expected schema_version: 1 regardless of input:\n%s", string(b))
	}
}

// --- YAML output structure ---

func TestMarshalCollection_YAMLContainsExpectedKeys(t *testing.T) {
	status := 200
	col := domain.Collection{
		Name: "structure-test",
		Vars: domain.Vars{"key": "val"},
		Requests: []domain.RequestSpec{
			{
				Name:    "req",
				Method:  domain.MethodPost,
				URL:     "https://e.com/",
				Headers: domain.Headers{"H": "V"},
				Body:    domain.BodySpec{Type: domain.BodyJSON, JSON: map[string]any{"a": "b"}},
				Tags:    []string{"t1"},
				Assert:  domain.AssertionsSpec{Status: &status},
				Extract: domain.ExtractSpec{"v": "$.v"},
			},
		},
	}
	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatal(err)
	}
	yaml := string(b)
	for _, key := range []string{"schema_version:", "name:", "vars:", "requests:", "method:", "url:", "headers:", "json:", "tags:", "assert:", "extract:"} {
		if !strings.Contains(yaml, key) {
			t.Errorf("expected YAML to contain %q:\n%s", key, yaml)
		}
	}
}

// --- HTTP methods ---

func TestMarshalCollection_AllHTTPMethods(t *testing.T) {
	methods := []domain.HTTPMethod{
		domain.MethodGet, domain.MethodPost, domain.MethodPut,
		domain.MethodPatch, domain.MethodDelete, domain.MethodHead, domain.MethodOptions,
	}
	for _, m := range methods {
		col := domain.Collection{
			Name: "method-" + strings.ToLower(string(m)),
			Requests: []domain.RequestSpec{
				{Name: "req", Method: m, URL: "https://e.com/"},
			},
		}
		b, err := MarshalCollection(col)
		if err != nil {
			t.Fatalf("method %s: %v", m, err)
		}

		tmp := t.TempDir()
		path := filepath.Join(tmp, "m.yaml")
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatal(err)
		}

		loader := NewLoader()
		loaded, err := loader.LoadCollection(path)
		if err != nil {
			t.Fatalf("method %s round-trip: %v", m, err)
		}
		if loaded.Requests[0].Method != m {
			t.Errorf("method: got %q, want %q", loaded.Requests[0].Method, m)
		}
	}
}

// --- Special characters ---

func TestMarshalCollection_SpecialCharsInValues(t *testing.T) {
	col := domain.Collection{
		Name: "special-chars: test & more",
		Vars: domain.Vars{"key": "value with spaces & 'quotes'"},
		Requests: []domain.RequestSpec{
			{
				Name:    "special",
				Method:  domain.MethodPost,
				URL:     "https://e.com/path?a=1&b=2",
				Headers: domain.Headers{"X-Val": `he said "hello"`},
				Body:    domain.BodySpec{Type: domain.BodyRaw, Raw: "line1\nline2\ttab"},
			},
		},
	}
	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatalf("MarshalCollection failed: %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "special.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	loaded, err := loader.LoadCollection(path)
	if err != nil {
		t.Fatalf("LoadCollection: %v\nYAML:\n%s", err, string(b))
	}
	if loaded.Name != col.Name {
		t.Errorf("name: got %q, want %q", loaded.Name, col.Name)
	}
	if loaded.Vars["key"] != col.Vars["key"] {
		t.Errorf("var: got %q, want %q", loaded.Vars["key"], col.Vars["key"])
	}
}

func TestMarshalCollection_UnicodeValues(t *testing.T) {
	col := domain.Collection{
		Name: "unicode-test",
		Requests: []domain.RequestSpec{
			{
				Name:    "emoji-req",
				Method:  domain.MethodGet,
				URL:     "https://e.com/",
				Headers: domain.Headers{"X-Lang": "日本語"},
			},
		},
	}
	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "unicode.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	loaded, err := loader.LoadCollection(path)
	if err != nil {
		t.Fatalf("LoadCollection: %v", err)
	}
	if loaded.Requests[0].Headers["X-Lang"] != "日本語" {
		t.Errorf("unicode header: got %q", loaded.Requests[0].Headers["X-Lang"])
	}
}

// --- Multiple requests ---

func TestMarshalCollection_ManyRequests(t *testing.T) {
	reqs := make([]domain.RequestSpec, 10)
	for i := range reqs {
		reqs[i] = domain.RequestSpec{
			Name:   strings.Repeat("r", i+1),
			Method: domain.MethodGet,
			URL:    "https://e.com/",
		}
	}
	col := domain.Collection{Name: "many", Requests: reqs}
	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "many.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	loaded, err := loader.LoadCollection(path)
	if err != nil {
		t.Fatalf("LoadCollection: %v", err)
	}
	if len(loaded.Requests) != 10 {
		t.Errorf("requests count: got %d, want 10", len(loaded.Requests))
	}
}

// --- Full round-trip with all fields ---

func TestMarshalCollection_AllFields_RoundTrip(t *testing.T) {
	status := 200
	col := domain.Collection{
		SchemaVersion: 1,
		Name:          "all-fields",
		Vars:          domain.Vars{"base": "https://api.dev", "key": "secret"},
		Requests: []domain.RequestSpec{
			{
				Name:   "full-request",
				Method: domain.MethodPut,
				URL:    "{{base}}/resource/1",
				Headers: domain.Headers{
					"Authorization": "Bearer {{key}}",
					"Content-Type":  "application/json",
					"Accept":        "application/json",
				},
				Body: domain.BodySpec{
					Type: domain.BodyJSON,
					JSON: map[string]any{"name": "updated", "count": float64(42)},
				},
				Tags:    []string{"update", "resource"},
				Assert:  domain.AssertionsSpec{Status: &status},
				Extract: domain.ExtractSpec{"version": "$.version"},
			},
		},
	}

	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "full.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	loaded, err := loader.LoadCollection(path)
	if err != nil {
		t.Fatalf("LoadCollection: %v\nYAML:\n%s", err, string(b))
	}

	r := loaded.Requests[0]
	if r.Name != "full-request" {
		t.Errorf("name: %q", r.Name)
	}
	if r.Method != domain.MethodPut {
		t.Errorf("method: %q", r.Method)
	}
	if r.URL != "{{base}}/resource/1" {
		t.Errorf("url: %q", r.URL)
	}
	if len(r.Headers) != 3 {
		t.Errorf("headers count: %d", len(r.Headers))
	}
	if r.Body.Type != domain.BodyJSON {
		t.Errorf("body type: %q", r.Body.Type)
	}
	if len(r.Tags) != 2 {
		t.Errorf("tags count: %d", len(r.Tags))
	}
	if r.Assert.Status == nil || *r.Assert.Status != 200 {
		t.Error("assert status mismatch")
	}
	if r.Extract["version"] != "$.version" {
		t.Errorf("extract: %q", r.Extract["version"])
	}
}

// --- Templating ---

func TestMarshalCollection_TemplateVarsPreserved(t *testing.T) {
	col := domain.Collection{
		Name: "templates",
		Vars: domain.Vars{"base_url": "https://api.dev"},
		Requests: []domain.RequestSpec{
			{
				Name:    "req",
				Method:  domain.MethodGet,
				URL:     "{{base_url}}/{{path}}",
				Headers: domain.Headers{"Authorization": "Bearer {{token}}"},
			},
		},
	}

	b, err := MarshalCollection(col)
	if err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "tpl.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	loader := NewLoader()
	loaded, err := loader.LoadCollection(path)
	if err != nil {
		t.Fatalf("LoadCollection: %v", err)
	}
	if loaded.Requests[0].URL != "{{base_url}}/{{path}}" {
		t.Errorf("URL templates lost: %q", loaded.Requests[0].URL)
	}
	if loaded.Requests[0].Headers["Authorization"] != "Bearer {{token}}" {
		t.Errorf("header templates lost: %q", loaded.Requests[0].Headers["Authorization"])
	}
}
