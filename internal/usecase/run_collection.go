package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/infra/httprunner"
	"github.com/aalvaropc/lynix/internal/ports"
	ucassert "github.com/aalvaropc/lynix/internal/usecase/assert"
	ucextract "github.com/aalvaropc/lynix/internal/usecase/extract"
)

// RunOpts groups behavioral parameters for RunCollection.
type RunOpts struct {
	FailFast   bool
	Only       []string
	Tags       []string
	Retries    int
	RetryDelay time.Duration
	Retry5xx   bool
	DryRun     bool
	Parallel   bool
}

type RunCollection struct {
	collections ports.CollectionLoader
	envs        ports.EnvironmentLoader
	runner      ports.RequestRunner
	store       ports.ArtifactStore // optional (can be nil)
	failFast    bool
	only        []string
	tags        []string
	retries     int
	retryDelay  time.Duration
	retry5xx    bool
	dryRun      bool
	parallel    bool
}

func NewRunCollection(
	cl ports.CollectionLoader,
	el ports.EnvironmentLoader,
	rr ports.RequestRunner,
	store ports.ArtifactStore,
	opts RunOpts,
) *RunCollection {
	return &RunCollection{
		collections: cl,
		envs:        el,
		runner:      rr,
		store:       store,
		failFast:    opts.FailFast,
		only:        opts.Only,
		tags:        opts.Tags,
		retries:     opts.Retries,
		retryDelay:  opts.RetryDelay,
		retry5xx:    opts.Retry5xx,
		dryRun:      opts.DryRun,
		parallel:    opts.Parallel,
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

	col.Requests, err = uc.filterRequests(col.Requests)
	if err != nil {
		return domain.RunResult{}, "", err
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

	if uc.parallel && !uc.dryRun {
		if err := uc.executeParallel(ctx, col.Requests, vars, schemaCache, &run); err != nil {
			run.EndedAt = time.Now()
			return run, "", err
		}
		run.EndedAt = time.Now()

		// Persist artifact (optional; skip in dry-run mode).
		if uc.store == nil || uc.dryRun {
			return run, "", nil
		}
		id, err := uc.store.SaveRun(run)
		if err != nil {
			return run, "", err
		}
		return run, id, nil
	}

	for i, req := range col.Requests {
		if err := ctx.Err(); err != nil {
			run.EndedAt = time.Now()
			return run, "", err
		}

		if uc.dryRun {
			rr, resolveErr := uc.resolveOnly(vars, req)
			if resolveErr != nil {
				rr.Error = domain.NewRunError(resolveErr)
			}
			run.Results = append(run.Results, rr)
			continue
		}

		if req.DelayMS != nil && *req.DelayMS > 0 {
			select {
			case <-ctx.Done():
				run.EndedAt = time.Now()
				return run, "", ctx.Err()
			case <-time.After(time.Duration(*req.DelayMS) * time.Millisecond):
			}
		}

		rr, runErr := uc.runWithRetries(ctx, req, vars)
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
				Error:    domain.NewRunError(runErr),
				Attempts: 1,
			})
			if uc.failFast {
				break
			}
			continue
		}

		// Assertions (always evaluated, even if rr.Error != nil)
		rr.Assertions = ucassert.Evaluate(req.Assert, rr.StatusCode, rr.LatencyMS, rr.Response.Body, schemaCache[i], rr.Response.Headers)

		extracted, extractResults := ucextract.Apply(rr.Response.Body, req.Extract)
		headerExtracted, headerExtractResults := ucextract.ApplyHeaders(rr.Response.Headers, req.ExtractHeaders)
		rr.Extracts = append(extractResults, headerExtractResults...)
		rr.Extracted = extracted

		// Merge header-extracted vars into extracted map.
		for k, v := range headerExtracted {
			rr.Extracted[k] = v
		}

		// Update runtime vars for next request (even if extract had failures, extracted map may be partial).
		for k, v := range rr.Extracted {
			vars[k] = v
		}

		run.Results = append(run.Results, rr)

		if uc.failFast && rr.Failed() {
			break
		}
	}

	run.EndedAt = time.Now()

	if err := ctx.Err(); err != nil {
		return run, "", err
	}

	// Persist artifact (optional; skip in dry-run mode).
	if uc.store == nil || uc.dryRun {
		return run, "", nil
	}

	id, err := uc.store.SaveRun(run)
	if err != nil {
		// Return run + error so callers can still inspect result if they want.
		return run, "", err
	}

	return run, id, nil
}

// runWithRetries wraps uc.runner.Run with retry logic for transient errors.
func (uc *RunCollection) runWithRetries(
	ctx context.Context,
	req domain.RequestSpec,
	vars domain.Vars,
) (domain.RequestResult, error) {
	maxAttempts := 1 + uc.retries
	var rr domain.RequestResult
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return rr, err
		}

		var runErr error
		rr, runErr = uc.runner.Run(ctx, req, vars)
		rr.Attempts = attempt

		// Config-level error: never retry.
		if runErr != nil {
			return rr, runErr
		}

		// Check if the result is retryable.
		shouldRetry := false
		if rr.Error != nil && domain.IsRetryable(rr.Error.Kind) {
			shouldRetry = true
		}
		if uc.retry5xx && rr.StatusCode >= 500 {
			shouldRetry = true
		}

		if !shouldRetry || attempt == maxAttempts {
			return rr, nil
		}

		// Wait before retrying (interruptible via context).
		if uc.retryDelay > 0 {
			select {
			case <-ctx.Done():
				return rr, ctx.Err()
			case <-time.After(uc.retryDelay):
			}
		}
	}
	return rr, nil
}

// filterRequests applies --only and --tags filtering.
// --only names must exist in the collection (error on typo).
// --tags with no match returns 0 results (not an error).
// Combination is intersection: request must match both.
func (uc *RunCollection) filterRequests(requests []domain.RequestSpec) ([]domain.RequestSpec, error) {
	if len(uc.only) == 0 && len(uc.tags) == 0 {
		return requests, nil
	}

	// Validate --only names exist.
	if len(uc.only) > 0 {
		nameSet := make(map[string]bool, len(requests))
		for _, r := range requests {
			nameSet[r.Name] = true
		}
		for _, name := range uc.only {
			if !nameSet[name] {
				return nil, fmt.Errorf("--only: request %q not found in collection", name)
			}
		}
	}

	onlySet := make(map[string]bool, len(uc.only))
	for _, n := range uc.only {
		onlySet[n] = true
	}

	tagsSet := make(map[string]bool, len(uc.tags))
	for _, t := range uc.tags {
		tagsSet[t] = true
	}

	var filtered []domain.RequestSpec
	for _, r := range requests {
		matchOnly := len(onlySet) == 0 || onlySet[r.Name]
		matchTags := len(tagsSet) == 0 || hasAnyTag(r.Tags, tagsSet)
		if matchOnly && matchTags {
			filtered = append(filtered, r)
		}
	}
	return filtered, nil
}

func hasAnyTag(tags []string, wanted map[string]bool) bool {
	for _, t := range tags {
		if wanted[t] {
			return true
		}
	}
	return false
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

// resolveOnly resolves variables in a request without executing it (dry-run mode).
func (uc *RunCollection) resolveOnly(vars domain.Vars, req domain.RequestSpec) (domain.RequestResult, error) {
	resolver := domain.NewVarResolver()
	rt, err := resolver.NewRuntime(vars)
	if err != nil {
		return domain.RequestResult{Name: req.Name, Method: req.Method, URL: req.URL}, err
	}

	resolved, err := rt.ResolveRequest(req)
	if err != nil {
		return domain.RequestResult{Name: req.Name, Method: req.Method, URL: req.URL}, err
	}

	return domain.RequestResult{
		Name:           resolved.Name,
		Method:         resolved.Method,
		URL:            resolved.URL,
		ResolvedURL:    resolved.URL,
		RequestHeaders: copyHeaders(resolved.Headers),
		RequestBody:    httprunner.SerializeBody(resolved.Body),
		Assertions:     []domain.AssertionResult{},
		Extracts:       []domain.ExtractResult{},
		Extracted:      domain.Vars{},
		Response:       domain.ResponseSnapshot{Headers: map[string][]string{}},
	}, nil
}

// executeParallel runs requests grouped by dependency levels.
// Independent requests within each level run concurrently.
func (uc *RunCollection) executeParallel(
	ctx context.Context,
	requests []domain.RequestSpec,
	vars domain.Vars,
	schemaCache map[int][]byte,
	run *domain.RunResult,
) error {
	graph := domain.BuildDepGraph(requests, vars)
	results := make([]domain.RequestResult, len(requests))

	for _, level := range graph.Levels {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Snapshot vars for this level — goroutines only read from this.
		levelVars := cloneVars(vars)

		var g *errgroup.Group
		var gctx context.Context
		if uc.failFast {
			g, gctx = errgroup.WithContext(ctx)
		} else {
			g, gctx = errgroup.WithContext(ctx)
		}

		for _, idx := range level {
			idx := idx // capture for goroutine
			req := requests[idx]

			g.Go(func() error {
				if req.DelayMS != nil && *req.DelayMS > 0 {
					select {
					case <-gctx.Done():
						return gctx.Err()
					case <-time.After(time.Duration(*req.DelayMS) * time.Millisecond):
					}
				}

				rr, runErr := uc.runWithRetries(gctx, req, levelVars)
				if runErr != nil {
					results[idx] = domain.RequestResult{
						Name:           req.Name,
						Method:         req.Method,
						URL:            req.URL,
						RequestHeaders: map[string]string{},
						Assertions:     []domain.AssertionResult{},
						Extracts:       []domain.ExtractResult{},
						Extracted:      domain.Vars{},
						Response:       domain.ResponseSnapshot{Headers: map[string][]string{}},
						Error:          domain.NewRunError(runErr),
						Attempts:       1,
					}
					if uc.failFast {
						return fmt.Errorf("request %q failed: %w", req.Name, runErr)
					}
					return nil
				}

				rr.Assertions = ucassert.Evaluate(req.Assert, rr.StatusCode, rr.LatencyMS, rr.Response.Body, schemaCache[idx], rr.Response.Headers)

				extracted, extractResults := ucextract.Apply(rr.Response.Body, req.Extract)
				headerExtracted, headerExtractResults := ucextract.ApplyHeaders(rr.Response.Headers, req.ExtractHeaders)
				rr.Extracts = append(extractResults, headerExtractResults...)
				rr.Extracted = extracted
				for k, v := range headerExtracted {
					rr.Extracted[k] = v
				}

				results[idx] = rr

				if uc.failFast && rr.Failed() {
					return fmt.Errorf("request %q failed assertions", req.Name)
				}
				return nil
			})
		}

		if err := g.Wait(); err != nil && uc.failFast {
			// Merge what we have and stop.
			break
		}

		// Single-threaded merge of extracted vars for the next level.
		for _, idx := range level {
			for k, v := range results[idx].Extracted {
				vars[k] = v
			}
		}
	}

	// Copy results in original order, skipping zero-value entries.
	for i := range results {
		if results[i].Name != "" {
			run.Results = append(run.Results, results[i])
		}
	}

	return nil
}

func cloneVars(v domain.Vars) domain.Vars {
	out := make(domain.Vars, len(v))
	for k, val := range v {
		out[k] = val
	}
	return out
}

func copyHeaders(h domain.Headers) map[string]string {
	if h == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		out[k] = v
	}
	return out
}
