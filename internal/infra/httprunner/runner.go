package httprunner

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/infra/httpclient"
	"github.com/aalvaropc/lynix/internal/ports"
)

const defaultMaxBodyBytes = 256 * 1024 // 256KB

// drainLimitBytes bounds how much of a truncated body is discarded to keep
// the connection reusable (an endless stream must not block the run).
const drainLimitBytes = 1 << 20 // 1MB

// defaultRequestTimeout applies when a request has no timeout_ms. It lives in
// the runner (as a context deadline) rather than http.Client.Timeout so an
// explicit larger timeout_ms is not silently capped by the client.
const defaultRequestTimeout = 30 * time.Second

type Runner struct {
	client         *http.Client
	maxBodyBytes   int64
	requestTimeout time.Duration
	resolver       *domain.VarResolver
	log            *slog.Logger
}

type Option func(*Runner)

func WithMaxBodyBytes(n int64) Option {
	return func(r *Runner) { r.maxBodyBytes = n }
}

// WithRequestTimeout sets the default per-request timeout used when a request
// does not declare timeout_ms. Zero disables the default.
func WithRequestTimeout(d time.Duration) Option {
	return func(r *Runner) { r.requestTimeout = d }
}

// WithLogger sets a structured logger for the runner.
func WithLogger(log *slog.Logger) Option {
	return func(r *Runner) { r.log = log }
}

func New(client *http.Client, opts ...Option) *Runner {
	r := &Runner{
		client:         client,
		maxBodyBytes:   defaultMaxBodyBytes,
		requestTimeout: defaultRequestTimeout,
		resolver:       domain.NewVarResolver(),
		log:            slog.New(slog.NewJSONHandler(io.Discard, nil)),
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
		Name:           resolved.Name,
		Method:         resolved.Method,
		URL:            resolved.URL,
		ResolvedURL:    resolved.URL,
		RequestHeaders: copyHeaders(resolved.Headers),
		RequestBody:    resolved.Body.Serialize(),
		Extracted:      domain.Vars{},
		Extracts:       []domain.ExtractResult{},
		Assertions:     []domain.AssertionResult{},
		Response: domain.ResponseSnapshot{
			Headers: map[string][]string{},
		},
	}

	// Per-request timeout: timeout_ms wins over the runner default.
	timeout := r.requestTimeout
	if req.TimeoutMS != nil && *req.TimeoutMS > 0 {
		timeout = time.Duration(*req.TimeoutMS) * time.Millisecond
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Per-request redirect control: an explicit follow_redirects overrides
	// the global --no-redirects flag in both directions.
	if req.FollowRedirects != nil {
		ctx = httpclient.ContextWithRedirectOverride(ctx, *req.FollowRedirects)
	}

	httpReq, err := httpclient.BuildRequest(ctx, resolved)
	if err != nil {
		return domain.RequestResult{}, err
	}

	r.log.Debug("httprunner.request",
		"name", resolved.Name,
		"method", string(resolved.Method),
		"url", resolved.URL,
	)

	start := time.Now()
	resp, err := r.client.Do(httpReq)
	lat := time.Since(start)
	result.LatencyMS = lat.Milliseconds()

	if err != nil {
		r.log.Debug("httprunner.request.error",
			"name", resolved.Name,
			"err", err,
			"latency_ms", result.LatencyMS,
		)
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

	// Drain (bounded) what remains of a truncated body so the connection
	// can be reused instead of being torn down.
	if truncated {
		_, _ = io.CopyN(io.Discard, resp.Body, drainLimitBytes)
	}

	r.log.Debug("httprunner.request.done",
		"name", resolved.Name,
		"status", result.StatusCode,
		"latency_ms", result.LatencyMS,
		"body_bytes", len(body),
		"truncated", truncated,
	)

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

func cloneHeaders(h http.Header) map[string][]string {
	out := make(map[string][]string, len(h))
	for k, v := range h {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}
