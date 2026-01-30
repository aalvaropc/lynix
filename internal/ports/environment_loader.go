package ports

import "github.com/aalvaropc/lynix/internal/domain"

// EnvironmentLoader loads environment variables from a source (e.g., filesystem).
type EnvironmentLoader interface {
	LoadEnvironment(path string) (domain.Environment, error)
}
