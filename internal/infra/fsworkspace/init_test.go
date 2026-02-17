package fsworkspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aalvaropc/lynix/internal/domain"
)

func TestInitializer_Init_CreatesWorkspaceFiles(t *testing.T) {
	tmp := t.TempDir()

	i := NewInitializer()
	if err := i.Init(domain.WorkspaceSpec{Root: tmp}, false); err != nil {
		t.Fatalf("Init error: %v", err)
	}

	assertFileExists(t, filepath.Join(tmp, "lynix.yaml"))
	assertFileExists(t, filepath.Join(tmp, "collections", "demo.yaml"))
	assertFileExists(t, filepath.Join(tmp, "env", "dev.yaml"))
	assertFileExists(t, filepath.Join(tmp, "env", "stg.yaml"))

	secretPath := filepath.Join(tmp, "env", "secrets.local.yaml")
	assertFileExists(t, secretPath)
	info, err := os.Stat(secretPath)
	if err != nil {
		t.Fatalf("stat secrets file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("expected secrets file mode 600, got %o", got)
	}
}

func TestInitializer_Init_SkipsExistingFilesUnlessForce(t *testing.T) {
	tmp := t.TempDir()

	lynixYAML := filepath.Join(tmp, "lynix.yaml")
	if err := os.WriteFile(lynixYAML, []byte("custom\n"), 0o644); err != nil {
		t.Fatalf("write existing lynix.yaml: %v", err)
	}

	i := NewInitializer()

	if err := i.Init(domain.WorkspaceSpec{Root: tmp}, false); err != nil {
		t.Fatalf("Init (force=false) error: %v", err)
	}

	b, err := os.ReadFile(lynixYAML)
	if err != nil {
		t.Fatalf("read lynix.yaml: %v", err)
	}
	if string(b) != "custom\n" {
		t.Fatalf("expected lynix.yaml preserved, got %q", string(b))
	}

	if err := i.Init(domain.WorkspaceSpec{Root: tmp}, true); err != nil {
		t.Fatalf("Init (force=true) error: %v", err)
	}

	b, err = os.ReadFile(lynixYAML)
	if err != nil {
		t.Fatalf("read lynix.yaml after force: %v", err)
	}
	if !strings.Contains(string(b), "lynix:") {
		t.Fatalf("expected lynix.yaml overwritten with template, got %q", string(b))
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s, stat err=%v", path, err)
	}
}
