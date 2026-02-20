package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aalvaropc/lynix/internal/domain"
)

func TestValidateCollection_PassesWithExtractedVarChain(t *testing.T) {
	col := domain.Collection{
		Name: "Demo",
		Requests: []domain.RequestSpec{
			{
				Name:   "auth.token",
				Method: domain.MethodGet,
				URL:    "{{base_url}}/auth",
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
			},
		},
	}

	env := domain.Environment{
		Name: "dev",
		Vars: domain.Vars{
			"base_url": "http://example",
		},
	}

	uc := NewValidateCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{env: env})
	if err := uc.Execute(context.Background(), "demo.yaml", "dev"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateCollection_FailsOnMissingVar(t *testing.T) {
	col := domain.Collection{
		Name: "Demo",
		Requests: []domain.RequestSpec{
			{
				Name:   "bad",
				Method: domain.MethodGet,
				URL:    "{{missing}}/x",
			},
		},
	}
	env := domain.Environment{Name: "dev", Vars: domain.Vars{}}

	uc := NewValidateCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{env: env})
	err := uc.Execute(context.Background(), "demo.yaml", "dev")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !domain.IsKind(err, domain.KindMissingVar) {
		t.Fatalf("expected KindMissingVar, got %v", err)
	}
}

func TestValidateCollection_ErrorLoadingCollection(t *testing.T) {
	loadErr := errors.New("collection not found")
	uc := NewValidateCollection(errCollectionLoader{err: loadErr}, fakeEnvLoader{})
	err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err == nil {
		t.Fatal("expected error loading collection")
	}
	if !errors.Is(err, loadErr) {
		t.Fatalf("expected wrapped loadErr, got %v", err)
	}
}

func TestValidateCollection_ErrorLoadingEnvironment(t *testing.T) {
	envErr := errors.New("env not found")
	uc := NewValidateCollection(fakeCollectionLoader{}, errEnvLoader{err: envErr})
	err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err == nil {
		t.Fatal("expected error loading environment")
	}
	if !errors.Is(err, envErr) {
		t.Fatalf("expected wrapped envErr, got %v", err)
	}
}

func TestValidateCollection_ContextCancelled(t *testing.T) {
	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://example.com"},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before Execute

	uc := NewValidateCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{})
	err := uc.Execute(ctx, "col.yaml", "env.yaml")
	if err == nil {
		t.Fatal("expected context error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestValidateCollection_WithVarResolver(t *testing.T) {
	// Inject a resolver whose UUID generator always errors; this verifies the
	// injected resolver is actually used instead of the default one.
	uuidErr := errors.New("uuid gen failed")
	vr := domain.NewVarResolver(
		domain.WithNow(func() time.Time { return time.Unix(1000, 0) }),
		domain.WithUUID(func() (string, error) { return "", uuidErr }),
	)

	col := domain.Collection{
		Requests: []domain.RequestSpec{
			{Name: "req1", Method: domain.MethodGet, URL: "http://example.com"},
		},
	}
	uc := NewValidateCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{}, WithVarResolver(vr))
	err := uc.Execute(context.Background(), "col.yaml", "env.yaml")
	if err == nil {
		t.Fatal("expected error from injected resolver's broken UUID generator")
	}
	// The error propagates from NewRuntime failing.
	if !errors.Is(err, uuidErr) {
		t.Fatalf("expected uuidErr in chain, got %v", err)
	}
}
