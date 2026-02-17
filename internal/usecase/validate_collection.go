package usecase

import (
	"context"
	"fmt"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/ports"
)

type ValidateCollection struct {
	collections ports.CollectionLoader
	envs        ports.EnvironmentLoader
	resolver    *domain.VarResolver
}

type ValidateOption func(*ValidateCollection)

func WithVarResolver(vr *domain.VarResolver) ValidateOption {
	return func(uc *ValidateCollection) {
		if vr != nil {
			uc.resolver = vr
		}
	}
}

func NewValidateCollection(cl ports.CollectionLoader, el ports.EnvironmentLoader, opts ...ValidateOption) *ValidateCollection {
	uc := &ValidateCollection{
		collections: cl,
		envs:        el,
		resolver:    domain.NewVarResolver(),
	}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

// Execute validates a collection + environment pair without performing HTTP calls.
// It resolves templated fields ({{vars}}) and performs a basic "static" check that
// variables referenced later can come from initial vars or earlier extract keys.
func (uc *ValidateCollection) Execute(ctx context.Context, collectionPath string, envNameOrPath string) error {
	col, err := uc.collections.LoadCollection(collectionPath)
	if err != nil {
		return err
	}

	env, err := uc.envs.LoadEnvironment(envNameOrPath)
	if err != nil {
		return err
	}

	// collection vars < env vars < extracted vars
	vars := mergeVars(col.Vars, env.Vars)

	for _, req := range col.Requests {
		if err := ctx.Err(); err != nil {
			return err
		}

		rt, err := uc.resolver.NewRuntime(vars)
		if err != nil {
			return err
		}

		if _, err := rt.ResolveRequest(req); err != nil {
			return fmt.Errorf("request %q: %w", req.Name, err)
		}

		// Assume extract keys become available for subsequent requests.
		for k := range req.Extract {
			if _, ok := vars[k]; !ok {
				vars[k] = "x"
			}
		}
	}

	return nil
}
