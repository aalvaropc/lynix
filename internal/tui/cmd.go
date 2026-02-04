package tui

import (
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

type Deps struct {
	WorkspaceFinder interface {
		FindRoot(start string) (string, error)
	}
	InitWorkspaceUC interface {
		Execute(path string, force bool) error
	}
}

func cmdRefreshWorkspace(d Deps) tea.Cmd {
	return func() tea.Msg {
		cwd, _ := os.Getwd()
		root, err := d.WorkspaceFinder.FindRoot(cwd)
		if err != nil || root == "" {
			return workspaceRefreshedMsg(WorkspaceStatus{Found: false, Root: "", Err: err})
		}
		return workspaceRefreshedMsg(WorkspaceStatus{Found: true, Root: root, Err: nil})
	}
}

func cmdInitWorkspaceHere(d Deps, force bool) tea.Cmd {
	return func() tea.Msg {
		cwd, _ := os.Getwd()

		err := d.InitWorkspaceUC.Execute(cwd, force)

		root, findErr := d.WorkspaceFinder.FindRoot(filepath.Clean(cwd))
		st := WorkspaceStatus{Found: findErr == nil && root != "", Root: root, Err: findErr}
		return initWorkspaceDoneMsg{Status: st, Err: err}
	}
}
