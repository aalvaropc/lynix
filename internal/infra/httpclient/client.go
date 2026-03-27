package httpclient

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"time"
)

type Config struct {
	// Total timeout for the entire request (includes redirects, reading body, etc).
	// A context deadline can still override this.
	Timeout time.Duration

	// Transport / dial timeouts.
	DialTimeout     time.Duration
	KeepAlive       time.Duration
	TLSHandshake    time.Duration
	ResponseHeader  time.Duration
	ExpectContinue  time.Duration
	IdleConnTimeout time.Duration

	MaxIdleConns        int
	MaxIdleConnsPerHost int

	// Insecure skips TLS certificate verification (for self-signed certs).
	Insecure bool

	// NoFollowRedirects disables HTTP redirect following globally.
	NoFollowRedirects bool
}

func DefaultConfig() Config {
	return Config{
		Timeout:             30 * time.Second,
		DialTimeout:         5 * time.Second,
		KeepAlive:           30 * time.Second,
		TLSHandshake:        5 * time.Second,
		ResponseHeader:      10 * time.Second,
		ExpectContinue:      1 * time.Second,
		IdleConnTimeout:     90 * time.Second,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
	}
}

// contextKey is an unexported type for context keys in this package.
type contextKey struct{ name string }

var noRedirectKey = &contextKey{"no-redirect"}

// ContextWithNoRedirect returns a context that instructs the HTTP client
// to not follow redirects for the request using this context.
func ContextWithNoRedirect(ctx context.Context) context.Context {
	return context.WithValue(ctx, noRedirectKey, true)
}

func noRedirectFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(noRedirectKey).(bool)
	return v
}

func New(cfg Config) *http.Client {
	dialer := &net.Dialer{
		Timeout:   cfg.DialTimeout,
		KeepAlive: cfg.KeepAlive,
	}

	tr := &http.Transport{
		Proxy:       http.ProxyFromEnvironment,
		DialContext: dialer.DialContext,

		ForceAttemptHTTP2: true,

		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:     cfg.IdleConnTimeout,

		TLSHandshakeTimeout:   cfg.TLSHandshake,
		ResponseHeaderTimeout: cfg.ResponseHeader,
		ExpectContinueTimeout: cfg.ExpectContinue,
	}

	if cfg.Insecure {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // user-requested via --insecure
	}

	noFollow := cfg.NoFollowRedirects
	return &http.Client{
		Transport: tr,
		Timeout:   cfg.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if noFollow || noRedirectFromContext(req.Context()) {
				return http.ErrUseLastResponse
			}
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			return nil
		},
	}
}
