package ports

import (
	"context"

	"github.com/aalvaropc/lynix/internal/domain"
)

// RequestRunner executes a single request with a resolved variable set.
type RequestRunner interface {
	Run(ctx context.Context, req domain.RequestSpec, vars domain.Vars) (domain.RunResult, error)
}
