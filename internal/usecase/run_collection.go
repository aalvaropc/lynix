package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/ports"
	ucassert "github.com/aalvaropc/lynix/internal/usecase/assert"
	ucextract "github.com/aalvaropc/lynix/internal/usecase/extract"
)

type RunCollection struct {
	collections ports.CollectionLoader
	envs        ports.EnvironmentLoader
	runner      ports.RequestRunner
	store       ports.ArtifactStore // optional (can be nil)
}

func NewRunCollection(
	cl ports.CollectionLoader,
	el ports.EnvironmentLoader,
	rr ports.RequestRunner,
	store ports.ArtifactStore,
) *RunCollection {
	return &RunCollection{
		collections: cl,
		envs:        el,
		runner:      rr,
		store:       store,
	}
}

// Execute runs a collection and (optionally) persists the artifact via ArtifactStore.
// Returns: run result, saved run ID ("" if not saved), error.
func (uc *RunCollection) Execute(
	ctx context.Context,
	collectionPath string,
	envNameOrPath string,
) (domain.RunResult, string, error) {
	col, err := uc.collections.LoadCollection(collectionPath)
	if err != nil {
		return domain.RunResult{}, "", err
	}

	env, err := uc.envs.LoadEnvironment(envNameOrPath)
	if err != nil {
		return domain.RunResult{}, "", err
	}

	// Pre-load schema files for requests that reference them.
	schemaCache := make(map[int][]byte) // request index → schema bytes
	for i, req := range col.Requests {
		sb, err := loadSchemaBytes(req.Assert)
		if err != nil {
			return domain.RunResult{}, "", fmt.Errorf("request %q: %w", req.Name, err)
		}
		if sb != nil {
			schemaCache[i] = sb
		}
	}

	// collection vars < env vars < extracted runtime vars (updated per request)
	vars := domain.Merge(col.Vars, env.Vars)

	run := domain.RunResult{
		CollectionName:  col.Name,
		CollectionPath:  collectionPath,
		EnvironmentName: env.Name,
		StartedAt:       time.Now(),
		Results:         make([]domain.RequestResult, 0, len(col.Requests)),
	}

	for i, req := range col.Requests {
		if err := ctx.Err(); err != nil {
			run.EndedAt = time.Now()
			return run, "", err
		}

		rr, runErr := uc.runner.Run(ctx, req, vars)
		if runErr != nil {
			// Runner error (config-level): continue but mark the request as failed.
			run.Results = append(run.Results, domain.RequestResult{
				Name:           req.Name,
				Method:         req.Method,
				URL:            req.URL,
				RequestHeaders: map[string]string{},
				Assertions:     []domain.AssertionResult{},
				Extracts:       []domain.ExtractResult{},
				Extracted:      domain.Vars{},
				Response: domain.ResponseSnapshot{
					Headers: map[string][]string{},
				},
				Error: domain.NewRunError(runErr),
			})
			continue
		}

		// Assertions (always evaluated, even if rr.Error != nil)
		rr.Assertions = ucassert.Evaluate(req.Assert, rr.StatusCode, rr.LatencyMS, rr.Response.Body, schemaCache[i])

		extracted, extractResults := ucextract.Apply(rr.Response.Body, req.Extract)
		rr.Extracts = extractResults
		rr.Extracted = extracted

		// Update runtime vars for next request (even if extract had failures, extracted map may be partial).
		for k, v := range extracted {
			vars[k] = v
		}

		run.Results = append(run.Results, rr)
	}

	run.EndedAt = time.Now()

	if err := ctx.Err(); err != nil {
		return run, "", err
	}

	// Persist artifact (optional).
	if uc.store == nil {
		return run, "", nil
	}

	id, err := uc.store.SaveRun(run)
	if err != nil {
		// Return run + error so callers can still inspect result if they want.
		return run, "", err
	}

	return run, id, nil
}

// loadSchemaBytes resolves the JSON Schema bytes from a file path or inline definition.
func loadSchemaBytes(spec domain.AssertionsSpec) ([]byte, error) {
	if spec.Schema != nil {
		b, err := os.ReadFile(*spec.Schema)
		if err != nil {
			return nil, fmt.Errorf("schema file %q: %w", *spec.Schema, err)
		}
		return b, nil
	}
	if spec.SchemaInline != nil {
		b, err := json.Marshal(spec.SchemaInline)
		if err != nil {
			return nil, fmt.Errorf("schema_inline: %w", err)
		}
		return b, nil
	}
	return nil, nil
}
