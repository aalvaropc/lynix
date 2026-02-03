package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/infra/httpclient"
	"github.com/aalvaropc/lynix/internal/infra/httprunner"
	"github.com/aalvaropc/lynix/internal/infra/runstore"
	"github.com/aalvaropc/lynix/internal/infra/workspacefinder"
	"github.com/aalvaropc/lynix/internal/infra/yamlcollection"
	"github.com/aalvaropc/lynix/internal/infra/yamlenv"
	"github.com/aalvaropc/lynix/internal/ports"
)

type workspaceCtx struct {
	root string
	cfg  domain.Config

	collections ports.CollectionLoader

	envs       ports.EnvironmentLoader
	envCatalog ports.EnvironmentCatalog

	runner ports.RequestRunner
	store  ports.ArtifactStore
}

func loadWorkspace(workspaceFlag string) (*workspaceCtx, error) {
	root, err := resolveWorkspaceRoot(workspaceFlag)
	if err != nil {
		return nil, err
	}

	cfg, err := workspacefinder.LoadConfig(root)
	if err != nil {
		return nil, err
	}

	colLoader := yamlcollection.NewLoader(
		yamlcollection.WithCollectionsDir(cfg.Paths.CollectionsDir),
	)

	envLoader := yamlenv.NewLoader(
		root,
		yamlenv.WithEnvDir(cfg.Paths.EnvironmentsDir),
	)

	client := httpclient.New(httpclient.DefaultConfig())
	runner := httprunner.New(client)

	store := runstore.NewJSONStore(root, cfg, runstore.WithIndex(true))

	return &workspaceCtx{
		root:        root,
		cfg:         cfg,
		collections: colLoader,
		envs:        envLoader,
		envCatalog:  envLoader,
		runner:      runner,
		store:       store,
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
	root, err := locator.FindRoot(wd)
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

	// If arg looks like a path (contains separators), resolve relative to workspace root.
	if looksLikePath(in) {
		p := in
		if !filepath.IsAbs(p) {
			p = filepath.Join(ws.root, p)
		}
		return filepath.Clean(p), nil
	}

	collectionsDir := filepath.Join(ws.root, ws.cfg.Paths.CollectionsDir)

	// If user provided "demo.yaml", treat it as file under collections dir.
	if hasYAMLExt(in) {
		p := filepath.Join(collectionsDir, in)
		if fileExists(p) {
			return p, nil
		}
	}

	// If user provided "demo", try demo.yaml / demo.yml in collections dir.
	p1 := filepath.Join(collectionsDir, in+".yaml")
	if fileExists(p1) {
		return p1, nil
	}
	p2 := filepath.Join(collectionsDir, in+".yml")
	if fileExists(p2) {
		return p2, nil
	}

	// As a last resort: match by collection "name" field.
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
		return ws.cfg.Defaults.Environment, nil
	}

	// If arg is a path, resolve relative to workspace root.
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
