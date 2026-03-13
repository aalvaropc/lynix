package redaction_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/infra/redaction"
	"github.com/aalvaropc/lynix/internal/infra/runstore"
)

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

	artifact := domain.RunArtifact{
		CollectionName:  "secrets-test",
		EnvironmentName: "dev",
		StartedAt:       time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC),
		EndedAt:         time.Date(2026, 3, 13, 10, 0, 1, 0, time.UTC),
		Results: []domain.RequestResult{
			{
				Name:   "login",
				Method: domain.MethodPost,
				URL:    "https://api.example.com/auth/login",
				RequestHeaders: map[string]string{
					"Authorization": "Bearer SUPER_SECRET_TOKEN",
					"Content-Type":  "application/json",
				},
				RequestBody: []byte(`{"username":"alice","password":"SUPER_SECRET_PASS","data":{"api_key":"SECRET_API_KEY"}}`),
				ResolvedURL: "https://api.example.com/auth/login?api_key=SECRET_API_KEY&page=1",
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
			},
		},
	}

	_, err := store.SaveRun(artifact)
	if err != nil {
		t.Fatalf("SaveRun failed: %v", err)
	}

	// Read the saved file from disk.
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

	// Assert no raw secrets on disk.
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

	// Assert mask placeholder is present.
	if !strings.Contains(content, "********") {
		t.Error("artifact on disk does not contain mask placeholder")
	}

	// Safe values should be preserved.
	if !strings.Contains(content, "alice") {
		t.Error("artifact should contain non-sensitive value 'alice'")
	}
	if !strings.Contains(content, "application/json") {
		t.Error("artifact should contain non-sensitive header 'application/json'")
	}

	// Verify it's valid JSON.
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Errorf("artifact is not valid JSON: %v", err)
	}
}
