package tui

import (
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type screen int

const (
	screenHome screen = iota
	screenCollections
	screenSettings
	screenRunWizard
	screenResults
	screenPlaceholder
)

type menuItem struct {
	title string
	desc  string
}

func (m menuItem) Title() string       { return m.title }
func (m menuItem) Description() string { return m.desc }
func (m menuItem) FilterValue() string { return m.title }

type collectionItem struct {
	ref domain.CollectionRef
}

func (c collectionItem) Title() string       { return c.ref.Name }
func (c collectionItem) Description() string { return filepath.Base(c.ref.Path) }
func (c collectionItem) FilterValue() string { return c.ref.Name }

type envItem struct {
	ref domain.EnvironmentRef
}

func (e envItem) Title() string       { return e.ref.Name }
func (e envItem) Description() string { return filepath.Base(e.ref.Path) }
func (e envItem) FilterValue() string { return e.ref.Name }

type model struct {
	theme                  Theme
	deps                   Deps
	log                    *slog.Logger
	scr                    screen
	width                  int
	height                 int
	menu                   list.Model
	cwd                    string
	workspaceFound         bool
	workspaceRoot          string
	workspaceErr           error
	toast                  string
	collectionsList        list.Model
	collectionsRefs        []domain.CollectionRef
	previewPath            string
	previewText            string
	previewErr             error
	lastInitErr            error
	wizardStep             int
	runColList             list.Model
	runEnvList             list.Model
	selectedCollectionPath string
	selectedEnvName        string
	running                bool
	spin                   spinner.Model
	runCh                  chan runnerDoneMsg
	runResult              domain.RunResult
	runID                  string
	runErr                 error
	resultsTable           table.Model
	resultTab              int // 0=details, 1=response
}

func Run(deps Deps) error {
	m := newModel(deps)
	sm := wrapSafe(m, m.log)
	p := tea.NewProgram(sm, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newModel(deps Deps) model {
	t := DefaultTheme()

	log := deps.Logger
	if log == nil {
		log = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}

	items := []list.Item{
		menuItem{"Run (Functional)", "Execute requests and checks (MVP target)"},
		menuItem{"Bench (Performance)", "Load testing and metrics (v0.2)"},
		menuItem{"Compare (Baselines)", "Detect regressions (v1.0)"},
		menuItem{"Collections", "Browse and validate YAML collections"},
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

	colList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	colList.Title = "Collections"
	colList.SetShowStatusBar(false)
	colList.SetFilteringEnabled(true)
	colList.SetShowHelp(false)

	runCol := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	runCol.Title = "Step 1/4 — Select collection"
	runCol.SetShowStatusBar(false)
	runCol.SetFilteringEnabled(true)
	runCol.SetShowHelp(false)

	runEnv := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	runEnv.Title = "Step 2/4 — Select environment"
	runEnv.SetShowStatusBar(false)
	runEnv.SetFilteringEnabled(true)
	runEnv.SetShowHelp(false)

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return model{
		theme:           t,
		deps:            deps,
		log:             log,
		scr:             screenHome,
		menu:            l,
		collectionsList: colList,
		runColList:      runCol,
		runEnvList:      runEnv,
		wizardStep:      0,
		spin:            sp,
		resultTab:       0,
	}
}

func (m model) Init() tea.Cmd {
	return cmdRefreshWorkspace(m.deps)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil

	case workspaceRefreshedMsg:
		m.cwd = msg.cwd
		m.workspaceFound = msg.found
		m.workspaceRoot = msg.root
		m.workspaceErr = msg.err

		m.collectionsRefs = nil
		m.previewPath = ""
		m.previewText = ""
		m.previewErr = nil
		m.selectedCollectionPath = ""
		m.selectedEnvName = ""
		m.runErr = nil
		m.runID = ""
		m.toast = ""

		if m.workspaceFound {
			return m, tea.Batch(
				cmdLoadCollections(m.workspaceRoot),
				cmdLoadEnvironments(m.workspaceRoot),
			)
		}
		return m, nil

	case initWorkspaceDoneMsg:
		m.lastInitErr = msg.err
		if msg.err != nil {
			m.toast = userMessage(msg.err)
		} else {
			m.toast = "Workspace initialized."
		}
		return m, cmdRefreshWorkspace(m.deps)

	case collectionsLoadedMsg:
		if msg.err != nil {
			m.toast = userMessage(msg.err)
			m.collectionsRefs = nil
			m.collectionsList.SetItems([]list.Item{})
			m.runColList.SetItems([]list.Item{})
			return m, nil
		}

		m.collectionsRefs = msg.refs
		items := make([]list.Item, 0, len(msg.refs))
		for _, r := range msg.refs {
			items = append(items, collectionItem{ref: r})
		}
		m.collectionsList.SetItems(items)
		m.runColList.SetItems(items)

		if len(msg.refs) > 0 {
			m.previewPath = msg.refs[0].Path
			return m, cmdPreviewCollection(m.previewPath)
		}
		return m, nil

	case envsLoadedMsg:
		if msg.err != nil {
			m.toast = userMessage(msg.err)
			m.runEnvList.SetItems([]list.Item{})
			return m, nil
		}

		items := make([]list.Item, 0, len(msg.refs))
		for _, r := range msg.refs {
			items = append(items, envItem{ref: r})
		}
		m.runEnvList.SetItems(items)
		return m, nil

	case collectionPreviewMsg:
		m.previewPath = msg.path
		m.previewText = msg.preview
		m.previewErr = msg.err
		return m, nil

	case runnerDoneMsg:
		m.running = false
		m.runErr = msg.err
		m.runID = msg.id
		m.runResult = msg.run

		m.buildResultsTable()
		m.scr = screenResults
		m.toast = ""
		if msg.err != nil {
			m.toast = userMessage(msg.err)
		} else {
			m.toast = "Run saved: " + msg.id
		}
		return m, nil

	case tea.KeyMsg:
		key := msg.String()

		if key == "ctrl+c" {
			return m, tea.Quit
		}
		if key == "q" {
			if m.scr == screenHome {
				return m, tea.Quit
			}
			m.scr = screenHome
			m.wizardStep = 0
			m.running = false
			m.toast = ""
			return m, nil
		}

		switch m.scr {

		case screenHome:
			switch key {
			case "enter":
				it, ok := m.menu.SelectedItem().(menuItem)
				if !ok {
					return m, nil
				}
				switch {
				case strings.EqualFold(it.title, "Quit"):
					return m, tea.Quit
				case strings.EqualFold(it.title, "Settings"):
					m.scr = screenSettings
					return m, nil
				case strings.EqualFold(it.title, "Collections"):
					m.scr = screenCollections
					return m, m.maybePreviewSelectedCollection(m.collectionsList)
				case strings.HasPrefix(it.title, "Run"):
					m.scr = screenRunWizard
					m.wizardStep = 1
					m.toast = ""
					return m, m.maybePreviewSelectedCollection(m.runColList)
				default:
					m.scr = screenPlaceholder
					m.toast = it.title
					return m, nil
				}
			}

		case screenSettings:
			switch key {
			case "esc", "b":
				m.scr = screenHome
				return m, nil
			case "i", "I":
				root := m.cwd
				if strings.TrimSpace(root) == "" {
					root = m.workspaceRoot
				}
				if strings.TrimSpace(root) == "" {
					m.toast = "Cannot init: unknown current directory"
					return m, nil
				}
				return m, cmdInitWorkspaceHere(m.deps, root)
			}

		case screenCollections:
			switch key {
			case "esc", "b":
				m.scr = screenHome
				return m, nil
			}

		case screenRunWizard:
			switch key {
			case "esc", "b":
				if m.running {
					m.toast = "Run in progress..."
					return m, nil
				}
				if m.wizardStep <= 1 {
					m.scr = screenHome
					m.wizardStep = 0
					return m, nil
				}
				m.wizardStep--
				return m, nil

			case "enter":
				if !m.workspaceFound {
					m.toast = "Workspace not found"
					return m, nil
				}

				switch m.wizardStep {
				case 1:
					ci, ok := m.runColList.SelectedItem().(collectionItem)
					if !ok {
						return m, nil
					}
					m.selectedCollectionPath = ci.ref.Path
					m.wizardStep = 2
					return m, nil

				case 2:
					ei, ok := m.runEnvList.SelectedItem().(envItem)
					if !ok {
						return m, nil
					}
					m.selectedEnvName = ei.ref.Name
					m.wizardStep = 3
					return m, nil

				case 3:
					if strings.TrimSpace(m.selectedCollectionPath) == "" || strings.TrimSpace(m.selectedEnvName) == "" {
						m.toast = "Missing selection"
						return m, nil
					}

					m.wizardStep = 4
					m.running = true
					m.toast = ""

					ch, listenCmd := startRunAsync(
						m.workspaceRoot,
						m.selectedCollectionPath,
						m.selectedEnvName,
						m.log,
						m.deps.Debug,
					)
					m.runCh = ch

					return m, tea.Batch(
						m.spin.Tick,
						listenCmd,
					)
				}
			}

		case screenResults:
			switch key {
			case "esc", "b":
				m.scr = screenHome
				return m, nil
			case "tab":
				m.resultTab = (m.resultTab + 1) % 2
				return m, nil
			}
		}
	}

	switch m.scr {

	case screenHome:
		var cmd tea.Cmd
		m.menu, cmd = m.menu.Update(msg)
		return m, cmd

	case screenCollections:
		var cmd tea.Cmd
		m.collectionsList, cmd = m.collectionsList.Update(msg)
		return m, tea.Batch(cmd, m.maybePreviewSelectedCollection(m.collectionsList))

	case screenRunWizard:
		if m.running {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}

		if m.wizardStep == 1 {
			var cmd tea.Cmd
			m.runColList, cmd = m.runColList.Update(msg)
			return m, tea.Batch(cmd, m.maybePreviewSelectedCollection(m.runColList))
		}
		if m.wizardStep == 2 {
			var cmd tea.Cmd
			m.runEnvList, cmd = m.runEnvList.Update(msg)
			return m, cmd
		}
		return m, nil

	case screenResults:
		var cmd tea.Cmd
		m.resultsTable, cmd = m.resultsTable.Update(msg)
		return m, cmd

	default:
		return m, nil
	}
}

func (m *model) resize() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	m.menu.SetSize(m.width-4, m.height-10)

	leftW := (m.width - 8) / 2
	rightW := (m.width - 8) - leftW
	listH := m.height - 10
	if listH < 8 {
		listH = 8
	}
	m.collectionsList.SetSize(leftW, listH)
	m.runColList.SetSize(leftW, listH)
	m.runEnvList.SetSize(leftW, listH)

	m.resultsTable.SetWidth(leftW)
	m.resultsTable.SetHeight(listH)
	_ = rightW
}

func (m *model) maybePreviewSelectedCollection(l list.Model) tea.Cmd {
	sel := l.SelectedItem()
	ci, ok := sel.(collectionItem)
	if !ok {
		return nil
	}
	if strings.TrimSpace(ci.ref.Path) == "" {
		return nil
	}
	if filepath.Clean(ci.ref.Path) == filepath.Clean(m.previewPath) && (m.previewErr == nil || m.previewText != "") {
		return nil
	}
	return cmdPreviewCollection(ci.ref.Path)
}

func (m *model) buildResultsTable() {
	cols := []table.Column{
		{Title: "Name", Width: 24},
		{Title: "Method", Width: 6},
		{Title: "Status", Width: 6},
		{Title: "ms", Width: 6},
	}

	rows := make([]table.Row, 0, len(m.runResult.Results))
	for _, rr := range m.runResult.Results {
		status := fmt.Sprintf("%d", rr.StatusCode)
		if rr.Error != nil && rr.StatusCode == 0 {
			status = "ERR"
		}
		rows = append(rows, table.Row{
			clampString(rr.Name, 24),
			string(rr.Method),
			status,
			fmt.Sprintf("%d", rr.LatencyMS),
		})
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
	)
	m.resultsTable = t
	m.resize()
}

func (m model) View() string {
	wrap := lipgloss.NewStyle().Padding(1, 2)

	header := m.theme.Title.Render("Lynix") + "\n" +
		m.theme.Subtitle.Render("TUI-first API tool (Go) — requests, checks, and performance") + "\n"

	workspaceLine := m.workspaceBanner()

	footer := ""
	if strings.TrimSpace(m.toast) != "" {
		footer = "\n" + m.theme.Help.Render(m.toast)
	}

	switch m.scr {

	case screenHome:
		help := m.theme.Help.Render("↑/↓ navigate • enter open • / search • q quit")
		return wrap.Render(header + "\n" + workspaceLine + "\n\n" + m.theme.Card.Render(m.menu.View()) + "\n" + help + footer)

	case screenSettings:
		return wrap.Render(header + "\n" + workspaceLine + "\n\n" + m.viewSettings() + footer)

	case screenCollections:
		return wrap.Render(header + "\n" + workspaceLine + "\n\n" + m.viewCollections() + footer)

	case screenRunWizard:
		return wrap.Render(header + "\n" + workspaceLine + "\n\n" + m.viewRunWizard() + footer)

	case screenResults:
		return wrap.Render(header + "\n" + workspaceLine + "\n\n" + m.viewResults() + footer)

	case screenPlaceholder:
		card := m.theme.Card.Render(
			fmt.Sprintf("%s\n\n%s\n\n%s",
				m.theme.Title.Render("Placeholder"),
				"This screen is not implemented yet.",
				m.theme.Help.Render("esc/b back • q home"),
			),
		)
		return wrap.Render(header + "\n" + workspaceLine + "\n\n" + card + footer)

	default:
		return wrap.Render(header + "\n" + "unknown state" + footer)
	}
}

func (m model) workspaceBanner() string {
	if m.workspaceFound {
		return m.theme.Help.Render(fmt.Sprintf("Workspace: FOUND  %s", m.workspaceRoot))
	}

	msg := "Workspace: NOT FOUND"
	if m.workspaceErr != nil {
		msg += "  (" + userMessage(m.workspaceErr) + ")"
	}
	return m.theme.Card.Render("⚠ " + msg + "\n\nGo to Settings → press I to init workspace here (force).")
}

func (m model) viewSettings() string {
	var b strings.Builder
	b.WriteString(m.theme.Title.Render("Settings"))
	b.WriteString("\n\n")
	b.WriteString("Workspace init:\n")
	b.WriteString("  - Press ")
	b.WriteString(m.theme.Title.Render("I"))
	b.WriteString(" to init workspace in current directory (force=true)\n\n")

	if m.lastInitErr != nil {
		b.WriteString("Last init error:\n  ")
		b.WriteString(userMessage(m.lastInitErr))
		b.WriteString("\n\n")
	}

	b.WriteString(m.theme.Help.Render("i init • esc/b back • q home"))
	return m.theme.Card.Render(b.String())
}

func (m model) viewCollections() string {
	left := m.theme.Card.Render(m.collectionsList.View())

	right := ""
	if m.previewErr != nil {
		right = m.theme.Card.Render(
			m.theme.Title.Render("Preview") + "\n\n" +
				userMessage(m.previewErr) + "\n\n" +
				m.theme.Help.Render("Fix the file, preview will update."),
		)
	} else if strings.TrimSpace(m.previewText) != "" {
		right = m.theme.Card.Render(
			m.theme.Title.Render("Preview") + "\n\n" + m.previewText,
		)
	} else {
		right = m.theme.Card.Render(
			m.theme.Title.Render("Preview") + "\n\n" + "(select a collection)",
		)
	}

	help := m.theme.Help.Render("↑/↓ select • / filter • esc/b back • q home")
	return lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right),
		"\n"+help,
	)
}

func (m model) viewRunWizard() string {
	switch m.wizardStep {

	case 1:
		left := m.theme.Card.Render(m.runColList.View())
		right := m.previewPanel()
		help := m.theme.Help.Render("enter next • ↑/↓ select • / filter • esc/b back • q home")
		return lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right),
			"\n"+help,
		)

	case 2:
		left := m.theme.Card.Render(m.runEnvList.View())
		right := m.theme.Card.Render(
			m.theme.Title.Render("Step 2/4 — Environment") + "\n\n" +
				"Select the environment YAML (dev/stg/etc).\n",
		)
		help := m.theme.Help.Render("enter next • ↑/↓ select • / filter • esc/b back • q home")
		return lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right),
			"\n"+help,
		)

	case 3:
		card := m.theme.Card.Render(
			m.theme.Title.Render("Step 3/4 — Confirm") + "\n\n" +
				fmt.Sprintf("Collection:\n  %s\n\nEnvironment:\n  %s\n\n", m.selectedCollectionPath, m.selectedEnvName) +
				m.theme.Help.Render("enter run • esc/b back • q home"),
		)
		return card

	case 4:
		card := m.theme.Card.Render(
			m.theme.Title.Render("Step 4/4 — Running") + "\n\n" +
				m.spin.View() + " Executing collection...\n\n" +
				m.theme.Help.Render("please wait"),
		)
		return card

	default:
		return m.theme.Card.Render("Wizard not started.")
	}
}

func (m model) previewPanel() string {
	if m.previewErr != nil {
		return m.theme.Card.Render(
			m.theme.Title.Render("Preview") + "\n\n" + userMessage(m.previewErr),
		)
	}
	if strings.TrimSpace(m.previewText) == "" {
		return m.theme.Card.Render(m.theme.Title.Render("Preview") + "\n\n(select a collection)")
	}
	return m.theme.Card.Render(m.theme.Title.Render("Preview") + "\n\n" + m.previewText)
}

func (m model) viewResults() string {
	left := m.theme.Card.Render(m.resultsTable.View())

	rightTitle := "Details"
	if m.resultTab == 1 {
		rightTitle = "Response"
	}

	rr := m.selectedResult()
	rightBody := "(no selection)"
	if rr != nil {
		if m.resultTab == 0 {
			rightBody = renderResultDetails(*rr)
		} else {
			rightBody = renderResultResponse(*rr)
		}
	}

	right := m.theme.Card.Render(
		m.theme.Title.Render("Result — "+rightTitle) + "\n\n" + rightBody,
	)

	help := m.theme.Help.Render("↑/↓ select • tab toggle • esc/b back • q home")
	return lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right),
		"\n"+help,
	)
}

func (m model) selectedResult() *domain.RequestResult {
	i := m.resultsTable.Cursor()
	if i < 0 || i >= len(m.runResult.Results) {
		return nil
	}
	return &m.runResult.Results[i]
}

var _ tea.Model = (*model)(nil)
