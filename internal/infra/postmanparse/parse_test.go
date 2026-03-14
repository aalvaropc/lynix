package postmanparse

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/infra/yamlcollection"
)

// ========================
// PostmanURL UnmarshalJSON
// ========================

func TestPostmanURL_UnmarshalJSON_String(t *testing.T) {
	var u PostmanURL
	if err := json.Unmarshal([]byte(`"https://example.com/test"`), &u); err != nil {
		t.Fatal(err)
	}
	if u.Raw != "https://example.com/test" {
		t.Errorf("raw: got %q", u.Raw)
	}
}

func TestPostmanURL_UnmarshalJSON_Object(t *testing.T) {
	var u PostmanURL
	data := `{"raw": "https://example.com/api", "query": [{"key": "q", "value": "test"}]}`
	if err := json.Unmarshal([]byte(data), &u); err != nil {
		t.Fatal(err)
	}
	if u.Raw != "https://example.com/api" {
		t.Errorf("raw: got %q", u.Raw)
	}
	if len(u.Query) != 1 {
		t.Fatalf("query count: got %d", len(u.Query))
	}
	if u.Query[0].Key != "q" || u.Query[0].Value != "test" {
		t.Errorf("query: got %+v", u.Query[0])
	}
}

func TestPostmanURL_UnmarshalJSON_InvalidData(t *testing.T) {
	var u PostmanURL
	err := json.Unmarshal([]byte(`12345`), &u)
	if err == nil {
		t.Error("expected error for numeric URL")
	}
}

func TestPostmanURL_UnmarshalJSON_EmptyObject(t *testing.T) {
	var u PostmanURL
	if err := json.Unmarshal([]byte(`{}`), &u); err != nil {
		t.Fatal(err)
	}
	if u.Raw != "" {
		t.Errorf("expected empty raw, got %q", u.Raw)
	}
}

func TestPostmanURL_UnmarshalJSON_EmptyString(t *testing.T) {
	var u PostmanURL
	if err := json.Unmarshal([]byte(`""`), &u); err != nil {
		t.Fatal(err)
	}
	if u.Raw != "" {
		t.Errorf("expected empty raw, got %q", u.Raw)
	}
}

// ========================
// Parse — basic collections
// ========================

func TestParse_SimpleCollection(t *testing.T) {
	input := `{
		"info": {"name": "Simple API", "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},
		"item": [
			{
				"name": "Get Users",
				"request": {
					"method": "GET",
					"url": "https://api.example.com/users"
				}
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}

	if r.Collection.Name != "Simple API" {
		t.Errorf("name: got %q", r.Collection.Name)
	}
	if r.Collection.SchemaVersion != 1 {
		t.Errorf("schema_version: got %d", r.Collection.SchemaVersion)
	}
	if len(r.Collection.Requests) != 1 {
		t.Fatalf("requests: got %d, want 1", len(r.Collection.Requests))
	}
	req := r.Collection.Requests[0]
	if req.Method != domain.MethodGet {
		t.Errorf("method: got %q", req.Method)
	}
	if req.URL != "https://api.example.com/users" {
		t.Errorf("url: got %q", req.URL)
	}
}

func TestParse_MultipleRequests(t *testing.T) {
	input := `{
		"info": {"name": "Multi", "schema": ""},
		"item": [
			{"name": "Req1", "request": {"method": "GET", "url": "https://e.com/1"}},
			{"name": "Req2", "request": {"method": "POST", "url": "https://e.com/2"}},
			{"name": "Req3", "request": {"method": "DELETE", "url": "https://e.com/3"}}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Collection.Requests) != 3 {
		t.Fatalf("requests: got %d, want 3", len(r.Collection.Requests))
	}
	if r.Collection.Requests[0].Method != domain.MethodGet {
		t.Error("req0 method")
	}
	if r.Collection.Requests[1].Method != domain.MethodPost {
		t.Error("req1 method")
	}
	if r.Collection.Requests[2].Method != domain.MethodDelete {
		t.Error("req2 method")
	}
}

func TestParse_MultipleRequestsWithHeaders(t *testing.T) {
	input := `{
		"info": {"name": "Multi", "schema": ""},
		"item": [
			{
				"name": "List",
				"request": {
					"method": "GET",
					"url": {"raw": "https://api.example.com/items", "query": []},
					"header": [
						{"key": "Accept", "value": "application/json"}
					]
				}
			},
			{
				"name": "Create",
				"request": {
					"method": "POST",
					"url": {"raw": "https://api.example.com/items"},
					"header": [
						{"key": "Content-Type", "value": "application/json"}
					],
					"body": {
						"mode": "raw",
						"raw": "{\"name\":\"test\"}",
						"options": {"raw": {"language": "json"}}
					}
				}
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Collection.Requests) != 2 {
		t.Fatalf("requests: got %d, want 2", len(r.Collection.Requests))
	}

	if r.Collection.Requests[0].Headers["Accept"] != "application/json" {
		t.Errorf("r0 Accept header: got %q", r.Collection.Requests[0].Headers["Accept"])
	}
	if r.Collection.Requests[1].Body.Type != domain.BodyJSON {
		t.Errorf("r1 body type: got %q, want json", r.Collection.Requests[1].Body.Type)
	}
	if r.Collection.Requests[1].Body.JSON["name"] != "test" {
		t.Errorf("r1 body json[name]: got %v", r.Collection.Requests[1].Body.JSON["name"])
	}
}

func TestParse_EmptyCollection(t *testing.T) {
	input := `{
		"info": {"name": "Empty", "schema": ""},
		"item": []
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Name != "Empty" {
		t.Errorf("name: got %q", r.Collection.Name)
	}
	if len(r.Collection.Requests) != 0 {
		t.Errorf("requests: got %d, want 0", len(r.Collection.Requests))
	}
}

// ========================
// Parse — methods
// ========================

func TestParse_MethodDefaultsToGET(t *testing.T) {
	input := `{
		"info": {"name": "NoMethod", "schema": ""},
		"item": [
			{
				"name": "Test",
				"request": {
					"method": "",
					"url": "https://e.com/"
				}
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Method != domain.MethodGet {
		t.Errorf("method: got %q, want GET (default)", r.Collection.Requests[0].Method)
	}
}

func TestParse_AllMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	for _, m := range methods {
		input := `{
			"info": {"name": "M", "schema": ""},
			"item": [{"name": "R", "request": {"method": "` + m + `", "url": "https://e.com/"}}]
		}`
		r, err := Parse(strings.NewReader(input))
		if err != nil {
			t.Fatalf("method %s: %v", m, err)
		}
		if string(r.Collection.Requests[0].Method) != m {
			t.Errorf("method %s: got %q", m, r.Collection.Requests[0].Method)
		}
	}
}

func TestParse_LowercaseMethod(t *testing.T) {
	input := `{
		"info": {"name": "Lower", "schema": ""},
		"item": [{"name": "R", "request": {"method": "post", "url": "https://e.com/"}}]
	}`
	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Method != domain.MethodPost {
		t.Errorf("method: got %q", r.Collection.Requests[0].Method)
	}
}

// ========================
// Parse — URL forms
// ========================

func TestParse_URLAsString(t *testing.T) {
	input := `{
		"info": {"name": "StringURL", "schema": ""},
		"item": [
			{
				"name": "Test",
				"request": {
					"method": "GET",
					"url": "https://api.example.com/test"
				}
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].URL != "https://api.example.com/test" {
		t.Errorf("url: got %q", r.Collection.Requests[0].URL)
	}
}

func TestParse_URLAsObject(t *testing.T) {
	input := `{
		"info": {"name": "ObjURL", "schema": ""},
		"item": [
			{
				"name": "Test",
				"request": {
					"method": "GET",
					"url": {"raw": "https://api.example.com/obj", "query": [{"key": "a", "value": "1"}]}
				}
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].URL != "https://api.example.com/obj" {
		t.Errorf("url: got %q", r.Collection.Requests[0].URL)
	}
}

func TestParse_PostmanVars_PassThrough(t *testing.T) {
	input := `{
		"info": {"name": "Vars", "schema": ""},
		"item": [
			{
				"name": "Test",
				"request": {
					"method": "GET",
					"url": "{{base_url}}/v1/users"
				}
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].URL != "{{base_url}}/v1/users" {
		t.Errorf("url: got %q — expected Postman vars to pass through", r.Collection.Requests[0].URL)
	}
}

// ========================
// Parse — headers
// ========================

func TestParse_MultipleHeaders(t *testing.T) {
	input := `{
		"info": {"name": "H", "schema": ""},
		"item": [
			{
				"name": "R",
				"request": {
					"method": "GET",
					"url": "https://e.com/",
					"header": [
						{"key": "Accept", "value": "application/json"},
						{"key": "Authorization", "value": "Bearer token"},
						{"key": "X-Custom", "value": "val"}
					]
				}
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	h := r.Collection.Requests[0].Headers
	if len(h) != 3 {
		t.Fatalf("headers count: got %d", len(h))
	}
	if h["Accept"] != "application/json" {
		t.Error("Accept mismatch")
	}
	if h["Authorization"] != "Bearer token" {
		t.Error("Authorization mismatch")
	}
	if h["X-Custom"] != "val" {
		t.Error("X-Custom mismatch")
	}
}

func TestParse_NoHeaders(t *testing.T) {
	input := `{
		"info": {"name": "NoH", "schema": ""},
		"item": [{"name": "R", "request": {"method": "GET", "url": "https://e.com/"}}]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Collection.Requests[0].Headers) != 0 {
		t.Errorf("expected empty headers, got %d", len(r.Collection.Requests[0].Headers))
	}
}

// ========================
// Parse — body types
// ========================

func TestParse_RawBodyJSON(t *testing.T) {
	input := `{
		"info": {"name": "JSON", "schema": ""},
		"item": [
			{
				"name": "Create",
				"request": {
					"method": "POST",
					"url": "https://e.com/",
					"body": {
						"mode": "raw",
						"raw": "{\"name\":\"test\",\"count\":42}",
						"options": {"raw": {"language": "json"}}
					}
				}
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	req := r.Collection.Requests[0]
	if req.Body.Type != domain.BodyJSON {
		t.Errorf("body type: got %q", req.Body.Type)
	}
	if req.Body.JSON["name"] != "test" {
		t.Errorf("json[name]: got %v", req.Body.JSON["name"])
	}
}

func TestParse_RawBodyJSON_InvalidJSON_FallbackToRaw(t *testing.T) {
	input := `{
		"info": {"name": "BadJSON", "schema": ""},
		"item": [
			{
				"name": "Bad",
				"request": {
					"method": "POST",
					"url": "https://e.com/",
					"body": {
						"mode": "raw",
						"raw": "not valid json {",
						"options": {"raw": {"language": "json"}}
					}
				}
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	req := r.Collection.Requests[0]
	if req.Body.Type != domain.BodyRaw {
		t.Errorf("expected raw fallback for invalid JSON, got %q", req.Body.Type)
	}
	if req.Body.Raw != "not valid json {" {
		t.Errorf("raw: got %q", req.Body.Raw)
	}
}

func TestParse_RawBodyNonJSON(t *testing.T) {
	input := `{
		"info": {"name": "RawText", "schema": ""},
		"item": [
			{
				"name": "SendXML",
				"request": {
					"method": "POST",
					"url": "https://api.example.com/xml",
					"body": {
						"mode": "raw",
						"raw": "<root>hello</root>"
					}
				}
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	req := r.Collection.Requests[0]
	if req.Body.Type != domain.BodyRaw {
		t.Errorf("body type: got %q, want raw", req.Body.Type)
	}
	if req.Body.Raw != "<root>hello</root>" {
		t.Errorf("raw body: got %q", req.Body.Raw)
	}
}

func TestParse_RawBodyWithOptionsNoLanguage(t *testing.T) {
	input := `{
		"info": {"name": "NoLang", "schema": ""},
		"item": [
			{
				"name": "R",
				"request": {
					"method": "POST",
					"url": "https://e.com/",
					"body": {
						"mode": "raw",
						"raw": "plain text",
						"options": {"raw": {"language": ""}}
					}
				}
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Body.Type != domain.BodyRaw {
		t.Errorf("body type: got %q, want raw", r.Collection.Requests[0].Body.Type)
	}
}

func TestParse_URLEncodedBody(t *testing.T) {
	input := `{
		"info": {"name": "Form", "schema": ""},
		"item": [
			{
				"name": "Login",
				"request": {
					"method": "POST",
					"url": "https://api.example.com/login",
					"body": {
						"mode": "urlencoded",
						"urlencoded": [
							{"key": "username", "value": "admin"},
							{"key": "password", "value": "secret"}
						]
					}
				}
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	req := r.Collection.Requests[0]
	if req.Body.Type != domain.BodyForm {
		t.Errorf("body type: got %q, want form", req.Body.Type)
	}
	if req.Body.Form["username"] != "admin" {
		t.Errorf("form[username]: got %q", req.Body.Form["username"])
	}
	if req.Body.Form["password"] != "secret" {
		t.Errorf("form[password]: got %q", req.Body.Form["password"])
	}
}

func TestParse_URLEncodedBody_Empty(t *testing.T) {
	input := `{
		"info": {"name": "EmptyForm", "schema": ""},
		"item": [
			{
				"name": "R",
				"request": {
					"method": "POST",
					"url": "https://e.com/",
					"body": {
						"mode": "urlencoded",
						"urlencoded": []
					}
				}
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Body.Type != domain.BodyForm {
		t.Errorf("body type: got %q", r.Collection.Requests[0].Body.Type)
	}
}

func TestParse_NoBody(t *testing.T) {
	input := `{
		"info": {"name": "NoBody", "schema": ""},
		"item": [{"name": "R", "request": {"method": "GET", "url": "https://e.com/"}}]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Body.Type != domain.BodyNone {
		t.Errorf("body type: got %q, want none", r.Collection.Requests[0].Body.Type)
	}
}

func TestParse_EmptyBodyMode(t *testing.T) {
	input := `{
		"info": {"name": "EmptyMode", "schema": ""},
		"item": [
			{
				"name": "R",
				"request": {
					"method": "GET",
					"url": "https://e.com/",
					"body": {"mode": ""}
				}
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Body.Type != domain.BodyNone {
		t.Errorf("body type: got %q, want none", r.Collection.Requests[0].Body.Type)
	}
}

// ========================
// Parse — variables
// ========================

func TestParse_CollectionVariables(t *testing.T) {
	input := `{
		"info": {"name": "Vars", "schema": ""},
		"item": [],
		"variable": [
			{"key": "base_url", "value": "https://api.example.com"},
			{"key": "api_key", "value": "secret123"}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Vars["base_url"] != "https://api.example.com" {
		t.Errorf("base_url: got %q", r.Collection.Vars["base_url"])
	}
	if r.Collection.Vars["api_key"] != "secret123" {
		t.Errorf("api_key: got %q", r.Collection.Vars["api_key"])
	}
}

func TestParse_NoVariables_VarsNil(t *testing.T) {
	input := `{
		"info": {"name": "NoVars", "schema": ""},
		"item": []
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Vars != nil {
		t.Errorf("expected nil vars, got %v", r.Collection.Vars)
	}
}

func TestParse_EmptyVariablesList_VarsNil(t *testing.T) {
	input := `{
		"info": {"name": "EmptyVars", "schema": ""},
		"item": [],
		"variable": []
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Vars != nil {
		t.Errorf("expected nil vars for empty list, got %v", r.Collection.Vars)
	}
}

func TestParse_ManyVariables(t *testing.T) {
	input := `{
		"info": {"name": "ManyVars", "schema": ""},
		"item": [],
		"variable": [
			{"key": "a", "value": "1"},
			{"key": "b", "value": "2"},
			{"key": "c", "value": "3"},
			{"key": "d", "value": "4"},
			{"key": "e", "value": "5"}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Collection.Vars) != 5 {
		t.Errorf("vars count: got %d, want 5", len(r.Collection.Vars))
	}
}

// ========================
// Parse — folders / nesting
// ========================

func TestParse_NestedFolders_Flattened(t *testing.T) {
	input := `{
		"info": {"name": "Nested", "schema": ""},
		"item": [
			{
				"name": "Auth",
				"item": [
					{
						"name": "Login",
						"request": {
							"method": "POST",
							"url": "https://api.example.com/auth/login"
						}
					},
					{
						"name": "Refresh",
						"request": {
							"method": "POST",
							"url": "https://api.example.com/auth/refresh"
						}
					}
				]
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Collection.Requests) != 2 {
		t.Fatalf("requests: got %d, want 2", len(r.Collection.Requests))
	}
	if r.Collection.Requests[0].Name != "auth.login" {
		t.Errorf("r0 name: got %q, want auth.login", r.Collection.Requests[0].Name)
	}
	if r.Collection.Requests[1].Name != "auth.refresh" {
		t.Errorf("r1 name: got %q, want auth.refresh", r.Collection.Requests[1].Name)
	}

	hasFlattened := false
	for _, w := range r.Warnings {
		if strings.Contains(w, "flattened") {
			hasFlattened = true
		}
	}
	if !hasFlattened {
		t.Error("expected flattening warning")
	}
}

func TestParse_DeepNesting(t *testing.T) {
	input := `{
		"info": {"name": "Deep", "schema": ""},
		"item": [
			{
				"name": "Level1",
				"item": [
					{
						"name": "Level2",
						"item": [
							{
								"name": "Req",
								"request": {
									"method": "GET",
									"url": "https://e.com/"
								}
							}
						]
					}
				]
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Collection.Requests) != 1 {
		t.Fatalf("requests: got %d", len(r.Collection.Requests))
	}
	if r.Collection.Requests[0].Name != "level1.level2.req" {
		t.Errorf("name: got %q, want level1.level2.req", r.Collection.Requests[0].Name)
	}
}

func TestParse_MixedFoldersAndRequests(t *testing.T) {
	input := `{
		"info": {"name": "Mixed", "schema": ""},
		"item": [
			{
				"name": "TopReq",
				"request": {"method": "GET", "url": "https://e.com/top"}
			},
			{
				"name": "Folder",
				"item": [
					{
						"name": "NestedReq",
						"request": {"method": "POST", "url": "https://e.com/nested"}
					}
				]
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Collection.Requests) != 2 {
		t.Fatalf("requests: got %d, want 2", len(r.Collection.Requests))
	}
	if r.Collection.Requests[0].Name != "topreq" {
		t.Errorf("r0: got %q", r.Collection.Requests[0].Name)
	}
	if r.Collection.Requests[1].Name != "folder.nestedreq" {
		t.Errorf("r1: got %q", r.Collection.Requests[1].Name)
	}
}

func TestParse_ItemNoRequestNoItems_Skipped(t *testing.T) {
	input := `{
		"info": {"name": "Skip", "schema": ""},
		"item": [
			{"name": "Empty Item"},
			{"name": "Real", "request": {"method": "GET", "url": "https://e.com/"}}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Collection.Requests) != 1 {
		t.Fatalf("requests: got %d, want 1", len(r.Collection.Requests))
	}
}

// ========================
// Parse — name sanitization
// ========================

func TestParse_NameSpaces_ReplacedWithHyphens(t *testing.T) {
	input := `{
		"info": {"name": "Names", "schema": ""},
		"item": [
			{"name": "Get All Users", "request": {"method": "GET", "url": "https://e.com/"}}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Name != "get-all-users" {
		t.Errorf("name: got %q, want get-all-users", r.Collection.Requests[0].Name)
	}
}

func TestParse_NameUppercase_Lowered(t *testing.T) {
	input := `{
		"info": {"name": "Case", "schema": ""},
		"item": [
			{"name": "MyRequest", "request": {"method": "GET", "url": "https://e.com/"}}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Name != "myrequest" {
		t.Errorf("name: got %q", r.Collection.Requests[0].Name)
	}
}

func TestParse_FolderPrefix_Sanitized(t *testing.T) {
	input := `{
		"info": {"name": "FP", "schema": ""},
		"item": [
			{
				"name": "My Folder",
				"item": [
					{"name": "My Request", "request": {"method": "GET", "url": "https://e.com/"}}
				]
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Name != "my-folder.my-request" {
		t.Errorf("name: got %q", r.Collection.Requests[0].Name)
	}
}

// ========================
// Parse — warnings
// ========================

func TestParse_AuthBlock_Warning(t *testing.T) {
	input := `{
		"info": {"name": "Auth", "schema": ""},
		"item": [
			{
				"name": "Protected",
				"request": {
					"method": "GET",
					"url": "https://api.example.com/secret",
					"auth": {"type": "bearer"}
				}
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	hasAuthWarning := false
	for _, w := range r.Warnings {
		if strings.Contains(w, "auth") && strings.Contains(w, "bearer") {
			hasAuthWarning = true
		}
	}
	if !hasAuthWarning {
		t.Error("expected warning about auth block")
	}
}

func TestParse_AuthTypes(t *testing.T) {
	types := []string{"basic", "bearer", "oauth2", "apikey"}
	for _, at := range types {
		input := `{
			"info": {"name": "AT", "schema": ""},
			"item": [
				{"name": "R", "request": {"method": "GET", "url": "https://e.com/", "auth": {"type": "` + at + `"}}}
			]
		}`
		r, err := Parse(strings.NewReader(input))
		if err != nil {
			t.Fatalf("auth type %s: %v", at, err)
		}
		found := false
		for _, w := range r.Warnings {
			if strings.Contains(w, at) {
				found = true
			}
		}
		if !found {
			t.Errorf("expected warning for auth type %q", at)
		}
	}
}

func TestParse_Scripts_Warning(t *testing.T) {
	input := `{
		"info": {"name": "Scripts", "schema": ""},
		"item": [
			{
				"name": "WithScript",
				"request": {
					"method": "GET",
					"url": "https://api.example.com/"
				},
				"event": [
					{"listen": "prerequest"},
					{"listen": "test"}
				]
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) < 2 {
		t.Errorf("expected at least 2 warnings for scripts, got %d", len(r.Warnings))
	}
	hasPrerequest := false
	hasTest := false
	for _, w := range r.Warnings {
		if strings.Contains(w, "prerequest") {
			hasPrerequest = true
		}
		if strings.Contains(w, "test") {
			hasTest = true
		}
	}
	if !hasPrerequest {
		t.Error("expected prerequest script warning")
	}
	if !hasTest {
		t.Error("expected test script warning")
	}
}

func TestParse_CollectionLevelScript_Warning(t *testing.T) {
	input := `{
		"info": {"name": "WithEvents", "schema": ""},
		"item": [],
		"event": [{"listen": "prerequest"}]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) == 0 {
		t.Error("expected warning for collection-level script")
	}
	if !strings.Contains(r.Warnings[0], "collection-level") {
		t.Errorf("expected collection-level in warning: %q", r.Warnings[0])
	}
}

func TestParse_MultipleCollectionEvents(t *testing.T) {
	input := `{
		"info": {"name": "MultiEv", "schema": ""},
		"item": [],
		"event": [{"listen": "prerequest"}, {"listen": "test"}]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 2 {
		t.Errorf("expected 2 warnings, got %d: %v", len(r.Warnings), r.Warnings)
	}
}

func TestParse_FormDataBody_Warning(t *testing.T) {
	input := `{
		"info": {"name": "Multipart", "schema": ""},
		"item": [
			{
				"name": "Upload",
				"request": {
					"method": "POST",
					"url": "https://api.example.com/upload",
					"body": {
						"mode": "formdata",
						"formdata": [{"key": "file", "value": "test"}]
					}
				}
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	hasWarning := false
	for _, w := range r.Warnings {
		if strings.Contains(w, "multipart") {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Error("expected warning about multipart form-data")
	}
	if r.Collection.Requests[0].Body.Type != domain.BodyNone {
		t.Errorf("expected BodyNone for unsupported formdata, got %q", r.Collection.Requests[0].Body.Type)
	}
}

func TestParse_UnsupportedBodyMode_Warning(t *testing.T) {
	input := `{
		"info": {"name": "GraphQL", "schema": ""},
		"item": [
			{
				"name": "Query",
				"request": {
					"method": "POST",
					"url": "https://e.com/graphql",
					"body": {"mode": "graphql", "raw": "query { users }"}
				}
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	hasWarning := false
	for _, w := range r.Warnings {
		if strings.Contains(w, "graphql") {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Error("expected warning about graphql body mode")
	}
}

func TestParse_DynamicVariables_Warning(t *testing.T) {
	input := `{
		"info": {"name": "Dynamic", "schema": ""},
		"item": [
			{
				"name": "Random",
				"request": {
					"method": "GET",
					"url": "https://api.example.com/{{$randomInt}}"
				}
			}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	hasDynWarning := false
	for _, w := range r.Warnings {
		if strings.Contains(w, "dynamic variable") {
			hasDynWarning = true
		}
	}
	if !hasDynWarning {
		t.Error("expected warning about Postman dynamic variables")
	}
}

func TestParse_NoDynamicVars_NoWarning(t *testing.T) {
	input := `{
		"info": {"name": "Static", "schema": ""},
		"item": [
			{"name": "R", "request": {"method": "GET", "url": "https://e.com/{{user_id}}"}}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range r.Warnings {
		if strings.Contains(w, "dynamic") {
			t.Errorf("unexpected dynamic variable warning for regular var: %q", w)
		}
	}
}

func TestParse_NoWarningsForCleanCollection(t *testing.T) {
	input := `{
		"info": {"name": "Clean", "schema": ""},
		"item": [
			{"name": "R", "request": {"method": "GET", "url": "https://e.com/"}}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d: %v", len(r.Warnings), r.Warnings)
	}
}

// ========================
// Parse — error cases
// ========================

func TestParse_InvalidJSON(t *testing.T) {
	_, err := Parse(strings.NewReader("not json at all"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParse_EmptyReader(t *testing.T) {
	_, err := Parse(strings.NewReader(""))
	if err == nil {
		t.Error("expected error for empty reader")
	}
}

func TestParse_PartialJSON(t *testing.T) {
	_, err := Parse(strings.NewReader(`{"info": {"name": "broken"`))
	if err == nil {
		t.Error("expected error for partial JSON")
	}
}

// ========================
// Parse — round-trip
// ========================

func TestParse_RoundTrip(t *testing.T) {
	input := `{
		"info": {"name": "RoundTrip API", "schema": ""},
		"item": [
			{
				"name": "Health",
				"request": {
					"method": "GET",
					"url": {"raw": "https://api.example.com/health"},
					"header": [{"key": "Accept", "value": "application/json"}]
				}
			}
		],
		"variable": [
			{"key": "env", "value": "dev"}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}

	b, err := yamlcollection.MarshalCollection(r.Collection)
	if err != nil {
		t.Fatalf("MarshalCollection: %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "imported.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	loader := yamlcollection.NewLoader()
	loaded, err := loader.LoadCollection(path)
	if err != nil {
		t.Fatalf("LoadCollection: %v\nYAML:\n%s", err, string(b))
	}
	if loaded.Name != "RoundTrip API" {
		t.Errorf("round-trip name: got %q", loaded.Name)
	}
	if len(loaded.Requests) != 1 {
		t.Fatalf("round-trip requests: got %d", len(loaded.Requests))
	}
	if loaded.Requests[0].Method != domain.MethodGet {
		t.Errorf("round-trip method: got %q", loaded.Requests[0].Method)
	}
	if loaded.Vars["env"] != "dev" {
		t.Errorf("round-trip var: got %q", loaded.Vars["env"])
	}
}

func TestParse_RoundTrip_ComplexCollection(t *testing.T) {
	input := `{
		"info": {"name": "Complex RT", "schema": ""},
		"item": [
			{
				"name": "Get Items",
				"request": {
					"method": "GET",
					"url": "{{base_url}}/items",
					"header": [
						{"key": "Authorization", "value": "Bearer {{token}}"},
						{"key": "Accept", "value": "application/json"}
					]
				}
			},
			{
				"name": "Create Item",
				"request": {
					"method": "POST",
					"url": "{{base_url}}/items",
					"header": [{"key": "Content-Type", "value": "application/json"}],
					"body": {
						"mode": "raw",
						"raw": "{\"name\":\"new\"}",
						"options": {"raw": {"language": "json"}}
					}
				}
			},
			{
				"name": "Login Form",
				"request": {
					"method": "POST",
					"url": "{{base_url}}/login",
					"body": {
						"mode": "urlencoded",
						"urlencoded": [
							{"key": "user", "value": "admin"},
							{"key": "pass", "value": "secret"}
						]
					}
				}
			}
		],
		"variable": [
			{"key": "base_url", "value": "https://api.dev"},
			{"key": "token", "value": "tok123"}
		]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}

	b, err := yamlcollection.MarshalCollection(r.Collection)
	if err != nil {
		t.Fatalf("MarshalCollection: %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "complex.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	loader := yamlcollection.NewLoader()
	loaded, err := loader.LoadCollection(path)
	if err != nil {
		t.Fatalf("LoadCollection: %v\nYAML:\n%s", err, string(b))
	}

	if len(loaded.Requests) != 3 {
		t.Fatalf("requests: got %d", len(loaded.Requests))
	}
	if loaded.Requests[0].Method != domain.MethodGet {
		t.Error("r0 method")
	}
	if loaded.Requests[1].Body.Type != domain.BodyJSON {
		t.Error("r1 body type")
	}
	if loaded.Requests[2].Body.Type != domain.BodyForm {
		t.Error("r2 body type")
	}
	if loaded.Vars["base_url"] != "https://api.dev" {
		t.Error("var base_url")
	}
}

// ========================
// Parse — testdata files
// ========================

func TestParse_TestdataSimple(t *testing.T) {
	f, err := os.Open("testdata/simple.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	r, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Name != "Simple Testdata" {
		t.Errorf("name: got %q", r.Collection.Name)
	}
	if len(r.Collection.Requests) != 1 {
		t.Fatalf("requests: got %d, want 1", len(r.Collection.Requests))
	}
	if r.Collection.Requests[0].Method != domain.MethodGet {
		t.Errorf("method: got %q", r.Collection.Requests[0].Method)
	}
	if r.Collection.Requests[0].Headers["Accept"] != "application/json" {
		t.Errorf("header: got %q", r.Collection.Requests[0].Headers["Accept"])
	}
}

func TestParse_TestdataNested(t *testing.T) {
	f, err := os.Open("testdata/nested.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	r, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Collection.Requests) != 2 {
		t.Fatalf("requests: got %d, want 2", len(r.Collection.Requests))
	}

	for _, req := range r.Collection.Requests {
		if !strings.Contains(req.Name, "users") {
			t.Errorf("expected folder prefix in name, got %q", req.Name)
		}
	}

	// Second request should have JSON body
	if r.Collection.Requests[1].Body.Type != domain.BodyJSON {
		t.Errorf("r1 body type: got %q", r.Collection.Requests[1].Body.Type)
	}
}

func TestParse_TestdataSimple_RoundTrip(t *testing.T) {
	f, err := os.Open("testdata/simple.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	r, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}

	b, err := yamlcollection.MarshalCollection(r.Collection)
	if err != nil {
		t.Fatalf("MarshalCollection: %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "simple.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	loader := yamlcollection.NewLoader()
	loaded, err := loader.LoadCollection(path)
	if err != nil {
		t.Fatalf("LoadCollection: %v", err)
	}
	if loaded.Name != "Simple Testdata" {
		t.Errorf("name: got %q", loaded.Name)
	}
}

func TestParse_TestdataNested_RoundTrip(t *testing.T) {
	f, err := os.Open("testdata/nested.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	r, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}

	b, err := yamlcollection.MarshalCollection(r.Collection)
	if err != nil {
		t.Fatalf("MarshalCollection: %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "nested.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	loader := yamlcollection.NewLoader()
	loaded, err := loader.LoadCollection(path)
	if err != nil {
		t.Fatalf("LoadCollection: %v", err)
	}
	if len(loaded.Requests) != 2 {
		t.Errorf("requests: got %d", len(loaded.Requests))
	}
}

// ========================
// Parse — combined warnings
// ========================

func TestParse_CombinedWarnings(t *testing.T) {
	input := `{
		"info": {"name": "AllWarnings", "schema": ""},
		"item": [
			{
				"name": "Folder",
				"item": [
					{
						"name": "WithAuth",
						"request": {
							"method": "GET",
							"url": "https://e.com/{{$randomInt}}",
							"auth": {"type": "bearer"},
							"body": {"mode": "formdata"}
						},
						"event": [{"listen": "test"}]
					}
				]
			}
		],
		"event": [{"listen": "prerequest"}]
	}`

	r, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}

	// Expected warnings: collection event, folder flattened, request script, auth, formdata, dynamic var
	if len(r.Warnings) < 5 {
		t.Errorf("expected at least 5 warnings, got %d: %v", len(r.Warnings), r.Warnings)
	}
}
