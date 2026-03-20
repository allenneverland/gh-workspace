package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

func TestView_LeftPaneRendersWorkspaceAndRepo(t *testing.T) {
	m := seededModelWithRepos()

	got := m.View()
	assertContains(t, got, "alpha")
	assertContains(t, got, "beta")
	assertContains(t, got, "api")
	assertContains(t, got, "web")
	assertContains(t, got, "a: add repo path")
}

func TestView_LeftPaneRendersInvalidRepoRecoveryHints(t *testing.T) {
	m := NewModel(Config{
		InitialState: workspace.State{
			SelectedWorkspaceID: "ws-1",
			Workspaces: []workspace.Workspace{
				{
					ID:             "ws-1",
					Name:           "alpha",
					SelectedRepoID: "repo-1",
					Repos: []workspace.Repo{
						{ID: "repo-1", Name: "api", Path: "/tmp/api", Health: workspace.RepoInvalid},
					},
				},
			},
		},
	})

	got := m.View()
	assertContains(t, got, "api [invalid]")
	assertContains(t, got, "enter: fix path")
	assertContains(t, got, "x: remove repo")
}

func TestView_FolderMode_HidesWorkspacesSection(t *testing.T) {
	m := seededModelWithRepos()
	m.UIMode = ModeFolder

	got := m.View()
	if strings.Contains(got, "Workspaces\n") {
		t.Fatalf("workspaces section should be hidden in folder mode, got:\n%s", got)
	}
}

func TestView_WorkspaceMode_HidesSystemWorkspaceEntry(t *testing.T) {
	m := seededModelWithSystemAndUserWorkspaces()

	got := m.View()
	if strings.Contains(got, workspace.LocalWorkspaceName) {
		t.Fatalf("system workspace should not be rendered in workspace mode, got:\n%s", got)
	}
	assertContains(t, got, "team-a")
}

func TestView_FolderMode_EmptyState_ShowsGuidance(t *testing.T) {
	m := NewModel(Config{InitialUIMode: ModeFolder})

	got := m.View()
	assertContains(t, got, "current folder is not a git repo")
	assertContains(t, got, "press a to add repo path")
	assertContains(t, got, "-w <name>")
}

func TestView_FolderMode_RepoPathInput_ShowsPrompt(t *testing.T) {
	m := seededFolderModeModelWithLocalRepo()
	m.RepoPathInputActive = true
	_, _, _ = m.RepoPathInput.Update(tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("/tmp/new-repo"),
	})

	got := m.View()
	assertContains(t, got, "repo path> /tmp/new-repo|")
}

func seededModelWithSystemAndUserWorkspaces() Model {
	return NewModel(Config{
		InitialUIMode: ModeWorkspace,
		InitialState: workspace.State{
			SelectedWorkspaceID: workspace.LocalWorkspaceID,
			Workspaces: []workspace.Workspace{
				{
					ID:   workspace.LocalWorkspaceID,
					Name: workspace.LocalWorkspaceName,
				},
				{
					ID:   "ws-team-a",
					Name: "team-a",
					Repos: []workspace.Repo{
						{ID: "repo-a", Name: "api", Path: "/tmp/api", Health: workspace.RepoHealthy},
					},
					SelectedRepoID: "repo-a",
				},
			},
		},
	})
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("expected output to contain %q, got:\n%s", want, got)
	}
}
