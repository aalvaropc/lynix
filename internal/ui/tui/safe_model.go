package tui

import (
	"fmt"
	"io"
	"log/slog"
	"runtime/debug"

	tea "github.com/charmbracelet/bubbletea"
)

type safeModel struct {
	m   model
	log *slog.Logger
}

func wrapSafe(m model, log *slog.Logger) safeModel {
	if log == nil {
		log = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}
	return safeModel{m: m, log: log}
}

func (s safeModel) Init() tea.Cmd {
	return s.m.Init()
}

func (s safeModel) Update(msg tea.Msg) (tm tea.Model, cmd tea.Cmd) {
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("panic.recovered",
				"where", "tui.update",
				"panic", fmt.Sprint(r),
				"stack", string(debug.Stack()),
			)

			// Clean up async resources to prevent goroutine leaks.
			if s.m.runCancel != nil {
				s.m.runCancel()
				s.m.runCancel = nil
			}
			if s.m.runCh != nil {
				// Drain the channel so the goroutine can exit.
				go func(ch chan runnerDoneMsg) {
					for range ch { //nolint:revive // drain so goroutine exits
					}
				}(s.m.runCh)
				s.m.runCh = nil
			}

			s.m.scr = screenHome
			s.m.wizardStep = 0
			s.m.running = false
			s.m.toast = "Unexpected error (see logs)"
			tm = s
			cmd = nil
		}
	}()

	inner, c := s.m.Update(msg)

	if mm, ok := inner.(model); ok {
		s.m = mm
	} else if sm, ok := inner.(safeModel); ok {
		s = sm
	}

	return s, c
}

func (s safeModel) View() (out string) {
	defer func() {
		if r := recover(); r != nil {
			s.log.Error("panic.recovered",
				"where", "tui.view",
				"panic", fmt.Sprint(r),
				"stack", string(debug.Stack()),
			)
			out = "Unexpected error (see logs)"
		}
	}()
	return s.m.View()
}

var _ tea.Model = (*safeModel)(nil)
