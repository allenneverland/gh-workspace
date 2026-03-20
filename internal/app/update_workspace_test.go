package app

import (
	"testing"

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
