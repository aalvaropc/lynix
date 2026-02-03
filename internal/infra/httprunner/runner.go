package httprunner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aalvaropc/lynix/internal/domain"
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

	httpReq, err := r.buildHTTPRequest(ctx, resolved)
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

func (r *Runner) buildHTTPRequest(ctx context.Context, req domain.RequestSpec) (*http.Request, error) {
	u := strings.TrimSpace(req.URL)
	if u == "" {
		return nil, &domain.OpError{
			Op:   "httprunner.build",
			Kind: domain.KindInvalidConfig,
			Err:  errors.New("empty url"),
		}
	}

	var body io.Reader
	headers := http.Header{}
	for k, v := range req.Headers {
		headers.Set(k, v)
	}

	switch req.Body.Type {
	case domain.BodyJSON:
		if req.Body.JSON != nil {
			b, err := json.Marshal(req.Body.JSON)
			if err != nil {
				return nil, &domain.OpError{
					Op:   "httprunner.build.json",
					Kind: domain.KindInvalidConfig,
					Err:  err,
				}
			}
			body = bytes.NewReader(b)
			if req.Body.ContentType != "" {
				headers.Set("Content-Type", req.Body.ContentType)
			} else if headers.Get("Content-Type") == "" {
				headers.Set("Content-Type", "application/json")
			}
		}

	case domain.BodyForm:
		if req.Body.Form != nil {
			vals := url.Values{}
			for k, v := range req.Body.Form {
				vals.Set(k, v)
			}
			encoded := vals.Encode()
			body = strings.NewReader(encoded)
			if req.Body.ContentType != "" {
				headers.Set("Content-Type", req.Body.ContentType)
			} else if headers.Get("Content-Type") == "" {
				headers.Set("Content-Type", "application/x-www-form-urlencoded")
			}
		}

	case domain.BodyRaw:
		if req.Body.Raw != "" {
			body = strings.NewReader(req.Body.Raw)
			if req.Body.ContentType != "" && headers.Get("Content-Type") == "" {
				headers.Set("Content-Type", req.Body.ContentType)
			}
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, string(req.Method), u, body)
	if err != nil {
		return nil, &domain.OpError{
			Op:   "httprunner.build",
			Kind: domain.KindInvalidConfig,
			Err:  err,
		}
	}
	httpReq.Header = headers
	return httpReq, nil
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
