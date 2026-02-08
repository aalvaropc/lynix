package httpclient

import (
	"context"
	"io"
	"net/http"
	"time"
)

// ResponseData captures the response details and duration.
type ResponseData struct {
	Status    int
	Headers   http.Header
	BodyBytes []byte
	Duration  time.Duration
}

// Executor executes HTTP requests with timing.
type Executor struct {
	client  *http.Client
	timeout time.Duration
}

// ExecutorOption allows configuring an Executor.
type ExecutorOption func(*Executor)

// WithTimeout sets the default timeout applied to requests.
func WithTimeout(timeout time.Duration) ExecutorOption {
	return func(e *Executor) { e.timeout = timeout }
}

// WithClient sets a custom HTTP client.
func WithClient(client *http.Client) ExecutorOption {
	return func(e *Executor) { e.client = client }
}

// NewExecutor builds an Executor with a default client and timeout.
func NewExecutor(opts ...ExecutorOption) *Executor {
	cfg := DefaultConfig()
	e := &Executor{
		client:  New(cfg),
		timeout: cfg.Timeout,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Do executes the request and returns response data plus duration.
func (e *Executor) Do(ctx context.Context, req *http.Request) (ResponseData, error) {
	start := time.Now()
	ctxWithTimeout := ctx
	cancel := func() {}
	if e.timeout > 0 {
		ctxWithTimeout, cancel = context.WithTimeout(ctx, e.timeout)
	}
	defer cancel()

	resp, err := e.client.Do(req.WithContext(ctxWithTimeout))
	duration := time.Since(start)
	if err != nil {
		return ResponseData{Duration: duration}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ResponseData{Duration: duration}, err
	}

	return ResponseData{
		Status:    resp.StatusCode,
		Headers:   resp.Header.Clone(),
		BodyBytes: body,
		Duration:  duration,
	}, nil
}
