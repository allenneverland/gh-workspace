package sync

import (
	"context"
	"testing"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

func TestStateBackedFetcher_FetchSelectedRepoStatus_UsesCurrentStateSnapshot(t *testing.T) {
	repoFetcher := &fakeRepoStatusFetcher{
		status: workspace.RepoStatus{
			PR:      workspace.StatusSuccess,
			CI:      workspace.StatusFailure,
			Release: workspace.StatusInProgress,
		},
	}
	fetcher := NewStateBackedFetcher(repoFetcher)
	fetcher.SetState(workspace.State{
		SelectedWorkspaceID: "ws-1",
		Workspaces: []workspace.Workspace{
			{
				ID:             "ws-1",
				Name:           "alpha",
				SelectedRepoID: "repo-1",
				Repos: []workspace.Repo{
					{ID: "repo-1", Name: "acme/api", Path: "/tmp/api"},
				},
			},
		},
	})

	status, err := fetcher.FetchSelectedRepoStatus(context.Background(), "ws-1", "repo-1")
	if err != nil {
		t.Fatalf("FetchSelectedRepoStatus() error = %v", err)
	}
	if status != repoFetcher.status {
		t.Fatalf("unexpected status\nwant=%#v\ngot=%#v", repoFetcher.status, status)
	}
	if repoFetcher.calls != 1 {
		t.Fatalf("expected exactly one repo fetch call, got %d", repoFetcher.calls)
	}
	if repoFetcher.lastRepo.ID != "repo-1" || repoFetcher.lastRepo.Name != "acme/api" {
		t.Fatalf("expected fetch on repo-1 acme/api, got id=%q name=%q", repoFetcher.lastRepo.ID, repoFetcher.lastRepo.Name)
	}
}

func TestStateBackedFetcher_FetchSelectedRepoStatus_MissingRepoIsNoop(t *testing.T) {
	repoFetcher := &fakeRepoStatusFetcher{}
	fetcher := NewStateBackedFetcher(repoFetcher)
	fetcher.SetState(workspace.State{
		SelectedWorkspaceID: "ws-1",
		Workspaces: []workspace.Workspace{
			{
				ID:   "ws-1",
				Name: "alpha",
				Repos: []workspace.Repo{
					{ID: "repo-1", Name: "acme/api", Path: "/tmp/api"},
				},
			},
		},
	})

	status, err := fetcher.FetchSelectedRepoStatus(context.Background(), "ws-1", "repo-2")
	if err != nil {
		t.Fatalf("FetchSelectedRepoStatus() error = %v", err)
	}
	if status != (workspace.RepoStatus{}) {
		t.Fatalf("expected zero status for missing repo, got %#v", status)
	}
	if repoFetcher.calls != 0 {
		t.Fatalf("expected no adapter calls for missing repo, got %d", repoFetcher.calls)
	}
}

type fakeRepoStatusFetcher struct {
	calls    int
	lastRepo workspace.Repo
	status   workspace.RepoStatus
	err      error
}

func (f *fakeRepoStatusFetcher) FetchRepoStatus(_ context.Context, repo workspace.Repo) (workspace.RepoStatus, error) {
	f.calls++
	f.lastRepo = repo
	return f.status, f.err
}
