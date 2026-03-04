package wiring

import (
	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/infra/httpclient"
	"github.com/aalvaropc/lynix/internal/infra/httprunner"
	"github.com/aalvaropc/lynix/internal/infra/runstore"
	"github.com/aalvaropc/lynix/internal/infra/yamlcollection"
	"github.com/aalvaropc/lynix/internal/infra/yamlenv"
	"github.com/aalvaropc/lynix/internal/ports"
)

// Adapters holds all the adapter instances wired for a workspace.
type Adapters struct {
	Collections ports.CollectionLoader
	Envs        ports.EnvironmentLoader
	Runner      ports.RequestRunner
	Store       ports.ArtifactStore
}

// NewAdapters creates all adapters for a workspace root and config.
// If enableStore is false, Store will be nil.
func NewAdapters(root string, cfg domain.Config, enableStore bool) Adapters {
	colLoader := yamlcollection.NewLoader(
		yamlcollection.WithCollectionsDir(cfg.Paths.CollectionsDir),
	)

	envLoader := yamlenv.NewLoader(
		root,
		yamlenv.WithEnvDir(cfg.Paths.EnvironmentsDir),
	)

	client := httpclient.New(httpclient.DefaultConfig())
	runner := httprunner.New(client)

	var store ports.ArtifactStore
	if enableStore {
		store = runstore.NewJSONStore(root, cfg, runstore.WithIndex(true))
	}

	return Adapters{
		Collections: colLoader,
		Envs:        envLoader,
		Runner:      runner,
		Store:       store,
	}
}
