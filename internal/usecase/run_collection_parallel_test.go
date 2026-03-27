package usecase

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/infra/httpclient"
	"github.com/aalvaropc/lynix/internal/infra/httprunner"
	"github.com/aalvaropc/lynix/internal/ports"
)

// safeCallCounter tracks concurrent runner calls in a thread-safe way.
type safeCallCounter struct {
	inner ports.RequestRunner
	calls atomic.Int32
	max   atomic.Int32
	mu    sync.Mutex
	cur   int32
}

func (s *safeCallCounter) Run(ctx context.Context, req domain.RequestSpec, vars domain.Vars) (domain.RequestResult, error) {
	s.calls.Add(1)
	s.mu.Lock()
	s.cur++
	cur := s.cur
	s.mu.Unlock()

	// Track max concurrency.
	for {
		old := s.max.Load()
		if cur <= old || s.max.CompareAndSwap(old, cur) {
			break
		}
	}

	res, err := s.inner.Run(ctx, req, vars)

	s.mu.Lock()
	s.cur--
	s.mu.Unlock()

	return res, err
}

func TestParallel_AllIndependent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	col := domain.Collection{
		Name: "parallel",
		Vars: domain.Vars{"base": srv.URL},
		Requests: []domain.RequestSpec{
			{Name: "a", Method: domain.MethodGet, URL: "{{base}}/a", Body: domain.BodySpec{Type: domain.BodyNone}},
			{Name: "b", Method: domain.MethodGet, URL: "{{base}}/b", Body: domain.BodySpec{Type: domain.BodyNone}},
			{Name: "c", Method: domain.MethodGet, URL: "{{base}}/c", Body: domain.BodySpec{Type: domain.BodyNone}},
		},
	}

	client := httpclient.New(httpclient.DefaultConfig())
	runner := httprunner.New(client)
	counter := &safeCallCounter{inner: runner}

	uc := NewRunCollection(
		fakeCollectionLoader{col: col},
		fakeEnvLoader{},
		counter,
		nil,
		RunOpts{Parallel: true},
	)

	run, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(run.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(run.Results))
	}
	if counter.calls.Load() != 3 {
		t.Errorf("expected 3 runner calls, got %d", counter.calls.Load())
	}
	for _, r := range run.Results {
		if r.Error != nil {
			t.Errorf("request %q: error: kind=%s msg=%s", r.Name, r.Error.Kind, r.Error.Message)
		}
		if r.StatusCode != 200 {
			t.Errorf("request %q: expected 200, got %d (url=%q resolved=%q)", r.Name, r.StatusCode, r.URL, r.ResolvedURL)
		}
	}
}

func TestParallel_LinearChain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"val":"extracted"}`))
	}))
	defer srv.Close()

	col := domain.Collection{
		Name: "chain",
		Vars: domain.Vars{"base": srv.URL},
		Requests: []domain.RequestSpec{
			{Name: "a", Method: domain.MethodGet, URL: "{{base}}/a", Body: domain.BodySpec{Type: domain.BodyNone}, Extract: domain.ExtractSpec{"x": "$.val"}},
			{Name: "b", Method: domain.MethodGet, URL: "{{base}}/{{x}}", Body: domain.BodySpec{Type: domain.BodyNone}, Extract: domain.ExtractSpec{"y": "$.val"}},
			{Name: "c", Method: domain.MethodGet, URL: "{{base}}/{{y}}", Body: domain.BodySpec{Type: domain.BodyNone}},
		},
	}

	client := httpclient.New(httpclient.DefaultConfig())
	runner := httprunner.New(client)

	uc := NewRunCollection(
		fakeCollectionLoader{col: col},
		fakeEnvLoader{},
		runner,
		nil,
		RunOpts{Parallel: true},
	)

	run, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(run.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(run.Results))
	}
	for _, r := range run.Results {
		if r.StatusCode != 200 {
			t.Errorf("request %q: expected 200, got %d (error: %v)", r.Name, r.StatusCode, r.Error)
		}
	}
}

func TestParallel_DiamondDeps(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"token":"abc"}`))
	}))
	defer srv.Close()

	col := domain.Collection{
		Name: "diamond",
		Vars: domain.Vars{"base": srv.URL},
		Requests: []domain.RequestSpec{
			{Name: "login", Method: domain.MethodPost, URL: "{{base}}/login", Body: domain.BodySpec{Type: domain.BodyNone}, Extract: domain.ExtractSpec{"token": "$.token"}},
			{Name: "profile", Method: domain.MethodGet, URL: "{{base}}/me", Body: domain.BodySpec{Type: domain.BodyNone}, Headers: domain.Headers{"Auth": "{{token}}"}},
			{Name: "settings", Method: domain.MethodGet, URL: "{{base}}/settings", Body: domain.BodySpec{Type: domain.BodyNone}, Headers: domain.Headers{"Auth": "{{token}}"}},
		},
	}

	client := httpclient.New(httpclient.DefaultConfig())
	runner := httprunner.New(client)

	uc := NewRunCollection(
		fakeCollectionLoader{col: col},
		fakeEnvLoader{},
		runner,
		nil,
		RunOpts{Parallel: true},
	)

	run, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(run.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(run.Results))
	}
	// All should succeed — login provides token, profile/settings consume it.
	for _, r := range run.Results {
		if r.StatusCode != 200 {
			t.Errorf("request %q: expected 200, got %d", r.Name, r.StatusCode)
		}
	}
}

func TestParallel_ResultsInOriginalOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	col := domain.Collection{
		Name: "order",
		Vars: domain.Vars{"base": srv.URL},
		Requests: []domain.RequestSpec{
			{Name: "first", Method: domain.MethodGet, URL: "{{base}}/1", Body: domain.BodySpec{Type: domain.BodyNone}},
			{Name: "second", Method: domain.MethodGet, URL: "{{base}}/2", Body: domain.BodySpec{Type: domain.BodyNone}},
			{Name: "third", Method: domain.MethodGet, URL: "{{base}}/3", Body: domain.BodySpec{Type: domain.BodyNone}},
		},
	}

	client := httpclient.New(httpclient.DefaultConfig())
	runner := httprunner.New(client)

	uc := NewRunCollection(
		fakeCollectionLoader{col: col},
		fakeEnvLoader{},
		runner,
		nil,
		RunOpts{Parallel: true},
	)

	run, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Results[0].Name != "first" || run.Results[1].Name != "second" || run.Results[2].Name != "third" {
		t.Errorf("results not in original order: %s, %s, %s",
			run.Results[0].Name, run.Results[1].Name, run.Results[2].Name)
	}
}

func TestParallel_ContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	col := domain.Collection{
		Name: "cancel",
		Vars: domain.Vars{"base": srv.URL},
		Requests: []domain.RequestSpec{
			{Name: "a", Method: domain.MethodGet, URL: "{{base}}/a", Body: domain.BodySpec{Type: domain.BodyNone}},
		},
	}

	client := httpclient.New(httpclient.DefaultConfig())
	runner := httprunner.New(client)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	uc := NewRunCollection(
		fakeCollectionLoader{col: col},
		fakeEnvLoader{},
		runner,
		nil,
		RunOpts{Parallel: true},
	)

	_, _, err := uc.Execute(ctx, "col.yaml", "env.yaml")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}
