package tui

import tea "github.com/charmbracelet/bubbletea"

func listenRunner(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return runnerDoneMsg{Err: nil}
		}
		return msg
	}
}
