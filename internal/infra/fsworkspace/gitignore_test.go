package fsworkspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureGitignore_CreatesFile(t *testing.T) {
	tmp := t.TempDir()

	if err := ensureGitignore(tmp); err != nil {
		t.Fatalf("ensureGitignore error: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(tmp, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}

	s := string(b)
	wants := []string{
		"# Lynix",
		"runs/",
		".lynix/",
		"lynix.lock",
		"env/secrets.local.yaml",
	}
	for _, w := range wants {
		if !strings.Contains(s, w) {
			t.Fatalf("expected .gitignore to contain %q, got:\n%s", w, s)
		}
	}
}

func TestEnsureGitignore_AppendsMissingEntries(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".gitignore")

	existing := "node_modules/\n# Lynix\nruns/\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}

	if err := ensureGitignore(tmp); err != nil {
		t.Fatalf("ensureGitignore error: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	s := string(b)

	if !strings.Contains(s, "node_modules/") {
		t.Fatalf("expected existing content preserved, got:\n%s", s)
	}
	if strings.Count(s, "# Lynix") != 1 {
		t.Fatalf("expected 1 header, got:\n%s", s)
	}
	if strings.Count(s, "runs/") != 1 {
		t.Fatalf("expected runs/ not duplicated, got:\n%s", s)
	}

	wants := []string{
		".lynix/",
		"lynix.lock",
		"env/secrets.local.yaml",
	}
	for _, w := range wants {
		if !strings.Contains(s, w) {
			t.Fatalf("expected .gitignore to contain %q, got:\n%s", w, s)
		}
	}
}
