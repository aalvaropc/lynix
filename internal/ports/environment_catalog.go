package ports

import (
	"context"

	"github.com/aalvaropc/lynix/internal/domain"
)

type EnvironmentCatalog interface {
	ListEnvironments(ctx context.Context, root string) ([]domain.EnvironmentRef, error)
}
