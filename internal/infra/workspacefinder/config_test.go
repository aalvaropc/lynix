package workspacefinder

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig_AppliesDefaults(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "ws")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Partial config (no paths/defaults)
	content := []byte("lynix:\n  masking:\n    enabled: false\n")
	if err := os.WriteFile(filepath.Join(root, "lynix.yaml"), content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if cfg.Masking.Enabled != false {
		t.Fatalf("expected masking=false, got=%v", cfg.Masking.Enabled)
	}
	if cfg.Defaults.Environment != "dev" {
		t.Fatalf("expected default env=dev, got=%s", cfg.Defaults.Environment)
	}
	if cfg.Paths.CollectionsDir != "collections" {
		t.Fatalf("expected collections dir=collections, got=%s", cfg.Paths.CollectionsDir)
	}
	if cfg.Paths.EnvironmentsDir != "env" {
		t.Fatalf("expected env dir=env, got=%s", cfg.Paths.EnvironmentsDir)
	}
	if cfg.Paths.RunsDir != "runs" {
		t.Fatalf("expected runs dir=runs, got=%s", cfg.Paths.RunsDir)
	}
}

func TestLoadConfig_MaskResponseHeaders(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "ws")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := []byte("lynix:\n  masking:\n    mask_response_headers: false\n")
	if err := os.WriteFile(filepath.Join(root, "lynix.yaml"), content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if cfg.Masking.MaskResponseHeaders != false {
		t.Fatalf("expected MaskResponseHeaders=false, got=%v", cfg.Masking.MaskResponseHeaders)
	}
}

func TestLoadConfig_MaskResponseHeaders_Default(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "ws")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := []byte("lynix:\n  masking:\n    enabled: true\n")
	if err := os.WriteFile(filepath.Join(root, "lynix.yaml"), content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if cfg.Masking.MaskResponseHeaders != true {
		t.Fatalf("expected MaskResponseHeaders default=true, got=%v", cfg.Masking.MaskResponseHeaders)
	}
}

func TestLoadConfig_FailOnDetectedSecret(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "ws")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := []byte("lynix:\n  masking:\n    fail_on_detected_secret: true\n")
	if err := os.WriteFile(filepath.Join(root, "lynix.yaml"), content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if cfg.Masking.FailOnDetectedSecret != true {
		t.Fatalf("expected FailOnDetectedSecret=true, got=%v", cfg.Masking.FailOnDetectedSecret)
	}
}

func TestLoadConfig_FailOnDetectedSecret_Default(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "ws")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := []byte("lynix:\n  masking:\n    enabled: true\n")
	if err := os.WriteFile(filepath.Join(root, "lynix.yaml"), content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if cfg.Masking.FailOnDetectedSecret != false {
		t.Fatalf("expected FailOnDetectedSecret default=false, got=%v", cfg.Masking.FailOnDetectedSecret)
	}
}

func TestLoadConfig_SchemaVersion_Present(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "ws")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := []byte("lynix:\n  schema_version: 1\n")
	if err := os.WriteFile(filepath.Join(root, "lynix.yaml"), content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if cfg.SchemaVersion != 1 {
		t.Fatalf("expected SchemaVersion=1, got=%d", cfg.SchemaVersion)
	}
}

func TestLoadConfig_SchemaVersion_Missing_DefaultsToOne(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "ws")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := []byte("lynix:\n  masking:\n    enabled: true\n")
	if err := os.WriteFile(filepath.Join(root, "lynix.yaml"), content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if cfg.SchemaVersion != 1 {
		t.Fatalf("expected SchemaVersion default=1, got=%d", cfg.SchemaVersion)
	}
}

func TestLoadConfig_SchemaVersion_Invalid_ReturnsError(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "ws")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := []byte("lynix:\n  schema_version: 0\n")
	if err := os.WriteFile(filepath.Join(root, "lynix.yaml"), content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := LoadConfig(root)
	if err == nil {
		t.Fatal("expected error for schema_version=0")
	}
}

func TestLoadConfig_MaskCLIOutput_NewKey(t *testing.T) {
	root := writeWorkspaceConfig(t, "lynix:\n  masking:\n    mask_cli_output: true\n")
	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if !cfg.Masking.MaskCLIOutput {
		t.Fatal("expected MaskCLIOutput=true")
	}
}

func TestLoadConfig_MaskCLIOutput_OldKeyAlias(t *testing.T) {
	root := writeWorkspaceConfig(t, "lynix:\n  masking:\n    apply_to_output: true\n")
	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if !cfg.Masking.MaskCLIOutput {
		t.Fatal("expected MaskCLIOutput=true via old apply_to_output alias")
	}
}

func TestLoadConfig_MaskCLIOutput_NewKeyTakesPrecedence(t *testing.T) {
	root := writeWorkspaceConfig(t, "lynix:\n  masking:\n    mask_cli_output: false\n    apply_to_output: true\n")
	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}
	if cfg.Masking.MaskCLIOutput {
		t.Fatal("expected MaskCLIOutput=false (new key takes precedence over old)")
	}
}

func writeWorkspaceConfig(t *testing.T, yaml string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "ws")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "lynix.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return root
}

func TestLoadConfig_RetrySettings(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "ws")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := []byte(`lynix:
  run:
    retries: 3
    retry_delay_ms: 500
    retry_5xx: true
`)
	if err := os.WriteFile(filepath.Join(root, "lynix.yaml"), content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if cfg.Run.Retries != 3 {
		t.Fatalf("expected retries=3, got=%d", cfg.Run.Retries)
	}
	if cfg.Run.RetryDelay != 500*time.Millisecond {
		t.Fatalf("expected retry_delay=500ms, got=%v", cfg.Run.RetryDelay)
	}
	if !cfg.Run.Retry5xx {
		t.Fatal("expected retry_5xx=true")
	}
}

func TestLoadConfig_InsecureSetting(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "ws")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := []byte(`lynix:
  run:
    insecure: true
`)
	if err := os.WriteFile(filepath.Join(root, "lynix.yaml"), content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if !cfg.Run.Insecure {
		t.Fatal("expected insecure=true")
	}
}

func TestLoadConfig_InsecureDefault_False(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "ws")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := []byte(`lynix:
  run:
    timeout_seconds: 60
`)
	if err := os.WriteFile(filepath.Join(root, "lynix.yaml"), content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := LoadConfig(root)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if cfg.Run.Insecure {
		t.Fatal("expected insecure=false by default")
	}
}
