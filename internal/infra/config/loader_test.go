package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCollection(t *testing.T) {
	path := filepath.Join("testdata", "collection.yaml")
	col, err := LoadCollection(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if col.Name != "Sample" {
		t.Fatalf("expected name Sample, got %q", col.Name)
	}
	if len(col.Requests) != 1 {
		t.Fatalf("expected one request")
	}
}

func TestLoadCollectionInvalid(t *testing.T) {
	path := filepath.Join("testdata", "collection_invalid.yaml")
	_, err := LoadCollection(path)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "requests[0].method") {
		t.Fatalf("expected field in error, got %v", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("expected path in error, got %v", err)
	}
}

func TestLoadEnvironment(t *testing.T) {
	path := filepath.Join("testdata", "env.yaml")
	env, err := LoadEnvironment(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Name != "env" {
		t.Fatalf("expected env name env, got %q", env.Name)
	}
	if env.Vars["token"] != "abc123" {
		t.Fatalf("expected token to map")
	}
}
