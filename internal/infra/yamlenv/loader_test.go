package yamlenv

import (
	"os"
	"path/filepath"
	"testing"
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
