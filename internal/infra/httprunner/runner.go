package httprunner

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
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
	log          *slog.Logger
}

type Option func(*Runner)

func WithMaxBodyBytes(n int64) Option {
	return func(r *Runner) { r.maxBodyBytes = n }
}

func WithResolver(vr *domain.VarResolver) Option {
	return func(r *Runner) { r.resolver = vr }
}

// WithLogger sets a structured logger for the runner.
func WithLogger(log *slog.Logger) Option {
	return func(r *Runner) { r.log = log }
}

func New(client *http.Client, opts ...Option) *Runner {
	r := &Runner{
		client:       client,
		maxBodyBytes: defaultMaxBodyBytes,
		resolver:     domain.NewVarResolver(),
		log:          slog.New(slog.NewJSONHandler(io.Discard, nil)),
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
		RequestBody:    serializeBody(resolved.Body),
		Extracted:      domain.Vars{},
		Extracts:       []domain.ExtractResult{},
		Assertions:     []domain.AssertionResult{},
		Response: domain.ResponseSnapshot{
			Headers: map[string][]string{},
		},
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

func serializeBody(body domain.BodySpec) []byte {
	switch body.Type {
	case domain.BodyJSON:
		if body.JSON != nil {
			b, err := json.Marshal(body.JSON)
			if err != nil {
				return nil
			}
			return b
		}
	case domain.BodyForm:
		if body.Form != nil {
			vals := make([]string, 0, len(body.Form))
			for k, v := range body.Form {
				vals = append(vals, k+"="+v)
			}
			return []byte(strings.Join(vals, "&"))
		}
	case domain.BodyRaw:
		if body.Raw != "" {
			return []byte(body.Raw)
		}
	}
	return nil
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
