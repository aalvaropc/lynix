package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type screen int

const (
	screenHome screen = iota
	screenPlaceholder
)

type menuItem struct {
	title string
	desc  string
}

func (m menuItem) Title() string       { return m.title }
func (m menuItem) Description() string { return m.desc }
func (m menuItem) FilterValue() string { return m.title }

type model struct {
	theme Theme

	scr        screen
	menu       list.Model
	activeName string
}

func Run() error {
	m := newModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newModel() model {
	t := DefaultTheme()

	items := []list.Item{
		menuItem{"Run (Functional)", "Execute requests and checks (MVP target)"},
		menuItem{"Bench (Performance)", "Load testing and metrics (v0.2)"},
		menuItem{"Compare (Baselines)", "Detect regressions (v1.0)"},
		menuItem{"Collections", "Create and edit YAML collections (MVP target)"},
		menuItem{"Environments", "Manage env vars and secrets (MVP target)"},
		menuItem{"Reports", "View/export reports (MVP+)"},
		menuItem{"Settings", "Workspace and defaults"},
		menuItem{"Quit", "Exit Lynix"},
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Lynix"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)

	return model{
		theme: t,
		scr:   screenHome,
		menu:  l,
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		w, h := msg.Width, msg.Height
		m.menu.SetSize(w-4, h-8)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.scr == screenHome {
				return m, tea.Quit
			}
			m.scr = screenHome
			m.activeName = ""
			return m, nil

		case "enter":
			if m.scr == screenHome {
				it, ok := m.menu.SelectedItem().(menuItem)
				if !ok {
					return m, nil
				}
				if strings.EqualFold(it.title, "Quit") {
					return m, tea.Quit
				}
				m.scr = screenPlaceholder
				m.activeName = it.title
				return m, nil
			}

		case "esc", "b":
			if m.scr != screenHome {
				m.scr = screenHome
				m.activeName = ""
				return m, nil
			}
		}
	}

	if m.scr == screenHome {
		var cmd tea.Cmd
		m.menu, cmd = m.menu.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) View() string {
	wrap := lipgloss.NewStyle().Padding(1, 2)
	header := m.theme.Title.Render("Lynix") + "\n" +
		m.theme.Subtitle.Render("TUI-first API tool (Go) — requests, checks, and performance") + "\n"

	switch m.scr {
	case screenHome:
		help := m.theme.Help.Render("↑/↓ navigate • enter open • / search • q quit")
		return wrap.Render(header + "\n" + m.theme.Card.Render(m.menu.View()) + "\n" + help)

	case screenPlaceholder:
		card := m.theme.Card.Render(
			fmt.Sprintf("%s\n\n%s\n\n%s",
				m.theme.Title.Render(m.activeName),
				"This screen is a placeholder. We'll implement it as part of the MVP.",
				m.theme.Help.Render("esc/b back • q home"),
			),
		)
		return wrap.Render(header + "\n" + card)

	default:
		return wrap.Render(header + "\n" + "unknown state")
	}
}
