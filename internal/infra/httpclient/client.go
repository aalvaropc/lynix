package httpclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
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

	// RootCAs adds trusted CAs (see LoadCAFile). Nil keeps the system pool.
	RootCAs *x509.CertPool

	// EnableCookieJar attaches an in-memory cookie jar so Set-Cookie
	// responses propagate to subsequent requests (session-based auth).
	EnableCookieJar bool

	// NoFollowRedirects disables HTTP redirect following globally.
	NoFollowRedirects bool
}

// LoadCAFile reads a PEM bundle and returns a cert pool extending the system
// roots, so a corporate CA does not require disabling verification entirely.
func LoadCAFile(path string) (*x509.CertPool, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read CA file: %w", err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("CA file %q contains no valid PEM certificates", path)
	}
	return pool, nil
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

var redirectOverrideKey = &contextKey{"redirect-override"}

// ContextWithRedirectOverride forces redirect behavior for the request using
// this context: an explicit per-request follow_redirects wins in BOTH
// directions over the global --no-redirects flag.
func ContextWithRedirectOverride(ctx context.Context, follow bool) context.Context {
	return context.WithValue(ctx, redirectOverrideKey, follow)
}

// ContextWithNoRedirect is a shorthand for ContextWithRedirectOverride(ctx, false).
func ContextWithNoRedirect(ctx context.Context) context.Context {
	return ContextWithRedirectOverride(ctx, false)
}

func redirectOverrideFromContext(ctx context.Context) (follow bool, ok bool) {
	v, ok := ctx.Value(redirectOverrideKey).(bool)
	return v, ok
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

	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.Insecure {
		tlsCfg.InsecureSkipVerify = true //nolint:gosec // user-requested via --insecure
	}
	if cfg.RootCAs != nil {
		tlsCfg.RootCAs = cfg.RootCAs
	}
	tr.TLSClientConfig = tlsCfg

	var jar http.CookieJar
	if cfg.EnableCookieJar {
		// cookiejar.New with default options never returns an error.
		jar, _ = cookiejar.New(nil)
	}

	noFollow := cfg.NoFollowRedirects
	return &http.Client{
		Transport: tr,
		Jar:       jar,
		Timeout:   cfg.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if follow, ok := redirectOverrideFromContext(req.Context()); ok {
				if !follow {
					return http.ErrUseLastResponse
				}
				// explicit follow_redirects: true overrides the global flag
			} else if noFollow {
				return http.ErrUseLastResponse
			}
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			return nil
		},
	}
}
