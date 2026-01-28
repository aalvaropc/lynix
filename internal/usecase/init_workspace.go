package usecase

import (
	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/ports"
)

type InitWorkspace struct {
	initializer ports.WorkspaceInitializer
}

func NewInitWorkspace(initializer ports.WorkspaceInitializer) *InitWorkspace {
	return &InitWorkspace{initializer: initializer}
}

func (uc *InitWorkspace) Execute(root string, force bool) error {
	return uc.initializer.Init(domain.WorkspaceSpec{Root: root}, force)
}
