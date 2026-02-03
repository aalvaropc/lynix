package ports

import "github.com/aalvaropc/lynix/internal/domain"

type EnvironmentCatalog interface {
	ListEnvironments(root string) ([]domain.EnvironmentRef, error)
}
