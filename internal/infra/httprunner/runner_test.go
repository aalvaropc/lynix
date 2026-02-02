package httprunner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/infra/httpclient"
)

func TestRunner_TruncatesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Test", "1")
		w.WriteHeader(http.StatusOK)
		// Produce > 256KB
		w.Write([]byte(strings.Repeat("a", 300*1024)))
	}))
	defer srv.Close()

	c := httpclient.New(httpclient.DefaultConfig())
	r := New(c) // default 256KB

	req := domain.RequestSpec{
		Name:   "big",
		Method: domain.MethodGet,
		URL:    srv.URL,
		Headers: domain.Headers{
			"Accept": "text/plain",
		},
		Body: domain.BodySpec{Type: domain.BodyNone},
	}

	res, err := r.Run(context.Background(), req, domain.Vars{})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if res.Error != nil {
		t.Fatalf("expected no run error, got: %+v", res.Error)
	}
	if res.StatusCode != 200 {
		t.Fatalf("expected 200, got=%d", res.StatusCode)
	}
	if !res.Response.Truncated {
		t.Fatalf("expected truncated=true")
	}
	if len(res.Response.Body) != 256*1024 {
		t.Fatalf("expected body len=256KB, got=%d", len(res.Response.Body))
	}
	if res.Response.Headers["X-Test"][0] != "1" {
		t.Fatalf("expected header X-Test=1")
	}
}

func TestRunner_ClassifiesTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := httpclient.DefaultConfig()
	cfg.Timeout = 50 * time.Millisecond
	c := httpclient.New(cfg)
	r := New(c)

	req := domain.RequestSpec{
		Name:    "slow",
		Method:  domain.MethodGet,
		URL:     srv.URL,
		Body:    domain.BodySpec{Type: domain.BodyNone},
		Headers: domain.Headers{},
	}

	res, err := r.Run(context.Background(), req, domain.Vars{})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if res.Error == nil {
		t.Fatalf("expected a run error")
	}
	if res.Error.Kind != domain.RunErrorTimeout {
		t.Fatalf("expected timeout kind, got=%s (msg=%s)", res.Error.Kind, res.Error.Message)
	}
}
