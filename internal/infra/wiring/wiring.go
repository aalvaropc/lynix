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
func NewAdapters(root string, cfg domain.Config, opts Opts) (Adapters, error) {
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
	hcfg.EnableCookieJar = cfg.Run.Cookies
	// Timeouts are enforced by the runner per request (context deadline);
	// a client-level timeout would silently cap larger timeout_ms values.
	hcfg.Timeout = 0
	if cfg.Run.TLS.CAFile != "" {
		pool, err := httpclient.LoadCAFile(cfg.Run.TLS.CAFile)
		if err != nil {
			return Adapters{}, &domain.OpError{
				Op:   "wiring.tls",
				Kind: domain.KindInvalidConfig,
				Path: cfg.Run.TLS.CAFile,
				Err:  err,
			}
		}
		hcfg.RootCAs = pool
	}
	client := httpclient.New(hcfg)

	var runnerOpts []httprunner.Option
	if cfg.Run.MaxBodyKB > 0 {
		runnerOpts = append(runnerOpts, httprunner.WithMaxBodyBytes(int64(cfg.Run.MaxBodyKB)*1024))
	}
	runner := httprunner.New(client, runnerOpts...)

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
	}, nil
}
