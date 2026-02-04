package tui

import "github.com/aalvaropc/lynix/internal/domain"

type workspaceRefreshedMsg struct {
	cwd   string
	found bool
	root  string
	err   error
}

type initWorkspaceDoneMsg struct {
	root string
	err  error
}

type collectionsLoadedMsg struct {
	root string
	refs []domain.CollectionRef
	err  error
}

type envsLoadedMsg struct {
	root string
	refs []domain.EnvironmentRef
	err  error
}

type collectionPreviewMsg struct {
	path    string
	preview string
	err     error
}

type runnerDoneMsg struct {
	run domain.RunResult
	id  string
	err error
}
