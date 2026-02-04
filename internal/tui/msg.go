package tui

type WorkspaceStatus struct {
	Found bool
	Root  string
	Err   error
}

type workspaceRefreshedMsg WorkspaceStatus

type initWorkspaceDoneMsg struct {
	Status WorkspaceStatus
	Err    error
}
