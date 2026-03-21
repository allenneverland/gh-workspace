package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestOverlay_KeyW_OpensSwitchMode_WorkspaceMode(t *testing.T) {
	m := seededModelWithSystemAndUserWorkspaces()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	got := updated.(Model)
	if !got.Overlay.Active || got.Overlay.Mode != OverlayModeSwitch {
		t.Fatalf("expected active switch overlay, got %#v", got.Overlay)
	}
}

func TestOverlay_KeyW_OpensSwitchMode_FolderMode(t *testing.T) {
	m := seededFolderModeModelWithLocalRepo()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	got := updated.(Model)
	if !got.Overlay.Active || got.Overlay.Mode != OverlayModeSwitch {
		t.Fatalf("expected active switch overlay, got %#v", got.Overlay)
	}
}

func TestOverlay_KeyEsc_ClosesOverlay(t *testing.T) {
	m := seededModelWithSystemAndUserWorkspaces()

	step, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	opened := step.(Model)
	updated, _ := opened.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if got.Overlay.Active {
		t.Fatalf("expected esc to close overlay, got %#v", got.Overlay)
	}
}

func TestOverlay_KeyC_FromSwitchMode_EntersCreateMode(t *testing.T) {
	m := seededModelWithSystemAndUserWorkspaces()

	step, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	opened := step.(Model)
	updated, _ := opened.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	got := updated.(Model)
	if !got.Overlay.Active || got.Overlay.Mode != OverlayModeCreate {
		t.Fatalf("expected active create overlay, got %#v", got.Overlay)
	}
}

func TestOverlay_KeyEsc_FromCreateMode_DiscardsDraftFields(t *testing.T) {
	m := seededModelWithSystemAndUserWorkspaces()

	step, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	opened := step.(Model)
	step, _ = opened.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	create := step.(Model)
	wantScanPath := workspaceOverlayDefaultScanPath(create.State)
	create.Overlay.CreateNameInput = "team-x"
	create.Overlay.ScanPathInput = "/tmp/projects"
	create.Overlay.Candidates = []RepoCandidate{{Name: "api", Path: "/tmp/api"}}
	create.Overlay.StagedRepos = []RepoCandidate{{Name: "web", Path: "/tmp/web"}}

	updated, _ := create.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(Model)
	if got.Overlay.Active {
		t.Fatalf("expected esc to close overlay, got %#v", got.Overlay)
	}
	if got.Overlay.CreateNameInput != "" {
		t.Fatalf("expected create name draft to be cleared, got %q", got.Overlay.CreateNameInput)
	}
	if got.Overlay.ScanPathInput != wantScanPath {
		t.Fatalf("expected scan path draft to reset to %q, got %q", wantScanPath, got.Overlay.ScanPathInput)
	}
	if len(got.Overlay.Candidates) != 0 {
		t.Fatalf("expected candidate draft to be cleared, got %#v", got.Overlay.Candidates)
	}
	if len(got.Overlay.StagedRepos) != 0 {
		t.Fatalf("expected staged repo draft to be cleared, got %#v", got.Overlay.StagedRepos)
	}
}
