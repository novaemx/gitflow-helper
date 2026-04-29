package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/novaemx/gitflow-helper/internal/config"
	"github.com/novaemx/gitflow-helper/internal/gitflow"
)

func TestModelInit_SetsFingerprintsAndCmd(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	s := spinner.New()
	s.Spinner = spinner.Pulse
	m := model{spinner: s, gf: &gitflow.Logic{Config: config.FlowConfig{ProjectRoot: root}}}
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("expected non-nil init cmd")
	}
}

func TestView_QuittingAndLoading(t *testing.T) {
	m := model{quitting: true}
	if got := m.View(); got != "" {
		t.Fatalf("expected empty view when quitting, got %q", got)
	}

	m = model{width: 0, height: 0}
	if got := m.View(); got != "Loading..." {
		t.Fatalf("expected loading view, got %q", got)
	}
}

func TestBlankViewport_ZeroAndSized(t *testing.T) {
	m := model{width: 0, height: 10}
	if got := m.blankViewport(); got != "" {
		t.Fatalf("expected blank empty when width zero, got %q", got)
	}

	m = model{width: 4, height: 3}
	got := m.blankViewport()
	rows := strings.Split(got, "\n")
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}
	for _, r := range rows {
		if len(r) != 4 {
			t.Fatalf("expected row width 4, got %q", r)
		}
	}
}

func TestHandleInputKey_Escape(t *testing.T) {
	a := action{Label: "Start", Command: "gitflow start feature %s", Tag: "start"}
	m := model{mode: viewInput, pendingAction: &a}
	next, _ := m.handleInputKey(tea.KeyMsg{Type: tea.KeyEsc})
	updated := next.(model)
	if updated.mode != viewDashboard {
		t.Fatalf("expected dashboard mode, got %v", updated.mode)
	}
	if updated.pendingAction != nil {
		t.Fatal("expected pendingAction cleared")
	}
}

func TestHandleInputKey_EnterNormalizesFeatureInput(t *testing.T) {
	a := action{Label: "Start feature", Command: "gitflow start feature %s", Tag: "start"}
	input := textinput.New()
	input.SetValue("My Feature 123")

	s := spinner.New()
	s.Spinner = spinner.Pulse
	m := model{
		mode:          viewInput,
		pendingAction: &a,
		inputField:    input,
		spinner:       s,
		gf:            &gitflow.Logic{Config: config.FlowConfig{ProjectRoot: t.TempDir()}},
	}

	next, _ := m.handleInputKey(tea.KeyMsg{Type: tea.KeyEnter})
	updated := next.(model)
	if updated.mode != viewDashboard {
		t.Fatalf("expected dashboard mode, got %v", updated.mode)
	}
	if !updated.running {
		t.Fatal("expected command to start running")
	}
	if updated.runningTitle != "Start feature" {
		t.Fatalf("expected running title set, got %q", updated.runningTitle)
	}
	if updated.pendingAction != nil {
		t.Fatal("expected pending action cleared")
	}
}
