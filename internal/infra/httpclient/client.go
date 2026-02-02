package httpclient

import (
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

	return &http.Client{
		Transport: tr,
		Timeout:   cfg.Timeout,
	}
}
