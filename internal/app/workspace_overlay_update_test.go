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

func TestOverlay_RepoPathInputActive_KeyW_TypesInputWithoutOpeningOverlay(t *testing.T) {
	m := seededFolderModeModelWithLocalRepo()

	step, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	active := step.(Model)
	updated, cmd := active.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	got := updated.(Model)

	if cmd != nil {
		t.Fatal("expected no command while typing in repo path input")
	}
	if got.RepoPathInput.Value() != "w" {
		t.Fatalf("expected repo path input to capture %q, got %q", "w", got.RepoPathInput.Value())
	}
	if got.Overlay.Active {
		t.Fatalf("expected overlay to remain closed, got %#v", got.Overlay)
	}
}

func TestOverlay_RepoPathInputActive_KeyQ_TypesInputWithoutQuitting(t *testing.T) {
	m := seededFolderModeModelWithLocalRepo()

	step, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	active := step.(Model)
	updated, cmd := active.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	got := updated.(Model)

	if cmd != nil {
		t.Fatal("expected no command while typing in repo path input")
	}
	if got.RepoPathInput.Value() != "q" {
		t.Fatalf("expected repo path input to capture %q, got %q", "q", got.RepoPathInput.Value())
	}
	if got.Overlay.Active {
		t.Fatalf("expected overlay to remain closed, got %#v", got.Overlay)
	}
}

func TestOverlay_LazygitActive_KeyW_ForwardsToPTYInsteadOfOpeningOverlay(t *testing.T) {
	m := seededModelWithRepos()
	manager := newFakeLazygitSessionManager()
	manager.sessionsByRepo["/tmp/api"] = LazygitSessionHandle{
		ID:       "session-api",
		RepoPath: "/tmp/api",
	}
	m.LazygitSessionManager = manager

	enteredTab, _ := m.Update(MsgSetActiveTab{Tab: TabLazygit})
	lazygitModel := enteredTab.(Model)

	updated, cmd := lazygitModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	got := updated.(Model)

	if cmd != nil {
		t.Fatal("expected no command for lazygit-owned key")
	}
	if got.Overlay.Active {
		t.Fatalf("expected overlay to remain closed while lazygit owns keys, got %#v", got.Overlay)
	}
	if len(manager.writeCalls) != 1 {
		t.Fatalf("expected one PTY write for lazygit-owned key, got %d", len(manager.writeCalls))
	}
	if payload := string(manager.writeCalls[0].data); payload != "w" {
		t.Fatalf("expected forwarded payload %q, got %q", "w", payload)
	}
}
