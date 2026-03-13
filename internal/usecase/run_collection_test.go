package usecase

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/infra/httpclient"
	"github.com/aalvaropc/lynix/internal/infra/httprunner"
	"github.com/aalvaropc/lynix/internal/ports"
)

// --- fakes used by both integration and unit tests ---

type fakeCollectionLoader struct {
	col domain.Collection
}

func (f fakeCollectionLoader) LoadCollection(_ string) (domain.Collection, error) {
	return f.col, nil
}
func (f fakeCollectionLoader) ListCollections(_ string) ([]domain.CollectionRef, error) {
	return nil, nil
}

type fakeEnvLoader struct {
	env domain.Environment
}

func (f fakeEnvLoader) LoadEnvironment(_ string) (domain.Environment, error) {
	return f.env, nil
}

type fakeStore struct {
	saved bool
	last  domain.RunArtifact
}

func (s *fakeStore) SaveRun(run domain.RunArtifact) (string, error) {
	s.saved = true
	s.last = run
	return "run-123", nil
}

// --- stubs for unit tests ---

type errCollectionLoader struct{ err error }

func (e errCollectionLoader) LoadCollection(_ string) (domain.Collection, error) {
	return domain.Collection{}, e.err
}
func (e errCollectionLoader) ListCollections(_ string) ([]domain.CollectionRef, error) {
	return nil, nil
}

type errEnvLoader struct{ err error }

func (e errEnvLoader) LoadEnvironment(_ string) (domain.Environment, error) {
	return domain.Environment{}, e.err
}

// stubRunner returns a fixed result/error pair.
type stubRunner struct {
	result domain.RequestResult
	err    error
}

func (s *stubRunner) Run(_ context.Context, _ domain.RequestSpec, _ domain.Vars) (domain.RequestResult, error) {
	return s.result, s.err
}

// multiCallRunner returns a different result/error per call and captures vars passed.
type multiCallRunner struct {
	results      []domain.RequestResult
	errs         []error
	capturedVars []domain.Vars
	idx          int
}

func (m *multiCallRunner) Run(_ context.Context, _ domain.RequestSpec, vars domain.Vars) (domain.RequestResult, error) {
	snap := make(domain.Vars, len(vars))
	for k, v := range vars {
		snap[k] = v
	}
	m.capturedVars = append(m.capturedVars, snap)

	i := m.idx
	m.idx++
	var res domain.RequestResult
	var err error
	if i < len(m.results) {
		res = m.results[i]
	}
	if i < len(m.errs) {
		err = m.errs[i]
	}
	return res, err
}

// ctxCancelRunner cancels the given context on first Run call then returns a fixed result.
type ctxCancelRunner struct {
	cancel context.CancelFunc
	result domain.RequestResult
	called int
}

func (r *ctxCancelRunner) Run(_ context.Context, _ domain.RequestSpec, _ domain.Vars) (domain.RequestResult, error) {
	r.called++
	if r.called == 1 {
		r.cancel()
	}
	return r.result, nil
}

// errStore always fails SaveRun.
type errStore struct{ err error }

func (s *errStore) SaveRun(_ domain.RunArtifact) (string, error) { return "", s.err }

// --- RunCollection.Execute unit tests ---

func TestRunCollection_Execute_StoreNil(t *testing.T) {
	col := domain.Collection{
		Name: "test",
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://example.com"},
		},
	}
	runner := &stubRunner{result: domain.RequestResult{
		StatusCode: 200,
		Response:   domain.ResponseSnapshot{Body: []byte(`{}`)},
	}}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, nil, RunOpts{})

	run, id, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "" {
		t.Fatalf("expected empty id when store is nil, got %q", id)
	}
	if run.CollectionName != "test" {
		t.Fatalf("expected CollectionName=test, got %q", run.CollectionName)
	}
}

func TestRunCollection_Execute_StoreCalled(t *testing.T) {
	col := domain.Collection{
		Name: "test",
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://example.com"},
		},
	}
	runner := &stubRunner{result: domain.RequestResult{
		StatusCode: 200,
		Response:   domain.ResponseSnapshot{Body: []byte(`{}`)},
	}}
	store := &fakeStore{}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, store, RunOpts{})

	_, id, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "run-123" {
		t.Fatalf("expected id=run-123, got %q", id)
	}
	if !store.saved {
		t.Fatal("expected SaveRun to be called")
	}
}

func TestRunCollection_Execute_ErrorLoadingCollection(t *testing.T) {
	loadErr := errors.New("collection not found")
	uc := NewRunCollection(errCollectionLoader{err: loadErr}, fakeEnvLoader{}, &stubRunner{}, nil, RunOpts{})

	_, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err == nil {
		t.Fatal("expected error loading collection")
	}
	if !errors.Is(err, loadErr) {
		t.Fatalf("expected wrapped loadErr, got %v", err)
	}
}

func TestRunCollection_Execute_ErrorLoadingEnv(t *testing.T) {
	envErr := errors.New("env not found")
	uc := NewRunCollection(fakeCollectionLoader{}, errEnvLoader{err: envErr}, &stubRunner{}, nil, RunOpts{})

	_, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err == nil {
		t.Fatal("expected error loading environment")
	}
	if !errors.Is(err, envErr) {
		t.Fatalf("expected wrapped envErr, got %v", err)
	}
}

func TestRunCollection_Execute_RunnerError_ContinuesNext(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://a.com"},
			{Name: "req2", Method: domain.MethodGet, URL: "http://b.com"},
		},
	}
	runner := &multiCallRunner{
		results: []domain.RequestResult{
			{},
			{StatusCode: 200, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
		},
		errs: []error{errors.New("runner failed"), nil},
	}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, nil, RunOpts{})

	run, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(run.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(run.Results))
	}
	if run.Results[0].Error == nil {
		t.Fatal("expected first request to be marked as failed")
	}
	if run.Results[1].Error != nil {
		t.Fatalf("expected second request to succeed, got error: %v", run.Results[1].Error)
	}
}

func TestRunCollection_Execute_ContextCancelledBeforeFirstRequest(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://example.com"},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before Execute

	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, &stubRunner{}, nil, RunOpts{})
	_, _, err := uc.Execute(ctx, "col.yaml", "env.yaml")
	if err == nil {
		t.Fatal("expected context error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestRunCollection_Execute_ContextCancelledDuringIteration(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://a.com"},
			{Name: "req2", Method: domain.MethodGet, URL: "http://b.com"},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	runner := &ctxCancelRunner{
		cancel: cancel,
		result: domain.RequestResult{StatusCode: 200, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
	}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, nil, RunOpts{})

	run, _, err := uc.Execute(ctx, "col.yaml", "env.yaml")
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	// First request completed before cancellation was detected.
	if len(run.Results) != 1 {
		t.Fatalf("expected 1 result (second request skipped), got %d", len(run.Results))
	}
}

func TestRunCollection_Execute_StoreSaveError(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://example.com"},
		},
	}
	runner := &stubRunner{result: domain.RequestResult{
		StatusCode: 200,
		Response:   domain.ResponseSnapshot{Body: []byte(`{}`)},
	}}
	saveErr := errors.New("store unavailable")
	store := &errStore{err: saveErr}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, store, RunOpts{})

	run, id, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err == nil {
		t.Fatal("expected error from store.SaveRun")
	}
	if !errors.Is(err, saveErr) {
		t.Fatalf("expected wrapped saveErr, got %v", err)
	}
	if id != "" {
		t.Fatalf("expected empty id on store error, got %q", id)
	}
	// run should still be returned so caller can inspect results.
	if len(run.Results) != 1 {
		t.Fatalf("expected 1 result even on store error, got %d", len(run.Results))
	}
}

func TestRunCollection_Execute_VarChainingViaExtract(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{
				Name:    "req1",
				Method:  domain.MethodGet,
				URL:     "http://example.com/auth",
				Extract: domain.ExtractSpec{"token": "$.token"},
			},
			{
				Name:   "req2",
				Method: domain.MethodGet,
				URL:    "http://example.com/users",
			},
		},
	}
	runner := &multiCallRunner{
		results: []domain.RequestResult{
			{StatusCode: 200, Response: domain.ResponseSnapshot{Body: []byte(`{"token":"abc123"}`)}},
			{StatusCode: 200, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
		},
		errs: []error{nil, nil},
	}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, nil, RunOpts{})

	_, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runner.capturedVars) != 2 {
		t.Fatalf("expected 2 runner calls, got %d", len(runner.capturedVars))
	}
	// Second call should have the extracted token available.
	if runner.capturedVars[1]["token"] != "abc123" {
		t.Fatalf("expected token=abc123 in second request vars, got %q", runner.capturedVars[1]["token"])
	}
}

// --- integration tests (real HTTP) ---

func TestRunCollection_ExtractsAndChainsVars(t *testing.T) {
	token := "abc123"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"token":"` + token + `"}`))
		case "/users":
			if r.Header.Get("Authorization") != "Bearer "+token {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	col := domain.Collection{
		Name: "Demo",
		Vars: domain.Vars{
			"base_url": srv.URL,
		},
		Requests: []domain.RequestSpec{
			{
				Name:   "auth.token",
				Method: domain.MethodGet,
				URL:    "{{base_url}}/auth",
				Headers: domain.Headers{
					"Accept": "application/json",
				},
				Body: domain.BodySpec{Type: domain.BodyNone},
				Assert: domain.AssertionsSpec{
					Status: ptrInt(200),
				},
				Extract: domain.ExtractSpec{
					"auth.token": "$.token",
				},
			},
			{
				Name:   "users.list",
				Method: domain.MethodGet,
				URL:    "{{base_url}}/users",
				Headers: domain.Headers{
					"Authorization": "Bearer {{auth.token}}",
				},
				Body: domain.BodySpec{Type: domain.BodyNone},
				Assert: domain.AssertionsSpec{
					Status: ptrInt(200),
				},
			},
		},
	}

	env := domain.Environment{
		Name: "dev",
		Vars: domain.Vars{}, // no overrides
	}

	cfg := httpclient.DefaultConfig()
	cfg.Timeout = 2 * time.Second
	r := httprunner.New(httpclient.New(cfg))

	st := &fakeStore{}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{env: env}, r, st, RunOpts{})

	out, id, err := uc.Execute(context.Background(), "demo.yaml", "dev")
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if id != "run-123" {
		t.Fatalf("expected run id, got=%q", id)
	}
	if !st.saved {
		t.Fatalf("expected run saved")
	}
	if st.last.CollectionName != "Demo" {
		t.Fatalf("expected saved collection name Demo, got=%q", st.last.CollectionName)
	}

	if len(out.Results) != 2 {
		t.Fatalf("expected 2 results, got=%d", len(out.Results))
	}

	first := out.Results[0]
	if first.Error != nil {
		t.Fatalf("expected no error in first request, got=%+v", first.Error)
	}
	if first.Extracted["auth.token"] != token {
		t.Fatalf("expected extracted auth.token=%s, got=%q", token, first.Extracted["auth.token"])
	}
	if len(first.Extracts) != 1 || !first.Extracts[0].Success {
		t.Fatalf("expected extract success, got=%+v", first.Extracts)
	}

	second := out.Results[1]
	if second.StatusCode != 200 {
		t.Fatalf("expected second status=200, got=%d", second.StatusCode)
	}
	if second.Error != nil {
		t.Fatalf("expected no error in second request, got=%+v", second.Error)
	}
	if len(second.Assertions) != 1 || !second.Assertions[0].Passed {
		t.Fatalf("expected assertions pass, got=%+v", second.Assertions)
	}
}

func TestRunCollection_ExtractFail_AllowsNextRequestToFailMissingVar(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"no_token":true}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	col := domain.Collection{
		Name: "Demo",
		Vars: domain.Vars{"base_url": srv.URL},
		Requests: []domain.RequestSpec{
			{
				Name:   "auth.token",
				Method: domain.MethodGet,
				URL:    "{{base_url}}/auth",
				Body:   domain.BodySpec{Type: domain.BodyNone},
				Extract: domain.ExtractSpec{
					"auth.token": "$.token",
				},
			},
			{
				Name:   "users.list",
				Method: domain.MethodGet,
				URL:    "{{base_url}}/users",
				Headers: domain.Headers{
					"Authorization": "Bearer {{auth.token}}", // will be missing
				},
				Body: domain.BodySpec{Type: domain.BodyNone},
			},
		},
	}

	env := domain.Environment{Name: "dev", Vars: domain.Vars{}}

	cfg := httpclient.DefaultConfig()
	cfg.Timeout = 2 * time.Second
	r := httprunner.New(httpclient.New(cfg))

	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{env: env}, r, nil, RunOpts{})

	out, _, err := uc.Execute(context.Background(), "demo.yaml", "dev")
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if len(out.Results) != 2 {
		t.Fatalf("expected 2 results, got=%d", len(out.Results))
	}

	// First request: extract failed
	if len(out.Results[0].Extracts) != 1 || out.Results[0].Extracts[0].Success {
		t.Fatalf("expected extract failure, got=%+v", out.Results[0].Extracts)
	}

	// Second request: should fail due to missing var (runner returns error -> we store RunError)
	if out.Results[1].Error == nil {
		t.Fatalf("expected error in second request")
	}
}

// --- fail-fast tests ---

func TestRunCollection_Execute_FailFast_StopsOnRunnerError(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://a.com"},
			{Name: "req2", Method: domain.MethodGet, URL: "http://b.com"},
			{Name: "req3", Method: domain.MethodGet, URL: "http://c.com"},
		},
	}
	runner := &multiCallRunner{
		results: []domain.RequestResult{
			{},
			{StatusCode: 200, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
			{StatusCode: 200, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
		},
		errs: []error{errors.New("runner failed"), nil, nil},
	}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, nil, RunOpts{FailFast: true})

	run, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(run.Results) != 1 {
		t.Fatalf("expected 1 result (fail-fast after runner error), got %d", len(run.Results))
	}
	if run.Results[0].Error == nil {
		t.Fatal("expected first request to have error")
	}
}

func TestRunCollection_Execute_FailFast_StopsOnAssertionFailure(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://a.com",
				Assert: domain.AssertionsSpec{Status: ptrInt(200)}},
			{Name: "req2", Method: domain.MethodGet, URL: "http://b.com",
				Assert: domain.AssertionsSpec{Status: ptrInt(200)}},
			{Name: "req3", Method: domain.MethodGet, URL: "http://c.com"},
		},
	}
	runner := &multiCallRunner{
		results: []domain.RequestResult{
			{StatusCode: 200, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
			{StatusCode: 500, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
			{StatusCode: 200, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
		},
		errs: []error{nil, nil, nil},
	}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, nil, RunOpts{FailFast: true})

	run, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(run.Results) != 2 {
		t.Fatalf("expected 2 results (fail-fast after assertion failure), got %d", len(run.Results))
	}
	if !run.Results[1].Failed() {
		t.Fatal("expected second request to be marked as failed")
	}
}

func TestRunCollection_Execute_FailFast_StopsOnExtractFailure(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://a.com",
				Extract: domain.ExtractSpec{"token": "$.missing"}},
			{Name: "req2", Method: domain.MethodGet, URL: "http://b.com"},
		},
	}
	runner := &multiCallRunner{
		results: []domain.RequestResult{
			{StatusCode: 200, Response: domain.ResponseSnapshot{Body: []byte(`{"ok":true}`)}},
			{StatusCode: 200, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
		},
		errs: []error{nil, nil},
	}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, nil, RunOpts{FailFast: true})

	run, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(run.Results) != 1 {
		t.Fatalf("expected 1 result (fail-fast after extract failure), got %d", len(run.Results))
	}
}

func TestRunCollection_Execute_FailFast_AllPass_RunsAll(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://a.com"},
			{Name: "req2", Method: domain.MethodGet, URL: "http://b.com"},
			{Name: "req3", Method: domain.MethodGet, URL: "http://c.com"},
		},
	}
	runner := &multiCallRunner{
		results: []domain.RequestResult{
			{StatusCode: 200, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
			{StatusCode: 200, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
			{StatusCode: 200, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
		},
		errs: []error{nil, nil, nil},
	}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, nil, RunOpts{FailFast: true})

	run, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(run.Results) != 3 {
		t.Fatalf("expected all 3 results when all pass, got %d", len(run.Results))
	}
}

func TestRunCollection_Execute_FailFast_StillSavesArtifact(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://a.com"},
			{Name: "req2", Method: domain.MethodGet, URL: "http://b.com"},
		},
	}
	runner := &multiCallRunner{
		results: []domain.RequestResult{
			{},
			{StatusCode: 200, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
		},
		errs: []error{errors.New("boom"), nil},
	}
	store := &fakeStore{}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, store, RunOpts{FailFast: true})

	run, id, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !store.saved {
		t.Fatal("expected artifact to be saved even with fail-fast")
	}
	if id != "run-123" {
		t.Fatalf("expected run id, got %q", id)
	}
	if len(run.Results) != 1 {
		t.Fatalf("expected 1 partial result saved, got %d", len(run.Results))
	}
}

// --- only/tags filter tests ---

func TestRunCollection_Execute_OnlyFilter_RunsSubset(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://a.com"},
			{Name: "req2", Method: domain.MethodGet, URL: "http://b.com"},
			{Name: "req3", Method: domain.MethodGet, URL: "http://c.com"},
		},
	}
	runner := &multiCallRunner{
		results: []domain.RequestResult{
			{Name: "req2", StatusCode: 200, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
		},
		errs: []error{nil},
	}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, nil, RunOpts{Only: []string{"req2"}})

	run, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(run.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(run.Results))
	}
	if runner.idx != 1 {
		t.Fatalf("expected runner called once, got %d", runner.idx)
	}
}

func TestRunCollection_Execute_OnlyFilter_UnknownName_ReturnsError(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://a.com"},
		},
	}
	runner := &stubRunner{result: domain.RequestResult{
		StatusCode: 200,
		Response:   domain.ResponseSnapshot{Body: []byte(`{}`)},
	}}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, nil, RunOpts{Only: []string{"nonexistent"}})

	_, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err == nil {
		t.Fatal("expected error for unknown --only name")
	}
}

func TestRunCollection_Execute_TagsFilter_RunsTaggedRequests(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://a.com", Tags: []string{"smoke"}},
			{Name: "req2", Method: domain.MethodGet, URL: "http://b.com", Tags: []string{"auth"}},
			{Name: "req3", Method: domain.MethodGet, URL: "http://c.com"},
		},
	}
	runner := &multiCallRunner{
		results: []domain.RequestResult{
			{StatusCode: 200, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
		},
		errs: []error{nil},
	}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, nil, RunOpts{Tags: []string{"smoke"}})

	run, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(run.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(run.Results))
	}
}

func TestRunCollection_Execute_OnlyAndTags_Intersection(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://a.com", Tags: []string{"smoke"}},
			{Name: "req2", Method: domain.MethodGet, URL: "http://b.com", Tags: []string{"auth"}},
			{Name: "req3", Method: domain.MethodGet, URL: "http://c.com", Tags: []string{"smoke"}},
		},
	}
	runner := &multiCallRunner{
		results: []domain.RequestResult{
			{StatusCode: 200, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
		},
		errs: []error{nil},
	}
	// only req1 and req3 match names, but only req1 and req3 have "smoke" tag
	// intersection: only req1 passes both
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, nil, RunOpts{Only: []string{"req1", "req3"}, Tags: []string{"smoke"}})

	run, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(run.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(run.Results))
	}
}

func TestRunCollection_Execute_TagsFilter_NoMatch_RunsNothing(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://a.com", Tags: []string{"auth"}},
			{Name: "req2", Method: domain.MethodGet, URL: "http://b.com"},
		},
	}
	runner := &stubRunner{result: domain.RequestResult{
		StatusCode: 200,
		Response:   domain.ResponseSnapshot{Body: []byte(`{}`)},
	}}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, nil, RunOpts{Tags: []string{"nonexistent"}})

	run, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(run.Results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(run.Results))
	}
}

// --- retry tests ---

func TestRunCollection_Execute_Retry_TransientError_Recovers(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://a.com"},
		},
	}
	// First call: connection error (transient), second call: success.
	runner := &multiCallRunner{
		results: []domain.RequestResult{
			{
				Error:    &domain.RunError{Kind: domain.RunErrorConn, Message: "connection refused"},
				Response: domain.ResponseSnapshot{Body: []byte(`{}`)},
			},
			{StatusCode: 200, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
		},
		errs: []error{nil, nil},
	}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, nil, RunOpts{Retries: 1})

	run, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(run.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(run.Results))
	}
	if run.Results[0].Attempts != 2 {
		t.Fatalf("expected Attempts=2, got %d", run.Results[0].Attempts)
	}
	if run.Results[0].Error != nil {
		t.Fatalf("expected success after retry, got error: %v", run.Results[0].Error)
	}
}

func TestRunCollection_Execute_Retry_AllAttemptsFail(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://a.com"},
		},
	}
	runner := &multiCallRunner{
		results: []domain.RequestResult{
			{Error: &domain.RunError{Kind: domain.RunErrorTimeout, Message: "timeout"}, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
			{Error: &domain.RunError{Kind: domain.RunErrorTimeout, Message: "timeout"}, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
			{Error: &domain.RunError{Kind: domain.RunErrorTimeout, Message: "timeout"}, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
		},
		errs: []error{nil, nil, nil},
	}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, nil, RunOpts{Retries: 2})

	run, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(run.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(run.Results))
	}
	if run.Results[0].Attempts != 3 {
		t.Fatalf("expected Attempts=3, got %d", run.Results[0].Attempts)
	}
	if run.Results[0].Error == nil {
		t.Fatal("expected error after all retries exhausted")
	}
}

func TestRunCollection_Execute_Retry_NonTransient_NoRetry(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://a.com"},
		},
	}
	runner := &multiCallRunner{
		results: []domain.RequestResult{
			{Error: &domain.RunError{Kind: domain.RunErrorCanceled, Message: "canceled"}, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
			{StatusCode: 200, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
		},
		errs: []error{nil, nil},
	}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, nil, RunOpts{Retries: 2})

	run, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Results[0].Attempts != 1 {
		t.Fatalf("expected Attempts=1 (non-transient, no retry), got %d", run.Results[0].Attempts)
	}
}

func TestRunCollection_Execute_Retry_ConfigError_NoRetry(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://a.com"},
		},
	}
	runner := &multiCallRunner{
		results: []domain.RequestResult{
			{},
			{StatusCode: 200, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
		},
		errs: []error{errors.New("config error"), nil},
	}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, nil, RunOpts{Retries: 2})

	run, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Config-level error: goes to the runner-error branch, never retried.
	if len(run.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(run.Results))
	}
	if run.Results[0].Attempts != 1 {
		t.Fatalf("expected Attempts=1 (config error, no retry), got %d", run.Results[0].Attempts)
	}
	if run.Results[0].Error == nil {
		t.Fatal("expected error for config-level failure")
	}
}

func TestRunCollection_Execute_Retry5xx_Enabled(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://a.com"},
		},
	}
	runner := &multiCallRunner{
		results: []domain.RequestResult{
			{StatusCode: 503, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
			{StatusCode: 200, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
		},
		errs: []error{nil, nil},
	}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, nil, RunOpts{Retries: 1, Retry5xx: true})

	run, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Results[0].Attempts != 2 {
		t.Fatalf("expected Attempts=2, got %d", run.Results[0].Attempts)
	}
	if run.Results[0].StatusCode != 200 {
		t.Fatalf("expected status 200 after retry, got %d", run.Results[0].StatusCode)
	}
}

func TestRunCollection_Execute_Retry5xx_Disabled(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://a.com"},
		},
	}
	runner := &multiCallRunner{
		results: []domain.RequestResult{
			{StatusCode: 503, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
			{StatusCode: 200, Response: domain.ResponseSnapshot{Body: []byte(`{}`)}},
		},
		errs: []error{nil, nil},
	}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, nil, RunOpts{Retries: 1, Retry5xx: false})

	run, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Results[0].Attempts != 1 {
		t.Fatalf("expected Attempts=1, got %d", run.Results[0].Attempts)
	}
}

func TestRunCollection_Execute_Retry_ZeroRetries_SingleAttempt(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://a.com"},
		},
	}
	runner := &stubRunner{result: domain.RequestResult{
		StatusCode: 200,
		Response:   domain.ResponseSnapshot{Body: []byte(`{}`)},
	}}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, runner, nil, RunOpts{Retries: 0})

	run, _, err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Results[0].Attempts != 1 {
		t.Fatalf("expected Attempts=1, got %d", run.Results[0].Attempts)
	}
}

func ptrInt(v int) *int { return &v }

// compile-time checks
var _ ports.CollectionLoader = (*fakeCollectionLoader)(nil)
var _ ports.EnvironmentLoader = (*fakeEnvLoader)(nil)
var _ ports.ArtifactStore = (*fakeStore)(nil)
