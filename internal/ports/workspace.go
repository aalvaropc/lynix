package ports

import "github.com/aalvaropc/lynix/internal/domain"

type WorkspaceInitializer interface {
	Init(spec domain.WorkspaceSpec, force bool) error
}
