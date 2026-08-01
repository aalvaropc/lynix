package usecase

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/PaesslerAG/jsonpath"
	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/ports"
)

type ValidateCollection struct {
	collections ports.CollectionLoader
	envs        ports.EnvironmentLoader
	resolver    *domain.VarResolver
	extraVars   domain.Vars
}

type ValidateOption func(*ValidateCollection)

func WithVarResolver(vr *domain.VarResolver) ValidateOption {
	return func(uc *ValidateCollection) {
		if vr != nil {
			uc.resolver = vr
		}
	}
}

// WithVars adds CLI-level variable overrides (--var key=value).
func WithVars(vars domain.Vars) ValidateOption {
	return func(uc *ValidateCollection) { uc.extraVars = vars }
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

	// collection vars < env vars < CLI --var overrides < extracted vars
	vars := domain.Merge(domain.Merge(col.Vars, env.Vars), uc.extraVars)

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

		// Validate schema file exists if referenced.
		if req.Assert.Schema != nil {
			if _, err := os.Stat(*req.Assert.Schema); err != nil {
				return fmt.Errorf("request %q: schema file %q: %w", req.Name, *req.Assert.Schema, err)
			}
		}

		// Compile JSONPath expressions and static regex patterns so typos
		// fail here, not at runtime. Patterns with {{var}} placeholders are
		// only resolvable at runtime and are skipped.
		if err := validateAssertionExpressions(req); err != nil {
			return fmt.Errorf("request %q: %w", req.Name, err)
		}

		// Assume extract keys become available for subsequent requests.
		for k := range req.Extract {
			if _, ok := vars[k]; !ok {
				vars[k] = "x"
			}
		}
		for k := range req.ExtractHeaders {
			if _, ok := vars[k]; !ok {
				vars[k] = "x"
			}
		}
	}

	return nil
}

// validateAssertionExpressions compiles JSONPath expressions (assert + extract)
// and regex patterns without {{var}} placeholders.
func validateAssertionExpressions(req domain.RequestSpec) error {
	checkPath := func(where, expr string) error {
		if strings.Contains(expr, "{{") {
			return nil // resolvable only at runtime
		}
		if _, err := jsonpath.New(expr); err != nil {
			return fmt.Errorf("%s: invalid jsonpath %q: %w", where, expr, err)
		}
		return nil
	}
	checkRegex := func(where string, p *string) error {
		if p == nil || strings.Contains(*p, "{{") {
			return nil
		}
		if _, err := regexp.Compile(*p); err != nil {
			return fmt.Errorf("%s: invalid regex %q: %w", where, *p, err)
		}
		return nil
	}

	for expr, a := range req.Assert.JSONPath {
		if err := checkPath("assert.jsonpath", expr); err != nil {
			return err
		}
		if err := checkRegex("assert.jsonpath["+expr+"].matches", a.Matches); err != nil {
			return err
		}
		if err := checkRegex("assert.jsonpath["+expr+"].not_matches", a.NotMatches); err != nil {
			return err
		}
	}
	for name, a := range req.Assert.Headers {
		if err := checkRegex("assert.headers["+name+"].matches", a.Matches); err != nil {
			return err
		}
		if err := checkRegex("assert.headers["+name+"].not_matches", a.NotMatches); err != nil {
			return err
		}
	}
	if b := req.Assert.Body; b != nil {
		if err := checkRegex("assert.body.matches", b.Matches); err != nil {
			return err
		}
		if err := checkRegex("assert.body.not_matches", b.NotMatches); err != nil {
			return err
		}
	}
	for name, expr := range req.Extract {
		if err := checkPath("extract."+name, expr); err != nil {
			return err
		}
	}
	return nil
}
