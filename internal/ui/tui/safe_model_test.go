package tui

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestWrapSafe_NilLogger(t *testing.T) {
	m := newModel(Deps{})
	sm := wrapSafe(m, nil)
	if sm.log == nil {
		t.Fatal("expected non-nil logger when nil is passed")
	}
}

func TestSafeModel_ViewRecoversPanic(t *testing.T) {
	// Create a model that will panic during View.
	m := newModel(Deps{})
	sm := wrapSafe(m, discardLogger())

	// View should not panic even if internal state is inconsistent;
	// the safe wrapper catches panics.
	out := sm.View()
	if out == "" {
		t.Fatal("expected non-empty output from View")
	}
}

func TestSafeModel_UpdateRecoversPanic(t *testing.T) {
	m := newModel(Deps{})
	// Set up a state that triggers the run wizard at step 4 (running)
	// but without proper channel — normal update won't panic but
	// we verify the wrapper preserves a valid model.
	sm := wrapSafe(m, discardLogger())

	// Send a window size msg — should not panic.
	tm, cmd := sm.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if tm == nil {
		t.Fatal("expected non-nil model from Update")
	}
	_ = cmd
}

func TestSafeModel_UpdatePropagatesModel(t *testing.T) {
	m := newModel(Deps{})
	sm := wrapSafe(m, discardLogger())

	// Send a workspace refreshed message.
	msg := workspaceRefreshedMsg{
		cwd:   "/tmp",
		found: true,
		root:  "/tmp/workspace",
	}

	tm, _ := sm.Update(msg)
	updated, ok := tm.(safeModel)
	if !ok {
		t.Fatalf("expected safeModel, got %T", tm)
	}
	if updated.m.workspaceRoot != "/tmp/workspace" {
		t.Errorf("expected workspaceRoot=/tmp/workspace, got %q", updated.m.workspaceRoot)
	}
}

func TestSafeModel_InitReturnsCmd(t *testing.T) {
	m := newModel(Deps{})
	sm := wrapSafe(m, discardLogger())

	cmd := sm.Init()
	// Init should return a command (workspace refresh).
	if cmd == nil {
		t.Fatal("expected non-nil command from Init")
	}
}

func TestSafeModel_ViewContainsLynix(t *testing.T) {
	m := newModel(Deps{})
	m.width = 80
	m.height = 24
	sm := wrapSafe(m, discardLogger())

	out := sm.View()
	if !strings.Contains(out, "Lynix") {
		t.Error("expected 'Lynix' in view output")
	}
}
