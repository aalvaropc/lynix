package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/ports"
)

type countingRunner struct {
	calls int
}

func (r *countingRunner) Run(_ context.Context, _ domain.RequestSpec, _ domain.Vars) (domain.RequestResult, error) {
	r.calls++
	return domain.RequestResult{
		Name:     "ok",
		Method:   domain.MethodGet,
		URL:      "http://example",
		Response: domain.ResponseSnapshot{Headers: map[string][]string{}},
	}, nil
}

func TestRunCollection_StopsOnContextCancel(t *testing.T) {
	col := domain.Collection{
		Name: "Demo",
		Vars: domain.Vars{},
		Requests: []domain.RequestSpec{
			{Name: "r1", Method: domain.MethodGet, URL: "http://example/1"},
			{Name: "r2", Method: domain.MethodGet, URL: "http://example/2"},
		},
	}
	env := domain.Environment{Name: "dev", Vars: domain.Vars{}}

	r := &countingRunner{}
	uc := NewRunCollection(fakeCollectionLoader{col: col}, fakeEnvLoader{env: env}, r, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	out, id, err := uc.Execute(ctx, "demo.yaml", "dev")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if id != "" {
		t.Fatalf("expected no run id, got %q", id)
	}
	if r.calls != 0 {
		t.Fatalf("expected 0 runner calls, got %d", r.calls)
	}
	if out.StartedAt.IsZero() {
		t.Fatalf("expected StartedAt set")
	}
	if out.EndedAt.IsZero() {
		t.Fatalf("expected EndedAt set")
	}
	if out.EndedAt.Before(out.StartedAt) {
		t.Fatalf("expected EndedAt >= StartedAt")
	}
	if len(out.Results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(out.Results))
	}
}

var _ ports.RequestRunner = (*countingRunner)(nil)
