package ports

import "github.com/aalvaropc/lynix/internal/domain"

// CollectionLoader loads collections from a source (e.g., filesystem).
type CollectionLoader interface {
	LoadCollection(path string) (domain.Collection, error)
	ListCollections(root string) ([]domain.CollectionRef, error)
}
