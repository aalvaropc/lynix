package usecase

import (
	"context"
	"testing"

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
