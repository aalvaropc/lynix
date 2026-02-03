package usecase

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/infra/httpclient"
	"github.com/aalvaropc/lynix/internal/infra/httprunner"
	"github.com/aalvaropc/lynix/internal/ports"
)

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
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{env: env}, r, st)

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

	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{env: env}, r, nil)

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

func ptrInt(v int) *int { return &v }

// compile-time checks
var _ ports.CollectionLoader = (*fakeCollectionLoader)(nil)
var _ ports.EnvironmentLoader = (*fakeEnvLoader)(nil)
var _ ports.ArtifactStore = (*fakeStore)(nil)
