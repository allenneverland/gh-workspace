package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

func TestUpdate_SelectRepo_ChangesCurrentRepo(t *testing.T) {
	m := seededModelWithRepos()

	updated, _ := m.Update(MsgSelectRepo{RepoID: "repo-2"})
	got := updated.(Model)
	if got.State.CurrentRepoID() != "repo-2" {
		t.Fatalf("expected selected repo %q, got %q", "repo-2", got.State.CurrentRepoID())
	}
}

func TestUpdate_SelectWorkspace_ChangesCurrentWorkspace(t *testing.T) {
	m := seededModelWithRepos()

	updated, _ := m.Update(MsgSelectWorkspace{WorkspaceID: "ws-2"})
	got := updated.(Model)
	if got.State.CurrentWorkspaceID() != "ws-2" {
		t.Fatalf("expected selected workspace %q, got %q", "ws-2", got.State.CurrentWorkspaceID())
	}
	if got.State.CurrentRepoID() != "repo-3" {
		t.Fatalf("expected selected repo %q after workspace switch, got %q", "repo-3", got.State.CurrentRepoID())
	}
}

func TestUpdate_KeyRemoveRepo_RemovesSelectedRepo(t *testing.T) {
	m := seededModelWithRepos()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	got := updated.(Model)

	workspace, ok := got.State.CurrentWorkspace()
	if !ok {
		t.Fatal("expected current workspace to exist")
	}
	if len(workspace.Repos) != 1 {
		t.Fatalf("expected one repo after remove, got %d", len(workspace.Repos))
	}
	if workspace.Repos[0].ID != "repo-2" {
		t.Fatalf("expected remaining repo %q, got %q", "repo-2", workspace.Repos[0].ID)
	}
	if workspace.SelectedRepoID != "repo-2" {
		t.Fatalf("expected selected repo %q after remove, got %q", "repo-2", workspace.SelectedRepoID)
	}
}

func TestUpdate_KeyAddRepo_TriggersRequestMessagePath(t *testing.T) {
	m := seededModelWithRepos()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatal("expected add-repo key to return a command")
	}

	msg := cmd()
	if _, ok := msg.(MsgRequestAddRepo); !ok {
		t.Fatalf("expected command message %T, got %T", MsgRequestAddRepo{}, msg)
	}

	afterMsg, _ := updated.(Model).Update(msg)
	got := afterMsg.(Model)
	if !got.AddRepoRequested {
		t.Fatal("expected AddRepoRequested to be true")
	}
	if got.StatusMessage == "" {
		t.Fatal("expected status message to be set")
	}
}

func TestUpdate_KeyEnter_InvalidRepoPathExists_MarksRepoHealthy(t *testing.T) {
	existingPath := t.TempDir()
	m := NewModel(Config{
		InitialState: singleRepoState(existingPath, workspace.RepoInvalid),
	})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	repo, ok := got.State.CurrentRepo()
	if !ok {
		t.Fatal("expected selected repo")
	}
	if repo.Health != workspace.RepoHealthy {
		t.Fatalf("expected repo health %q, got %q", workspace.RepoHealthy, repo.Health)
	}
	if !strings.Contains(got.StatusMessage, "recovered") {
		t.Fatalf("expected recovery status message, got %q", got.StatusMessage)
	}
}

func TestUpdate_KeyEnter_InvalidRepoPathMissing_StaysInvalid(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "does-not-exist")
	m := NewModel(Config{
		InitialState: singleRepoState(missingPath, workspace.RepoInvalid),
	})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	repo, ok := got.State.CurrentRepo()
	if !ok {
		t.Fatal("expected selected repo")
	}
	if repo.Health != workspace.RepoInvalid {
		t.Fatalf("expected repo health %q, got %q", workspace.RepoInvalid, repo.Health)
	}
	if !strings.Contains(got.StatusMessage, "invalid") {
		t.Fatalf("expected invalid status message, got %q", got.StatusMessage)
	}
}

func TestUpdate_DoesNotMutateSourceModel_OnSelectRepo(t *testing.T) {
	m := seededModelWithRepos()

	updated, _ := m.Update(MsgSelectRepo{RepoID: "repo-2"})
	got := updated.(Model)
	if got.State.CurrentRepoID() != "repo-2" {
		t.Fatalf("expected updated model repo %q, got %q", "repo-2", got.State.CurrentRepoID())
	}
	if m.State.CurrentRepoID() != "repo-1" {
		t.Fatalf("expected source model repo to remain %q, got %q", "repo-1", m.State.CurrentRepoID())
	}
}

func TestUpdate_DoesNotMutateSourceModel_OnRemoveRepo(t *testing.T) {
	m := seededModelWithRepos()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	got := updated.(Model)
	updatedWorkspace, ok := got.State.CurrentWorkspace()
	if !ok {
		t.Fatal("expected updated workspace")
	}
	if len(updatedWorkspace.Repos) != 1 {
		t.Fatalf("expected updated model repo count %d, got %d", 1, len(updatedWorkspace.Repos))
	}

	sourceWorkspace, ok := m.State.CurrentWorkspace()
	if !ok {
		t.Fatal("expected source workspace")
	}
	if len(sourceWorkspace.Repos) != 2 {
		t.Fatalf("expected source model repo count %d, got %d", 2, len(sourceWorkspace.Repos))
	}
	if sourceWorkspace.SelectedRepoID != "repo-1" {
		t.Fatalf("expected source selected repo %q, got %q", "repo-1", sourceWorkspace.SelectedRepoID)
	}
}

func TestUpdate_SelectRepoBinding_DrivesInvalidRecovery(t *testing.T) {
	existingPath := t.TempDir()
	m := NewModel(Config{
		InitialState: singleRepoState(existingPath, workspace.RepoInvalid),
	})
	m.Keys.SelectRepo = key.NewBinding(key.WithKeys("s"))

	enterUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	enterModel := enterUpdated.(Model)
	enterRepo, ok := enterModel.State.CurrentRepo()
	if !ok {
		t.Fatal("expected repo after enter")
	}
	if enterRepo.Health != workspace.RepoInvalid {
		t.Fatalf("expected enter to not recover with remapped key binding, got %q", enterRepo.Health)
	}

	bindingUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	bindingModel := bindingUpdated.(Model)
	bindingRepo, ok := bindingModel.State.CurrentRepo()
	if !ok {
		t.Fatal("expected repo after binding key")
	}
	if bindingRepo.Health != workspace.RepoHealthy {
		t.Fatalf("expected bound select key to recover repo to %q, got %q", workspace.RepoHealthy, bindingRepo.Health)
	}
}

func seededModelWithRepos() Model {
	return NewModel(Config{
		InitialState: workspace.State{
			SelectedWorkspaceID: "ws-1",
			Workspaces: []workspace.Workspace{
				{
					ID:             "ws-1",
					Name:           "alpha",
					SelectedRepoID: "repo-1",
					Repos: []workspace.Repo{
						{ID: "repo-1", Name: "api", Path: "/tmp/api", Health: workspace.RepoHealthy},
						{ID: "repo-2", Name: "web", Path: "/tmp/web", Health: workspace.RepoHealthy},
					},
				},
				{
					ID:             "ws-2",
					Name:           "beta",
					SelectedRepoID: "repo-3",
					Repos: []workspace.Repo{
						{ID: "repo-3", Name: "ops", Path: "/tmp/ops", Health: workspace.RepoHealthy},
					},
				},
			},
		},
	})
}

func singleRepoState(path string, health workspace.RepoHealth) workspace.State {
	return workspace.State{
		SelectedWorkspaceID: "ws-1",
		Workspaces: []workspace.Workspace{
			{
				ID:             "ws-1",
				Name:           "alpha",
				SelectedRepoID: "repo-1",
				Repos: []workspace.Repo{
					{ID: "repo-1", Name: "api", Path: path, Health: health},
				},
			},
		},
	}
}
