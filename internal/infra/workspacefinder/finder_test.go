package workspacefinder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aalvaropc/lynix/internal/domain"
)

func TestFindRoot_FindsWorkspaceFromNestedDir(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "ws")
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Create lynix.yaml at root
	if err := os.WriteFile(filepath.Join(root, "lynix.yaml"), []byte("lynix:\n  masking:\n    enabled: true\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	f := NewFinder()
	got, err := f.FindRoot(nested)
	if err != nil {
		t.Fatalf("FindRoot returned error: %v", err)
	}
	if got != root {
		t.Fatalf("expected root=%s, got=%s", root, got)
	}
}

func TestFindRoot_NotFound(t *testing.T) {
	tmp := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmp, "a", "b"), 0o755)

	f := NewFinder()
	_, err := f.FindRoot(filepath.Join(tmp, "a", "b"))
	if err == nil {
		t.Fatalf("expected error")
	}

	if !domain.IsKind(err, domain.KindNotFound) {
		t.Fatalf("expected KindNotFound, got: %v", err)
	}
}
