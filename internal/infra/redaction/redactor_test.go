package redaction

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/aalvaropc/lynix/internal/domain"
)

func defaultMasking() domain.MaskingConfig {
	return domain.MaskingConfig{
		Enabled:             true,
		MaskRequestHeaders:  true,
		MaskRequestBody:     true,
		MaskResponseHeaders: true,
		MaskResponseBody:    true,
		MaskQueryParams:     true,
	}
}

func TestRedact_Disabled(t *testing.T) {
	cfg := defaultMasking()
	cfg.Enabled = false
	r := New(cfg)

	run := domain.RunArtifact{
		Results: []domain.RequestResult{{
			RequestHeaders: map[string]string{"Authorization": "Bearer FAKE_TEST_VALUE"},
			Extracted:      domain.Vars{"token": "FAKE_TOK"},
		}},
	}

	out := r.Redact(run)
	if out.Results[0].RequestHeaders["Authorization"] != "Bearer FAKE_TEST_VALUE" {
		t.Error("should not mask when disabled")
	}
	if out.Results[0].Extracted["token"] != "FAKE_TOK" {
		t.Error("should not mask extracted vars when disabled")
	}
}

func TestRedact_RequestHeaders(t *testing.T) {
	r := New(defaultMasking())

	run := domain.RunArtifact{
		Results: []domain.RequestResult{{
			RequestHeaders: map[string]string{
				"Authorization": "Bearer FAKE_TEST_VALUE",
				"X-API-Key":     "FAKE_KEY",
				"Content-Type":  "application/json",
				"X-Custom":      "safe",
			},
		}},
	}

	out := r.Redact(run)
	h := out.Results[0].RequestHeaders

	if h["Authorization"] != maskValue {
		t.Errorf("Authorization should be masked, got %q", h["Authorization"])
	}
	if h["X-API-Key"] != maskValue {
		t.Errorf("X-API-Key should be masked, got %q", h["X-API-Key"])
	}
	if h["Content-Type"] != "application/json" {
		t.Errorf("Content-Type should NOT be masked, got %q", h["Content-Type"])
	}
	if h["X-Custom"] != "safe" {
		t.Errorf("X-Custom should NOT be masked, got %q", h["X-Custom"])
	}
}

func TestRedact_RequestHeaders_Disabled(t *testing.T) {
	cfg := defaultMasking()
	cfg.MaskRequestHeaders = false
	r := New(cfg)

	run := domain.RunArtifact{
		Results: []domain.RequestResult{{
			RequestHeaders: map[string]string{"Authorization": "Bearer FAKE_TEST_VALUE"},
		}},
	}

	out := r.Redact(run)
	if out.Results[0].RequestHeaders["Authorization"] != "Bearer FAKE_TEST_VALUE" {
		t.Error("should not mask request headers when MaskRequestHeaders is false")
	}
}

func TestRedact_ResponseHeaders(t *testing.T) {
	r := New(defaultMasking())

	run := domain.RunArtifact{
		Results: []domain.RequestResult{{
			Response: domain.ResponseSnapshot{
				Headers: map[string][]string{
					"Set-Cookie":   {"session=FAKE_SESSION_ID"},
					"Content-Type": {"application/json"},
				},
			},
		}},
	}

	out := r.Redact(run)
	rh := out.Results[0].Response.Headers

	if rh["Set-Cookie"][0] != maskValue {
		t.Errorf("Set-Cookie should be masked, got %q", rh["Set-Cookie"][0])
	}
	if rh["Content-Type"][0] != "application/json" {
		t.Errorf("Content-Type should NOT be masked, got %q", rh["Content-Type"][0])
	}
}

func TestRedact_ExtractedVars(t *testing.T) {
	r := New(defaultMasking())

	run := domain.RunArtifact{
		Results: []domain.RequestResult{{
			Extracted: domain.Vars{
				"auth_token": "FAKE_TOK",
				"user_id":    "42",
				"password":   "FAKE_PASS",
			},
		}},
	}

	out := r.Redact(run)
	ev := out.Results[0].Extracted

	if ev["auth_token"] != maskValue {
		t.Errorf("auth_token should be masked, got %q", ev["auth_token"])
	}
	if ev["password"] != maskValue {
		t.Errorf("password should be masked, got %q", ev["password"])
	}
	if ev["user_id"] != "42" {
		t.Errorf("user_id should NOT be masked, got %q", ev["user_id"])
	}
}

func TestRedact_QueryParams(t *testing.T) {
	r := New(defaultMasking())

	run := domain.RunArtifact{
		Results: []domain.RequestResult{{
			ResolvedURL: "https://api.example.com/v1?api_key=FAKE_KEY&page=1&token=FAKE_TOK",
		}},
	}

	out := r.Redact(run)
	u := out.Results[0].ResolvedURL

	if strings.Contains(u, "FAKE_KEY") {
		t.Errorf("api_key value should be masked in URL: %s", u)
	}
	if strings.Contains(u, "FAKE_TOK") {
		t.Errorf("token value should be masked in URL: %s", u)
	}
	if !strings.Contains(u, "page=1") {
		t.Errorf("page param should NOT be masked in URL: %s", u)
	}
}

func TestRedact_QueryParams_Disabled(t *testing.T) {
	cfg := defaultMasking()
	cfg.MaskQueryParams = false
	r := New(cfg)

	run := domain.RunArtifact{
		Results: []domain.RequestResult{{
			ResolvedURL: "https://api.example.com?api_key=FAKE_KEY",
		}},
	}

	out := r.Redact(run)
	if !strings.Contains(out.Results[0].ResolvedURL, "FAKE_KEY") {
		t.Error("should not mask query params when MaskQueryParams is false")
	}
}

func TestRedact_RequestBodyJSON(t *testing.T) {
	r := New(defaultMasking())

	body := `{"username":"alice","password":"FAKE_PASS","data":{"api_key":"FAKE_KEY","value":42}}`

	run := domain.RunArtifact{
		Results: []domain.RequestResult{{
			RequestBody: []byte(body),
		}},
	}

	out := r.Redact(run)
	var doc map[string]any
	if err := json.Unmarshal(out.Results[0].RequestBody, &doc); err != nil {
		t.Fatalf("masked body is not valid JSON: %v", err)
	}

	if doc["password"] != maskValue {
		t.Errorf("password should be masked, got %v", doc["password"])
	}
	if doc["username"] != "alice" {
		t.Errorf("username should NOT be masked, got %v", doc["username"])
	}

	nested := doc["data"].(map[string]any)
	if nested["api_key"] != maskValue {
		t.Errorf("nested api_key should be masked, got %v", nested["api_key"])
	}
	if nested["value"] != float64(42) {
		t.Errorf("nested value should NOT be masked, got %v", nested["value"])
	}
}

func TestRedact_ResponseBodyJSON(t *testing.T) {
	r := New(defaultMasking())

	body := `{"access_token":"FAKE_TOK","name":"test"}`

	run := domain.RunArtifact{
		Results: []domain.RequestResult{{
			Response: domain.ResponseSnapshot{
				Headers: map[string][]string{},
				Body:    []byte(body),
			},
		}},
	}

	out := r.Redact(run)
	var doc map[string]any
	if err := json.Unmarshal(out.Results[0].Response.Body, &doc); err != nil {
		t.Fatalf("masked response body is not valid JSON: %v", err)
	}

	if doc["access_token"] != maskValue {
		t.Errorf("access_token should be masked, got %v", doc["access_token"])
	}
	if doc["name"] != "test" {
		t.Errorf("name should NOT be masked, got %v", doc["name"])
	}
}

func TestRedact_NonJSONBody(t *testing.T) {
	r := New(defaultMasking())

	body := "this is plain text, not json"

	run := domain.RunArtifact{
		Results: []domain.RequestResult{{
			RequestBody: []byte(body),
		}},
	}

	out := r.Redact(run)
	// Non-JSON body should be returned as-is.
	if string(out.Results[0].RequestBody) != body {
		t.Errorf("non-JSON body should be returned as-is, got %q", string(out.Results[0].RequestBody))
	}
}

func TestRedact_CustomRules(t *testing.T) {
	cfg := defaultMasking()
	cfg.Rules = []domain.RedactionRule{
		{Pattern: "ssn", Scope: domain.RedactionScopeAll},
		{Pattern: "internal-id", Scope: domain.RedactionScopeHeader},
	}
	r := New(cfg)

	run := domain.RunArtifact{
		Results: []domain.RequestResult{{
			RequestHeaders: map[string]string{
				"X-SSN":         "000-00-0000",
				"X-Internal-Id": "id-42",
				"X-Safe":        "ok",
			},
			Extracted: domain.Vars{
				"user_ssn": "000-00-0000",
				"name":     "alice",
			},
		}},
	}

	out := r.Redact(run)

	if out.Results[0].RequestHeaders["X-SSN"] != maskValue {
		t.Error("custom rule 'ssn' should mask X-SSN header")
	}
	if out.Results[0].RequestHeaders["X-Internal-Id"] != maskValue {
		t.Error("custom rule 'internal_id' should mask X-Internal-Id header")
	}
	if out.Results[0].RequestHeaders["X-Safe"] != "ok" {
		t.Error("X-Safe should not be masked")
	}
	if out.Results[0].Extracted["user_ssn"] != maskValue {
		t.Error("custom rule 'ssn' (scope=all) should mask extracted var user_ssn")
	}
	if out.Results[0].Extracted["name"] != "alice" {
		t.Error("name should not be masked")
	}
}

func TestRedact_DoesNotMutateInput(t *testing.T) {
	r := New(defaultMasking())

	run := domain.RunArtifact{
		Results: []domain.RequestResult{{
			RequestHeaders: map[string]string{"Authorization": "Bearer FAKE_TEST_VALUE"},
			Extracted:      domain.Vars{"token": "FAKE_TOK"},
			Response: domain.ResponseSnapshot{
				Headers: map[string][]string{"Set-Cookie": {"val"}},
				Body:    []byte(`{"password":"FAKE_PASS"}`),
			},
		}},
	}

	_ = r.Redact(run)

	// Original should be unchanged.
	if run.Results[0].RequestHeaders["Authorization"] != "Bearer FAKE_TEST_VALUE" {
		t.Error("input RequestHeaders was mutated")
	}
	if run.Results[0].Extracted["token"] != "FAKE_TOK" {
		t.Error("input Extracted was mutated")
	}
	if run.Results[0].Response.Headers["Set-Cookie"][0] != "val" {
		t.Error("input Response.Headers was mutated")
	}
	if string(run.Results[0].Response.Body) != `{"password":"FAKE_PASS"}` {
		t.Error("input Response.Body was mutated")
	}
}

func TestRedact_EmptyRun(t *testing.T) {
	r := New(defaultMasking())
	out := r.Redact(domain.RunArtifact{})
	if len(out.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(out.Results))
	}
}

func TestRedact_ResponseHeaders_Disabled(t *testing.T) {
	cfg := defaultMasking()
	cfg.MaskResponseHeaders = false
	r := New(cfg)

	run := domain.RunArtifact{
		Results: []domain.RequestResult{{
			Response: domain.ResponseSnapshot{
				Headers: map[string][]string{
					"Set-Cookie": {"session=FAKE_SESSION_ID"},
				},
			},
		}},
	}

	out := r.Redact(run)
	if out.Results[0].Response.Headers["Set-Cookie"][0] != "session=FAKE_SESSION_ID" {
		t.Error("should not mask response headers when MaskResponseHeaders is false")
	}
}

// --- CheckForSecrets tests ---

func TestCheckForSecrets_Clean_NoError(t *testing.T) {
	r := New(defaultMasking())

	run := domain.RunArtifact{
		Results: []domain.RequestResult{{
			Name:           "test",
			RequestHeaders: map[string]string{"Authorization": maskValue, "Content-Type": "application/json"},
			Response: domain.ResponseSnapshot{
				Headers: map[string][]string{"Set-Cookie": {maskValue}},
				Body:    []byte(`{"password":"` + maskValue + `","name":"alice"}`),
			},
			ResolvedURL: "https://api.example.com?api_key=" + maskValue + "&page=1",
			Extracted:   domain.Vars{"token": maskValue, "user_id": "42"},
		}},
	}

	if err := r.CheckForSecrets(run); err != nil {
		t.Errorf("expected no error for clean artifact, got: %v", err)
	}
}

func TestCheckForSecrets_UnmaskedHeader_ReturnsError(t *testing.T) {
	r := New(defaultMasking())

	run := domain.RunArtifact{
		Results: []domain.RequestResult{{
			Name:           "login",
			RequestHeaders: map[string]string{"Authorization": "Bearer real-token"},
		}},
	}

	err := r.CheckForSecrets(run)
	if err == nil {
		t.Fatal("expected error for unmasked request header")
	}
	if !errors.Is(err, ErrSecretDetected) {
		t.Errorf("expected ErrSecretDetected, got: %v", err)
	}
}

func TestCheckForSecrets_UnmaskedResponseHeader_ReturnsError(t *testing.T) {
	r := New(defaultMasking())

	run := domain.RunArtifact{
		Results: []domain.RequestResult{{
			Name: "login",
			Response: domain.ResponseSnapshot{
				Headers: map[string][]string{"Set-Cookie": {"session=real-value"}},
			},
		}},
	}

	err := r.CheckForSecrets(run)
	if err == nil {
		t.Fatal("expected error for unmasked response header")
	}
	if !errors.Is(err, ErrSecretDetected) {
		t.Errorf("expected ErrSecretDetected, got: %v", err)
	}
}

func TestCheckForSecrets_UnmaskedBodyField_ReturnsError(t *testing.T) {
	r := New(defaultMasking())

	run := domain.RunArtifact{
		Results: []domain.RequestResult{{
			Name:        "login",
			RequestBody: []byte(`{"password":"real-password"}`),
		}},
	}

	err := r.CheckForSecrets(run)
	if err == nil {
		t.Fatal("expected error for unmasked body field")
	}
	if !errors.Is(err, ErrSecretDetected) {
		t.Errorf("expected ErrSecretDetected, got: %v", err)
	}
}

func TestCheckForSecrets_UnmaskedQueryParam_ReturnsError(t *testing.T) {
	r := New(defaultMasking())

	run := domain.RunArtifact{
		Results: []domain.RequestResult{{
			Name:        "fetch",
			ResolvedURL: "https://api.example.com?api_key=real-key",
		}},
	}

	err := r.CheckForSecrets(run)
	if err == nil {
		t.Fatal("expected error for unmasked query param")
	}
	if !errors.Is(err, ErrSecretDetected) {
		t.Errorf("expected ErrSecretDetected, got: %v", err)
	}
}

func TestCheckForSecrets_UnmaskedExtractedVar_ReturnsError(t *testing.T) {
	r := New(defaultMasking())

	run := domain.RunArtifact{
		Results: []domain.RequestResult{{
			Name:      "login",
			Extracted: domain.Vars{"token": "real-token-value"},
		}},
	}

	err := r.CheckForSecrets(run)
	if err == nil {
		t.Fatal("expected error for unmasked extracted var")
	}
	if !errors.Is(err, ErrSecretDetected) {
		t.Errorf("expected ErrSecretDetected, got: %v", err)
	}
}

func TestRedact_ErrorMessage_MasksURLQueryParams(t *testing.T) {
	cfg := domain.MaskingConfig{Enabled: true, MaskQueryParams: true}
	r := New(cfg)

	run := domain.RunArtifact{Results: []domain.RequestResult{{
		Name: "broken",
		Error: &domain.RunError{
			Kind:    domain.RunErrorConn,
			Message: `Get "https://api.example.com/x?api_key=RAW_SECRET&page=1": dial tcp: connection refused`,
		},
	}}}

	out := r.Redact(run)
	msg := out.Results[0].Error.Message
	if strings.Contains(msg, "RAW_SECRET") {
		t.Errorf("error message still contains the secret: %s", msg)
	}
	if !strings.Contains(msg, "connection refused") {
		t.Errorf("error message should keep diagnostic text: %s", msg)
	}
}

func TestRedact_FormBody_MasksSensitiveKeys(t *testing.T) {
	cfg := domain.MaskingConfig{Enabled: true, MaskRequestBody: true}
	r := New(cfg)

	run := domain.RunArtifact{Results: []domain.RequestResult{{
		Name:        "login",
		RequestBody: []byte("username=alice&password=hunter2"),
	}}}

	out := r.Redact(run)
	body := string(out.Results[0].RequestBody)
	if strings.Contains(body, "hunter2") {
		t.Errorf("form body still contains the password: %s", body)
	}
	if !strings.Contains(body, "alice") {
		t.Errorf("form body should keep non-sensitive values: %s", body)
	}
}

func TestRedact_KnownSecretValue_ScrubbedUnderInnocuousKey(t *testing.T) {
	cfg := domain.MaskingConfig{Enabled: true}
	r := New(cfg)
	r.AddSecretValues("KNOWN_SECRET_VALUE")

	run := domain.RunArtifact{Results: []domain.RequestResult{{
		Name:      "req",
		Extracted: domain.Vars{"sid": "KNOWN_SECRET_VALUE"},
		Assertions: []domain.AssertionResult{{
			Name:    "jsonpath.eq",
			Passed:  false,
			Message: `jsonpath "$.sid": expected "x", got "KNOWN_SECRET_VALUE"`,
		}},
	}}}

	out := r.Redact(run)
	if strings.Contains(out.Results[0].Extracted["sid"], "KNOWN_SECRET_VALUE") {
		t.Error("extracted var with innocuous name still contains the known secret")
	}
	if strings.Contains(out.Results[0].Assertions[0].Message, "KNOWN_SECRET_VALUE") {
		t.Error("assertion message still contains the known secret")
	}
}

func TestCheckForSecrets_NotCircular_DetectsKnownValue(t *testing.T) {
	// Regression: CheckForSecrets used the same predicates as Redact, so it
	// could never detect what Redact missed. Value-based scanning must flag a
	// known secret under a key no predicate matches.
	cfg := domain.MaskingConfig{Enabled: true}
	r := New(cfg)
	r.AddSecretValues("LEAKED_VALUE_123")

	run := domain.RunArtifact{Results: []domain.RequestResult{{
		Name:      "req",
		Extracted: domain.Vars{"sid": "LEAKED_VALUE_123"},
	}}}

	if err := r.CheckForSecrets(run); err == nil {
		t.Fatal("expected CheckForSecrets to detect the known value under an innocuous key")
	}
}

func TestCheckForSecrets_DetectsCredentialFormats(t *testing.T) {
	cfg := domain.MaskingConfig{Enabled: true}
	r := New(cfg)

	run := domain.RunArtifact{Results: []domain.RequestResult{{
		Name:      "req",
		Extracted: domain.Vars{"x": "ghp_abcdefghijklmnopqrstuv0123456789"},
	}}}

	if err := r.CheckForSecrets(run); err == nil {
		t.Fatal("expected CheckForSecrets to detect a GitHub token format")
	}
}

func TestScrubText_LongestSecretFirst(t *testing.T) {
	cfg := domain.MaskingConfig{Enabled: true}
	r := New(cfg)
	// Registered shortest-first on purpose: masking must not leave the
	// suffix of the longer secret behind.
	r.AddSecretValues("abc123")
	r.AddSecretValues("abc123XYZTOKEN")

	got := r.scrubText("value abc123XYZTOKEN end")
	if strings.Contains(got, "XYZTOKEN") {
		t.Fatalf("longer secret left a residue: %q", got)
	}
}

func TestRedact_FormBody_WithSpacesAndSemicolons(t *testing.T) {
	cfg := domain.MaskingConfig{Enabled: true, MaskRequestBody: true}
	r := New(cfg)

	cases := []string{
		"user=bob&password=my pass",
		"a=1;b=2&password=x",
	}
	for _, body := range cases {
		run := domain.RunArtifact{Results: []domain.RequestResult{{
			Name:        "login",
			RequestBody: []byte(body),
		}}}
		out := string(r.Redact(run).Results[0].RequestBody)
		if strings.Contains(out, "my pass") || strings.Contains(out, "password=x") {
			t.Errorf("form body %q leaked the password: %q", body, out)
		}
	}
}

func TestRedact_FormBody_NonFormTextUntouched(t *testing.T) {
	cfg := domain.MaskingConfig{Enabled: true, MaskRequestBody: true}
	r := New(cfg)

	body := "dGVzdA==" // base64 payload, not a form
	run := domain.RunArtifact{Results: []domain.RequestResult{{
		Name:        "bin",
		RequestBody: []byte(body),
	}}}
	out := string(r.Redact(run).Results[0].RequestBody)
	if out != body {
		t.Fatalf("non-form body was rewritten: %q -> %q", body, out)
	}
}

func TestRedact_JSONBody_EscapedSecretScrubbed(t *testing.T) {
	cfg := domain.MaskingConfig{Enabled: true, MaskResponseBody: true}
	r := New(cfg)
	r.AddSecretValues(`pa"ssw0rd!`)

	run := domain.RunArtifact{Results: []domain.RequestResult{{
		Name: "r",
		Response: domain.ResponseSnapshot{
			Body: []byte(`{"note":"the value pa\"ssw0rd! leaked"}`),
		},
	}}}
	out := string(r.Redact(run).Results[0].Response.Body)
	if strings.Contains(out, "ssw0rd") {
		t.Fatalf("escaped secret leaked: %q", out)
	}
}

func TestRedact_JSONBody_PreservesInt64Precision(t *testing.T) {
	cfg := domain.MaskingConfig{Enabled: true, MaskResponseBody: true}
	r := New(cfg)

	run := domain.RunArtifact{Results: []domain.RequestResult{{
		Name: "r",
		Response: domain.ResponseSnapshot{
			Body: []byte(`{"id":9007199254740993,"token":"x"}`),
		},
	}}}
	out := string(r.Redact(run).Results[0].Response.Body)
	if !strings.Contains(out, "9007199254740993") {
		t.Fatalf("int64 id corrupted by mask round-trip: %q", out)
	}
}

func TestRedact_QueryParam_EncodedSecretScrubbed(t *testing.T) {
	cfg := domain.MaskingConfig{Enabled: true, MaskQueryParams: true}
	r := New(cfg)
	r.AddSecretValues("p@ss w0rd")

	run := domain.RunArtifact{Results: []domain.RequestResult{{
		Name:        "r",
		URL:         "https://api.example.com/x?user_input=p%40ss+w0rd",
		ResolvedURL: "https://api.example.com/x?user_input=p%40ss+w0rd",
	}}}
	out := r.Redact(run).Results[0].ResolvedURL
	if strings.Contains(out, "p%40ss") || strings.Contains(out, "p@ss") {
		t.Fatalf("percent-encoded secret leaked in URL: %q", out)
	}
}

func TestAddSecretValues_IgnoresCorruptingValues(t *testing.T) {
	cfg := domain.MaskingConfig{Enabled: true}
	r := New(cfg)
	r.AddSecretValues("true", "****", "null")

	got := r.scrubText(`{"active": true, "name": "construe"}`)
	if got != `{"active": true, "name": "construe"}` {
		t.Fatalf("common literals must not be scrubbed: %q", got)
	}
	if err := r.CheckForSecrets(domain.RunArtifact{Results: []domain.RequestResult{{
		Name:      "r",
		Extracted: domain.Vars{"masked": "********"},
	}}}); err != nil {
		t.Fatalf("mask placeholder must not trip the check: %v", err)
	}
}
