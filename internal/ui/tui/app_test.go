package tui

import (
	"testing"

	"github.com/aalvaropc/lynix/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
)

func TestModel_Update_WindowSize(t *testing.T) {
	m := newModel(Deps{})
	tm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	updated := tm.(model)
	if updated.width != 120 || updated.height != 40 {
		t.Errorf("expected 120x40, got %dx%d", updated.width, updated.height)
	}
}

func TestModel_Update_CtrlC_Quits(t *testing.T) {
	m := newModel(Deps{})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit command on ctrl+c")
	}
}

func TestModel_Update_Q_FromHome_Quits(t *testing.T) {
	m := newModel(Deps{})
	m.scr = screenHome
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit command on 'q' from home screen")
	}
}

func TestModel_Update_Q_FromOther_GoesHome(t *testing.T) {
	m := newModel(Deps{})
	m.scr = screenSettings
	tm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	updated := tm.(model)
	if updated.scr != screenHome {
		t.Errorf("expected screenHome, got %d", updated.scr)
	}
}

func TestModel_Update_Q_WhileRunning_ShowsToast(t *testing.T) {
	m := newModel(Deps{})
	m.scr = screenRunWizard
	m.running = true
	tm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	updated := tm.(model)
	if updated.toast == "" {
		t.Error("expected toast message when q pressed while running")
	}
}

func TestModel_Update_WorkspaceRefreshed_Found(t *testing.T) {
	m := newModel(Deps{})
	msg := workspaceRefreshedMsg{
		cwd:   "/home/user/project",
		found: true,
		root:  "/home/user/project",
	}
	tm, _ := m.Update(msg)
	updated := tm.(model)
	if !updated.workspaceFound {
		t.Error("expected workspaceFound=true")
	}
	if updated.workspaceRoot != "/home/user/project" {
		t.Errorf("expected root=/home/user/project, got %q", updated.workspaceRoot)
	}
}

func TestModel_Update_WorkspaceRefreshed_NotFound(t *testing.T) {
	m := newModel(Deps{})
	msg := workspaceRefreshedMsg{
		cwd:   "/home/user",
		found: false,
	}
	tm, _ := m.Update(msg)
	updated := tm.(model)
	if updated.workspaceFound {
		t.Error("expected workspaceFound=false")
	}
}

func TestModel_Update_CollectionsLoaded(t *testing.T) {
	m := newModel(Deps{})
	m.width = 80
	m.height = 24
	refs := []domain.CollectionRef{
		{Name: "Demo", Path: "/tmp/demo.yaml"},
		{Name: "Auth", Path: "/tmp/auth.yaml"},
	}
	msg := collectionsLoadedMsg{root: "/tmp", refs: refs}
	tm, _ := m.Update(msg)
	updated := tm.(model)
	if len(updated.collectionsRefs) != 2 {
		t.Errorf("expected 2 collections, got %d", len(updated.collectionsRefs))
	}
}

func TestModel_Update_CollectionsLoaded_Error(t *testing.T) {
	m := newModel(Deps{})
	msg := collectionsLoadedMsg{
		root: "/tmp",
		err:  &domain.OpError{Op: "yamlcollection.list", Kind: domain.KindNotFound},
	}
	tm, _ := m.Update(msg)
	updated := tm.(model)
	if updated.toast == "" {
		t.Error("expected toast on collections load error")
	}
	if updated.collectionsRefs != nil {
		t.Error("expected nil collectionsRefs on error")
	}
}

func TestModel_Update_RunnerDone_Success(t *testing.T) {
	m := newModel(Deps{})
	m.width = 80
	m.height = 24
	m.running = true
	m.scr = screenRunWizard

	msg := runnerDoneMsg{
		run: domain.RunResult{
			CollectionName: "Demo",
			Results: []domain.RequestResult{
				{Name: "req1", StatusCode: 200},
			},
		},
		id: "run-123",
	}

	tm, _ := m.Update(msg)
	updated := tm.(model)

	if updated.running {
		t.Error("expected running=false after done")
	}
	if updated.scr != screenResults {
		t.Errorf("expected screenResults, got %d", updated.scr)
	}
	if updated.runID != "run-123" {
		t.Errorf("expected runID=run-123, got %q", updated.runID)
	}
}

func TestModel_Update_RunnerDone_Error(t *testing.T) {
	m := newModel(Deps{})
	m.width = 80
	m.height = 24
	m.running = true

	msg := runnerDoneMsg{
		err: &domain.OpError{Op: "runner", Kind: domain.KindExecution},
	}

	tm, _ := m.Update(msg)
	updated := tm.(model)

	if updated.running {
		t.Error("expected running=false")
	}
	if updated.toast == "" {
		t.Error("expected toast on error")
	}
}

func TestModel_Update_EscFromSettings_GoesHome(t *testing.T) {
	m := newModel(Deps{})
	m.scr = screenSettings
	tm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := tm.(model)
	if updated.scr != screenHome {
		t.Errorf("expected screenHome, got %d", updated.scr)
	}
}

func TestModel_Update_EscFromCollections_GoesHome(t *testing.T) {
	m := newModel(Deps{})
	m.scr = screenCollections
	tm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := tm.(model)
	if updated.scr != screenHome {
		t.Errorf("expected screenHome, got %d", updated.scr)
	}
}

func TestModel_Update_EscFromResults_GoesHome(t *testing.T) {
	m := newModel(Deps{})
	m.scr = screenResults
	m.width = 80
	m.height = 24
	tm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := tm.(model)
	if updated.scr != screenHome {
		t.Errorf("expected screenHome, got %d", updated.scr)
	}
}

func TestModel_Update_TabInResults_TogglesTab(t *testing.T) {
	m := newModel(Deps{})
	m.scr = screenResults
	m.width = 80
	m.height = 24
	m.resultTab = 0

	tm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	updated := tm.(model)
	if updated.resultTab != 1 {
		t.Errorf("expected resultTab=1, got %d", updated.resultTab)
	}

	tm2, _ := updated.Update(tea.KeyMsg{Type: tea.KeyTab})
	updated2 := tm2.(model)
	if updated2.resultTab != 0 {
		t.Errorf("expected resultTab=0 after second toggle, got %d", updated2.resultTab)
	}
}

func TestModel_Update_WizardStep_BackFromStep1_GoesHome(t *testing.T) {
	m := newModel(Deps{})
	m.scr = screenRunWizard
	m.wizardStep = 1

	tm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := tm.(model)
	if updated.scr != screenHome {
		t.Errorf("expected screenHome, got %d", updated.scr)
	}
}

func TestModel_Update_WizardStep_BackFromStep2_GoesStep1(t *testing.T) {
	m := newModel(Deps{})
	m.scr = screenRunWizard
	m.wizardStep = 2

	tm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := tm.(model)
	if updated.wizardStep != 1 {
		t.Errorf("expected wizardStep=1, got %d", updated.wizardStep)
	}
}

func TestModel_Update_WizardStep_BackWhileRunning_ShowsToast(t *testing.T) {
	m := newModel(Deps{})
	m.scr = screenRunWizard
	m.wizardStep = 4
	m.running = true

	tm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := tm.(model)
	if updated.toast == "" {
		t.Error("expected toast when esc pressed while running")
	}
}

func TestModel_Update_WizardStep3_ToggleSave(t *testing.T) {
	m := newModel(Deps{})
	m.scr = screenRunWizard
	m.wizardStep = 3
	m.saveRun = true

	tm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	updated := tm.(model)
	if updated.saveRun {
		t.Error("expected saveRun=false after toggle")
	}

	tm2, _ := updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	updated2 := tm2.(model)
	if !updated2.saveRun {
		t.Error("expected saveRun=true after second toggle")
	}
}

func TestModel_View_AllScreens(t *testing.T) {
	screens := []struct {
		name string
		scr  screen
	}{
		{"home", screenHome},
		{"settings", screenSettings},
		{"collections", screenCollections},
		{"runWizard", screenRunWizard},
		{"results", screenResults},
	}

	for _, tc := range screens {
		t.Run(tc.name, func(t *testing.T) {
			m := newModel(Deps{})
			m.width = 80
			m.height = 24
			m.scr = tc.scr
			m.wizardStep = 1
			m.resize()
			out := m.View()
			if out == "" {
				t.Error("expected non-empty View output")
			}
		})
	}
}

func TestModel_SelectedResult_OutOfBounds(t *testing.T) {
	m := newModel(Deps{})
	m.runResult = domain.RunResult{Results: []domain.RequestResult{}}

	if rr := m.selectedResult(); rr != nil {
		t.Error("expected nil for empty results")
	}
}

func TestModel_SelectedResult_Valid(t *testing.T) {
	m := newModel(Deps{})
	m.width = 80
	m.height = 24
	m.runResult = domain.RunResult{
		Results: []domain.RequestResult{
			{Name: "req1", StatusCode: 200},
		},
	}
	m.buildResultsTable()

	rr := m.selectedResult()
	if rr == nil {
		t.Fatal("expected non-nil result")
	}
	if rr.Name != "req1" {
		t.Errorf("expected name=req1, got %q", rr.Name)
	}
}
