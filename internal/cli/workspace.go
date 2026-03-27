package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/infra/redaction"
	"github.com/aalvaropc/lynix/internal/infra/wiring"
	"github.com/aalvaropc/lynix/internal/infra/workspacefinder"
	"github.com/aalvaropc/lynix/internal/ports"
)

type workspaceCtx struct {
	root       string
	cfg        domain.Config
	standalone bool // true when running without a workspace

	collections ports.CollectionLoader

	envs       ports.EnvironmentLoader
	envCatalog ports.EnvironmentCatalog

	runner   ports.RequestRunner
	store    ports.ArtifactStore
	redactor *redaction.Redactor
}

func loadWorkspace(workspaceFlag string, opts wiring.Opts) (*workspaceCtx, error) {
	root, err := resolveWorkspaceRoot(workspaceFlag)
	if err != nil {
		return nil, err
	}

	cfg, err := workspacefinder.LoadConfig(root)
	if err != nil {
		return nil, err
	}

	adapters := wiring.NewAdapters(root, cfg, true, opts)

	return &workspaceCtx{
		root:        root,
		cfg:         cfg,
		collections: adapters.Collections,
		envs:        adapters.Envs,
		envCatalog:  adapters.Envs.(ports.EnvironmentCatalog),
		runner:      adapters.Runner,
		store:       adapters.Store,
		redactor:    adapters.Redactor,
	}, nil
}

// loadStandalone creates a minimal context for running without a workspace.
// Uses DefaultConfig, CWD for relative path resolution, and no artifact storage.
func loadStandalone(opts wiring.Opts) (*workspaceCtx, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	cfg := domain.DefaultConfig()
	adapters := wiring.NewAdapters(cwd, cfg, false, opts)

	return &workspaceCtx{
		root:        cwd,
		cfg:         cfg,
		standalone:  true,
		collections: adapters.Collections,
		envs:        adapters.Envs,
		runner:      adapters.Runner,
	}, nil
}

func resolveWorkspaceRoot(workspaceFlag string) (string, error) {
	w := strings.TrimSpace(workspaceFlag)
	if w != "" {
		abs, err := filepath.Abs(w)
		if err != nil {
			return "", fmt.Errorf("invalid workspace path: %w", err)
		}
		return abs, nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	locator := workspacefinder.NewFinder()
	root, err := locator.FindRoot(context.Background(), wd)
	if err != nil {
		return "", fmt.Errorf("workspace not found from %q (tip: run `lynix init`): %w", wd, err)
	}
	return root, nil
}

func resolveCollectionPath(ws *workspaceCtx, arg string) (string, error) {
	in := strings.TrimSpace(arg)
	if in == "" {
		return "", fmt.Errorf("collection is required (use --collection or -c)")
	}

	// If arg looks like a path (contains separators) or has YAML extension, resolve from CWD/root.
	if looksLikePath(in) || hasYAMLExt(in) {
		p := in
		if !filepath.IsAbs(p) {
			p = filepath.Join(ws.root, p)
		}
		return filepath.Clean(p), nil
	}

	// Bare name resolution requires a workspace.
	if ws.standalone {
		return "", fmt.Errorf("collection %q: bare names require a workspace (use a file path or run `lynix init`)", in)
	}

	collectionsDir := filepath.Join(ws.root, ws.cfg.Paths.CollectionsDir)

	// Try name.yaml / name.yml in collections dir.
	p1 := filepath.Join(collectionsDir, in+".yaml")
	if fileExists(p1) {
		return p1, nil
	}
	p2 := filepath.Join(collectionsDir, in+".yml")
	if fileExists(p2) {
		return p2, nil
	}

	// Match by collection "name" field.
	refs, err := ws.collections.ListCollections(ws.root)
	if err == nil {
		for _, r := range refs {
			if strings.EqualFold(r.Name, in) {
				return r.Path, nil
			}
		}
	}

	return "", fmt.Errorf("collection %q not found in %q", in, collectionsDir)
}

func resolveEnvironmentArg(ws *workspaceCtx, arg string) (string, error) {
	in := strings.TrimSpace(arg)
	if in == "" {
		if ws.standalone {
			return "", nil // empty env in standalone mode
		}
		return ws.cfg.Defaults.Environment, nil
	}

	// If arg is a path, resolve relative to root.
	if looksLikePath(in) {
		p := in
		if !filepath.IsAbs(p) {
			p = filepath.Join(ws.root, p)
		}
		return filepath.Clean(p), nil
	}

	// If user provided "dev.yaml", treat it as file under env dir.
	if hasYAMLExt(in) {
		envDir := filepath.Join(ws.root, ws.cfg.Paths.EnvironmentsDir)
		p := filepath.Join(envDir, in)
		if fileExists(p) {
			return p, nil
		}
		// fall back to passing as-is (loader will treat it as path-like because of ".yaml")
		return p, nil
	}

	// Otherwise, treat it as an env name ("dev") and let the loader resolve it.
	return in, nil
}

func looksLikePath(s string) bool {
	return strings.Contains(s, "/") || strings.Contains(s, string(filepath.Separator))
}

func hasYAMLExt(s string) bool {
	ext := strings.ToLower(filepath.Ext(s))
	return ext == ".yaml" || ext == ".yml"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
