package yamlenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aalvaropc/lynix/internal/domain"
)

func TestLoadEnvironment_MergesSecrets(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "ws")
	envDir := filepath.Join(root, "env")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(envDir, "dev.yaml"), []byte("vars:\n  base_url: http://localhost:8080\n  token: base\n"), 0o644); err != nil {
		t.Fatalf("write dev: %v", err)
	}
	if err := os.WriteFile(filepath.Join(envDir, "secrets.local.yaml"), []byte("vars:\n  token: secret\n"), 0o644); err != nil {
		t.Fatalf("write secrets: %v", err)
	}

	l := NewLoader(root)
	env, err := l.LoadEnvironment("dev")
	if err != nil {
		t.Fatalf("LoadEnvironment error: %v", err)
	}

	if env.Vars["base_url"] != "http://localhost:8080" {
		t.Fatalf("expected base_url, got=%s", env.Vars["base_url"])
	}
	if env.Vars["token"] != "secret" {
		t.Fatalf("expected token=secret override, got=%s", env.Vars["token"])
	}
}

func TestLoadEnvironment_SecretsMissing(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "ws")
	envDir := filepath.Join(root, "env")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(envDir, "dev.yaml"), []byte("vars:\n  base_url: http://localhost:8080\n"), 0o644); err != nil {
		t.Fatalf("write dev: %v", err)
	}

	l := NewLoader(root)
	env, err := l.LoadEnvironment("dev")
	if err != nil {
		t.Fatalf("LoadEnvironment error: %v", err)
	}

	if env.Vars["base_url"] != "http://localhost:8080" {
		t.Fatalf("expected base_url, got=%s", env.Vars["base_url"])
	}
}

func TestLoadEnvironment_EnvMissing(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "ws")
	envDir := filepath.Join(root, "env")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	l := NewLoader(root)
	_, err := l.LoadEnvironment("dev")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLoadEnvironment_SupportsYML(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "ws")
	envDir := filepath.Join(root, "env")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(envDir, "prod.yml"), []byte("vars:\n  base_url: https://api.example.com\n"), 0o644); err != nil {
		t.Fatalf("write prod: %v", err)
	}

	l := NewLoader(root)
	env, err := l.LoadEnvironment("prod")
	if err != nil {
		t.Fatalf("LoadEnvironment error: %v", err)
	}

	if env.Name != "prod" {
		t.Fatalf("expected name=prod, got=%s", env.Name)
	}
	if env.Vars["base_url"] != "https://api.example.com" {
		t.Fatalf("expected base_url, got=%s", env.Vars["base_url"])
	}
}

func TestLoadEnvironment_EmptyName_ReturnsEmptyEnv(t *testing.T) {
	l := NewLoader(t.TempDir())
	env, err := l.LoadEnvironment("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Name != "(none)" {
		t.Errorf("expected name=(none), got=%q", env.Name)
	}
	if len(env.Vars) != 0 {
		t.Errorf("expected empty vars, got=%v", env.Vars)
	}
}

func TestLoadEnvironment_WhitespaceOnly_ReturnsEmptyEnv(t *testing.T) {
	l := NewLoader(t.TempDir())
	env, err := l.LoadEnvironment("   ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Name != "(none)" {
		t.Errorf("expected name=(none), got=%q", env.Name)
	}
}

func TestLoadEnvironment_UnknownFieldRejected(t *testing.T) {
	tmp := t.TempDir()
	envDir := filepath.Join(tmp, "env")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := []byte("varz:\n  key: value\n")
	if err := os.WriteFile(filepath.Join(envDir, "dev.yaml"), content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	l := NewLoader(tmp)
	_, err := l.LoadEnvironment("dev")
	if err == nil {
		t.Fatal("expected error for unknown field 'varz'")
	}
	if !domain.IsKind(err, domain.KindInvalidConfig) {
		t.Fatalf("expected KindInvalidConfig, got: %v", err)
	}
}

func TestLoadEnvironment_NotFoundMentionsBothExtensions(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "env"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	l := NewLoader(tmp)
	_, err := l.LoadEnvironment("missing")
	if err == nil {
		t.Fatal("expected error for missing environment")
	}
	if !strings.Contains(err.Error(), ".yaml") || !strings.Contains(err.Error(), ".yml") {
		t.Errorf("expected both extensions in error, got: %v", err)
	}
}
