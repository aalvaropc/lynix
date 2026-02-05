package tui

import (
	"log/slog"

	"github.com/aalvaropc/lynix/internal/ports"
)

type Deps struct {
	WorkspaceLocator     ports.WorkspaceLocator
	WorkspaceInitializer ports.WorkspaceInitializer

	Logger *slog.Logger
	Debug  bool
}
