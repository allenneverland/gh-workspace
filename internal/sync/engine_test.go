package sync

import (
	"context"
	"testing"
	"time"

	"github.com/allenneverland/gh-workspace/internal/domain/workspace"
)

func TestSyncEngine_AutoPoll_OnlySelectedRepo(t *testing.T) {
	fetcher := &fakeStatusFetcher{}
	engine := NewEngine(fetcher, WithInterval(time.Minute))

	if _, err := engine.OnSelectionChanged(context.Background(), "workspace-1", "repo-2"); err != nil {
		t.Fatalf("OnSelectionChanged() error = %v", err)
	}
	fetcher.reset()

	if _, err := engine.OnTick(context.Background()); err != nil {
		t.Fatalf("OnTick() error = %v", err)
	}

	if len(fetcher.calls) != 1 {
		t.Fatalf("expected one polling call, got %d", len(fetcher.calls))
	}
	call := fetcher.calls[0]
	if call.workspaceID != "workspace-1" || call.repoID != "repo-2" {
		t.Fatalf("expected call to workspace-1/repo-2, got %s/%s", call.workspaceID, call.repoID)
	}
}

func TestSyncEngine_OnRepoSwitch_TriggersImmediateRefresh(t *testing.T) {
	fetcher := &fakeStatusFetcher{}
	engine := NewEngine(fetcher, WithInterval(time.Minute))

	if _, err := engine.OnSelectionChanged(context.Background(), "workspace-1", "repo-2"); err != nil {
		t.Fatalf("OnSelectionChanged() initial error = %v", err)
	}
	fetcher.reset()

	if _, err := engine.OnSelectionChanged(context.Background(), "workspace-1", "repo-3"); err != nil {
		t.Fatalf("OnSelectionChanged() switch error = %v", err)
	}

	if len(fetcher.calls) != 1 {
		t.Fatalf("expected one refresh call after switch, got %d", len(fetcher.calls))
	}
	call := fetcher.calls[0]
	if call.workspaceID != "workspace-1" || call.repoID != "repo-3" {
		t.Fatalf("expected immediate refresh for workspace-1/repo-3, got %s/%s", call.workspaceID, call.repoID)
	}
}

type fakeStatusFetcher struct {
	calls []fetchCall
}

type fetchCall struct {
	workspaceID string
	repoID      string
}

func (f *fakeStatusFetcher) FetchSelectedRepoStatus(_ context.Context, workspaceID, repoID string) (workspace.RepoStatus, error) {
	f.calls = append(f.calls, fetchCall{workspaceID: workspaceID, repoID: repoID})
	return workspace.RepoStatus{
		PR:      workspace.StatusSuccess,
		CI:      workspace.StatusSuccess,
		Release: workspace.StatusSuccess,
	}, nil
}

func (f *fakeStatusFetcher) reset() {
	f.calls = nil
}
