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
