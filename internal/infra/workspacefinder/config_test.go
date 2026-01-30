package workspacefinder

import (
	"os"
	"path/filepath"
	"testing"
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
