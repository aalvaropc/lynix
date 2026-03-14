package curlparse

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/infra/yamlcollection"
)

// ========================
// tokenize tests
// ========================

func TestTokenize_EmptyString(t *testing.T) {
	tokens, err := tokenize("")
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(tokens))
	}
}

func TestTokenize_SingleToken(t *testing.T) {
	tokens, err := tokenize("curl")
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 1 || tokens[0] != "curl" {
		t.Errorf("tokens: %v", tokens)
	}
}

func TestTokenize_MultipleTokens(t *testing.T) {
	tokens, err := tokenize("curl -X GET https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 4 {
		t.Fatalf("expected 4 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[0] != "curl" || tokens[1] != "-X" || tokens[2] != "GET" || tokens[3] != "https://example.com" {
		t.Errorf("tokens: %v", tokens)
	}
}

func TestTokenize_SingleQuotes(t *testing.T) {
	tokens, err := tokenize(`curl -d '{"key":"value"}'`)
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[2] != `{"key":"value"}` {
		t.Errorf("single-quoted token: got %q", tokens[2])
	}
}

func TestTokenize_DoubleQuotes(t *testing.T) {
	tokens, err := tokenize(`curl -H "Content-Type: application/json"`)
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d", len(tokens))
	}
	if tokens[2] != "Content-Type: application/json" {
		t.Errorf("double-quoted token: got %q", tokens[2])
	}
}

func TestTokenize_MixedQuotes(t *testing.T) {
	tokens, err := tokenize(`curl -d '{"key":"val"}' -H "Accept: text/html"`)
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 5 {
		t.Fatalf("expected 5 tokens, got %d: %v", len(tokens), tokens)
	}
}

func TestTokenize_BackslashEscapeInDouble(t *testing.T) {
	tokens, err := tokenize(`curl -d "hello\"world"`)
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[2] != `hello"world` {
		t.Errorf("escaped token: got %q", tokens[2])
	}
}

func TestTokenize_BackslashNewlineContinuation(t *testing.T) {
	tokens, err := tokenize("curl \\\nhttps://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[1] != "https://example.com" {
		t.Errorf("continuation token: got %q", tokens[1])
	}
}

func TestTokenize_MultipleSpaces(t *testing.T) {
	tokens, err := tokenize("curl    -X   GET    https://e.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 4 {
		t.Errorf("expected 4 tokens, got %d", len(tokens))
	}
}

func TestTokenize_Tabs(t *testing.T) {
	tokens, err := tokenize("curl\t-X\tGET\thttps://e.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 4 {
		t.Errorf("expected 4 tokens, got %d", len(tokens))
	}
}

func TestTokenize_NewlinesUnquoted(t *testing.T) {
	tokens, err := tokenize("curl\nhttps://e.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 2 {
		t.Errorf("expected 2 tokens, got %d", len(tokens))
	}
}

func TestTokenize_UnterminatedSingleQuote(t *testing.T) {
	_, err := tokenize("curl -d 'unterminated")
	if err == nil {
		t.Error("expected error for unterminated single quote")
	}
}

func TestTokenize_UnterminatedDoubleQuote(t *testing.T) {
	_, err := tokenize(`curl -H "unterminated`)
	if err == nil {
		t.Error("expected error for unterminated double quote")
	}
}

func TestTokenize_EmptyQuotedStrings(t *testing.T) {
	tokens, err := tokenize(`curl -d '' -H ""`)
	if err != nil {
		t.Fatal(err)
	}
	// -d, (empty string), -H, (empty string) — but empty strings produce 0-length tokens
	// that aren't appended. Actually let me check: single quotes toggle inSingle, but current.Len() == 0.
	// Empty quotes produce empty tokens that never get appended (no content written).
	// So we get: curl, -d, -H
	if len(tokens) != 3 {
		t.Errorf("expected 3 tokens (empty quoted strings skipped), got %d: %v", len(tokens), tokens)
	}
}

func TestTokenize_CarriageReturn(t *testing.T) {
	tokens, err := tokenize("curl\r\nhttps://e.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 2 {
		t.Errorf("expected 2 tokens, got %d", len(tokens))
	}
}

func TestTokenize_BackslashInSingleQuotes_Literal(t *testing.T) {
	// In single quotes, backslash is literal (not an escape)
	tokens, err := tokenize(`curl -d 'hello\nworld'`)
	if err != nil {
		t.Fatal(err)
	}
	if tokens[2] != `hello\nworld` {
		t.Errorf("expected literal backslash in single quotes: got %q", tokens[2])
	}
}

func TestTokenize_TrailingBackslash(t *testing.T) {
	// Trailing backslash with no following char — escaped flag stays true, but no unterminated quote
	tokens, err := tokenize(`curl \`)
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 1 {
		t.Errorf("expected 1 token, got %d: %v", len(tokens), tokens)
	}
}

func TestTokenize_DoubleQuotesInsideSingle(t *testing.T) {
	tokens, err := tokenize(`curl -H '"key": "val"'`)
	if err != nil {
		t.Fatal(err)
	}
	if tokens[2] != `"key": "val"` {
		t.Errorf("got %q", tokens[2])
	}
}

func TestTokenize_SingleQuotesInsideDouble(t *testing.T) {
	tokens, err := tokenize(`curl -H "it's fine"`)
	if err != nil {
		t.Fatal(err)
	}
	if tokens[2] != "it's fine" {
		t.Errorf("got %q", tokens[2])
	}
}

func TestTokenize_AdjacentQuotedStrings(t *testing.T) {
	// "hello"'world' → helloworld (adjacent quoted strings concatenate)
	tokens, err := tokenize(`curl "hello"'world'`)
	if err != nil {
		t.Fatal(err)
	}
	if tokens[1] != "helloworld" {
		t.Errorf("adjacent quoted: got %q", tokens[1])
	}
}

// ========================
// parseHeader tests
// ========================

func TestParseHeader_Valid(t *testing.T) {
	k, v, ok := parseHeader("Content-Type: application/json")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if k != "Content-Type" {
		t.Errorf("key: %q", k)
	}
	if v != "application/json" {
		t.Errorf("value: %q", v)
	}
}

func TestParseHeader_NoColon(t *testing.T) {
	_, _, ok := parseHeader("InvalidHeader")
	if ok {
		t.Error("expected ok=false for header without colon")
	}
}

func TestParseHeader_EmptyValue(t *testing.T) {
	k, v, ok := parseHeader("X-Empty:")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if k != "X-Empty" {
		t.Errorf("key: %q", k)
	}
	if v != "" {
		t.Errorf("value: got %q, want empty", v)
	}
}

func TestParseHeader_MultipleColons(t *testing.T) {
	k, v, ok := parseHeader("X-Time: 12:34:56")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if k != "X-Time" {
		t.Errorf("key: %q", k)
	}
	if v != "12:34:56" {
		t.Errorf("value: %q", v)
	}
}

func TestParseHeader_WhitespaceAroundColon(t *testing.T) {
	k, v, ok := parseHeader("  Key  :  Value  ")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if k != "Key" {
		t.Errorf("key: %q", k)
	}
	if v != "Value" {
		t.Errorf("value: %q", v)
	}
}

// ========================
// deriveRequestName tests
// ========================

func TestDeriveRequestName_EmptyPath(t *testing.T) {
	name := deriveRequestName("GET", "")
	if name != "get" {
		t.Errorf("expected %q, got %q", "get", name)
	}
}

func TestDeriveRequestName_RootPath(t *testing.T) {
	name := deriveRequestName("GET", "/")
	if name != "get" {
		t.Errorf("expected %q, got %q", "get", name)
	}
}

func TestDeriveRequestName_SimplePath(t *testing.T) {
	name := deriveRequestName("POST", "/v1/users")
	if name != "post-v1-users" {
		t.Errorf("expected %q, got %q", "post-v1-users", name)
	}
}

func TestDeriveRequestName_LongPath_Truncated(t *testing.T) {
	longPath := "/" + strings.Repeat("a/", 50)
	name := deriveRequestName("GET", longPath)
	// method- prefix + slug truncated to 60
	if len(name) > 64 { // "get-" + 60
		t.Errorf("expected truncation, got len=%d: %q", len(name), name)
	}
}

func TestDeriveRequestName_Uppercase_Lowered(t *testing.T) {
	name := deriveRequestName("DELETE", "/API/V2/Resource")
	if name != "delete-api-v2-resource" {
		t.Errorf("got %q", name)
	}
}

// ========================
// Parse — success cases
// ========================

func TestParse_SimpleGET(t *testing.T) {
	r, err := Parse(`curl https://api.example.com/users`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Collection.Requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(r.Collection.Requests))
	}
	req := r.Collection.Requests[0]
	if req.Method != domain.MethodGet {
		t.Errorf("method: got %q, want GET", req.Method)
	}
	if !strings.Contains(req.URL, "{{base_url}}") {
		t.Errorf("URL should use base_url variable: %q", req.URL)
	}
	if r.Collection.Vars["base_url"] != "https://api.example.com" {
		t.Errorf("base_url: got %q", r.Collection.Vars["base_url"])
	}
	if req.Name != "get-users" {
		t.Errorf("name: got %q, want get-users", req.Name)
	}
}

func TestParse_POSTWithJSON(t *testing.T) {
	input := `curl -X POST -H "Content-Type: application/json" -d '{"name":"test"}' https://api.example.com/users`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	req := r.Collection.Requests[0]
	if req.Method != domain.MethodPost {
		t.Errorf("method: got %q, want POST", req.Method)
	}
	if req.Body.Type != domain.BodyJSON {
		t.Errorf("body type: got %q, want json", req.Body.Type)
	}
	if req.Body.JSON["name"] != "test" {
		t.Errorf("body json[name]: got %v", req.Body.JSON["name"])
	}
	if req.Headers["Content-Type"] != "application/json" {
		t.Errorf("content-type: got %q", req.Headers["Content-Type"])
	}
}

func TestParse_JSONFlag(t *testing.T) {
	input := `curl --json '{"key":"val"}' https://api.example.com/data`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	req := r.Collection.Requests[0]
	if req.Method != domain.MethodPost {
		t.Errorf("method: got %q, want POST (inferred from --json)", req.Method)
	}
	if req.Body.Type != domain.BodyJSON {
		t.Errorf("body type: got %q, want json", req.Body.Type)
	}
	if req.Headers["Content-Type"] != "application/json" {
		t.Errorf("content-type: got %q", req.Headers["Content-Type"])
	}
	if req.Headers["Accept"] != "application/json" {
		t.Errorf("accept: got %q", req.Headers["Accept"])
	}
}

func TestParse_JSONFlag_DoesNotOverrideExistingHeaders(t *testing.T) {
	input := `curl --json '{"k":"v"}' -H "Content-Type: text/plain" -H "Accept: text/html" https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	req := r.Collection.Requests[0]
	if req.Headers["Content-Type"] != "text/plain" {
		t.Errorf("expected existing Content-Type preserved, got %q", req.Headers["Content-Type"])
	}
	if req.Headers["Accept"] != "text/html" {
		t.Errorf("expected existing Accept preserved, got %q", req.Headers["Accept"])
	}
}

func TestParse_JSONFlag_WithExplicitMethod(t *testing.T) {
	input := `curl --json '{"k":"v"}' -X PUT https://api.example.com/items/1`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Method != domain.MethodPut {
		t.Errorf("expected PUT, got %q", r.Collection.Requests[0].Method)
	}
}

func TestParse_BasicAuth(t *testing.T) {
	input := `curl -u user:pass https://api.example.com/secret`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	req := r.Collection.Requests[0]
	auth := req.Headers["Authorization"]
	if !strings.HasPrefix(auth, "Basic ") {
		t.Errorf("expected Basic auth header, got %q", auth)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if string(decoded) != "user:pass" {
		t.Errorf("decoded auth: got %q, want %q", string(decoded), "user:pass")
	}
}

func TestParse_BasicAuth_LongForm(t *testing.T) {
	input := `curl --user admin:secret123 https://api.example.com/admin`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	auth := r.Collection.Requests[0].Headers["Authorization"]
	if !strings.HasPrefix(auth, "Basic ") {
		t.Errorf("expected Basic auth, got %q", auth)
	}
}

func TestParse_MultipleHeaders(t *testing.T) {
	input := `curl -H "Accept: application/json" -H "X-Custom: foo" https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	req := r.Collection.Requests[0]
	if req.Headers["Accept"] != "application/json" {
		t.Errorf("Accept: got %q", req.Headers["Accept"])
	}
	if req.Headers["X-Custom"] != "foo" {
		t.Errorf("X-Custom: got %q", req.Headers["X-Custom"])
	}
}

func TestParse_HeaderLongForm(t *testing.T) {
	input := `curl --header "X-Test: value" https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Headers["X-Test"] != "value" {
		t.Errorf("header: got %q", r.Collection.Requests[0].Headers["X-Test"])
	}
}

func TestParse_URLWithQueryParams(t *testing.T) {
	input := `curl "https://api.example.com/search?q=test&page=1"`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	req := r.Collection.Requests[0]
	if !strings.Contains(req.URL, "q=test&page=1") {
		t.Errorf("expected query params in URL: %q", req.URL)
	}
}

func TestParse_URLWithPort(t *testing.T) {
	input := `curl https://api.example.com:8443/health`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Vars["base_url"] != "https://api.example.com:8443" {
		t.Errorf("base_url: got %q", r.Collection.Vars["base_url"])
	}
}

func TestParse_URLRootPath(t *testing.T) {
	input := `curl https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].URL != "{{base_url}}/" {
		t.Errorf("url: got %q", r.Collection.Requests[0].URL)
	}
}

func TestParse_URLWithFragment(t *testing.T) {
	input := `curl "https://api.example.com/docs#section"`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	// Fragment is typically stripped by URL parser in RequestURI, but let's verify it doesn't error
	if r.Collection.Requests[0].Method != domain.MethodGet {
		t.Errorf("method: got %q", r.Collection.Requests[0].Method)
	}
}

func TestParse_MethodInferenceFromData(t *testing.T) {
	input := `curl -d '{"x":1}' https://api.example.com/data`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Method != domain.MethodPost {
		t.Errorf("method: got %q, want POST (inferred from -d)", r.Collection.Requests[0].Method)
	}
}

func TestParse_ExplicitGET(t *testing.T) {
	input := `curl -X GET https://api.example.com/items`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Method != domain.MethodGet {
		t.Errorf("method: got %q, want GET", r.Collection.Requests[0].Method)
	}
}

func TestParse_RequestLongForm(t *testing.T) {
	input := `curl --request DELETE https://api.example.com/items/1`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Method != domain.MethodDelete {
		t.Errorf("method: got %q, want DELETE", r.Collection.Requests[0].Method)
	}
}

func TestParse_PUTMethod(t *testing.T) {
	input := `curl -X PUT -d '{"name":"updated"}' https://api.example.com/items/1`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Method != domain.MethodPut {
		t.Errorf("method: got %q", r.Collection.Requests[0].Method)
	}
}

func TestParse_PATCHMethod(t *testing.T) {
	input := `curl -X PATCH -d '{"status":"active"}' https://api.example.com/items/1`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Method != domain.MethodPatch {
		t.Errorf("method: got %q", r.Collection.Requests[0].Method)
	}
}

func TestParse_HEADMethod(t *testing.T) {
	input := `curl -X HEAD https://api.example.com/health`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Method != domain.MethodHead {
		t.Errorf("method: got %q", r.Collection.Requests[0].Method)
	}
}

func TestParse_OPTIONSMethod(t *testing.T) {
	input := `curl -X OPTIONS https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Method != domain.MethodOptions {
		t.Errorf("method: got %q", r.Collection.Requests[0].Method)
	}
}

func TestParse_LowercaseMethod(t *testing.T) {
	input := `curl -X post https://api.example.com/data`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Method != domain.MethodPost {
		t.Errorf("method: got %q, want POST", r.Collection.Requests[0].Method)
	}
}

func TestParse_QuotedArguments(t *testing.T) {
	input := `curl -H "X-Msg: hello world" https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Headers["X-Msg"] != "hello world" {
		t.Errorf("header value: got %q", r.Collection.Requests[0].Headers["X-Msg"])
	}
}

func TestParse_MultilineBackslash(t *testing.T) {
	input := "curl \\\n  -X POST \\\n  -H \"Content-Type: application/json\" \\\n  -d '{\"a\":1}' \\\n  https://api.example.com/data"
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	req := r.Collection.Requests[0]
	if req.Method != domain.MethodPost {
		t.Errorf("method: got %q, want POST", req.Method)
	}
	if req.Body.Type != domain.BodyJSON {
		t.Errorf("body type: got %q, want json", req.Body.Type)
	}
}

func TestParse_RawBody(t *testing.T) {
	input := `curl -X POST -d 'key=value&foo=bar' https://api.example.com/form`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	req := r.Collection.Requests[0]
	if req.Body.Type != domain.BodyRaw {
		t.Errorf("body type: got %q, want raw (non-JSON body)", req.Body.Type)
	}
	if req.Body.Raw != "key=value&foo=bar" {
		t.Errorf("raw body: got %q", req.Body.Raw)
	}
}

func TestParse_DataRawFlag(t *testing.T) {
	input := `curl --data-raw '{"id":42}' https://api.example.com/items`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Body.Type != domain.BodyJSON {
		t.Errorf("body type: got %q, want json", r.Collection.Requests[0].Body.Type)
	}
}

func TestParse_DataBinaryFlag(t *testing.T) {
	input := `curl --data-binary '{"bin":true}' https://api.example.com/upload`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Body.Type != domain.BodyJSON {
		t.Errorf("body type: got %q, want json", r.Collection.Requests[0].Body.Type)
	}
}

func TestParse_DataLongForm(t *testing.T) {
	input := `curl --data '{"long":"form"}' https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Body.Type != domain.BodyJSON {
		t.Errorf("body type: got %q", r.Collection.Requests[0].Body.Type)
	}
}

func TestParse_JSONBodyWithNestedObjects(t *testing.T) {
	input := `curl -d '{"user":{"name":"alice","roles":["admin"]}}' https://api.example.com/users`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	req := r.Collection.Requests[0]
	if req.Body.Type != domain.BodyJSON {
		t.Errorf("body type: got %q", req.Body.Type)
	}
	userMap, ok := req.Body.JSON["user"].(map[string]any)
	if !ok {
		t.Fatal("expected nested user object")
	}
	if userMap["name"] != "alice" {
		t.Errorf("user.name: got %v", userMap["name"])
	}
}

func TestParse_JSONArrayBody_TreatedAsRaw(t *testing.T) {
	// Top-level JSON array can't be unmarshaled to map[string]any → fallback to raw
	input := `curl -d '[1,2,3]' https://api.example.com/data`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Body.Type != domain.BodyRaw {
		t.Errorf("expected raw body for JSON array, got %q", r.Collection.Requests[0].Body.Type)
	}
}

func TestParse_NoBody_BodyTypeNone(t *testing.T) {
	input := `curl https://api.example.com/health`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Body.Type != domain.BodyNone {
		t.Errorf("body type: got %q, want none", r.Collection.Requests[0].Body.Type)
	}
}

func TestParse_PathCurl(t *testing.T) {
	input := `/usr/bin/curl https://api.example.com/health`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Method != domain.MethodGet {
		t.Errorf("method: got %q", r.Collection.Requests[0].Method)
	}
}

func TestParse_CollectionName_FromHost(t *testing.T) {
	r, err := Parse(`curl https://myapi.dev/health`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Name != "Imported from myapi.dev" {
		t.Errorf("collection name: got %q", r.Collection.Name)
	}
}

func TestParse_CollectionSchemaVersion(t *testing.T) {
	r, err := Parse(`curl https://api.example.com/health`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.SchemaVersion != 1 {
		t.Errorf("schema version: got %d, want 1", r.Collection.SchemaVersion)
	}
}

func TestParse_BaseURLExtraction(t *testing.T) {
	r, err := Parse(`curl https://api.example.com/v1/users?page=1`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Vars["base_url"] != "https://api.example.com" {
		t.Errorf("base_url: got %q", r.Collection.Vars["base_url"])
	}
	if r.Collection.Requests[0].URL != "{{base_url}}/v1/users?page=1" {
		t.Errorf("url: got %q", r.Collection.Requests[0].URL)
	}
}

func TestParse_NoWarningsForCleanCommand(t *testing.T) {
	input := `curl -X GET -H "Accept: application/json" https://api.example.com/users`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d: %v", len(r.Warnings), r.Warnings)
	}
}

func TestParse_NoCurlPrefix(t *testing.T) {
	// Input without "curl" prefix — bare URL
	input := `https://api.example.com/health`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Method != domain.MethodGet {
		t.Errorf("method: got %q", r.Collection.Requests[0].Method)
	}
}

// ========================
// Parse — warning cases
// ========================

func TestParse_UnsupportedFlags_ProduceWarnings(t *testing.T) {
	input := `curl --compressed -k -L -v -s https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 5 {
		t.Errorf("expected 5 warnings, got %d: %v", len(r.Warnings), r.Warnings)
	}
}

func TestParse_UnsupportedInsecureLongForm(t *testing.T) {
	input := `curl --insecure https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(r.Warnings))
	}
}

func TestParse_UnsupportedLocationLongForm(t *testing.T) {
	input := `curl --location https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(r.Warnings))
	}
}

func TestParse_UnsupportedVerboseLongForm(t *testing.T) {
	input := `curl --verbose https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(r.Warnings))
	}
}

func TestParse_UnsupportedSilentLongForm(t *testing.T) {
	input := `curl --silent https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(r.Warnings))
	}
}

func TestParse_UnsupportedShowError(t *testing.T) {
	input := `curl -S https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(r.Warnings))
	}
}

func TestParse_UnsupportedInclude(t *testing.T) {
	input := `curl -i https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(r.Warnings))
	}
}

func TestParse_UnsupportedGloboff(t *testing.T) {
	input := `curl -g https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(r.Warnings))
	}
}

func TestParse_UnsupportedCert(t *testing.T) {
	input := `curl --cert /path/to/cert https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(r.Warnings))
	}
}

func TestParse_UnsupportedKey(t *testing.T) {
	input := `curl --key /path/to/key https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(r.Warnings))
	}
}

func TestParse_UnsupportedOutputShort(t *testing.T) {
	input := `curl -o /tmp/out.txt https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(r.Warnings))
	}
}

func TestParse_UnsupportedOutputLong(t *testing.T) {
	input := `curl --output /tmp/out.txt https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(r.Warnings))
	}
}

func TestParse_FileReference_Warning(t *testing.T) {
	input := `curl -d @data.json https://api.example.com/upload`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) == 0 {
		t.Error("expected warning for file reference @data.json")
	}
	if r.Collection.Requests[0].Body.Type != domain.BodyNone {
		t.Errorf("body should be none after file ref, got %q", r.Collection.Requests[0].Body.Type)
	}
}

func TestParse_MultipartForm_Warning(t *testing.T) {
	input := `curl -F "file=@photo.jpg" https://api.example.com/upload`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) == 0 {
		t.Error("expected warning for multipart form")
	}
	found := false
	for _, w := range r.Warnings {
		if strings.Contains(w, "multipart") {
			found = true
		}
	}
	if !found {
		t.Error("expected warning to mention multipart")
	}
}

func TestParse_MultipartFormLongForm_Warning(t *testing.T) {
	input := `curl --form "name=value" https://api.example.com/upload`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) == 0 {
		t.Error("expected warning for multipart form --form")
	}
}

func TestParse_UnknownFlag_Warning(t *testing.T) {
	input := `curl --unknown-flag https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 1 {
		t.Errorf("expected 1 warning for unknown flag, got %d", len(r.Warnings))
	}
	if !strings.Contains(r.Warnings[0], "unknown") {
		t.Errorf("expected 'unknown' in warning: %q", r.Warnings[0])
	}
}

func TestParse_MultipleUnsupportedAndUnknown(t *testing.T) {
	input := `curl --compressed --insecure --unknown -v https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) != 4 {
		t.Errorf("expected 4 warnings, got %d: %v", len(r.Warnings), r.Warnings)
	}
}

// ========================
// Parse — error cases
// ========================

func TestParse_EmptyInput(t *testing.T) {
	_, err := Parse("")
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestParse_WhitespaceOnly(t *testing.T) {
	_, err := Parse("   \t  \n  ")
	if err == nil {
		t.Error("expected error for whitespace-only input")
	}
}

func TestParse_OnlyCurl(t *testing.T) {
	_, err := Parse("curl")
	if err == nil {
		t.Error("expected error for 'curl' with no args")
	}
}

func TestParse_NoURL(t *testing.T) {
	_, err := Parse("curl -X GET")
	if err == nil {
		t.Error("expected error when no URL is provided")
	}
}

func TestParse_NoURL_OnlyFlags(t *testing.T) {
	_, err := Parse("curl -X POST -H \"Content-Type: text/plain\"")
	if err == nil {
		t.Error("expected error with only flags, no URL")
	}
}

func TestParse_XMissingValue(t *testing.T) {
	_, err := Parse("curl -X")
	if err == nil {
		t.Error("expected error for -X without value")
	}
}

func TestParse_HMissingValue(t *testing.T) {
	_, err := Parse("curl https://e.com -H")
	if err == nil {
		t.Error("expected error for -H without value")
	}
}

func TestParse_DMissingValue(t *testing.T) {
	_, err := Parse("curl https://e.com -d")
	if err == nil {
		t.Error("expected error for -d without value")
	}
}

func TestParse_JsonFlagMissingValue(t *testing.T) {
	_, err := Parse("curl https://e.com --json")
	if err == nil {
		t.Error("expected error for --json without value")
	}
}

func TestParse_UMissingValue(t *testing.T) {
	_, err := Parse("curl https://e.com -u")
	if err == nil {
		t.Error("expected error for -u without value")
	}
}

func TestParse_FMissingValue(t *testing.T) {
	_, err := Parse("curl https://e.com -F")
	if err == nil {
		t.Error("expected error for -F without value")
	}
}

func TestParse_UnterminatedQuote(t *testing.T) {
	_, err := Parse(`curl -H "unterminated https://e.com`)
	if err == nil {
		t.Error("expected error for unterminated quote")
	}
}

// ========================
// Parse — round-trip
// ========================

func TestParse_RoundTrip(t *testing.T) {
	input := `curl -X POST -H "Content-Type: application/json" -d '{"email":"a@b.com"}' https://api.example.com/v1/register`
	r, err := Parse(input)
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
	if loaded.Name != r.Collection.Name {
		t.Errorf("round-trip name: got %q, want %q", loaded.Name, r.Collection.Name)
	}
	if len(loaded.Requests) != 1 {
		t.Fatalf("round-trip request count: got %d", len(loaded.Requests))
	}
	if loaded.Requests[0].Method != domain.MethodPost {
		t.Errorf("round-trip method: got %q", loaded.Requests[0].Method)
	}
}

func TestParse_RoundTrip_GET(t *testing.T) {
	input := `curl -H "Authorization: Bearer tok" https://api.example.com/items`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}

	b, err := yamlcollection.MarshalCollection(r.Collection)
	if err != nil {
		t.Fatalf("MarshalCollection: %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "get.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	loader := yamlcollection.NewLoader()
	loaded, err := loader.LoadCollection(path)
	if err != nil {
		t.Fatalf("LoadCollection: %v", err)
	}
	if loaded.Requests[0].Method != domain.MethodGet {
		t.Errorf("method: got %q", loaded.Requests[0].Method)
	}
	if loaded.Requests[0].Headers["Authorization"] != "Bearer tok" {
		t.Errorf("header: got %q", loaded.Requests[0].Headers["Authorization"])
	}
}

func TestParse_RoundTrip_RawBody(t *testing.T) {
	input := `curl -X POST -d 'plain-text-body' https://api.example.com/raw`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}

	b, err := yamlcollection.MarshalCollection(r.Collection)
	if err != nil {
		t.Fatalf("MarshalCollection: %v", err)
	}

	tmp := t.TempDir()
	path := filepath.Join(tmp, "raw.yaml")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	loader := yamlcollection.NewLoader()
	loaded, err := loader.LoadCollection(path)
	if err != nil {
		t.Fatalf("LoadCollection: %v", err)
	}
	if loaded.Requests[0].Body.Type != domain.BodyRaw {
		t.Errorf("body type: got %q", loaded.Requests[0].Body.Type)
	}
	if loaded.Requests[0].Body.Raw != "plain-text-body" {
		t.Errorf("raw body: got %q", loaded.Requests[0].Body.Raw)
	}
}

// ========================
// Parse — edge cases
// ========================

func TestParse_HeaderWithoutColon_Ignored(t *testing.T) {
	input := `curl -H "InvalidHeader" https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Collection.Requests[0].Headers) != 0 {
		t.Errorf("expected no headers for invalid header value, got %v", r.Collection.Requests[0].Headers)
	}
}

func TestParse_MultipleDataFlags_LastWins(t *testing.T) {
	input := `curl -d '{"first":1}' -d '{"second":2}' https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	req := r.Collection.Requests[0]
	if req.Body.Type != domain.BodyJSON {
		t.Fatalf("body type: got %q", req.Body.Type)
	}
	if _, ok := req.Body.JSON["second"]; !ok {
		t.Error("expected last -d value to win")
	}
}

func TestParse_DataWithExplicitGET(t *testing.T) {
	// -d with explicit -X GET should use GET, not infer POST
	input := `curl -X GET -d '{"q":"search"}' https://api.example.com/search`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Method != domain.MethodGet {
		t.Errorf("method: got %q, want GET (explicit)", r.Collection.Requests[0].Method)
	}
	if r.Collection.Requests[0].Body.Type == domain.BodyNone {
		t.Error("expected body even with GET")
	}
}

func TestParse_HTTPSWithPathAndQuery(t *testing.T) {
	input := `curl "https://api.example.com:443/v2/items?status=active&limit=10"`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Vars["base_url"] != "https://api.example.com:443" {
		t.Errorf("base_url: got %q", r.Collection.Vars["base_url"])
	}
	if !strings.Contains(r.Collection.Requests[0].URL, "status=active&limit=10") {
		t.Errorf("url: got %q", r.Collection.Requests[0].URL)
	}
}

func TestParse_RequestName_RootPath(t *testing.T) {
	input := `curl https://api.example.com/`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Name != "get" {
		t.Errorf("name: got %q, want %q", r.Collection.Requests[0].Name, "get")
	}
}

func TestParse_RequestName_DeepPath(t *testing.T) {
	input := `curl -X DELETE https://api.example.com/v1/orgs/123/members/456`
	r, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if r.Collection.Requests[0].Name != "delete-v1-orgs-123-members-456" {
		t.Errorf("name: got %q", r.Collection.Requests[0].Name)
	}
}
