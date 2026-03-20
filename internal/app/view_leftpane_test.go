package app

import (
	"strings"
	"testing"

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

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("expected output to contain %q, got:\n%s", want, got)
	}
}
