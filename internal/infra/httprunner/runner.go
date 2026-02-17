package httprunner

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/infra/httpclient"
	"github.com/aalvaropc/lynix/internal/ports"
)

const defaultMaxBodyBytes = 256 * 1024 // 256KB

type Runner struct {
	client       *http.Client
	maxBodyBytes int64
	resolver     *domain.VarResolver
}

type Option func(*Runner)

func WithMaxBodyBytes(n int64) Option {
	return func(r *Runner) { r.maxBodyBytes = n }
}

func WithResolver(vr *domain.VarResolver) Option {
	return func(r *Runner) { r.resolver = vr }
}

func New(client *http.Client, opts ...Option) *Runner {
	r := &Runner{
		client:       client,
		maxBodyBytes: defaultMaxBodyBytes,
		resolver:     domain.NewVarResolver(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

var _ ports.RequestRunner = (*Runner)(nil)

func (r *Runner) Run(ctx context.Context, req domain.RequestSpec, vars domain.Vars) (domain.RequestResult, error) {
	rt, err := r.resolver.NewRuntime(vars)
	if err != nil {
		return domain.RequestResult{}, err
	}

	resolved, err := rt.ResolveRequest(req)
	if err != nil {
		// Config-level issue: missing var, invalid placeholder, etc.
		return domain.RequestResult{}, err
	}

	result := domain.RequestResult{
		Name:       resolved.Name,
		Method:     resolved.Method,
		URL:        resolved.URL,
		Extracted:  domain.Vars{},
		Extracts:   []domain.ExtractResult{},
		Assertions: []domain.AssertionResult{},
		Response: domain.ResponseSnapshot{
			Headers: map[string][]string{},
		},
	}

	httpReq, err := httpclient.BuildRequest(ctx, resolved)
	if err != nil {
		return domain.RequestResult{}, err
	}

	start := time.Now()
	resp, err := r.client.Do(httpReq)
	lat := time.Since(start)
	result.LatencyMS = lat.Milliseconds()

	if err != nil {
		result.Error = domain.NewRunError(err)
		return result, nil
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.Response.Headers = cloneHeaders(resp.Header)

	body, truncated, readErr := readBounded(resp.Body, r.maxBodyBytes)
	if readErr != nil {
		result.Error = domain.NewRunError(readErr)
		return result, nil
	}

	result.Response.Body = body
	result.Response.Truncated = truncated
	return result, nil
}

func readBounded(r io.Reader, maxBytes int64) ([]byte, bool, error) {
	lim := io.LimitReader(r, maxBytes+1)
	b, err := io.ReadAll(lim)
	if err != nil {
		return nil, false, err
	}
	if int64(len(b)) > maxBytes {
		return b[:maxBytes], true, nil
	}
	return b, false, nil
}

func cloneHeaders(h http.Header) map[string][]string {
	out := make(map[string][]string, len(h))
	for k, v := range h {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}
