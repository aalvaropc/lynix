package runstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aalvaropc/lynix/internal/domain"
)

func TestSaveRun_CreatesJSONFile(t *testing.T) {
	tmp := t.TempDir()

	cfg := domain.DefaultConfig()
	cfg.Paths.RunsDir = "runs"
	cfg.Masking.Enabled = false

	store := NewJSONStore(tmp, cfg)

	start := time.Date(2026, 2, 3, 10, 11, 12, 0, time.UTC)
	run := domain.RunArtifact{
		CollectionName:  "Demo API",
		CollectionPath:  "collections/demo.yaml",
		EnvironmentName: "dev",
		StartedAt:       start,
		EndedAt:         start.Add(2 * time.Second),
		Results: []domain.RequestResult{
			{
				Name:       "health",
				Method:     domain.MethodGet,
				URL:        "http://x/health",
				StatusCode: 200,
				LatencyMS:  10,
				Assertions: []domain.AssertionResult{
					{Name: "status", Passed: true, Message: "ok"},
				},
				Extracts:  []domain.ExtractResult{},
				Extracted: domain.Vars{},
				Response: domain.ResponseSnapshot{
					Headers: map[string][]string{"X-Test": {"1"}},
					Body:    []byte("ok"),
				},
			},
		},
	}

	id, err := store.SaveRun(run)
	if err != nil {
		t.Fatalf("SaveRun error: %v", err)
	}

	wantFile := filepath.Join(tmp, "runs", "20260203T101112Z_demo-api.json")
	if _, err := os.Stat(wantFile); err != nil {
		t.Fatalf("expected file at %s, stat err=%v (id=%s)", wantFile, err, id)
	}

	b, err := os.ReadFile(wantFile)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	var decoded domain.RunResult
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.CollectionName != "Demo API" {
		t.Fatalf("expected collection name, got=%q", decoded.CollectionName)
	}
	if len(decoded.Results) != 1 {
		t.Fatalf("expected 1 result, got=%d", len(decoded.Results))
	}
	if decoded.Results[0].StatusCode != 200 {
		t.Fatalf("expected status=200, got=%d", decoded.Results[0].StatusCode)
	}
}

func TestSaveRun_MasksSensitiveExtractedWhenEnabled(t *testing.T) {
	tmp := t.TempDir()

	cfg := domain.DefaultConfig()
	cfg.Paths.RunsDir = "runs"
	cfg.Masking.Enabled = true

	store := NewJSONStore(tmp, cfg)

	start := time.Date(2026, 2, 3, 10, 11, 12, 0, time.UTC)

	run := domain.RunArtifact{
		CollectionName:  "Mask Demo",
		CollectionPath:  "collections/demo.yaml",
		EnvironmentName: "dev",
		StartedAt:       start,
		EndedAt:         start.Add(1 * time.Second),
		Results: []domain.RequestResult{
			{
				Name: "auth.token",
				Extracted: domain.Vars{
					"auth.token":    "abc123",
					"db_password":   "p@ss",
					"user.id":       "7",
					"not_sensitive": "ok",
				},
				Response: domain.ResponseSnapshot{Headers: map[string][]string{}},
			},
		},
	}

	// Ensure we do NOT mutate original run.
	origToken := run.Results[0].Extracted["auth.token"]

	_, err := store.SaveRun(run)
	if err != nil {
		t.Fatalf("SaveRun error: %v", err)
	}
	if run.Results[0].Extracted["auth.token"] != origToken {
		t.Fatalf("expected original run not mutated")
	}

	path := filepath.Join(tmp, "runs", "20260203T101112Z_mask-demo.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	var decoded domain.RunResult
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	got := decoded.Results[0].Extracted
	if got["auth.token"] != maskValue {
		t.Fatalf("expected auth.token masked, got=%q", got["auth.token"])
	}
	if got["db_password"] != maskValue {
		t.Fatalf("expected db_password masked, got=%q", got["db_password"])
	}
	if got["user.id"] != "7" {
		t.Fatalf("expected user.id preserved, got=%q", got["user.id"])
	}
	if got["not_sensitive"] != "ok" {
		t.Fatalf("expected not_sensitive preserved, got=%q", got["not_sensitive"])
	}
}

<<<<<<< HEAD
func TestSaveRun_MasksSensitiveResponseHeadersWhenEnabled(t *testing.T) {
	tmp := t.TempDir()

	cfg := domain.DefaultConfig()
	cfg.Paths.RunsDir = "runs"
	cfg.Masking.Enabled = true

	store := NewJSONStore(tmp, cfg)

	start := time.Date(2026, 2, 3, 10, 11, 12, 0, time.UTC)
	run := domain.RunArtifact{
		CollectionName:  "Mask Headers",
		CollectionPath:  "collections/demo.yaml",
		EnvironmentName: "dev",
		StartedAt:       start,
		EndedAt:         start.Add(1 * time.Second),
		Results: []domain.RequestResult{
			{
				Name: "headers",
				Response: domain.ResponseSnapshot{
					Headers: map[string][]string{
						"Authorization": {"Bearer abc"},
						"Set-Cookie":    {"session=abc"},
						"X-Test":        {"1"},
					},
					Body: []byte("ok"),
				},
			},
		},
	}

	// Ensure we do NOT mutate original run.
	origAuth := run.Results[0].Response.Headers["Authorization"][0]

	_, err := store.SaveRun(run)
	if err != nil {
		t.Fatalf("SaveRun error: %v", err)
	}
	if run.Results[0].Response.Headers["Authorization"][0] != origAuth {
		t.Fatalf("expected original run not mutated")
	}

	path := filepath.Join(tmp, "runs", "20260203T101112Z_mask-headers.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	var decoded domain.RunResult
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	h := decoded.Results[0].Response.Headers
	if h["Authorization"][0] != maskValue {
		t.Fatalf("expected Authorization masked, got=%q", h["Authorization"][0])
	}
	if h["Set-Cookie"][0] != maskValue {
		t.Fatalf("expected Set-Cookie masked, got=%q", h["Set-Cookie"][0])
	}
	if h["X-Test"][0] != "1" {
		t.Fatalf("expected X-Test preserved, got=%q", h["X-Test"][0])
	}
}

func TestSaveRun_UsesUniqueFilenameOnCollision(t *testing.T) {
	tmp := t.TempDir()

	cfg := domain.DefaultConfig()
	cfg.Paths.RunsDir = "runs"
	cfg.Masking.Enabled = false

	store := NewJSONStore(tmp, cfg)

	start := time.Date(2026, 2, 3, 10, 11, 12, 0, time.UTC)
	run := domain.RunArtifact{
		CollectionName:  "Demo API",
		CollectionPath:  "collections/demo.yaml",
		EnvironmentName: "dev",
		StartedAt:       start,
		EndedAt:         start.Add(1 * time.Second),
		Results: []domain.RequestResult{
			{
				Name:       "health",
				Method:     domain.MethodGet,
				URL:        "http://x/health",
				StatusCode: 200,
				Response:   domain.ResponseSnapshot{Headers: map[string][]string{}},
			},
		},
	}

	id1, err := store.SaveRun(run)
	if err != nil {
		t.Fatalf("SaveRun #1 error: %v", err)
	}
	id2, err := store.SaveRun(run)
	if err != nil {
		t.Fatalf("SaveRun #2 error: %v", err)
	}
	if id1 == id2 {
		t.Fatalf("expected unique ids, got %q", id1)
	}

	p1 := filepath.Join(tmp, "runs", id1+".json")
	if _, err := os.Stat(p1); err != nil {
		t.Fatalf("expected first file at %s, stat err=%v", p1, err)
	}

	p2 := filepath.Join(tmp, "runs", id2+".json")
	if _, err := os.Stat(p2); err != nil {
		t.Fatalf("expected second file at %s, stat err=%v", p2, err)
	}
	if id2 != id1+"_2" {
		t.Fatalf("expected second id %q, got %q", id1+"_2", id2)
	}
}
