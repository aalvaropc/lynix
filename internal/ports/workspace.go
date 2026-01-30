package ports

import "github.com/aalvaropc/lynix/internal/domain"

// WorkspaceInitializer creates the standard Lynix workspace layout.
type WorkspaceInitializer interface {
	Init(spec domain.WorkspaceSpec, force bool) error
}
