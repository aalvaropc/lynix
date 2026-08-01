package redaction_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/infra/httpclient"
	"github.com/aalvaropc/lynix/internal/infra/httprunner"
	"github.com/aalvaropc/lynix/internal/infra/redaction"
	"github.com/aalvaropc/lynix/internal/infra/runstore"
	"github.com/aalvaropc/lynix/internal/ports"
	"github.com/aalvaropc/lynix/internal/usecase"
)

type stubCollectionLoader struct{ col domain.Collection }

func (s stubCollectionLoader) LoadCollection(string) (domain.Collection, error) { return s.col, nil }
func (s stubCollectionLoader) ListCollections(string) ([]domain.CollectionRef, error) {
	return nil, nil
}

type stubEnvLoader struct{ env domain.Environment }

func (s stubEnvLoader) LoadEnvironment(string) (domain.Environment, error) { return s.env, nil }

var _ ports.CollectionLoader = stubCollectionLoader{}
var _ ports.EnvironmentLoader = stubEnvLoader{}

// TestIntegration_RealRunner_ArtifactOnDisk_NoSecrets executes requests through
// the real httprunner against a live test server, persists the artifact via the
// real JSONStore, and asserts no secret reaches the disk on ANY surface:
// URL field, resolved URL, form body, response body, assertion messages, and
// network-error messages (which embed the full URL).
func TestIntegration_RealRunner_ArtifactOnDisk_NoSecrets(t *testing.T) {
	tmp := t.TempDir()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Set-Cookie", "session=SECRET_SESSION_ID")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"SECRET_ACCESS_TOKEN","user":"alice"}`))
	}))
	defer srv.Close()

	eqWrong := "WRONG_ON_PURPOSE"
	col := domain.Collection{
		Name: "secrets-e2e",
		Requests: []domain.RequestSpec{
			{
				Name:    "login",
				Method:  domain.MethodPost,
				URL:     srv.URL + "/auth/login?api_key={{api_key}}",
				Headers: domain.Headers{"Authorization": "Bearer {{bearer}}"},
				Body: domain.BodySpec{
					Type: domain.BodyForm,
					Form: map[string]string{"username": "alice", "password": "{{password}}"},
				},
				Assert: domain.AssertionsSpec{
					// Failing eq: its message embeds the observed secret value.
					JSONPath: map[string]domain.ValueAssertion{
						"$.access_token": {Eq: &eqWrong},
					},
				},
				Extract: domain.ExtractSpec{"sid": "$.access_token"},
			},
			{
				// Unreachable port: the *url.Error message embeds the full URL.
				Name:   "broken",
				Method: domain.MethodGet,
				URL:    "http://127.0.0.1:1/x?api_key={{api_key}}",
			},
		},
	}

	env := domain.Environment{
		Name: "dev",
		Vars: domain.Vars{
			"api_key":  "SECRET_API_KEY",
			"bearer":   "SECRET_BEARER_TOKEN",
			"password": "SECRET_FORM_PASS",
		},
		// Simulates secrets.local.yaml: every value is a literal scrub target.
		SecretValues: []string{"SECRET_API_KEY", "SECRET_BEARER_TOKEN", "SECRET_FORM_PASS"},
	}

	cfg := domain.DefaultConfig()
	cfg.Masking.Enabled = true
	cfg.Masking.MaskRequestHeaders = true
	cfg.Masking.MaskRequestBody = true
	cfg.Masking.MaskResponseHeaders = true
	cfg.Masking.MaskResponseBody = true
	cfg.Masking.MaskQueryParams = true
	cfg.Masking.FailOnDetectedSecret = true

	redactor := redaction.New(cfg.Masking)
	redactor.AddSecretsFromEnv(env)
	// The extracted response token is only known at runtime; register it the
	// way the response-body key masking would catch it, plus by value.
	redactor.AddSecretValues("SECRET_ACCESS_TOKEN", "SECRET_SESSION_ID")

	store := runstore.NewJSONStore(tmp, cfg,
		runstore.WithRedacter(redactor),
		runstore.WithNow(func() time.Time {
			return time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
		}),
	)

	runner := httprunner.New(httpclient.New(httpclient.DefaultConfig()))
	uc := usecase.NewRunCollection(
		stubCollectionLoader{col: col},
		stubEnvLoader{env: env},
		runner,
		store,
		usecase.RunOpts{},
	)

	_, id, err := uc.Execute(context.Background(), "col.yaml", "dev")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if id == "" {
		t.Fatal("expected artifact to be saved")
	}

	runsDir := filepath.Join(tmp, "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	var content string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			b, readErr := os.ReadFile(filepath.Join(runsDir, e.Name()))
			if readErr != nil {
				t.Fatalf("ReadFile failed: %v", readErr)
			}
			content = string(b)
		}
	}
	if content == "" {
		t.Fatal("no JSON artifact found on disk")
	}

	// Bodies serialize as base64, so scan both the raw file and the decoded
	// artifact (headers, URLs, messages are plain; bodies need decoding).
	var decoded domain.RunResult
	if err := json.Unmarshal([]byte(content), &decoded); err != nil {
		t.Fatalf("artifact is not valid JSON: %v", err)
	}
	scannable := content
	for _, rr := range decoded.Results {
		scannable += string(rr.RequestBody) + string(rr.Response.Body)
	}

	for _, s := range []string{
		"SECRET_API_KEY",
		"SECRET_BEARER_TOKEN",
		"SECRET_FORM_PASS",
		"SECRET_ACCESS_TOKEN",
		"SECRET_SESSION_ID",
	} {
		if strings.Contains(scannable, s) {
			t.Errorf("artifact on disk contains raw secret %q", s)
		}
	}

	if !strings.Contains(scannable, "alice") {
		t.Error("artifact should preserve non-sensitive value 'alice'")
	}
}

func TestIntegration_ArtifactOnDisk_NoSecrets(t *testing.T) {
	tmp := t.TempDir()

	cfg := domain.DefaultConfig()
	cfg.Masking.Enabled = true
	cfg.Masking.MaskRequestHeaders = true
	cfg.Masking.MaskRequestBody = true
	cfg.Masking.MaskResponseHeaders = true
	cfg.Masking.MaskResponseBody = true
	cfg.Masking.MaskQueryParams = true

	r := redaction.New(cfg.Masking)

	store := runstore.NewJSONStore(tmp, cfg,
		runstore.WithRedacter(r),
		runstore.WithNow(func() time.Time {
			return time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC)
		}),
	)

	// URL and ResolvedURL carry the same resolved value, exactly like the
	// real runner produces them (the old fixture hid a leak by differing).
	resolvedURL := "https://api.example.com/auth/login?api_key=SECRET_API_KEY&page=1"
	artifact := domain.RunArtifact{
		CollectionName:  "secrets-test",
		EnvironmentName: "dev",
		StartedAt:       time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC),
		EndedAt:         time.Date(2026, 3, 13, 10, 0, 1, 0, time.UTC),
		Results: []domain.RequestResult{
			{
				Name:   "login",
				Method: domain.MethodPost,
				URL:    resolvedURL,
				RequestHeaders: map[string]string{
					"Authorization": "Bearer SUPER_SECRET_TOKEN",
					"Content-Type":  "application/json",
				},
				RequestBody: []byte(`{"username":"alice","password":"SUPER_SECRET_PASS","data":{"api_key":"SECRET_API_KEY"}}`),
				ResolvedURL: resolvedURL,
				StatusCode:  200,
				LatencyMS:   42,
				Response: domain.ResponseSnapshot{
					Headers: map[string][]string{
						"Set-Cookie":   {"session=SECRET_SESSION_ID"},
						"Content-Type": {"application/json"},
					},
					Body: []byte(`{"access_token":"SECRET_ACCESS_TOKEN","user":"alice"}`),
				},
				Extracted: domain.Vars{
					"auth_token": "SECRET_ACCESS_TOKEN",
					"user_name":  "alice",
				},
				Error: &domain.RunError{
					Kind:    domain.RunErrorTimeout,
					Message: `Get "https://api.example.com/auth/login?api_key=SECRET_API_KEY&page=1": context deadline exceeded`,
				},
			},
		},
	}

	_, err := store.SaveRun(artifact)
	if err != nil {
		t.Fatalf("SaveRun failed: %v", err)
	}

	runsDir := filepath.Join(tmp, "runs")
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	var jsonFile string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			jsonFile = filepath.Join(runsDir, e.Name())
			break
		}
	}
	if jsonFile == "" {
		t.Fatal("no JSON artifact found on disk")
	}

	data, err := os.ReadFile(jsonFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	content := string(data)

	secrets := []string{
		"SUPER_SECRET_TOKEN",
		"SUPER_SECRET_PASS",
		"SECRET_API_KEY",
		"SECRET_SESSION_ID",
		"SECRET_ACCESS_TOKEN",
	}
	for _, s := range secrets {
		if strings.Contains(content, s) {
			t.Errorf("artifact on disk contains raw secret %q", s)
		}
	}

	if !strings.Contains(content, "********") {
		t.Error("artifact on disk does not contain mask placeholder")
	}

	if !strings.Contains(content, "alice") {
		t.Error("artifact should contain non-sensitive value 'alice'")
	}
	if !strings.Contains(content, "application/json") {
		t.Error("artifact should contain non-sensitive header 'application/json'")
	}

	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Errorf("artifact is not valid JSON: %v", err)
	}
}
