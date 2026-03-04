package ports

import "context"

// WorkspaceLocator finds a Lynix workspace root starting from an arbitrary directory.
type WorkspaceLocator interface {
	FindRoot(ctx context.Context, startDir string) (string, error)
}
