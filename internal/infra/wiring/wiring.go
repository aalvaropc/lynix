package wiring

import (
	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/infra/httpclient"
	"github.com/aalvaropc/lynix/internal/infra/httprunner"
	"github.com/aalvaropc/lynix/internal/infra/redaction"
	"github.com/aalvaropc/lynix/internal/infra/runstore"
	"github.com/aalvaropc/lynix/internal/infra/yamlcollection"
	"github.com/aalvaropc/lynix/internal/infra/yamlenv"
	"github.com/aalvaropc/lynix/internal/ports"
)

// Adapters holds all the adapter instances wired for a workspace.
// Envs is the concrete loader: it implements both ports.EnvironmentLoader and
// ports.EnvironmentCatalog, sparing callers an unchecked type assertion.
type Adapters struct {
	Collections ports.CollectionLoader
	Envs        *yamlenv.Loader
	Runner      ports.RequestRunner
	Store       ports.ArtifactStore
	Redactor    *redaction.Redactor
	Config      domain.Config
}

// Opts carries runtime flags that affect adapter construction
// but are not part of the persisted workspace config.
type Opts struct {
	Insecure          bool // skip TLS certificate verification
	NoFollowRedirects bool // disable HTTP redirect following globally
	EnableStore       bool // persist run artifacts (Store is nil when false)
}

// NewAdapters creates all adapters for a workspace root and config.
func NewAdapters(root string, cfg domain.Config, opts Opts) Adapters {
	colLoader := yamlcollection.NewLoader(
		yamlcollection.WithCollectionsDir(cfg.Paths.CollectionsDir),
	)

	envLoader := yamlenv.NewLoader(
		root,
		yamlenv.WithEnvDir(cfg.Paths.EnvironmentsDir),
	)

	hcfg := httpclient.DefaultConfig()
	hcfg.Insecure = cfg.Run.Insecure || opts.Insecure
	hcfg.NoFollowRedirects = opts.NoFollowRedirects
	// Timeouts are enforced by the runner per request (context deadline);
	// a client-level timeout would silently cap larger timeout_ms values.
	hcfg.Timeout = 0
	client := httpclient.New(hcfg)
	runner := httprunner.New(client)

	redactor := redaction.New(cfg.Masking)

	var store ports.ArtifactStore
	if opts.EnableStore {
		store = runstore.NewJSONStore(root, cfg,
			runstore.WithIndex(true),
			runstore.WithRedacter(redactor),
		)
	}

	return Adapters{
		Collections: colLoader,
		Envs:        envLoader,
		Runner:      runner,
		Store:       store,
		Redactor:    redactor,
		Config:      cfg,
	}
}
