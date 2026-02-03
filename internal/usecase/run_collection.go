package usecase

import (
	"context"
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
}

func NewRunCollection(cl ports.CollectionLoader, el ports.EnvironmentLoader, rr ports.RequestRunner) *RunCollection {
	return &RunCollection{
		collections: cl,
		envs:        el,
		runner:      rr,
	}
}

func (uc *RunCollection) Execute(ctx context.Context, collectionPath string, envNameOrPath string) (domain.RunResult, error) {
	col, err := uc.collections.LoadCollection(collectionPath)
	if err != nil {
		return domain.RunResult{}, err
	}

	env, err := uc.envs.LoadEnvironment(envNameOrPath)
	if err != nil {
		return domain.RunResult{}, err
	}

	// collection vars < env vars < extracted runtime vars (updated per request)
	vars := mergeVars(col.Vars, env.Vars)

	run := domain.RunResult{
		CollectionName:  col.Name,
		CollectionPath:  collectionPath,
		EnvironmentName: env.Name,
		StartedAt:       time.Now(),
		Results:         make([]domain.RequestResult, 0, len(col.Requests)),
	}

	for _, req := range col.Requests {
		rr, runErr := uc.runner.Run(ctx, req, vars)
		if runErr != nil {
			// Runner error (config-level): continue but mark the request as failed.
			run.Results = append(run.Results, domain.RequestResult{
				Name:       req.Name,
				Method:     req.Method,
				URL:        req.URL,
				Assertions: []domain.AssertionResult{},
				Extracts:   []domain.ExtractResult{},
				Extracted:  domain.Vars{},
				Response: domain.ResponseSnapshot{
					Headers: map[string][]string{},
				},
				Error: domain.NewRunError(runErr),
			})
			continue
		}

		// Assertions (always evaluated, even if rr.Error != nil)
		rr.Assertions = ucassert.Evaluate(req.Assert, rr.StatusCode, rr.LatencyMS, rr.Response.Body)

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
	return run, nil
}

func mergeVars(collectionVars domain.Vars, envVars domain.Vars) domain.Vars {
	out := domain.Vars{}

	// collection first
	for k, v := range collectionVars {
		out[k] = v
	}
	// env overrides collection
	for k, v := range envVars {
		out[k] = v
	}
	return out
}
